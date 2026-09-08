package routes

import (
	"net/http"
	"strconv"

	"github.com/aosanya/mwanachama-backend-comm"
)

// ChatActivityRoutes is chatActivity, GET /v1/chat/activity.
func ChatActivityRoutes(reader mwanachamacomm.ChatActivityReader) []Route {
	return []Route{
		{Method: http.MethodGet, Path: "/v1/chat/activity", Handler: ChatActivity(reader)},
	}
}

// ChatActivity handles GET /v1/chat/activity — decode structure_id/limit/
// offset, call, encode. The gateway wraps this with its own
// CapChatActivityRead capability check externally (route-table wrapping,
// the same pattern actor's HierarchyChecker callers use) — this handler
// answers only "given that a caller may act, here is the summary".
func ChatActivity(reader mwanachamacomm.ChatActivityReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := mwanachamacomm.ChatActivityQuery{StructureID: r.URL.Query().Get("structure_id")}
		if v := r.URL.Query().Get("limit"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				writeErr(w, http.StatusBadRequest, "limit must be a non-negative integer")
				return
			}
			q.Limit = &n
		}
		if v := r.URL.Query().Get("offset"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				writeErr(w, http.StatusBadRequest, "offset must be a non-negative integer")
				return
			}
			q.Offset = n
		}
		out, err := reader.Activity(r.Context(), q)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "internal error")
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}
