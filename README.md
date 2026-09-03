# mwanachama-backend-comm

Chapter chat, direct/group messaging and message moderation for
[mwanachama-backend-api-gateway](../mwanachama-backend-api-gateway) — extracted
from that repo's `internal/domain/{chat,directmessage,moderation}` and
`internal/store/{postgres,memory}`.

No gRPC, no sub-service shape — imported directly by the gateway, the same
pattern [mwanachama-backend-taskmanager](../mwanachama-backend-taskmanager)
uses. Unlike taskmanager, this module's storage is hand-rolled SQL rather than
`mwanachama-backend-shared`'s generic entity-graph store, because that is what
chat/directmessage/moderation already were before the move.

Gateway-side tables carry a `comm_` prefix (`comm_chat_thread`,
`comm_dm_thread`, `comm_message_report`, ...) — see the gateway's
`internal/store/postgres/migrations/000065_extract_comm_tables.up.sql` for the
rename that took them there, and this repo's `schema.sql` for this module's
own test-fixture schema.

See [documentation/](documentation/) for design and task board, and
[CLAUDE.md](CLAUDE.md) for the two decisions a reader is most likely to
question: why moderation's act-log write doesn't import the gateway's custody
package, and why `schema.sql` isn't a real migration.
