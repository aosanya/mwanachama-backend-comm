# mwanachama-backend-comm

Chat, direct and group messaging, message moderation, addresses and notifications for whatever mounts it, extracted from the retired `mwanachama-backend-api-gateway`'s `internal/domain/{chat,directmessage,moderation}`.

A Go library, imported directly by the host (`mwanachama-wakala-api`); there is no service to run and no gRPC. Its objects are declared in [`comm.blueprint.json`](comm.blueprint.json) and its route table in [`comm.operations.json`](comm.operations.json); storage goes through `mwanachama-backend-shared/specstore` on Postgres in production and sqlite in this module's own tests. There are no row structs and no hand-written handlers.

```go
s, _ := mwanachamacomm.SpecFor("app")
_ = mwanachamacomm.Provision(db, s)
cm, _ := mwanachamacomm.NewCommManager(db, s, clock, actWriter, actorsTable)
```

`SpecFor(instance)` returns the default civic domain ([`civic.comm.json`](civic.comm.json)) for a host's instance; `spec/examples/school.comm.json` fills the same roles with a school's nouns. `routes/` builds the REST routes and MCP tools from the operations file through `mwanachama-backend-shared/dispatch`. `models/` holds the domain types and the rules a declaration cannot state.

```sh
make test      # sqlite, no database needed
make test-pg   # needs POSTGRES_URL
```

See [documentation/](documentation/) for design and the task board, and [CLAUDE.md](CLAUDE.md) for the invariants.
