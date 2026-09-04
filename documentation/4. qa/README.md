Test coverage for mwanachama-backend-comm:

- Pure domain-type unit tests: `models/chat_domain_test.go`,
  `models/directmessage_domain_test.go`, `models/moderation_domain_test.go`.
- Store unit tests against the GORM stores, run on an in-memory sqlite
  database (`testdb_test.go`'s `newTestDB`): `chat_impl_test.go`,
  `dm_impl_test.go`, `moderation_impl_test.go`. Replaces the pre-2026-09-04
  `store_memory_*_test.go` files, which tested a separate hand-rolled memory
  backend that no longer exists.
- `routes/` handler tests, using `net/http/httptest` against the same GORM
  stores: `routes/routes_test.go`.
- Live tests against a real Postgres instance (`-tags postgres`,
  `POSTGRES_URL` required): `postgres_integration_test.go`. Replaces the
  pre-2026-09-04 `postgres_scratch_test.go`/`schema.sql` pair —
  `gormstore.Migrate` is now the schema source for both dialects, so there
  is no separate fixture to apply.

The gateway's own route-level HTTP tests (`internal/api/http/*_test.go` in
mwanachama-backend-api-gateway — chat/dm/moderation's 15 test files) still
cover the 18 of 37 routes that stayed in the gateway (see
`routes/doc.go`'s scope note) — those, and the routes the gateway has not
yet repointed at this module's `routes/` package even where it could (todo.md
W13), are unaffected by anything in `routes/routes_test.go`.
