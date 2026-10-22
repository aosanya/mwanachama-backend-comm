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

## Not yet built

- Declared operations (create, list, stop) and the manager wiring, with the
  author and the stopper bound from the caller and the group fence applied.
- A schedule given as a list of dates rather than a rule (`recur` supports
  both).

## Open questions

- Who may set up and who may Stop a standing message: any member the
  `Belonging` fence admits, or only the room's leads?
- After downtime, should every missed occurrence be sent, only the latest,
  or none?
