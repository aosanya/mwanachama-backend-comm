// notification.go — every notification route. Unlike this package's other
// domains, all five qualify: none reaches a gateway-internal package in its
// handler body, and the caller's own id (the recipient, for every read and
// write here) comes from [Identity] rather than a URL path segment, the
// same reason DM's roster routes need it.
package routes

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/aosanya/mwanachama-backend-comm"
)

// maxNotificationPage bounds one List call from the wire, independent of
// mwanachamacomm.NotificationDefaultPage (the fallback when no limit is given at
// all).
const maxNotificationPage = 500

// NotificationRoutes is every notification and notification-preference
// route, at the same paths the gateway already serves them at.
func NotificationRoutes(notif mwanachamacomm.NotificationRepository, identity Identity) []Route {
	return []Route{
		{Method: http.MethodGet, Path: "/v1/notifications", Handler: ListNotifications(notif, identity)},
		{Method: http.MethodGet, Path: "/v1/notifications/unread", Handler: NotificationUnreadCount(notif, identity)},
		{Method: http.MethodPost, Path: "/v1/notifications/read", Handler: MarkNotificationsRead(notif, identity)},
		{Method: http.MethodGet, Path: "/v1/notification-preferences", Handler: ListNotificationPreferences(notif, identity)},
		{Method: http.MethodPut, Path: "/v1/notification-preferences/{category}", Handler: SetNotificationPreference(notif, identity)},
	}
}

// notificationJSON is the wire shape of one notification.
type notificationJSON struct {
	ID              string  `json:"id"`
	Event           string  `json:"event"`
	Category        string  `json:"category"`
	SubjectKind     string  `json:"subject_kind"`
	SubjectID       string  `json:"subject_id"`
	StructureID     string  `json:"structure_id,omitempty"`
	AuthorActorID   string  `json:"author_actor_id,omitempty"`
	SeatRoleKindID  string  `json:"seat_role_kind_id,omitempty"`
	SeatStructureID string  `json:"seat_structure_id,omitempty"`
	CreatedAt       string  `json:"created_at"`
	ReadAt          *string `json:"read_at"`
	SMS             bool    `json:"sms"`
}

func toNotificationJSON(n mwanachamacomm.Notification) notificationJSON {
	out := notificationJSON{
		ID:              n.ID,
		Event:           string(n.Event),
		Category:        string(n.Category),
		SubjectKind:     string(n.SubjectKind),
		SubjectID:       n.SubjectID,
		StructureID:     n.StructureID,
		AuthorActorID:   n.AuthorActorID,
		SeatRoleKindID:  n.SeatRoleKindID,
		SeatStructureID: n.SeatStructureID,
		CreatedAt:       n.CreatedAt.UTC().Format(time.RFC3339),
		SMS:             mwanachamacomm.NotificationSMSChannel(n.Category),
	}
	if n.ReadAt != nil {
		s := n.ReadAt.UTC().Format(time.RFC3339)
		out.ReadAt = &s
	}
	return out
}

// preferenceJSON is the wire shape of one category's preference.
type preferenceJSON struct {
	Category  string `json:"category"`
	Muted     bool   `json:"muted"`
	Set       bool   `json:"set"`
	Exempt    bool   `json:"exempt"`
	SMS       bool   `json:"sms"`
	ChangedAt string `json:"changed_at,omitempty"`
}

func writeNotificationErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, mwanachamacomm.ErrNotificationNotFound):
		writeErr(w, http.StatusNotFound, "not found")
	case errors.Is(err, mwanachamacomm.ErrNotificationCategoryExempt):
		writeErr(w, http.StatusConflict, err.Error())
	case errors.Is(err, mwanachamacomm.ErrNotificationCapSpent):
		writeErr(w, http.StatusConflict, "that cap is already spent for this subject")
	case errors.Is(err, mwanachamacomm.ErrNotificationInvalid):
		writeErr(w, http.StatusBadRequest, err.Error())
	default:
		writeErr(w, http.StatusInternalServerError, "notifications unavailable")
	}
}

// ListNotifications handles GET /v1/notifications?limit= — the caller's
// own list, newest first, plus their unread badge in the same response.
func ListNotifications(notif mwanachamacomm.NotificationRepository, identity Identity) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := mwanachamacomm.NotificationDefaultPage
		if v := r.URL.Query().Get("limit"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n <= 0 {
				writeErr(w, http.StatusBadRequest, "limit must be a positive integer")
				return
			}
			limit = min(n, maxNotificationPage)
		}
		caller := identity.CallerID(r)
		out, err := notif.List(r.Context(), caller, limit)
		if err != nil {
			writeNotificationErr(w, err)
			return
		}
		unread, err := notif.UnreadCount(r.Context(), caller)
		if err != nil {
			writeNotificationErr(w, err)
			return
		}
		wire := make([]notificationJSON, len(out))
		for i, n := range out {
			wire[i] = toNotificationJSON(n)
		}
		writeJSON(w, http.StatusOK, map[string]any{"notifications": wire, "unread": unread})
	}
}

