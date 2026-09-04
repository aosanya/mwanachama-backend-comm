# Comm refactored onto the actor template

**Date:** 2026-09-04 · **Surface:** comm · **Status:** ✅ done

W1-W7's extraction (see [todo_done.md](../todo_done.md)) left comm as a flat
package over hand-rolled `pgx` SQL with a parallel memory-store backend and
no HTTP surface of its own — the shape taskmanager set. `actor` had since
moved past that same starting shape (2026-09-04, its own CLAUDE.md): GORM
storage, a `models/`+`gormstore/` package split, and its own `routes/`
package. User asked for comm to follow, fully — storage, package layout and
HTTP routes together.

## What was built

- `models/` — domain types and the three repository interfaces
  (`ChatRepository`/`DMRepository`/`ModerationRepository`), plus
  `moderation_act.go`'s `ActEntry`/`ActWriter`/`Actor` (moved here, not left
  in the root package, since the interfaces name `Actor` in their own
  signatures).
- `gormstore/` — row structs, row↔domain conversion, and `Migrate`
  (`tables.go`, `chat.go`, `dm_thread.go`, `dm_message.go`,
  `dm_device_key.go`, `moderation.go`, `moderation_dismissal.go`).
- One `ChatStore`/`DMStore`/`ModerationStore` per domain
  (`chat_impl.go`, `dm_impl.go`/`dm_roster_impl.go`/`dm_message_impl.go`,
  `moderation_impl.go`/`moderation_dismissal_impl.go`), each on a
  `*gorm.DB` — replacing the old `*PostgresStore`/`*MemoryStore` pairs.
  Runs against Postgres in production and sqlite in this repo's own tests.
- `routes/` — 19 of the gateway's 37 chat/dm/moderation HTTP routes: every
  operation that doesn't reach a gateway-internal domain package
  (`chapter`/`member`/`role`/`address`) from inside its handler body. The
  other 18 stay in the gateway; `routes/doc.go` names each and why.
- `schema.sql` and `postgres_scratch_test.go` deleted — `gormstore.Migrate`
  is now the schema source for both dialects; replaced by
  `postgres_integration_test.go` (`-tags postgres`, same Makefile target).
- `CLAUDE.md`/`README.md`/`Makefile` rewritten to record the move.

## Key decisions

- **Ids stay human-readable and prefix-numbered** (`msg-1042`) on Postgres,
  via `gormstore.mintID` reading a same-named `SEQUENCE` in a
  `BeforeCreate` hook; sqlite (no `SEQUENCE`) falls back to a
  uuid-suffixed id with the same prefix. Chosen over switching to plain
  UUIDs (actor's own choice) to keep comm's ids matching what's already in
  the gateway's live Postgres tables.
- **The `ActWriter` seam (`*sql.Tx`) is unchanged** — `moderation_impl.go`'s
  `sqlTxFrom` recovers a real `*sql.Tx` out of a `db.Transaction(func(tx
  *gorm.DB) error {...})` callback via `tx.Statement.ConnPool.(*sql.Tx)`,
  so the gateway's custody adapter never has to know GORM is involved.
- **`routes/` covers far less of comm's surface than actor's covered
  member/chapter's** (19/37 vs. actor's near-total) — chat and moderation
  lean heavily on gateway-internal domains (`requireChapterMember`,
  `wallLabel`, `nearestRoleClass`, in-body `hasCapabilityAt`), which this
  module cannot import. Confirmed against a full survey of the gateway's
  own handler files before writing any route.
- **Scope stops at this repo**, mirroring actor's own precedent: the
  gateway's `internal/store/{postgres,memory}` adapters (which construct
  the old `*PostgresStore`/`*MemoryStore` types) will not compile until a
  follow-up rewires them, and the gateway's own handlers are not repointed
  at the new `routes/` package.

## Validation

`go build ./...`, `go vet ./...` and `go test -count=1 ./...` all pass, both
with and without `-tags postgres` (Postgres-tagged tests skip cleanly
without `POSTGRES_URL` — no scratch Postgres instance was stood up this
session). `gofmt -l .` clean. Every file is under the repo's 300-line
convention (two — `dm_impl.go`, `models/moderation.go` — were split to get
there). 29 new/ported tests across the root package and `models/`, plus 6
new `routes/` handler tests using `httptest`.

## Follow-ups

- **Gateway wiring is broken** — `cmd/server/stores.go`/`comm_act_writer.go`
  construct this module's old store types and need repointing at the new
  GORM constructors (`NewChatStore`/`NewDMStore`/`NewModerationStore`).
  Blocks the gateway build until done.
- Gateway's `chat_handlers.go`/`dm_handlers.go`/`moderation_handlers.go`
  are not yet repointed at the 19 routes this module's `routes/` package
  now covers.
- W9 (Postman journey confirmation, [todo.md](../todo.md)) should be re-run
  once the gateway wiring above lands, to confirm the route surface is
  still unchanged end-to-end.
