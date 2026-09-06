// Package routes is mwanachama-backend-comm's own HTTP surface, mirroring
// mwanachama-backend-actor/routes: decode a request, call one models
// repository method, encode the response — reachable from an HTTP mux
// without a mounting process reimplementing the request/response shape.
//
// Scope, as of 2026-09-04: every chat/dm/moderation operation that is,
// underneath, a plain call against models.ChatActivityReader/DMRepository/
// ModerationRepository with no gateway-only policy composed into the
// handler body — same-domain composition (a DM handler checking the
// caller's own roster state via ListParticipants) is fine, a gateway-
// internal domain package reached from the handler body is not, the same
// line actor's own doc.go draws. Nineteen of the gateway's thirty-seven
// chat/dm/moderation routes qualify:
//
//   - Chat: ChatActivityRoutes — chatActivity only.
//   - DM: DMRoutes + DMMessageRoutes — sixteen of nineteen.
//   - Moderation: ModerationRoutes — listReportQueue, listReportsForMessage.
//
// Extended 2026-09-06 when address and notification joined this module
// (see this repo's CLAUDE.md): two of address's seven routes qualify
// (AddressRoutes — listMyAddresses, retireAddress) and all five of
// notification's do (NotificationRoutes). Address's own arrival here does
// NOT retroactively move any of DM's three address-touching exclusions
// (createDMThread, blockThreadOrigin, postDMMessage) into this package in
// this pass — that is a DM-boundary decision this address+notification
// port did not open, scoped as it is to mirroring address_handlers.go/
// address_directory_handlers.go/notification_handlers.go/
// notification_authz.go, not dm_handlers.go. One of the three,
// blockThreadOrigin, is worth flagging rather than silently carrying
// forward: it calls only DMRepository.GetThread, DMRepository.
// ListParticipants (via requireThreadParticipant) and
// AddressRepository.Block — all three now live in this module, so this
// package's own portability rule would in fact admit it today. It stays a
// gateway-composed route for now, pending that DM-boundary decision;
// createDMThread and postDMMessage still also reach member.Repository
// (gateway-internal) regardless and cannot move on the same rule.
//
// [Routes] returns the whole set as one list of addresses a mounting
// process can range over to build a mux from; the six *Routes functions
// return one group at a time for a mounting process that wraps different
// domains in different policy (the gateway does, today — chat activity's
// CapChatActivityRead is not moderation's CapModerateChat).
//
// # Deliberately NOT here, and not a future TODO — a considered exclusion
//
// Chat (six of seven routes): mintChatThread, listChatThreads,
// resolveChatThread, postChatMessage, listChatMessages, chatStream all call
// requireChapterMember in their own body — member.Repository +
// chapter.Repository — before ever reaching ChatRepository. This package
// cannot import a gateway-internal package (Go's `internal/` visibility
// forbids it even across modules, and the gateway importing this module
// rules out the reverse), so the fence and the operation it guards both
// stay in the gateway rather than being split across two repos.
//
// DM (three of nineteen): createDMThread, blockThreadOrigin, postDMMessage
// all reach address.Repository in-body (resolveRecipients/sharesChapter/
// Addresses.Block/requireOpeningAddressNotSilenced), and createDMThread
// additionally reaches member.Repository (sharesChapter). Same reasoning as
// chat's exclusion.
//
// Moderation (nine of eleven): reportMessage, removeMessage, dismissReports
// and decideDispute all resolve wallLabel (chapter.Repository) and
// callerModerationActor (member.Repository) — the `wall`/`Actor` pair
// CreateRemoval/DismissReports/DecideDispute take — plus, for
// reportMessage/removeMessage/dismissReports, nearestRoleClass
// (role.Repository + chapter.Repository) for ReporterRoleClass/
// ActorRoleClass. messageRemovalStatus and messageReportOutcome call
// requireChapterMember, chat's own fence. getRemoval and getDispute run an
// in-body hasCapabilityAt check (role.Repository + chapter.Repository) with
// no external route-table wrapper to lean on instead — unlike
// listReportQueue/listReportsForMessage/chatActivity, whose capability gate
// the gateway's own route table wraps from outside. disputeRemoval resolves
// ReviewChapterID via chapter.Repository and enforces an author-only-may-
// raise policy in-body. None of this is reproducible without importing a
// gateway-internal package — see models.ActWriter/models.MemoryActWriter
// for the same constraint moderation's act-log write already works around
// by dependency inversion; this package draws the identical line rather
// than inventing a new one.
//
// Address (five of seven): publishAddress, resolveAddress and blockAddress
// all reach phonesalt.Repository in-body (hashAddress) — a gateway-internal
// package this module does not import, for the same never-crosses-a-
// module-boundary reason chat/DM/moderation's exclusions give. myPublic-
// AddressCap and updateAddressSettings additionally (updateAddressSettings)
// or solely (myPublicAddressCap) reach orgpolicy.Repository for the public-
// address cap. searchDirectory is not a handler exclusion in the same
// sense — AddressDirectory is fully implemented in this module (see
// address_directory_impl.go) — but the gateway's own construction of it
// still resolves the caller's own id and the 503-when-unwired branch
// itself, so the route registration, not the domain logic, stays there.
//
// Notification (zero of five): every notification route is a plain call
// against models.NotificationRepository with the caller's own id from
// [Identity] and nothing else in the handler body — the whole domain
// arrived with no gateway-internal coupling to begin with (see this
// repo's CLAUDE.md), which is the same reason it was picked for this move
// over harder cases.
//
// realtime.Hub (SSE stream + pub/sub publish — inviteDM's/reEnableDM's
// post-write notify, chatStream, postChatMessage, postDMMessage,
// dmInboxStream) is gateway infra, not a domain. The ported InviteDM/
// ReEnableDM handlers do the plain domain call and encode, the same way
// actor's Deregister doesn't compose an act-log write — a mounting process
// that wants the realtime fan-out keeps doing it itself around the
// returned handler.
//
// A route built from this package still needs a caller-identity/capability
// gate wrapped around it before it is safe to serve — this package answers
// "what happens once that gate has passed", never "who may pass it". Every
// DM route additionally needs an [Identity] supplied at construction, the
// same way Group's writes need a [HierarchyChecker]-shaped answer in actor:
// an externally supplied fact, not a lookup this package performs.
package routes