// NotificationUnreadCount handles GET /v1/notifications/unread — the badge.
func NotificationUnreadCount(notif mwanachamacomm.NotificationRepository, identity Identity) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		unread, err := notif.UnreadCount(r.Context(), identity.CallerID(r))
		if err != nil {
			writeNotificationErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"unread": unread})
	}
}

// markNotificationsReadBody is the request body of MarkNotificationsRead.
type markNotificationsReadBody struct {
	IDs    []string `json:"ids"`
	ReadAt string   `json:"read_at"`
}

// MarkNotificationsRead handles POST /v1/notifications/read — stamps
// read_at on the caller's own named rows.
func MarkNotificationsRead(notif mwanachamacomm.NotificationRepository, identity Identity) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body markNotificationsReadBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		readAt := time.Now().UTC()
		if body.ReadAt != "" {
			t, err := time.Parse(time.RFC3339, body.ReadAt)
			if err != nil {
				writeErr(w, http.StatusBadRequest, "read_at must be an RFC3339 timestamp")
				return
			}
			readAt = t.UTC()
		}
		if len(body.IDs) > mwanachamacomm.NotificationMaxMarkRead {
			writeErr(w, http.StatusBadRequest, "at most "+strconv.Itoa(mwanachamacomm.NotificationMaxMarkRead)+" notifications in one call")
			return
		}
		marked, err := notif.MarkRead(r.Context(), identity.CallerID(r), body.IDs, readAt)
		if err != nil {
			writeNotificationErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"marked": marked})
	}
}

// ListNotificationPreferences handles GET /v1/notification-preferences —
// one row per category, always, even where the caller never wrote one:
// absence means on.
func ListNotificationPreferences(notif mwanachamacomm.NotificationRepository, identity Identity) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := notif.ListPreferences(r.Context(), identity.CallerID(r))
		if err != nil {
			writeNotificationErr(w, err)
			return
		}
		byCategory := make(map[mwanachamacomm.NotificationCategory]mwanachamacomm.NotificationPreference, len(rows))
		for _, p := range rows {
			byCategory[p.Category] = p
		}
		out := make([]preferenceJSON, 0, len(mwanachamacomm.NotificationCategories()))
		for _, c := range mwanachamacomm.NotificationCategories() {
			p, set := byCategory[c]
			pj := preferenceJSON{
				Category: string(c),
				Exempt:   mwanachamacomm.IsNotificationCategoryExempt(c),
				SMS:      mwanachamacomm.NotificationSMSChannel(c),
			}
			if set {
				pj.Muted = p.Muted
				pj.Set = true
				pj.ChangedAt = p.ChangedAt.UTC().Format(time.RFC3339)
			}
			out = append(out, pj)
		}
		writeJSON(w, http.StatusOK, map[string]any{"preferences": out})
	}
}

// setNotificationPreferenceBody is the request body of
// SetNotificationPreference.
type setNotificationPreferenceBody struct {
	Muted *bool `json:"muted"`
}

// SetNotificationPreference handles PUT
// /v1/notification-preferences/{category} — mutes or unmutes one category
// for the caller. `survey` and `security` are refused with the reason.
func SetNotificationPreference(notif mwanachamacomm.NotificationRepository, identity Identity) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body setNotificationPreferenceBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Muted == nil {
			writeErr(w, http.StatusBadRequest, "body must carry a boolean `muted`")
			return
		}
		category := mwanachamacomm.NotificationCategory(r.PathValue("category"))
		if !mwanachamacomm.IsNotificationCategory(category) {
			writeErr(w, http.StatusNotFound, "no such notification category")
			return
		}
		p, err := notif.SetPreference(r.Context(), identity.CallerID(r), category, *body.Muted)
		if err != nil {
			writeNotificationErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, preferenceJSON{
			Category:  string(p.Category),
			Muted:     p.Muted,
			Set:       true,
			Exempt:    mwanachamacomm.IsNotificationCategoryExempt(p.Category),
			SMS:       mwanachamacomm.NotificationSMSChannel(p.Category),
			ChangedAt: p.ChangedAt.UTC().Format(time.RFC3339),
		})
	}
}
