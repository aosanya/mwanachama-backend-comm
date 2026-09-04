# mwanachama-backend-comm — completed work

| Task | Title | Completed | Notes |
|---|---|---|---|
| W1 | Scaffold the module | 2026-09-03 | Flat root package `mwanachamacomm` (module-suffix convention, matching `mwanachamataskmanager`), `go.mod`/`.gitignore`/`Makefile`/`README.md`/`CLAUDE.md`, four-phase `documentation/`. |
| W2 | Port chat/directmessage/moderation domain types | 2026-09-03 | `chat.go`, `chat_activity.go`, `directmessage.go`, `moderation.go`, `moderation_repository.go` — unchanged in shape; collision-prone names (`Thread`, `Message`, `Repository`, `ErrNotFound`) prefixed by domain (`Chat*`/`DM*`/`Moderation*`/`ErrModerationNotFound`) once flattened into one package. See CLAUDE.md. |
| W3 | Design and port the custody dependency inversion | 2026-09-03 | `moderation_act.go`'s `ActEntry`/`ActKind`/`ActWriter`/`MemoryActWriter`, replacing the direct `internal/domain/custody` import moderation's act.go could no longer make. Validated against the gateway's `internal/domain/custody/act_writer_census_test.go` (DEV-1257) via `custodyKindOf`'s explicit per-kind mapping in the gateway's `comm_act_writer.go`. |
| W4 | Port both store backends | 2026-09-03 | `store_postgres_*.go` (chat/dm/moderation, `comm_`-prefixed table names) and `store_memory_*.go`, plus their ported unit tests — all passing. |
| W5 | Build the postgres scratch-test schema and live suite | 2026-09-03 | `schema.sql` (squashed, comm_-prefixed, two deliberate divergences from production — see CLAUDE.md) and `postgres_scratch_test.go`, run successfully against a real Postgres instance during development (idempotent mint, DM lifecycle, moderation act-log-in-same-transaction including rollback-on-failure, cross-backend participant-order agreement). |
| W6 | Rewire the gateway to import this module | 2026-09-03 | `cmd/server/stores.go`/`comm_act_writer.go`, `internal/api/http/deps.go`/`errors.go`/`member_visibility.go`/`dm_eligibility.go` and all chat/dm/moderation handler files repointed at this module; old `internal/domain/{chat,directmessage,moderation}` and their store implementations deleted. Every HTTP route path is byte-identical to before the move. Full gateway test suite (memory backend) green. |
| W7 | Write the production rename migration | 2026-09-03 | `internal/store/postgres/migrations/000065_extract_comm_tables.{up,down}.sql` — pure `ALTER TABLE/SEQUENCE ... RENAME`, no data touched. Not yet applied to any real deployment (see todo.md). |
| W11 | Refactor onto the `mwanachama-backend-actor` template | 2026-09-04 | Storage moved from hand-rolled `pgx` SQL + a parallel memory store to one GORM-backed store per domain (`models/`+`gormstore/` split); a new `routes/` package covers the 19 of 37 gateway chat/dm/moderation routes that carry no gateway-internal policy. `schema.sql`/`postgres_scratch_test.go` retired in favor of `gormstore.Migrate` + `postgres_integration_test.go`. Supersedes W4/W5's store backends and scratch schema. See [the session write-up](coding_sessions/2026-09-04_actor-template-refactor.md) and CLAUDE.md. Gateway wiring left broken — see todo.md W12/W13. |

## Archived board context

### Why this repo exists

Extracted from mwanachama-backend-api-gateway's chat, directmessage and
moderation domains at the user's request, following the in-process-library
architecture mwanachama-backend-taskmanager set (not a network service — see
CLAUDE.md and the gateway's README.md). Scope (chat+directmessage+moderation,
not notification) and the `comm_` table-prefix convention were both decided
with the user before implementation began.
