The chat/directmessage/moderation schema, ported unchanged in shape from
mwanachama-backend-api-gateway. See the repo root [CLAUDE.md](../../CLAUDE.md)
for the design decisions specific to this extraction and its later
storage/package/routes refactor (2026-09-04, onto the
`mwanachama-backend-actor` template):

- The `ActEntry`/`ActWriter`/`MemoryActWriter` dependency inversion that lets
  moderation write the gateway's chapter act log without importing the
  gateway's custody package — unaffected by the 2026-09-04 GORM move.
- Where the real production schema change
  (`000065_extract_comm_tables.up.sql`) lives. There is no `schema.sql`
  fixture here any more — `gormstore.Migrate` is this module's own schema
  source for both Postgres and the sqlite dialect its tests run against
  (see `postgres_integration_test.go` for the Postgres-only ad hoc setup a
  couple of live tests still need).

Table-by-table datamodel notes (message-report.md, message-removal.md,
removal-dispute.md) stayed in the gateway's own
`documentation/2. design/architecture/datamodel/` — they describe the product
shape, which did not change, not the Go package boundary, which did.
