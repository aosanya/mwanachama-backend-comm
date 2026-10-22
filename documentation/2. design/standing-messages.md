# Standing messages (CM102)

A standing message is a message to a group that goes out on a schedule
(member app decision 339). It takes up CM58's standing invitation.

## What is stored

`standing_message` holds the group (`structure_id`), an optional thread, the
author, the body, and the schedule as an RFC 5545 `rule` read by
`mwanachama-backend-shared/recur` in an IANA `zone` from `starts_at`, with an
optional `ends_at`. `stopped_at`/`stopped_by` record a Stop;
`last_sent_at` records the occurrence most recently sent.

A schedule is checked when the message is set up: an unknown zone, a rule
`recur` cannot read, or an end before the start is `ErrStandingInvalid`.

## Sending

The host calls `StandingStore.SendDueStandingMessages(ctx, now)` on its own
tick. comm runs no timer of its own.

For each running message it asks the schedule for the occurrences after
`last_sent_at` (or from `starts_at`) up to and including `now`, and sends the
latest of them into the group's chat as an ordinary chat message by the
author.

- **At most once.** `last_sent_at` is moved forward under a compare-and-swap
  on its previous value before the post, so two hosts calling at once send an
  occurrence once. A post that fails after the claim is not retried: the
  occurrence is lost rather than sent twice.
- **A host that was down sends one message, not a backlog.** Only the latest
  missed occurrence goes out. This is provisional; see the open question
  below.
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
- **The room's fence applies.** Every call asks `Belonging` first, and a
  message aimed at a thread is fenced like the thread itself, so a `leads`
  thread's standing message is out of sight to anybody who cannot read that
  thread. An outsider, an invented id and another structure's id all answer
  the same 404, and the list leaves out what the caller cannot read.
- `starts_at` and `ends_at` are RFC3339 text on the wire; a bad one, a bad
  zone or a bad rule is `ErrStandingInvalid` (400). A second stop is 409.
- **The host's tick is `CommManager.SendDueStandingMessages(ctx, now)`**,
  the store's call promoted onto the manager. Calling it on a timer is the
  host's (wakala-api WK203).
- **Who may set one up or stop one is deliberately not decided here.** comm
  admits any member `Belonging` admits; the distinct `create` and `stop`
  actions are where a mount's authorizer narrows that to leads, until the
  owner answers the first question below.

## Not yet built

- A schedule given as a list of dates rather than a rule (`recur` supports
  both).
- The host's tick (wakala-api WK203).

## Open questions

- Who may set up and who may Stop a standing message: any member the
  `Belonging` fence admits, or only the room's leads?
- After downtime, should every missed occurrence be sent, only the latest,
  or none?
