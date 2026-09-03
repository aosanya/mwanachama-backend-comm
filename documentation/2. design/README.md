The chat/directmessage/moderation schema, ported unchanged in shape from
mwanachama-backend-api-gateway. See the repo root [CLAUDE.md](../../CLAUDE.md)
for the two design decisions specific to this extraction:

- The `ActEntry`/`ActWriter`/`MemoryActWriter` dependency inversion that lets
  moderation write the gateway's chapter act log without importing the
  gateway's custody package.
- Why `schema.sql` is a test-only squashed fixture rather than a real
  migration, and where the real production schema change
  (`000065_extract_comm_tables.up.sql`) lives.

Table-by-table datamodel notes (message-report.md, message-removal.md,
removal-dispute.md) stayed in the gateway's own
`documentation/2. design/architecture/datamodel/` — they describe the product
shape, which did not change, not the Go package boundary, which did.
