# comm's declared route table

Forty-three addresses, declared in `comm.operations.json` and served by a
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
| POST | `/groups/{structure_id}/chat/threads` | `comm.chat_thread.create` |
| POST | `/groups/{structure_id}/chat/threads/{thread_id}/open` | `comm.chat_thread.open` |
| POST | `/groups/{structure_id}/chat/threads/{thread_id}/decline` | `comm.chat_thread.decline` |
| GET | `/groups/{structure_id}/chat/threads/{thread_id}/messages` | `comm.chat_message.list` |
| POST | `/groups/{structure_id}/chat/threads/{thread_id}/messages` | `comm.chat_message.post` |
| POST | `/groups/{structure_id}/chat/threads/{thread_id}/read` | `comm.chat_thread.mark_read` |
| GET | `/groups/{structure_id}/chat/threads/archived` | `comm.chat_thread.list_archived` |
| POST | `/groups/{structure_id}/chat/threads/{thread_id}/archive` | `comm.chat_thread.archive` |
| DELETE | `/groups/{structure_id}/chat/threads/{thread_id}/archive` | `comm.chat_thread.unarchive` |
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
| POST | `/notifications/{notification_id}/acknowledge` | `comm.notification.acknowledge` |
| GET | `/notification-receipts` | `comm.notification.receipts` |
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

