# Raising one notification to the set behind a record (CM63)

A merged item can stand for many arrivals: forty residents raise one
pothole, and each of them must be answered on their own channel. Before
CM63 comm raised to the recipients it was handed, so the fan-out was the
caller's to build and nothing checked that all forty went.

## The seam

`UseAudience` gives the manager an `Audience` function: given a record's
`subject_kind` and `subject_id`, it names the actors behind that record.
comm never learns who they are or why — the host reads its own arrivals.
A manager given none has no audience.

`RaiseToAudience(ctx, template)` takes a notification with no `actor_id`
and a record named by `subject_kind`/`subject_id`, asks the audience, and
raises one row per actor through the ordinary `Raise`, so each row is
validated, muting is honoured and a Web Push follows exactly as for any
other raise. The event and category are the caller's: no new vocabulary is
declared for this, so CM26's rule that comm declares only its own acts is
untouched.

## Nothing drops silently

The answer is an `AudienceRaise`: `Resolved` (distinct actors the
audience named, blanks and repeats removed), `Raised` (the rows written)
and `Missed` (each actor not written, with the reason — muted, or a spent
cap). `Complete()` is `Raised + Missed == Resolved`, which always holds on
a nil error; a caller that must prove every raiser was answered reads
`Missed`.

- No audience configured, or an audience naming nobody, is
  `ErrNotificationNoAudience` — never a success that raised nothing.
- A template naming an actor, or naming no record, is
  `ErrNotificationInvalid`.
- The template is checked against the spec and `Validate` before the first
  write, so an invalid event writes nothing for anyone.
- Any other store error stops the run and returns what was raised so far
  with the error. The rows are separate inserts, not one transaction,
  because each `Raise` also schedules its push; a retry after a partial
  failure re-raises to everyone, so an event that must be once per actor
  should carry a cap.

## What the owner must decide

No address is declared. Exposing `RaiseToAudience` on
`comm.operations.json` needs a gating action, and who may raise to a
record's whole set — the item's answerer, a lead, any holder of the
board's permission — is a product decision, not comm's. Until it is made the
host calls the manager method directly.
