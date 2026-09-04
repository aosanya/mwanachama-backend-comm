package routes

import "net/http"

// Identity resolves the caller's own identity from an authenticated
// request — the one gateway-session concern this package cannot own
// itself. Actor's routes read the acting-on id from the URL path
// ({actorID}); DM's roster/message operations instead act on or as the
// caller themselves (acceptDM/leaveDM act on the caller; inviteDM/kickDM/
// promoteDM/reEnableDM need the caller as the admin "by"; publishDeviceKey
// overwrites member/device fields from the session rather than the body),
// and there is no path segment to read that from. Same relationship
// HierarchyChecker (mwanachama-backend-actor/routes) has to actor's
// hierarchy/level tables: an externally supplied answer, not a lookup this
// package performs. The mounting process's own auth middleware is what
// actually authenticates a request; this interface only asks it what it
// already knows.
type Identity interface {
	// CallerID returns the authenticated caller's own member id.
	CallerID(r *http.Request) string
	// CallerDeviceID returns the device id the caller's session carries, or
	// "" — a session minted by the phone flow carries no device, and that
	// is a legitimate answer, not a missing one.
	CallerDeviceID(r *http.Request) string
}
