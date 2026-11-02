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
all answer the same 404, for the reason the structure fence does: a different
answer would tell the caller which people exist. A failing `RoomsOf` is a
redacted 500, never a 404 that would pass for "she has no rooms".

## Tests

`models/chat_reach_test.go` holds the reason table, covering the cases of
`chat_audience_test.go` with the reason added and `ReadableBy` held equal to its
`ok`. `routes/person_chat_reach_test.go` crosses the real mux: a person who only
belongs gets the group threads and those she asked for, a lead also gets the
`leads` ones, a non-member and an unseeded seam are not found, a caller without
the action is refused, the person is the path's and never the caller's, and a
row carries nothing but its six fields. Each was mutation-checked.
