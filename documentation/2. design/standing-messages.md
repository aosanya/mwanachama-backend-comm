# Standing messages (CM102)

A standing message is a message to a group that goes out on a schedule
(member app decision 339). It takes up CM58's standing invitation.

## What is stored

`standing_message` holds the group (`structure_id`), an optional thread, the
author, the body, and the schedule, read by
`mwanachama-backend-shared/recur` in an IANA `zone` from `starts_at`, with an
optional `ends_at`. The schedule is exactly one of an RFC 5545 `rule` or
`dates`, a declared `json` list of `YYYY-MM-DD` dates (the same shape forms'
schedule takes, e.g. school terms). Each date is sent at `starts_at`'s local
time in the zone, and a date before `starts_at` or after `ends_at` is never
sent. `stopped_at`/`stopped_by` record a Stop;
`last_sent_at` records the occurrence most recently sent.

A schedule is checked when the message is set up: an unknown zone, a rule
`recur` cannot read, a date that is not `YYYY-MM-DD` or not on the calendar,
both a rule and dates or neither, or an end before the start is
`ErrStandingInvalid` (400 on the create address, which takes `rule` or
`dates` in the body).

## Sending

The host calls `StandingStore.SendDueStandingMessages(ctx, now)` on its own
tick. comm runs no timer of its own.

For each running message it asks the schedule for the occurrences after
`last_sent_at` (or from `starts_at`) up to and including `now`, and sends the
latest of them into the group's chat as an ordinary chat message by the
author.

- **Exactly once (CM102).** The claim and the post are one transaction:
  `last_sent_at` is moved forward under a compare-and-swap on its previous
  value, then the message is posted, and both commit together. Two hosts
  calling at once still send an occurrence once, because the loser's
  compare-and-swap matches no row. A post that fails rolls the claim back, so
  the occurrence stays due and the next tick sends it. Before CM102 the claim
  committed on its own before the post, and a failed post lost the
  occurrence.
- **One failing message does not end the run (CM102).** Every running
  message is tried; the ones that fail are returned as one joined error, each
  naming its standing message id, beside the messages that were sent. Before
  CM102 the first failure returned and the rest waited for the next tick.
- **A host that was down sends one message, not a backlog.** Only the latest
  missed occurrence goes out (decision 365).
- **A stopped message sends nothing more**, and stopping twice answers
  `ErrStandingStopped`.
- **The end is the schedule's.** `recur` drops occurrences after `ends_at`.

## The operations

Four addresses in `comm.operations.json`, each under the room's own path and
each with an action of its own so the mount's authorizer decides who may use
it:

| Address | Action | Call |
| --- | --- | --- |
| `POST /groups/{structure_id}/chat/standing` | `comm.standing_message.create` | `CreateStandingMessageFor` |
| `GET /groups/{structure_id}/chat/standing` | `comm.standing_message.list` | `ListStandingMessagesFor` |
| `GET /groups/{structure_id}/chat/standing/{standing_id}` | `comm.standing_message.get` | `GetStandingMessageFor` |
| `POST /groups/{structure_id}/chat/standing/{standing_id}/stop` | `comm.standing_message.stop` | `StopStandingMessageFor` |

- **The author and the stopper are the session's**, bound `{"from": "caller"}`;
  a body naming another is overwritten.
- **Only the room's leads set one up or stop one (CM102, decisions 364,
  367).** `CreateStandingMessageFor` and `StopStandingMessageFor` ask
  `Leading` after `Belonging`, the same `asLead` gate a decline uses, and a
  member who does not lead is answered 404 exactly as an outsider. Reading
  and listing stay open to every member the fence admits. Who the leads are
  is the host's `Leading`: the group's leaders plus the agency's owners and
  agents.
- **The room's fence applies.** Every call asks `Belonging` first, and a
  message aimed at a thread is fenced like the thread itself, so a `leads`
  thread's standing message is out of sight to anybody who cannot read that
  thread. An outsider, an invented id and another structure's id all answer
  the same 404, and the list leaves out what the caller cannot read.
- **The schedule body is `recur.Wire`** (CM102, decision 367): `start`,
  `end`, `zone`, and `rule` or `dates`, read through
  `recur.Wire.Schedule()` rather than a copy of comm's own, so every module
  that takes a schedule accepts the same shape. `start` and `end` are
  RFC3339 text on the wire, and a standing message answers with the same two
  names. The stored columns keep their names, `starts_at` and `ends_at`. A
  bad instant, a bad zone, a bad rule or a list of dates with no `end` is
  `ErrStandingInvalid` (400). A second stop is 409.
- **The host's tick is `CommManager.SendDueStandingMessages(ctx, now)`**,
  the store's call promoted onto the manager. Calling it on a timer is the
  host's (wakala-api WK203).

## Not yet built

- The host's tick (wakala-api WK203).

## Open questions

None. Who may set one up or stop one was settled as the room's leads, and
after downtime only the latest occurrence is sent (owner, 2026-10-23,
decisions 364, 365, 367).
