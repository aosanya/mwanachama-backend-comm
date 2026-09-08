package routes

import (
	"net/http"

	"github.com/aosanya/mwanachama-backend-comm"
)

// ModerationRoutes is listReportQueue and listReportsForMessage — plain
// shells behind the gateway's own externally-wrapped CapModerateChat gate
// (the mounting process wraps these the same way it wraps ChatActivity's
// CapChatActivityRead).
func ModerationRoutes(mod mwanachamacomm.ModerationRepository) []Route {
	return []Route{
		{Method: http.MethodGet, Path: "/v1/groups/{groupID}/moderation/reports", Handler: ListReportQueue(mod)},
		{Method: http.MethodGet, Path: "/v1/groups/{groupID}/moderation/messages/{messageID}/reports", Handler: ListReportsForMessage(mod)},
	}
}

// ListReportQueue handles GET /v1/groups/{groupID}/moderation/reports.
func ListReportQueue(mod mwanachamacomm.ModerationRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out, err := mod.ListReportQueue(r.Context(), r.PathValue("groupID"))
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "internal error")
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// ListReportsForMessage handles
// GET /v1/groups/{groupID}/moderation/messages/{messageID}/reports.
func ListReportsForMessage(mod mwanachamacomm.ModerationRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out, err := mod.ListReportsForMessage(r.Context(), r.PathValue("messageID"))
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "internal error")
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}
