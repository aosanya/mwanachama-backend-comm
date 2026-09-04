package routes

import (
	"net/http"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

// Route is one address this package answers, relative to wherever the
// mounting process prefixes it (e.g. "/v1") — enough to build one
// *http.ServeMux entry from, without the mounting process hand-spelling
// each path/method pair itself. Mirrors mwanachama-backend-actor/routes'
// Route exactly.
type Route struct {
	Method  string
	Path    string
	Handler http.HandlerFunc
}

// Pattern returns the http.ServeMux registration pattern for this route
// once mounted under prefix.
func (r Route) Pattern(prefix string) string {
	return r.Method + " " + prefix + r.Path
}

// Routes is every address this package answers today: ChatActivityRoutes,
// DMRoutes, DMMessageRoutes and ModerationRoutes concatenated. A mounting
// process that wants all of it in one loop uses this; one that wants to
// wrap each domain's gate differently (the gateway does, today — chat
// activity's CapChatActivityRead is not moderation's report-queue gate)
// calls the four functions separately instead. See doc.go for what is
// deliberately not included here and why.
func Routes(chat models.ChatActivityReader, dm models.DMRepository, moderation models.ModerationRepository, identity Identity) []Route {
	out := ChatActivityRoutes(chat)
	out = append(out, DMRoutes(dm, identity)...)
	out = append(out, DMMessageRoutes(dm, identity)...)
	out = append(out, ModerationRoutes(moderation)...)
	return out
}
