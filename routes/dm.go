// dm.go — DM thread + roster routes: every DM operation the survey found
// portable except message/device-key/reaction ones (dm_message.go,
// 300-line split mirroring the original store file split). createDMThread,
// blockThreadOrigin and postDMMessage stay in the gateway — see doc.go.
package routes

import (
	"errors"
	"net/http"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

// dmStatusFor maps a DMRepository error to a status code. Only
// ErrDMNotFound and ErrDMAlreadyActive/ErrDMNotLastToLeave get a specific
// code; everything else (most commonly the unexported "not admin" refusal
// every roster write can return) is a 403 — the gateway's own inviteDM only
// branches ErrDMAlreadyActive->409 from a blanket 403, and every other
// roster handler is undecorated further still, so this package matches
// that shape rather than inventing finer-grained codes the gateway itself
// doesn't distinguish.
func dmStatusFor(err error) int {
	switch {
	case errors.Is(err, models.ErrDMNotFound):
		return http.StatusNotFound
	case errors.Is(err, models.ErrDMAlreadyActive):
		return http.StatusConflict
	case errors.Is(err, models.ErrDMNotLastToLeave):
		return http.StatusForbidden
	default:
		return http.StatusForbidden
	}
}

func writeDMErr(w http.ResponseWriter, err error) {
	writeErr(w, dmStatusFor(err), err.Error())
}

// forCaller resolves MyAddressIndex for the given caller from whichever of
// OpenedViaAddressIndex/SentFromAddressIndex belongs to them — mirrors the
// gateway's dm_handlers.go forCaller. OpenedViaAddressIndex/
// SentFromAddressIndex/the two *Owner fields are already `json:"-"` on
// models.DMThread, so this is the only thing standing between them and a
// caller who isn't party to the pair.
func forCaller(t models.DMThread, callerID string) models.DMThread {
	switch callerID {
	case t.OpenedViaAddressOwner:
		t.MyAddressIndex = t.OpenedViaAddressIndex
	case t.SentFromAddressOwner:
		t.MyAddressIndex = t.SentFromAddressIndex
	}
	return t
}

// isParticipant reports whether callerID has any roster row (any state) on
// threadID — requireThreadParticipant's DM-internal gate, ported as a
// composition over ListParticipants rather than a new DMRepository method.
func isParticipant(dm models.DMRepository, r *http.Request, threadID, callerID string) (bool, error) {
	parts, err := dm.ListParticipants(r.Context(), threadID)
	if err != nil {
		return false, err
	}
	for _, p := range parts {
		if p.MemberID == callerID {
			return true, nil
		}
	}
	return false, nil
}

// DMRoutes is the nine thread+roster DM operations the survey found
// portable, addressed at the same paths the gateway already serves them at.
func DMRoutes(dm models.DMRepository, identity Identity) []Route {
	return []Route{
		{Method: http.MethodGet, Path: "/v1/dm/threads", Handler: ListDMThreads(dm, identity)},
		{Method: http.MethodGet, Path: "/v1/dm/threads/{threadID}", Handler: GetDMThread(dm, identity)},
		{Method: http.MethodGet, Path: "/v1/dm/threads/{threadID}/participants", Handler: ListDMParticipants(dm, identity)},
		{Method: http.MethodPost, Path: "/v1/dm/threads/{threadID}/invite", Handler: InviteDM(dm, identity)},
		{Method: http.MethodPost, Path: "/v1/dm/threads/{threadID}/accept", Handler: AcceptDM(dm, identity)},
		{Method: http.MethodPost, Path: "/v1/dm/threads/{threadID}/leave", Handler: LeaveDM(dm, identity)},
		{Method: http.MethodPost, Path: "/v1/dm/threads/{threadID}/kick", Handler: KickDM(dm, identity)},
		{Method: http.MethodPost, Path: "/v1/dm/threads/{threadID}/promote", Handler: PromoteDM(dm, identity)},
		{Method: http.MethodPost, Path: "/v1/dm/threads/{threadID}/re-enable", Handler: ReEnableDM(dm, identity)},
	}
}

// ListDMThreads handles GET /v1/dm/threads — every thread the caller is
// currently invited to or active in.
func ListDMThreads(dm models.DMRepository, identity Identity) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller := identity.CallerID(r)
		out, err := dm.ListThreadsFor(r.Context(), caller)
		if err != nil {
			writeDMErr(w, err)
			return
		}
		projected := make([]models.DMThread, len(out))
		for i, t := range out {
			projected[i] = forCaller(t, caller)
		}
		writeJSON(w, http.StatusOK, projected)
	}
}

