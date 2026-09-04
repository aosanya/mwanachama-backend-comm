.PHONY: build test test-pg vet lint clean

# Verify the module compiles cleanly.
build:
	go build ./...

# Unit tests (no external DB required) — chat/directmessage/moderation's
# business logic against the GORM stores running on an in-memory sqlite
# database, plus the pure domain-type tests.
test:
	go test -count=1 ./...

# Live tests against a real Postgres instance. Expects POSTGRES_URL, e.g.
#   POSTGRES_URL=postgres://postgres:postgres@localhost:5432/testdb?sslmode=disable make test-pg
# Runs gormstore.Migrate against a scratch set of comm_-prefixed tables and
# drops them on cleanup — see postgres_integration_test.go.
test-pg:
	go test -count=1 -tags postgres ./...

vet:
	go vet ./...

clean:
	rm -rf bin/
