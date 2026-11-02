# What one person can read (HS691)

A manager asking "what can this person read, and why" had no door. The room
list is bound to the session caller (`{"from": "caller"}`) and dispatch
refuses a request that names another caller, so no request could ask on
somebody's behalf. `list_person_chat_reach` is that door: the person is a path
argument, not a session fact, and the gate is its own action.

## The one rule

`ChatThread.ReadableReason(actorID, leads)` answers the reason and whether the
actor reads the thread. `ReadableBy` is its `ok`, so a room list, a read and
this report can never disagree.

| Reason | When |
| --- | --- |
| `group` | the thread is open to everybody who belongs (and every row older than the audience column) |
| `leads` | the thread is still with the leads and the actor is one of them |
| `asked` | the thread is still with the leads and the actor is the one who asked for it |

A lead who also asked for a thread reads it as `leads`, the order `ReadableBy`
always checked. A declined thread keeps its audience, so it stays `leads` for
the leads and `asked` for its starter and nobody else's.

## The three pieces

- `CommManager.ChatReachOf(ctx, structureID, personID)` is one room. Nothing
  comes back when `Belonging` says she is not in it, and a room with no
  readable thread is also nothing.
- `UseRoomsOf(func(ctx, actorID) ([]string, error))` is the host seam that
  names every structure a person is in, so one call covers all her rooms. It is
  declared beside `UseBelonging` and `UseLeading`, and defaulted the same way:
  a manager given none has no person with a room.
- `CommManager.ChatReachOfPerson(ctx, personID)` asks the seam, re-checks each
  structure against `Belonging` (the host's two answers must agree, and a room
  `Belonging` denies is skipped), and concatenates the rooms' reach.

## The address

`GET /people/{actor_id}/chat/reach`, operation `list_person_chat_reach`, action
`comm.person_reach.list`. The catalogue permission and the Hub manager's grant
are HS653's, not comm's.

Each row is `{structure_id, thread_id, label, audience, requested_by, reason}`
and nothing else: no post, no unread count, no read point, so the report says
what a person may open without opening it. `requested_by` is absent when the
leads started the thread. Direct messages are never listed; whether a manager
may see who talks to whom is a privacy decision that belongs to the owner.

A person who belongs to no room, one the host does not know and an invented id
all answer the same `404 {"error":"chat: not found"}`, so the 404 does not say
whether the person exists. That is all it hides. A person who belongs to a room
with no readable thread answers `200 []`, so 200 against 404 tells a holder of
the action whether an id belongs to at least one room. What counts as a room is
the host's `RoomsOf`: in wakala-api an owner or an agent is in every group of
the agency, so for them the answer is 200 whenever the agency has any group. A
failing `RoomsOf` is a redacted 500, never a 404 that would pass for "she has no
rooms".

## Cost

Per room the person is in, the reach asks `Belonging` once, reads that room's
threads with one `ListThreads` and asks `Leading` once. A person in 40 rooms
with 5 threads each costs 200 rows, about 30 KB of JSON and 40 SELECTs on the
comm side, and the host's belonging and leading seams run 40 times each, once
per room. The count was measured through the real route with a person who asked
for all 200 threads; the size moves with the labels. Nothing batches the rooms
today, so the report suits a manager opening one person, not a list of people.

## Tests

`models/chat_reach_test.go` holds the reason table, covering the cases of
`chat_audience_test.go` with the reason added and `ReadableBy` held equal to its
`ok`. `routes/person_chat_reach_test.go` crosses the real mux: a person who only
belongs gets the group threads and those she asked for, a lead also gets the
`leads` ones, a non-member and an unseeded seam are not found, a caller without
the action is refused, the person is the path's and never the caller's, and a
row carries nothing but its six fields. Each was mutation-checked.

The gate is held in three places, each with a mount whose `Authorize` records
the action it is asked and refuses: the route (`routes/person_chat_reach_test.go`),
the tool called directly (`routes/person_reach_tool_test.go`) and the tool
through a real MCP client (`mcp/mcp_test.go`). Each wants the refusal, exactly
one ask of the gate and it for `comm.person_reach.list`, and no call to the
host's `RoomsOf`. Each was checked in a scratch copy by letting `dispatch` skip
the gate for that action alone: the route test fails when the route's gate is
skipped, the tool tests when the tool's is, and both pass with it restored.
`TestNothingIsAnonymous` covers the route by its empty allowlist: naming the
action in `AnonymousActions` fails it.
