# mwanachama-backend-comm

Chapter chat, direct/group messaging and message moderation for
[mwanachama-backend-api-gateway](../mwanachama-backend-api-gateway) — extracted
from that repo's `internal/domain/{chat,directmessage,moderation}` and
`internal/store/{postgres,memory}`.

No gRPC, no sub-service shape — imported directly by the gateway, the same
pattern [mwanachama-backend-taskmanager](../mwanachama-backend-taskmanager)
uses. Storage is GORM (`gormstore/`), backed by Postgres in production and
sqlite in this module's own tests — the same shape
[mwanachama-backend-actor](../mwanachama-backend-actor) uses, which this
module moved onto 2026-09-04 (see CLAUDE.md).

Gateway-side tables carry a `comm_` prefix (`comm_chat_thread`,
`comm_dm_thread`, `comm_message_report`, ...) — see the gateway's
`internal/store/postgres/migrations/000065_extract_comm_tables.up.sql` for the
rename that took them there. `gormstore.Migrate` is this module's own schema
source against those same table names (`gormstore.DefaultTableNames()`) —
there is no separate `schema.sql` fixture.

`models/` holds the domain types and repository interfaces
(`ChatRepository`/`DMRepository`/`ModerationRepository`); `routes/` holds
this module's own HTTP surface for the subset of the gateway's chat/dm/
moderation routes that carry no gateway-only policy (see `routes/doc.go`).

See [documentation/](documentation/) for design and task board, and
[CLAUDE.md](CLAUDE.md) for the two decisions a reader is most likely to
question: why moderation's act-log write doesn't import the gateway's custody
package, and why this module's HTTP surface (`routes/`) covers only 19 of
the gateway's 37 chat/dm/moderation routes.