A fifth joined them as CM41 (2026-10-20): starting a thread, which is the
"minting a thread" half above. It sits behind the same fence; see
[starting a thread](#starting-a-thread) below.

Two more joined them as CM49 (2026-10-21): archiving a thread for oneself
and bringing it back. They sit behind the same fence; see
[chat-archive.md](chat-archive.md).

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

`ListChatThreadsFor`, `StartChatThreadFor`, `OpenChatThreadFor`,
`ListChatMessagesFor`, `PostChatMessageFor`, `MarkChatThreadReadFor`, `ArchiveChatThreadFor` and `UnarchiveChatThreadFor` ask it before anything is read or written. Four rules hold it:

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

## Starting a thread

CM41, 2026-10-20, for the member app's decision 100: a group's room holds
several threads, each with a label.

`POST /groups/{structure_id}/chat/threads` (`comm.chat_thread.create`) takes
`{"label": "Road repairs"}` and answers 201 with the thread. It goes through
`StartChatThreadFor`, which asks the structure fence first, then
`ChatStore.StartThread`, which mints through the same resolve-or-insert path
as `MintThread`.

- **The label is a field on the thread.** `chat_thread.label` in the
  blueprint and `ChatThread.Label` in `models/`, together. It is not
  `required` in the declaration, because a thread minted by its topic alone
  through `MintThread` has none, and a `not null` column added to a populated
  table would refuse to migrate. Starting a thread requires one: an absent or
  blank label, or one with no letter or digit in it, is `ErrChatInvalid`, a
  400 (`TestAThreadNeedsALabel`).
- **The label is stored tidied.** Leading, trailing and repeated whitespace
  is collapsed, so `"  Road   repairs "` is stored and listed as
  `"Road repairs"`.
- **An existing label answers with that thread.** A started thread's
  `tag_path` is derived from its label: lower-cased, its runs of letters and
  digits joined by `-`, under `/` — `"Road repairs"` is `/road-repairs`,
  `"Általános"` is `/általános`. That path is what the existing unique index
  on `(structure_id, tag_path)` already guards, so starting `"General"`,
  `"general"` or `" GENERAL "` again returns the first thread rather than a
  second one, still with 201, and the same label in another structure is
  another thread (`TestStartingAnExistingLabelAnswersWithThatThread`). A
  thread minted by topic at the same path, `/general`, is the same thread,
  and keeps whatever label it had.
- **The list carries the label.** `list_chat_threads` returns `ChatThreadView`,
  which embeds the thread, so `label` is on every row with `unread` and
  `last_message_at` beside it (see [read-markers.md](read-markers.md)).
- **Anybody who belongs starts one.** Since CM48 every caller `Belonging`
  admits may start a thread; the mount's gate on `comm.chat_thread.create`
  still applies on top. What the caller gets depends on whether the host's
  `Leading` names them a lead of the structure: a lead's thread is the leads'
  with no requester, anybody else's is the leads' with the caller as its
  requester and tells the leads. See
  [Threads with the leads](#threads-with-the-leads) and
  [A thread a member starts](#a-thread-a-member-starts). From CM42 until
  CM48 only a lead started one and anybody else answered the same 404 as an
  outsider.

The fence holds exactly as on the other room addresses: an outsider and an
invented structure answer the same 404, and with no `Belonging` mounted
nobody starts one (`TestStartingAThreadIsFencedLikeEveryOtherRoomAddress`,
`TestWithNoBelongingSuppliedNobodyStartsAThread`, both mutation-checked
against dropping the fence).

## Threads with the leads

CM42, 2026-10-20, for the member app's decisions 103 and 107: a thread can be
read by the structure's leads only until one of them opens it to the group.

- **The audience is a field on the thread.** `chat_thread.audience` is an enum
  of `leads` and `group`, declared with the default `group` so every row older
  than the column, and every thread minted by its topic alone through
  `MintThread`, stays the group's. `ChatStore.StartThread` mints `leads`.
  `chat_thread.requested_by` names the member who started a thread without
  leading the structure, who reads and writes in it beside the leads, see
  [A thread a member starts](#a-thread-a-member-starts).
- **Who leads is the host's.** comm does not know who a structure's leaders or
  an agency's operators are, so the host says, through a second seam beside
  `UseBelonging`:

  ```go
  type Leading func(ctx context.Context, actorID, structureID string) (bool, error)

  func (m *CommManager) UseLeading(l Leading)
  ```

  It is asked only after `Belonging` has answered yes, so a lead the host does
  not also count as belonging is still fenced out of the room. A manager with
  no `Leading` has no leads: nobody opens a thread through the table, every
  start is a member's, and a `leads` thread is readable by nobody but its
  requester.
- **The leads are the room's admins.** The member app calls them admins
  (decisions 150 and 151): the group's leaders and the agency's owners and
  agents. comm draws no line between the three; `Leading` answers yes for all
  of them and `Leads` names all of them, and which actors those are is the
  host's fact entirely. A host that answers only for the group's leaders
  leaves the agency's owners and agents as members of the room, so their
  starts carry them as requester and they cannot open or decline.
- **`models.ChatThread.ReadableBy(actor, leads)`** is the one rule: a `group`
  thread is everybody's who belongs, a `leads` thread is the leads' and the
  requester's.
- **The list, the posts, a post and the read marker honour it.**
  `ListChatThreadsFor` leaves out a thread the caller may not read;
  `ListChatMessagesFor`, `PostChatMessageFor` and `MarkChatThreadReadFor`
  answer such a thread exactly as an invented one, the same 404 with the same
  text, so a member cannot learn that a leads thread exists
  (`TestAStartedThreadBeginsWithTheLeadsAndAMemberCannotSeeItExists`).
- **Opening it.** `POST /groups/{structure_id}/chat/threads/{thread_id}/open`
  (`comm.chat_thread.open`) moves the thread to `group` and answers 200 with
  it. Nothing about its posts changes, so every member reads what the leads
  wrote before, unread (`TestOpeningAThreadGivesItToTheGroupWithWhatWasWrittenBefore`).
  Opening an open thread answers it unchanged. Only a lead of that structure
  opens one; a member, the requester, a lead of another structure and an
  invented thread all answer the same 404
  (`TestOnlyALeadOfTheStructureOpensAThread`). The requester is
  refused with 404 rather than 403 too, so the address never needs a second
  refusal shape.
- **Mutation-checked.** Reading every thread as a lead in the list, in the
  thread fence, or letting a non-lead open one each turns a test red.
- **The room's General.** The member app's decision 100 has every room hold an
  open General thread, which the host makes. It can mint one by topic
  (`MintThread`, already the group's) or start a labelled one and open it at
  once with `ChatStore.OpenThread`, which needs no caller
  (`TestAHostOpensAThreadItStartedSoTheRoomHasAnOpenGeneral`).

## A thread a member starts

CM43, 2026-10-20, for the member app's decisions 103, 107 and 108, and CM48,
2026-10-21, for decisions 150 and 151: any member starts a thread, the leads
talk in it with the member who started it, and the leads either open it to
the group (CM42) or decline it.

- **Starting.** `POST /groups/{structure_id}/chat/threads`
  (`comm.chat_thread.create`) takes a `label` from a caller `Belonging`
  admits and `Leading` does not, and answers 201 with a `leads` thread whose
  `requested_by` is the caller from the session, never the body. The label is
  normalised as a lead's is. It is fenced like every room address: an
  outsider and an invented structure answer the same 404, and a label with no
  letter or digit is a 400.
- **The request address folded into start.** CM43 shipped this as its own
  address, `POST /groups/{structure_id}/chat/threads/request` with the action
  `comm.chat_thread.request` and a `topic`. CM48 removed both: the member app
  offers one Start a thread to everybody (decision 150), so one address serves
  everybody and the caller's standing decides the shape. The stored event
  keeps its name, `thread_requested`, because a stored enum value outlives a
  rename.
- **Who is told.** comm knows neither a group's leaders nor an agency's
  operators, so the host names them through a third seam beside `Belonging`
  and `Leading`:

  ```go
  type Leads func(ctx context.Context, structureID string) ([]string, error)

  func (m *CommManager) UseLeads(l Leads)
  ```

  Each actor it names gets one `thread_requested` notification, raised in the
  same transaction as the thread: category `chat`, subject `chat_thread` and
  the thread's id, `author_actor_id` the asker, `structure_id` the room.
  Duplicates, empty ids and the asker are dropped, and so is anybody
  `Belonging` or `Leading` does not confirm, so a host's list cannot tell
  somebody who could not then read the thread. A muted recipient is skipped,
  and each raised row is pushed after the commit like every other. A manager
  with no `Leads` still takes a member's start and tells nobody — the leads find them
  in the room's list.
- **Starting never reveals a thread.** A label the room already holds in a
  thread the caller may read — the open General, or their own earlier start
  — answers with that thread and tells nobody. A label held by a thread the
  caller may *not* read, another member's thread or a thread the leads
  started, is given a thread of its own under a tag path suffixed with the new
  thread's id, so the answer is the same 201 with a fresh thread either way
  (`TestAMemberStartingNeverRevealsAThreadTheyCannotRead`). The leads may therefore
  see two threads with one label; which one to open is theirs.
- **Declining.** `POST /groups/{structure_id}/chat/threads/{thread_id}/decline`
  (`comm.chat_thread.decline`) is a lead's act, fenced exactly as opening is:
  the asker, another member, a lead of another structure and an invented
  thread all answer the same 404. It stamps `declined_at` and `declined_by`
  and answers 200 with the thread. Only a thread somebody asked for, still
  with the leads, can be declined; one the leads started themselves, or one
  already opened, is `ErrChatNotDeclinable`, a 409, which tells a lead nothing
  they could not already read. Declining a declined thread answers it
  unchanged and tells nobody again.
- **Who is told of a decline.** The asker gets one `thread_declined`: category
  `chat`, subject `chat_thread`, `author_actor_id` the lead who declined (the
  member app's M05.1 shows "from the Chair"), `structure_id` the room. A lead
  who declines a thread they asked for themselves is not told.
- **A declined thread stays readable, read-only, by its starter and the
  leads** (CM49, member app decisions 152, 153, 203–205). It stays on their
  lists with `declined_at` set and its posts stay readable to them; to anybody
  else it answers exactly as a thread that was never minted, because
  `ChatThread.ReadableBy` treats it as the leads' thread it still is. A post to
  it is refused `ErrChatThreadDeclined`, a 409. A lead may still open it: the
  open clears `declined_at` and `declined_by` and makes it an ordinary group
  thread, though its tag path stays aside
  (`TestADeclinedThreadStaysReadOnlyToItsAskerAndTheLeadsAndTellsTheAsker`).
- **A declined topic is free again.** Declining moves the thread's tag path
  aside, suffixed with its own id, so its label's path is empty again: starting
  that label again mints one new thread at the label's own path,
  and every later start answers that one, exactly as for a label
  never declined (`TestADeclinedLabelIsStartedOnceAgainNotOnEveryStart`).
  Before this a declined thread kept the path, and each start of its label
  minted yet another thread beside it, which broke a host's idempotent
  General (wakala-api WK121).
- **Mutation-checked.** Letting a member's start reuse a thread they cannot
  read, telling a name `Leading` does not confirm, reading a declined thread,
  opening a declined thread, reusing a declined thread's label and declining
  twice each turn a test red.

## Posts since a known one

`list_chat_messages` takes an optional `after` query argument, a post's id
(HS27, Hub Steward gap 4). Omitted, the listing is unchanged: every post in
the thread, oldest first. Given, only the posts that sort after that one in
the same `created_at, id` order the full listing uses, so a client holding
the newest post it has seen asks for what is new without fetching the
thread again. Posts written in the same instant are ordered by id, which is
why the comparison is on the pair rather than on the time alone.

The room fence runs first, so an outsider naming any `after` gets the
not-found an outsider always gets. An `after` that names no post, or a post
of another thread or another structure, answers the same not-found as an
invented thread rather than an empty list or a 400, so the argument cannot
be used to learn which posts exist elsewhere. Pinned in
`routes/chat_messages_after_test.go`.

## Reading by id (CM51)

A notice names one thing — a `thread_reply` a subject thread, a
`message_removed` a chat message — and a client opening it has only that id.
Two addresses answer it, both behind the fence the thing already sits behind
(member app decision 191):

- `GET /subject-threads/{thread_id}` answers the thread's `subject_kind`,
  `subject_id` and `audience` (`private` or `group`). It is the same
  `readableThread` check the thread's messages use, so a caller without
  standing on the subject, another actor's private thread and an invented id
  are the same 404.
- `GET /chat/messages/{message_id}` answers the message with its
  `structure_id` and `thread_id`. Its author reads it while they belong to
  the structure, removed or not, because the removal notice is theirs and has
  to open. Anybody else reads it only through the thread fence (a thread
  still with the leads stays theirs and its requester's) and only while no
  removal names it. Every miss — outside the structure, outside the thread,
  removed and not the author's, invented — is the same 404.

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

`errors` maps twenty-seven sentinels; `ErrPushSubscriptionInvalid` is CM35's 400 and `ErrChatNotDeclinable` CM43's 409. Every status is the one the hand-written
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

## Who is told of comm's own raises (CM106)

**As built CM106, 2026-10-23.** Two of comm's raises let the host decide who
is told: `thread_declined` from `DeclineChatThreadFor`, and the reply raised
by `PostSubjectMessageFor` (`thread_reply`, or `record_message` on a record's
private thread). `UseRecipients` gives the manager a `Recipients` function,
shaped like `UseLeads`: it receives the event, a `RaiseRoom` naming where the
raise happens (`StructureID` and `ThreadID` for a decline; `ThreadID`,
`SubjectKind` and `SubjectID` for a reply) and comm's default recipients, and
returns who is told. The defaults are what comm tells today: the thread's
requester on a decline, and the audience authors, the owner or the
`Answerers` on a reply. The hook receives a copy, so changing the slice it was
given changes nothing.

- **No hook keeps today's recipients exactly.** The defaults pass through
  untouched, unfiltered and in order.
- **What the hook returns is the list.** Empty names and repeats are dropped;
  nothing else is checked, because the hook is the host's rule (member app
  decisions 186, 187, wakala-api WK146) and comm has no fact to check it
  against. Returning nothing tells nobody.
- **A hook error refuses the act, the same way a failing `Answerers` or
  `Leads` does.** It is asked inside the act's transaction, so a failure rolls
  back the decline or the post and answers 500. Telling the wrong people, or
  nobody, while reporting success would be the worse surprise.

Pinned in `routes/recipients_test.go`, over a real mux with a real caller.

## Tests

`routes/` has 67 tests across thirteen files, every one through a real
`httptest` server with the session carried on the context, because both of
the bugs this repo's CLAUDE.md records escaped by calling handlers in Go.

The four security properties are mutation-checked — removing the wide fence,
answering forbidden instead of not-found, widening the narrow fence to any
roster row, and dropping the per-caller projection each turn a test red.
CM15's two guards and W14's remaining shapes are ported onto the new table,
and `claimEmptiedThread`'s conditional `UPDATE` is mutation-checked there.
