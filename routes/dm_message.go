// dm_message.go — DM message/device-key/reaction routes (300-line split
// mirroring store_postgres_dm_messages.go). postDMMessage itself stays in
// the gateway (address.Repository) — see doc.go.
package routes

import (
	"net/http"
	"unicode/utf8"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

// maxReactionRunes mirrors the gateway's own dm_message_handlers.go bound —
// an emoji, or a short cluster of them, never a sentence.
const maxReactionRunes = 8

// isActiveParticipant reports whether callerID is currently active (not
// merely invited/left/kicked) on threadID — requireActiveThreadParticipant's
// gate.
func isActiveParticipant(dm models.DMRepository, r *http.Request, threadID, callerID string) (bool, error) {
	parts, err := dm.ListParticipants(r.Context(), threadID)
	if err != nil {
		return false, err
	}
	for _, p := range parts {
		if p.MemberID == callerID && p.State == models.DMStateActive {
			return true, nil
		}
	}
	return false, nil
}

// requireActiveThreadParticipantForMessage resolves messageID to its
// thread and applies isActiveParticipant to it — the gate setDMReaction/
// clearDMReaction use.
func requireActiveThreadParticipantForMessage(dm models.DMRepository, r *http.Request, messageID, callerID string) (bool, error) {
	msg, err := dm.GetMessage(r.Context(), messageID)
	if err != nil {
		return false, err
	}
	return isActiveParticipant(dm, r, msg.ThreadID, callerID)
}

// DMMessageRoutes is the six message/device-key/reaction DM operations the
// survey found portable.
func DMMessageRoutes(dm models.DMRepository, identity Identity) []Route {
	return []Route{
		{Method: http.MethodGet, Path: "/v1/dm/threads/{threadID}/messages", Handler: ListDMMessages(dm, identity)},
		{Method: http.MethodPost, Path: "/v1/dm/device-keys", Handler: PublishDMDeviceKey(dm, identity)},
		{Method: http.MethodPost, Path: "/v1/dm/device-keys/lookup", Handler: LookupDMDeviceKeys(dm)},
		{Method: http.MethodGet, Path: "/v1/dm/threads/{threadID}/reactions", Handler: ListDMReactions(dm, identity)},
		{Method: http.MethodPut, Path: "/v1/dm/messages/{messageID}/reaction", Handler: SetDMReaction(dm, identity)},
		{Method: http.MethodDelete, Path: "/v1/dm/messages/{messageID}/reaction", Handler: ClearDMReaction(dm, identity)},
	}
}

// ListDMMessages handles GET /v1/dm/threads/{threadID}/messages — gated by
// requireActiveThreadParticipant.
func ListDMMessages(dm models.DMRepository, identity Identity) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		threadID := r.PathValue("threadID")
		ok, err := isActiveParticipant(dm, r, threadID, identity.CallerID(r))
		if err != nil {
			writeDMErr(w, err)
			return
		}
		if !ok {
			// 404, never 403 — see dm.go's identical note; the same byte-identical answer a nonexistent thread or message gives.
			writeErr(w, http.StatusNotFound, models.ErrDMNotFound.Error())
			return
		}
		out, err := dm.ListMessages(r.Context(), threadID)
		if err != nil {
			writeDMErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// PublishDMDeviceKey handles POST /v1/dm/device-keys.
//
// Decodes straight into [models.DMDeviceKey] rather than a narrower body
// struct, on purpose: MemberID/PublishedBy/DeviceID/RetiredAt are session
// facts, and DEV-1265's own rule is that a client's opinion about them is
// not honoured, not that offering one is a 400. A body claiming someone
// else's provenance (`{"public_key":"...", "published_by":"someone-else",
// "device_id":"someone-else's-device"}`) must decode cleanly and then be
// silently overwritten — decoding into a struct with no field for those
// names would instead reject the request outright under readJSON's
// DisallowUnknownFields, which is a different and wrong refusal.
func PublishDMDeviceKey(dm models.DMRepository, identity Identity) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in models.DMDeviceKey
		if err := readJSON(r, &in); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		caller := identity.CallerID(r)
		in.MemberID = caller
		in.PublishedBy = caller
		in.DeviceID = identity.CallerDeviceID(r)
		in.RetiredAt = nil
		out, err := dm.PublishDeviceKey(r.Context(), in)
		if err != nil {
			writeDMErr(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, out)
	}
}

// lookupDeviceKeysBody is the wire shape for LookupDMDeviceKeys.
type lookupDeviceKeysBody struct {
	MemberIDs []string `json:"member_ids"`
}

// LookupDMDeviceKeys handles POST /v1/dm/device-keys/lookup — a plain
// shell, no caller identity needed.
func LookupDMDeviceKeys(dm models.DMRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body lookupDeviceKeysBody
		if err := readJSON(r, &body); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		out, err := dm.LookupDeviceKeys(r.Context(), body.MemberIDs)
		if err != nil {
			writeDMErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// ListDMReactions handles GET /v1/dm/threads/{threadID}/reactions — gated
// by requireActiveThreadParticipant.
func ListDMReactions(dm models.DMRepository, identity Identity) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		threadID := r.PathValue("threadID")
		ok, err := isActiveParticipant(dm, r, threadID, identity.CallerID(r))
		if err != nil {
			writeDMErr(w, err)
			return
		}
		if !ok {
			// 404, never 403 — see dm.go's identical note; the same byte-identical answer a nonexistent thread or message gives.
			writeErr(w, http.StatusNotFound, models.ErrDMNotFound.Error())
			return
		}
		out, err := dm.ListReactions(r.Context(), threadID)
		if err != nil {
			writeDMErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// reactionBody is the wire shape for SetDMReaction.
type reactionBody struct {
	Emoji string `json:"emoji"`
}

// SetDMReaction handles PUT /v1/dm/messages/{messageID}/reaction — gated by
// requireActiveThreadParticipantForMessage. emoji is required and capped at
// maxReactionRunes, validated here (pure, no domain read) the same way the
// gateway's own handler does before it ever reaches SetReaction.
func SetDMReaction(dm models.DMRepository, identity Identity) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		messageID := r.PathValue("messageID")
		caller := identity.CallerID(r)
		ok, err := requireActiveThreadParticipantForMessage(dm, r, messageID, caller)
		if err != nil {
			writeDMErr(w, err)
			return
		}
		if !ok {
			// 404, never 403 — see dm.go's identical note; the same byte-identical answer a nonexistent thread or message gives.
			writeErr(w, http.StatusNotFound, models.ErrDMNotFound.Error())
			return
		}
		var body reactionBody
		if err := readJSON(r, &body); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if body.Emoji == "" {
			writeErr(w, http.StatusBadRequest, "emoji is required")
			return
		}
		if utf8.RuneCountInString(body.Emoji) > maxReactionRunes {
			writeErr(w, http.StatusBadRequest, "emoji is too long")
			return
		}
		if err := dm.SetReaction(r.Context(), models.DMReaction{MessageID: messageID, MemberID: caller, Emoji: body.Emoji}); err != nil {
			writeDMErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// ClearDMReaction handles DELETE /v1/dm/messages/{messageID}/reaction —
// same gate as SetDMReaction.
func ClearDMReaction(dm models.DMRepository, identity Identity) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		messageID := r.PathValue("messageID")
		caller := identity.CallerID(r)
		ok, err := requireActiveThreadParticipantForMessage(dm, r, messageID, caller)
		if err != nil {
			writeDMErr(w, err)
			return
		}
		if !ok {
			// 404, never 403 — see dm.go's identical note; the same byte-identical answer a nonexistent thread or message gives.
			writeErr(w, http.StatusNotFound, models.ErrDMNotFound.Error())
			return
		}
		if err := dm.ClearReaction(r.Context(), messageID, caller); err != nil {
			writeDMErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
