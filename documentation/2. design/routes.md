# comm's declared route table

Thirty-five addresses, declared in `comm.operations.json` and served by a
seventy-line adapter over `dispatch.Table`. The format is documented once, in
[dispatcher.md](../../../mwanachama-backend-shared/documentation/2.%20design/dispatcher.md);
this page records only what is true of **comm**.

Landed as CM21, 2026-10-01. It replaced ten hand-written route files.

## The surface

Paths are relative and the mount supplies the prefix. `mwanachama-wakala-api`
mounts comm under `/comm`, so `/dm/threads` is served at `/comm/dm/threads`.
Before the conversion every path hard-coded `/v1`, which is why a consumer
mounting with an empty prefix sees a different address afterwards — that
consumer is the gateway, and it is CM28.

| Method | Path | Action |
| --- | --- | --- |
| GET | `/chat/activity` | `comm.chat.activity.read` |
| GET | `/groups/{structure_id}/chat/threads` | `comm.chat_thread.list` |
| GET | `/groups/{structure_id}/chat/threads/{thread_id}/messages` | `comm.chat_message.list` |
| POST | `/groups/{structure_id}/chat/threads/{thread_id}/messages` | `comm.chat_message.post` |
| POST | `/groups/{structure_id}/chat/threads/{thread_id}/read` | `comm.chat_thread.mark_read` |
| POST | `/subjects/{subject_kind}/{subject_id}/threads` | `comm.subject_thread.open` |
| GET | `/subjects/{subject_kind}/{subject_id}/threads` | `comm.subject_thread.list` |
| GET | `/subject-threads/{thread_id}/messages` | `comm.subject_message.list` |
| POST | `/subject-threads/{thread_id}/messages` | `comm.subject_message.post` |
| GET | `/dm/threads` | `comm.dm_thread.list` |
| GET | `/dm/threads/{thread_id}` | `comm.dm_thread.read` |
| GET | `/dm/threads/{thread_id}/participants` | `comm.dm_participant.list` |
| POST | `/dm/threads/{thread_id}/invite` | `comm.dm_participant.invite` |
| POST | `/dm/threads/{thread_id}/accept` | `comm.dm_participant.accept` |
| POST | `/dm/threads/{thread_id}/leave` | `comm.dm_participant.leave` |
| POST | `/dm/threads/{thread_id}/kick` | `comm.dm_participant.kick` |
| POST | `/dm/threads/{thread_id}/promote` | `comm.dm_participant.promote` |
| POST | `/dm/threads/{thread_id}/re-enable` | `comm.dm_participant.reenable` |
| GET | `/dm/threads/{thread_id}/messages` | `comm.dm_message.list` |
| GET | `/dm/threads/{thread_id}/reactions` | `comm.dm_reaction.list` |
| PUT | `/dm/messages/{message_id}/reaction` | `comm.dm_reaction.set` |
| DELETE | `/dm/messages/{message_id}/reaction` | `comm.dm_reaction.clear` |
| POST | `/dm/device-keys` | `comm.dm_device_key.publish` |
| POST | `/dm/device-keys/lookup` | `comm.dm_device_key.lookup` |
| GET | `/dm/addresses` | `comm.address.list_mine` |
| POST | `/dm/addresses/{index}/retire` | `comm.address.retire` |
| GET | `/groups/{structure_id}/moderation/reports` | `comm.report.queue` |
| GET | `/groups/{structure_id}/moderation/messages/{message_id}/reports` | `comm.report.for_message` |
| GET | `/notifications` | `comm.notification.list` |
| GET | `/notifications/unread` | `comm.notification.unread` |
| POST | `/notifications/read` | `comm.notification.mark_read` |
| GET | `/notification-preferences` | `comm.notification_preference.list` |
| PUT | `/notification-preferences/{category}` | `comm.notification_preference.set` |
| POST | `/push-subscriptions` | `comm.push_subscription.subscribe` |
| POST | `/push-subscriptions/unsubscribe` | `comm.push_subscription.unsubscribe` |

`routes.Shape()` prints this without a database, and
`TestEveryDeclaredAddressIsReachable` fails if the count drifts or an address
carries no action.

## Nothing is anonymous

`AnonymousActions` is empty. Every address here reads or writes somebody's
own conversations, addresses or notifications, so there is nothing a caller
without a session should reach.

The list names what is **public**, never what is protected. An operation
added to the spec later and not named there arrives gated, so the failure
direction is a 401 rather than the whole table on the public internet.
`TestNothingIsAnonymous` asserts both halves: the allowlist is empty, and
`PublicRoutes` returns none.

## The exclusion list is gone

