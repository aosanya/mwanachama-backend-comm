Test coverage for mwanachama-backend-comm:

- Pure domain-type unit tests: `chat_test.go`, `directmessage_test.go`,
  `moderation_test.go`.
- Store unit tests against the memory backend: `store_memory_*_test.go`.
- SQL-shape tests with no database: `chat_sql_test.go`.
- Live tests against a real Postgres instance (`-tags postgres`,
  `POSTGRES_URL` required): `postgres_scratch_test.go`.

The gateway's own route-level HTTP tests (`internal/api/http/*_test.go` in
mwanachama-backend-api-gateway — chat/dm/moderation's 15 test files) are what
prove the extraction didn't change behavior at the HTTP boundary; they stayed
in the gateway since the routes did.