// GetDMThread handles GET /v1/dm/threads/{threadID} — gated by
// requireThreadParticipant.
func GetDMThread(dm models.DMRepository, identity Identity) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		threadID := r.PathValue("threadID")
		caller := identity.CallerID(r)
		ok, err := isParticipant(dm, r, threadID, caller)
		if err != nil {
			writeDMErr(w, err)
			return
		}
		if !ok {
			// **404, never 403.** A thread that exists but the caller is not
			// on and a thread that was never minted must be indistinguishable
			// — this is the byte-identical answer GetThread/ListParticipants
			// themselves give for "no such thread", so a caller who is not on
			// the roster learns nothing about whether it exists (the DEV-1137
			// enumeration-oracle fix this package's port must preserve).
			writeErr(w, http.StatusNotFound, models.ErrDMNotFound.Error())
			return
		}
		out, err := dm.GetThread(r.Context(), threadID)
		if err != nil {
			writeDMErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, forCaller(out, caller))
	}
}

// ListDMParticipants handles GET /v1/dm/threads/{threadID}/participants —
// gated by requireThreadParticipant.
func ListDMParticipants(dm models.DMRepository, identity Identity) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		threadID := r.PathValue("threadID")
		ok, err := isParticipant(dm, r, threadID, identity.CallerID(r))
		if err != nil {
			writeDMErr(w, err)
			return
		}
		if !ok {
			// **404, never 403.** A thread that exists but the caller is not
			// on and a thread that was never minted must be indistinguishable
			// — this is the byte-identical answer GetThread/ListParticipants
			// themselves give for "no such thread", so a caller who is not on
			// the roster learns nothing about whether it exists (the DEV-1137
			// enumeration-oracle fix this package's port must preserve).
			writeErr(w, http.StatusNotFound, models.ErrDMNotFound.Error())
			return
		}
		out, err := dm.ListParticipants(r.Context(), threadID)
		if err != nil {
			writeDMErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// dmMemberBody is the wire shape every roster write but accept/leave takes:
// the target member, from the request body — the caller's own id comes from
// Identity, never the body.
type dmMemberBody struct {
	MemberID string `json:"member_id"`
}

// InviteDM handles POST /v1/dm/threads/{threadID}/invite.
func InviteDM(dm models.DMRepository, identity Identity) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body dmMemberBody
		if err := readJSON(r, &body); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		out, err := dm.Invite(r.Context(), r.PathValue("threadID"), body.MemberID, identity.CallerID(r))
		if err != nil {
			writeDMErr(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, out)
	}
}

// AcceptDM handles POST /v1/dm/threads/{threadID}/accept — the caller
// accepts their own invite.
func AcceptDM(dm models.DMRepository, identity Identity) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out, err := dm.Accept(r.Context(), r.PathValue("threadID"), identity.CallerID(r))
		if err != nil {
			writeDMErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// LeaveDM handles POST /v1/dm/threads/{threadID}/leave — the caller leaves.
func LeaveDM(dm models.DMRepository, identity Identity) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := dm.Leave(r.Context(), r.PathValue("threadID"), identity.CallerID(r)); err != nil {
			writeDMErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// KickDM handles POST /v1/dm/threads/{threadID}/kick (admin-only).
func KickDM(dm models.DMRepository, identity Identity) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body dmMemberBody
		if err := readJSON(r, &body); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := dm.Kick(r.Context(), r.PathValue("threadID"), body.MemberID, identity.CallerID(r)); err != nil {
			writeDMErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// PromoteDM handles POST /v1/dm/threads/{threadID}/promote (admin-only).
func PromoteDM(dm models.DMRepository, identity Identity) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body dmMemberBody
		if err := readJSON(r, &body); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		out, err := dm.Promote(r.Context(), r.PathValue("threadID"), body.MemberID, identity.CallerID(r))
		if err != nil {
			writeDMErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// ReEnableDM handles POST /v1/dm/threads/{threadID}/re-enable — two acts
// wearing one route, per models.DMRepository.ReEnable's doc. An omitted
// member_id defaults to the caller's own id, the self-reenable case; an
// admin restoring somebody else names them explicitly.
func ReEnableDM(dm models.DMRepository, identity Identity) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body dmMemberBody
		if err := readJSON(r, &body); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		caller := identity.CallerID(r)
		target := body.MemberID
		if target == "" {
			target = caller
		}
		out, err := dm.ReEnable(r.Context(), r.PathValue("threadID"), target, caller)
		if err != nil {
			writeDMErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}