`routes/doc.go` used to carry a long, named list of which of the gateway's 37
chat/DM/moderation/address/notification routes this module could portably
serve, and which reached a gateway-internal domain (`chapter`, `member`,
`role`, `phonesalt`, `orgpolicy`) from the handler body and had to stay
behind. Nineteen of 37 qualified; with address and notification's own routes
it was 25.

Those 25 are now the declared table, and the split that list encoded is
`AnonymousActions` plus the mount's authorizer. A route that needs a
gateway-internal fact still composes it in the gateway — but it composes it
*around* a declared operation rather than instead of one.

What has **not** moved, and was the gateway's until it was retired:
`createDMThread`, `blockThreadOrigin` and `postDMMessage` reach
`address.Repository` in a way that is now intra-module but was not when the
line was drawn; moderation's nine writes resolve `callerModerationActor` or
`nearestRoleClass`; and five of address's seven reach `phonesalt`/`orgpolicy`.
Moving any of them is a separate decision, not a side effect of declaring the
table.

Three of chat's routes did move, as CM32 (2026-10-19): listing a room's
threads, reading a thread's posts and posting one. What kept them in the
gateway was that they needed to know which structures the caller belongs to,
and that fact now comes from the host through `Belonging` — see
[the structure fence](#the-structure-fence) below. The rest of chat's
gateway surface — minting a thread, room-level posts with no thread, and the
`wallLabel` presentation — is still not declared.

A fourth joined them as CM40 (2026-10-20): marking a thread read, with the
thread list carrying each thread's unread count for the caller. It sits behind
the same fence; see [read-markers.md](read-markers.md).

## What the manager carries, and why

`dispatch` turns a request into one manager call. Six of comm's addresses are
not that shape, so `CommManager` carries the difference.

**Two roster fences, answering 404 and never 403.** A conversation that
exists but the caller is not on, and a conversation that was never minted,
must be indistinguishable — otherwise the status code is an enumeration
oracle. This is DEV-1137, and it is pinned here because it regressed once
already: the hand-written handlers answered 403 until a real caller went
through a real gateway.

The fences are not one rule but two:

- **any roster row** — reading a conversation or its roster. An invited
  participant may look.
- **an active roster row** — reading messages or reactions, and setting or
  clearing one. Being invited is not being in the conversation.

A reaction is fenced through *its own message's* thread, so a message id
nobody may read is a 404 for the same reason a thread id is.

**`forCaller`.** A thread carries two address pairs — the one it was opened
via and the one it was sent from — and each has an owner and an index.
`MyAddressIndex` is whichever belongs to **this reader**. The owners and the
raw indexes are `json:"-"`, so this projection is the only thing standing
between them and somebody not party to the pair.

Both are same-domain composition, which this package's own `routes/doc.go`
always allowed; what changed is that they sit beside the store now rather
than inside a handler, where `dispatch` can reach them.

## The structure fence

A room belongs to a structure, and only somebody who belongs to that
structure reads or posts in it. comm stores rooms, threads and posts; it does
not know who belongs to what, because that is the host's directory — on one
deployment a group's registrations, on another a class list. So the host
tells it, through one seam on the manager:

```go
type Belonging func(ctx context.Context, actorID, structureID string) (bool, error)

func (m *CommManager) UseBelonging(b Belonging)
```

`ListChatThreadsFor`, `ListChatMessagesFor`, `PostChatMessageFor` and
`MarkChatThreadReadFor` ask it before anything is read or written. Four rules hold it:

- **404, never 403.** A caller who does not belong, a structure that does not
  exist, a thread that was never minted and a thread of *another* structure
  addressed under this one all answer `ErrChatNotFound` with the same text,
  for the reason the roster fences do: a different status would be an
  enumeration oracle for which structures and threads exist.
  `TestAStructureFenceAnswersAnOutsiderExactlyAsAnInventedThread` compares
  status and message across all four.
- **Closed when nothing is supplied.** A manager with no `Belonging`, an empty
  caller or an empty structure id belongs nowhere, so a host that mounts the
  table without wiring the seam serves 404 to everybody rather than every room
  to everybody (`TestWithNoBelongingSuppliedNobodyReadsOrPosts`).
- **A failed lookup is a 500**, through `dispatch`'s redaction, never a 404
  that would pass for "you do not belong" and never the host's own reason
  (`TestABelongingFailureIsNeitherNotFoundNorForbidden`).
- **The address outranks the body, and the author is the session's.** The
  post's structure and thread come from the path, its author from
  `{"from": "caller"}`, and its body is the only value a request supplies; a
  body naming an `author_id`, an `id` or a `structure_id` is ignored
  (`TestAPostsAuthorIsTheSessionNotTheBody`). An empty or blank post is
  `ErrChatInvalid`, a 400.

Each of the first three is mutation-checked: dropping the belonging check,
answering forbidden, dropping the thread-in-structure check, and opening the
fence when no seam is supplied each turn a test red.

**Why a resolver on the manager, and not a session fact.** The alternative was
for the host to put the caller's structures on the request, bound like
`{"from": "caller"}`. `dispatch`'s session facts are one string each
(`Mount.Caller`, `Mount.Device`), so a list of structures would need a new
kind of session fact in `mwanachama-backend-shared`, and it would make every
request pay for a directory read whether or not it touched a room. A resolver
asks one question, about one structure, only on the four operations that
need it — the shape `mwanachama-backend-agency`'s `ManagerResolver` and
`mwanachama-wakala-api`'s own `mustBeMember` already take, and the one
`dispatch` itself takes for the caller (`Caller` is a function the mount
supplies, not a value). It is a field set at boot rather than a constructor
argument so that `NewCommManager`'s signature, which wakala-api calls, did not
change. Forms has the same choice open as its F14; if the owner settles that
the other way, this seam is the one place to change.

**What a structure is, and where belonging comes from, is the host's.**
comm's `structure_id` is opaque. Which of the host's things a structure is —
an agency, a group inside one — and which directory answers `Belonging` is a
decision for the host's own board, not this module's.

## The subject fence

The four `subject` addresses (CM34, 2026-10-20) are fenced by a second host
seam, `UseStanding`, in the same shape as the structure fence above and for
the same reasons: one question about one caller and one subject, asked only
by the operations that need it, closed when nothing is supplied, a failed
lookup a redacted 500, and every miss the same 404. What a subject is, who
its audience is and who answers for it are recorded in
[subject-threads.md](subject-threads.md).

## The session, and what a body may not claim

Two arguments come from the session rather than the request:

- `{"from": "caller"}` — who is acting. An accept, a leave, a reaction and a
  read mark all take the actor from here, so nobody can act as somebody else.
- `{"from": "device"}` — which handset. Added to `dispatch` for this module
  (shared's S45).

`subscribe_push` binds the caller and the handset the same way, but takes
the subscription as named body arguments rather than the whole body, so a
browser's `expirationTime` is ignored rather than refused and an `actor_id`
in the body never reaches the row. See [web-push.md](web-push.md).

`publish_dm_device_key` takes the whole body *and* both session facts, and
the manager overwrites the body's `actor_id`, `published_by` and `device_id`
from the session. Overwriting rather than refusing is deliberate and is
DEV-1265: these are session facts, and a client's opinion of them is replaced
silently rather than turned into an error. The same binding keeps both out of
any MCP tool schema, so an agent that supplies one is overruled rather than
obeyed.

## Statuses, transcribed rather than redesigned

`errors` maps twenty-six sentinels; `ErrPushSubscriptionInvalid` is CM35's 400. Every status is the one the hand-written
handler gave, with one deliberate exception.

**The exception is CM31.** `dmStatusFor` ended in `default:
http.StatusForbidden`, so any error the switch did not name — a dropped
connection, a constraint nobody anticipated — reached the caller as 403 with
the driver's message. The arm existed to catch the package's unexported
`errNotAdmin`, which had no name a declaration could reference. That sentinel
is now the exported `ErrDMNotAdmin` mapped to 403, and everything unmapped is
`dispatch`'s own redacted 500. `TestAnUnmappedFailureIsNotReportedAsForbidden`
drops the roster table and asserts the 500 and the redaction.

**CM30 is the other correction.** An unknown notification category answered
404 from a Go slice that, after CM25, described the *civic* domain rather than
the module — so a school instance's own `attendance` was refused a preference
it was entitled to. The check now asks the mounted domain's declared `values`,
through `ErrNotificationNoSuchCategory`, which keeps the 404 and makes it
right for every domain.

Four behaviours needed carrying across by hand and would otherwise have been
lost quietly:

- `re-enable` defaults its target to the caller, which is what makes a self
  re-enable a body-less POST. A declared `required` would not have done it:
  on the HTTP path `required` is schema documentation rather than a gate
  (shared's S35).
- `mark_notifications_read` still accepts a caller-supplied `read_at` and
  falls back to the server's clock, refusing a malformed one with a 400.
- `list_reports_for_message` takes `{structure_id}` in its path and ignores
  it, because the report is addressed by message. Declared `"ignored": true`
  so the path stays what the gateway serves.
- the 204 operations declare a return even though they render no body.

## Tests

`routes/` has 50 tests across ten files, every one through a real
`httptest` server with the session carried on the context, because both of
the bugs this repo's CLAUDE.md records escaped by calling handlers in Go.

The four security properties are mutation-checked — removing the wide fence,
answering forbidden instead of not-found, widening the narrow fence to any
roster row, and dropping the per-caller projection each turn a test red.
CM15's two guards and W14's remaining shapes are ported onto the new table,
and `claimEmptiedThread`'s conditional `UPDATE` is mutation-checked there.
