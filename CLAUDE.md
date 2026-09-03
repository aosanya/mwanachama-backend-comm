# CLAUDE.md

Guidance for Claude Code working in this repository.

## Project: mwanachama-backend-comm

Chapter chat, direct/group messaging (unified as one `DMThread` model — a
group is a `DMThread` with more than one recipient, not a separate concept)
and message moderation, ported unchanged in business logic from
[mwanachama-backend-api-gateway](../mwanachama-backend-api-gateway)'s
`internal/domain/{chat,directmessage,moderation}` and
`internal/store/{postgres,memory}`. Module path
`github.com/aosanya/mwanachama-backend-comm`.

`mwanachama-backend-api-gateway` runs as one service and imports this package
directly — there is no separate process, matching
[mwanachama-backend-taskmanager](../mwanachama-backend-taskmanager)'s CLAUDE.md.
HTTP routes and handlers stayed in the gateway; only the domain types, the
repository interfaces and both store backends moved here.

## Naming: three packages flattened into one

`chat`, `directmessage` and `moderation` were three separate Go packages in
the gateway. This module is flat (one package, `mwanachamacomm`, matching
taskmanager's `mwanachamataskmanager` convention), so names that collided
across the three are prefixed by domain:

- `chat.Thread`/`chat.Message`/`chat.Repository`/`chat.ErrNotFound` →
  `ChatThread`/`ChatMessage`/`ChatRepository`/`ErrChatNotFound`
- `directmessage.Thread`/`.Message`/`.Repository`/`.ErrNotFound`/`.ParticipantState`/
  `.StateActive` etc. → `DMThread`/`DMMessage`/`DMRepository`/`ErrDMNotFound`/
  `DMParticipantState`/`DMStateActive` etc.
- `moderation.Repository`/`.ErrNotFound` → `ModerationRepository`/
  `ErrModerationNotFound` — moderation's own vocabulary (`Report`, `Removal`,
  `Dismissal`, `Dispute`, `Actor`, `ReportReason`, `RemovalReason`,
  `DisputeState`, `ErrAlreadyDecided`, `ErrReviewerIsRemover`,
  `ErrAlreadyRemoved`) had no collisions and ported unchanged.

Postgres/memory store types follow `<Domain>PostgresStore`/`<Domain>MemoryStore`
(`ChatPostgresStore`, `DMMemoryStore`, `ModerationPostgresStore`, ...).

## Why moderation doesn't import custody

`internal/domain/moderation/act.go` used to import the gateway's
`internal/domain/custody` for `custody.ChapterActLogEntry`/`ActKind`, because
`CreateRemoval`/`DismissReports`/`DecideDispute` write the chapter's act-log
row in the **same Postgres transaction** as the moderation row. This module
cannot import a gateway-internal package (Go `internal/` visibility forbids
it even across modules), and the gateway importing this module rules out the
reverse.

The fix is dependency inversion, in `moderation_act.go`:

- `ActEntry` is this module's own narrowed copy of exactly the fields
  moderation ever populated on `custody.ChapterActLogEntry` (never
  `EscalationLevel`/`ToChapterID`, which belong to a different act this
  domain never writes).
- `ActKind` is this module's own 4-value string enum
  (`post_withheld`/`post_restored`/`removal_left_standing`/
  `report_left_standing`) — the literals **must** stay byte-identical to
  `custody.ActKind`'s, since the gateway's adapter maps between them
  explicitly by value, not by a bare string cast (see
  `internal/store/postgres/comm_act_writer.go`'s `custodyKindOf` in the
  gateway repo — it names each constant individually so
  `internal/domain/custody/act_writer_census_test.go`'s DEV-1257 census, which
  proves every act kind has a writer by walking the gateway's source for a
  `custody.ActXxx` reference, can still see that these four are written; a
  bare `custody.ActKind(e.Kind)` conversion would be invisible to it and read
  as a silently lost writer).
- `ActWriter` (postgres) takes a `*sql.Tx`, so the gateway's adapter can pass
  it straight through to its own `insertAct` and land in the same transaction
  as the moderation write. `MemoryActWriter` is the memory backend's
  equivalent (no transaction to pass through).
- The gateway wires both adapters at construction time
  (`cmd/server/stores.go`, `cmd/server/comm_act_writer.go` for memory,
  `internal/store/postgres/comm_act_writer.go` for postgres) — **always with
  the same `*sql.DB`/custody store the gateway's own custody code uses**, or
  the transaction guarantee breaks silently.

## Why schema.sql isn't a real migration

Chat/directmessage/moderation's Postgres tables were built across ~15
historical migrations in the gateway (`000005`, `000006`, `000014`, plus a
chain of ALTERs through `000045`, several interleaved with unrelated domains'
schema — `member_address`, `auth_device`). Those files are frozen history in
the gateway repo and were never copied here.

`schema.sql` is instead a **squashed, comm_-prefixed consolidation** of the
final column shape each table holds today, used only by
`postgres_scratch_test.go`'s own scratch-DB tests (mirroring taskmanager's
`postgres_integration_test.go` pattern, adapted: taskmanager generates its
schema from `mwanachama-backend-shared/postgres`'s generic DDL generator,
which doesn't apply to bespoke SQL like this module's). It deliberately
diverges from production in two ways, both noted in the file's own header:
the four moderation tables' `REFERENCES member(id)` foreign keys are dropped
(this scratch DB has no `member` table), and it includes a
`comm_test_act_log` table that exists nowhere in production, standing in for
the gateway's real `chapter_act_log_entry` so the transactional-write test can
run without depending on custody's schema.

The real production schema change is the gateway's own single forward
migration, `internal/store/postgres/migrations/000065_extract_comm_tables.up.sql`
— a pure `ALTER TABLE ... RENAME` from `chat_thread`/`dm_thread`/
`message_report`/etc. to their `comm_`-prefixed names, applied to the
gateway's live database as a normal migration, never mirrored here.

## Conventions

- Task status lives on
  [documentation/3. implementation/todo.md](documentation/3.%20implementation/todo.md)
  (this repo's actual precedent, not `dev-research.md`'s stated-but-unfollowed
  `documentation/2. design/todo.md` — see taskmanager's own board location).
- Four-phase `documentation/` layout — see
  [documentation/README.md](documentation/README.md).
- No file over 300 lines; split by responsibility (the gateway's own
  convention, [[file-length-limit]] — moderation's postgres store is already
  split into `store_postgres_moderation.go` / `store_postgres_moderation_dismissal.go`
  for this reason, mirroring the gateway's original split).
