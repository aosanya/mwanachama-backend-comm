# Acknowledging a notification (CM16)

A notification already says who was told: it is raised for exactly one recipient, so "which wards were told" is the set of rows that share a subject. What it could not say was who then acted. The party-mobilization template's cascade ("every branch within a day") needs the centre to see which wards confirmed and which went quiet. CM77 and the three template wishes it absorbed ask for the same state.

## What is declared

One object, role `notification_acknowledgement`: `id`, `notification_id`, `actor_id`, `acknowledged_at`. The domain names it (civic: `notification_acknowledgement`; the school example: `announcement_acknowledgement`).

- `notification_id` is unique, so there is one acknowledgement per notification and acknowledging again answers the first one with its original time.
- `actor_id` is the notification's own recipient, taken from the session. Nobody acknowledges on another's behalf: a notification that is somebody else's answers exactly as one that does not exist, a 404.
- It is a separate row, not a column on `notification`, so a notification stays what it was when raised and the recipient's act has its own time.

Acknowledging does not set `read_at`. The read mark stays the recipient's own assertion and the only column a client writes (see the field's description in the blueprint). A client that wants both calls `comm.notification.mark_read` too.

## The two addresses

| Method and path | Action | Does |
| --- | --- | --- |
| `POST /notifications/{notification_id}/acknowledge` | `comm.notification.acknowledge` | the caller acknowledges one of their own notifications |
| `GET /notification-receipts?subject_kind=&subject_id=` | `comm.notification.receipts` | lists every recipient a subject was raised for, with `read_at` and `acknowledged_at` |

`receipts` takes optional `event`, `after` and `limit`. Receipts are ordered by notification id; `after` is the id of the last receipt already read, and `limit` defaults to 200 and is capped at 1000, so a cascade to thousands of branches is read in pages. A recipient with a null `acknowledged_at` has not acted.

A recipient who muted the category was never raised for, so has no row and does not appear. The sender therefore sees who was told and who acted, never who muted, which keeps the rule that the sender is told a count of the muted and not who they were.

## Who may call which

comm decides nothing about who the centre is. Both are declared actions, so the mount gates them: `acknowledge` is for the recipient (a resident included), while `receipts` reveals other people's inboxes and belongs only to whoever the host names as sending.

## Tests

`notification_acknowledgement_test.go` for the store and `routes/notification_acknowledgement_test.go` through a real mux, including a mount that allows only `acknowledge` and is refused on `receipts`.
