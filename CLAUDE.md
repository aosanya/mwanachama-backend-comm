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

**Storage moved from hand-rolled SQL to GORM, the package split into
models/+gormstore/, and a routes/ package was added — 2026-09-04,** to bring
this repo onto the same template
[mwanachama-backend-actor](../mwanachama-backend-actor) had already moved to
for the same "extracted from the gateway" reason. This reverses three
decisions this file used to record as deliberate (flat single package
matching taskmanager; hand-rolled `pgx` SQL because "that is what chat/
directmessage/moderation already were"; HTTP routes staying in the gateway)
— each is superseded below in favour of actor's shape. **Scoped to this repo
only**, mirroring actor's own precedent: the gateway's
`internal/store/{postgres,memory}` adapters, which construct this package's
old `ChatPostgresStore`/`DMMemoryStore`/etc., will not compile against the
new constructors, and the gateway's own `chat_handlers.go`/`dm_handlers.go`/
`moderation_handlers.go` are not rewired to this package's new `routes/` —
both are explicit, not-done-here follow-ups.

- `models/` holds the domain types and the three repository interfaces
  (`ChatRepository`/`DMRepository`/`ModerationRepository`) they're read and
  written through, plus `moderation_act.go`'s `ActEntry`/`ActKind`/
  `ActWriter`/`MemoryActWriter`/`Actor` (moved here from the root package
  because `ModerationRepository`'s methods take `Actor` in their own
  signature — Go requires the interface and every type its methods name to
  share a package). `gormstore/` holds every GORM-specific piece: row
  structs, row↔domain conversion, and `Migrate`, one file per entity area,
  mirroring actor's `gormstore/`. The root package keeps `chat_impl.go`,
  `dm_impl.go`/`dm_roster_impl.go`/`dm_message_impl.go`, and
  `moderation_impl.go`/`moderation_dismissal_impl.go` — one `ChatStore`/
  `DMStore`/`ModerationStore` per domain, each built on a `*gorm.DB`,
  replacing that domain's Postgres-store-plus-memory-store pair. Each store
  runs against Postgres in production and, in this repo's own tests, an
  in-memory sqlite database (`glebarez/sqlite`, matching actor's
  `testdb_test.go`) — the fast-test role the hand-rolled memory stores used
  to play, now exercising real SQL instead of a parallel implementation
  that could (and, per this repo's own former
  `TestParticipantOrderMatchesAcrossBackendsLive`, once needed a dedicated
  test to prove didn't) drift from Postgres.
- **Ids are still human-readable and prefix-numbered** (`msg-1042`,
  `dm-77`, ...) on Postgres, minted by `gormstore.mintID`'s
  `BeforeCreate` hooks reading a same-named `SEQUENCE` `gormstore.Migrate`
  creates alongside `AutoMigrate` — the same "drop to raw SQL where
  `AutoMigrate` can't reach" pattern actor's own `Migrate` already uses for
  its partial unique indexes. sqlite (tests only, no `SEQUENCE` support)
  falls back to a uuid-suffixed id with the same prefix; every behavioural
  assertion this repo's tests make holds under either dialect, only the
  exact digits after the prefix can differ.
- **The `ActWriter`/`MemoryActWriter` seam (see "Why moderation doesn't
  import custody" below) is completely unaffected by the storage swap** —
  it predates GORM and still does the same job. What changed is how
  `CreateRemoval`/`DismissReports`/`DecideDispute` obtain the `*sql.Tx`
  `ActWriter.WriteAct` requires: `moderation_impl.go`'s `sqlTxFrom` recovers
  it from inside a `db.Transaction(func(tx *gorm.DB) error {...})` callback
  via `tx.Statement.ConnPool.(*sql.Tx)` — every SQL dialect this repo uses
  backs its GORM connection with `database/sql` underneath, so the
  assertion holds on both Postgres and sqlite.
- **`schema.sql` and `postgres_scratch_test.go` are gone.** The whole reason
  `schema.sql` existed — "comm's tables are bespoke SQL, so there's no
  generic DDL generator to lean on, unlike taskmanager's
  `mwanachama-backend-shared/postgres`-generated schema" — stops applying
  once `gormstore.Migrate` is that generator; actor never needed one either.
  The former squashed-fixture divergences (dropped `member(id)` FKs, a
  `comm_test_act_log` stand-in table) live on only as ad hoc setup inside
  `postgres_integration_test.go` (`//go:build postgres`, gated on
  `POSTGRES_URL`, mirroring actor's own file of the same name), which
  replaces `postgres_scratch_test.go`.
- **`routes/` is new — 19 of the gateway's 37 chat/dm/moderation routes**,
  the ones that are, underneath, a plain call against
  `models.ChatActivityReader`/`DMRepository`/`ModerationRepository` with no
  gateway-only policy composed into the handler body (same-domain
  composition — a DM handler checking the caller's own roster state via
  `ListParticipants` — is fine; a gateway-internal domain package
  (`chapter`/`member`/`role`/`address`) reached from the handler body is
  not, the identical line actor's own `routes/doc.go` draws). Chat
  contributes one route (`chatActivity`; the other six all call
  `requireChapterMember`, `member.Repository` + `chapter.Repository`
  in-body); DM contributes sixteen of nineteen (`createDMThread`,
  `blockThreadOrigin`, `postDMMessage` reach `address.Repository` in-body
  and stay); moderation contributes two of eleven (`listReportQueue`,
  `listReportsForMessage`; every write and every other read resolves
  `wallLabel`/`callerModerationActor`/`nearestRoleClass`
  (`chapter.Repository`/`member.Repository`/`role.Repository`) or runs an
  in-body `hasCapabilityAt` check with no external route-table wrapper to
  lean on). See `routes/doc.go` for the full, named exclusion list — it is
  considerably longer than actor's, since comm's HTTP surface leans much
  more heavily on gateway-internal domains than member/chapter's did.
  `routes/identity.go`'s `Identity` interface is the one seam this package
  needed that actor's `routes/` didn't: almost every portable DM handler
  needs the *caller's own* id from the session (accept/leave act on the
  caller; invite/kick/promote/re-enable need the caller as the admin "by";
  `publishDeviceKey` overwrites member/device fields from session) rather
  than a URL path segment, the way `{actorID}` supplies it in actor's
  routes.

## `address` and `notification` joined this module — 2026-09-06

Per the given decision (owner, 2026-08-24/2026-09-05 for `address`) and the
domain-decomposition research (`architecture-domain-decomposition.md` in
the gateway's `documentation/2. design/`), `internal/domain/address` and
`internal/domain/notification` ported into this module's `models/` +
`gormstore/`, following the exact template chat/directmessage/moderation's
2026-09-04 GORM move set: `models/address.go` (+`address_hours.go`,
`address_directory.go`) and `models/notification.go`
(+`notification_category.go`) hold the domain types; `gormstore/address.go`
and `gormstore/notification.go` hold the row structs; `address_impl.go`,
`address_directory_impl.go` and `notification_impl.go` hold the GORM
stores, at the repo root alongside `chat_impl.go` etc.

- **Same domain-prefixing convention chat/directmessage/moderation set,
  applied by design rather than only where a literal collision forced it**
  — `Address`/`Settings`/`Hours` etc. had no bare-name collision to force a
  rename the way `chat.Thread`/`directmessage.Thread` did, but this
  package's own `ChatActivityQuery` et al. already prefix fresh names on
  principle (five domains, one flat `models/` namespace), so address's and
  notification's exported names are `Address*`/`Notification*` throughout:
  `ErrAddressNotFound`, `AddressSettings`, `AddressRepository`,
  `ErrNotificationInvalid`, `NotificationCategory`, and so on.
- **Fifteen tables now, not eleven.** `gormstore.DefaultTableNames` gained
  `Addresses`/`AddressBlocks` (`comm_member_address`/
  `comm_member_address_block`) and `Notifications`/
  `NotificationPreferences` (`comm_notification`/
  `comm_notification_preference`). `AddressRow` mints no id — its primary
  key is the hash itself, exactly as the gateway's original
  `member_address` table had it — so it needs no entry in `seqNames`.
  Notification's two G63/G278 caps (one reminder per member per subject,
  one nudge per chapter per subject) are partial unique indexes
  `gormstore.syncNotificationCapIndexes` creates by raw SQL after
  `AutoMigrate`, mirroring actor's `syncUniqueAttributeIndexes` for the
  identical reason — partial-index syntax is identical on Postgres and
  sqlite, so one statement per index covers both dialects unlike actor's
  own dialect-branching version.
- **`AddressDirectoryStore` joins a table this module does not own, by
  name, not by import.** A directory listing is an address's plaintext and
  its owner's display name, and the display name lives in
  `mwanachama-backend-actor`'s `member_actors` table — a fact this module
  reaches with a plain SQL `JOIN member_actors m ON m.id = a.member_id`
  (`address_directory_impl.go`), never a Go import of that module's types.
  This is the same fact this repo's own `migrations/000002_comm_tables.up.sql`
  already leans on for moderation's `reported_by`/`removed_by`/etc. FKs —
  every domain the gateway composes shares one physical Postgres database,
  and reaching a table by name is not a module dependency. The join table
  name is a constructor parameter (`membersTable`), defaulting to
  `DefaultMembersTable = "member_actors"`, so this package's own tests can
  point it at a scratch stand-in instead (`testdb_test.go`'s
  `createTestMembers`, mirroring `createTestActLog`'s reasoning for the
  custody-log stand-in).
- **ILIKE and `COLLATE "C"` do not exist on sqlite**, so the directory
  search uses `LOWER(...) LIKE LOWER(...)` for case-insensitive matching
  (works on both dialects) and appends `COLLATE "C"` to the `ORDER BY`
  only when `db.Dialector.Name() == "postgres"` — sqlite's own default TEXT
  collation is already byte-order, so no dialect branch is needed there.
- **Routes, per this package's own portability rule (`routes/doc.go`):**
  two of address's seven (`AddressRoutes` — `ListMyAddresses`,
  `RetireAddress`) and all five of notification's (`NotificationRoutes`).
  The other five address routes reach `phonesalt`/`orgpolicy` in-body and
  stay in the gateway, same as chat/DM/moderation's exclusions — see
  `routes/doc.go` for the full reasoning, including a flagged (but
  deliberately not acted on) finding that DM's `blockThreadOrigin` would
  now also qualify under this package's own rule, since address moving
  here made its one remaining dependency (`AddressRepository`) intra-module.
- **The gateway's own HTTP cutover for chat/DM/moderation/address/
  notification is tracked as one board**,
  `mwanachama-backend-api-gateway/documentation/3. implementation/todo_comm_absorb.md`
  (DEV-1664…1669).
- **Two DM route bugs surfaced and were fixed only once the 2026-09-04
  chat/DM/moderation port was actually wired into a live gateway for the
  first time, 2026-09-06** — neither had a test inside this repo to catch
  it, because this repo's own fixtures call each `routes.Xxx` handler
  directly rather than through a real caller who could be an outsider or a
  forger:
  1. `GetDMThread`/`ListDMParticipants` (`dm.go`) and
     `ListDMMessages`/`ListDMReactions`/`SetDMReaction`/`ClearDMReaction`
     (`dm_message.go`) answered a caller who is not on a thread's roster
     with `403 "not a participant"` — the gateway's own pre-port handlers
     answered `404` with `models.ErrDMNotFound`'s exact message, on purpose
     (DEV-1137: a thread that exists but the caller is not on and a thread
     that was never minted must be indistinguishable, or the status code
     itself becomes an enumeration oracle). Fixed to answer 404 uniformly;
     see `TestGetDMThreadRefusesAnOutsiderWithNotFound`.
  2. `PublishDMDeviceKey` decoded into a narrow `publishDeviceKeyBody{KeyID,
     PublicKey}` struct, so a body naming a claimed `published_by`/
     `device_id` (DEV-1265's own adversarial case — those two are session
     facts a client's opinion must be silently overwritten, not honoured)
     was refused outright as an unknown field under this package's own
     `readJSON`'s `DisallowUnknownFields`. Fixed by decoding straight into
     `models.DMDeviceKey` (which has fields for both) and overwriting them
     after, matching the gateway's original handler exactly; see
     `TestPublishDMDeviceKeyIgnoresClaimedProvenance`.

  Neither bug is exotic; both are the generic shape "a port's own fixtures
  called the handler function directly, so a discrepancy visible only to an
  actual unauthorized caller or an actual forged field went unexercised
  until a mounting process's own integration tests ran against it." Worth
  remembering for the next repo's `routes/` port: writing at least one test
  through an actual HTTP boundary with an adversarial caller, not just a
  direct handler call with a cooperative one, would have caught both here.

## Naming: three packages flattened into one

`chat`, `directmessage` and `moderation` were three separate Go packages in
the gateway. This module is flat (one package, `mwanachamacomm`, matching
taskmanager's `mwanachamataskmanager` convention) for the root package's
implementation files, and `models/` follows the same flat-naming rule for
the domain types and interfaces it now holds — names that collided across
the three are prefixed by domain:

- `chat.Thread`/`chat.Message`/`chat.Repository`/`chat.ErrNotFound` →
  `models.ChatThread`/`ChatMessage`/`ChatRepository`/`ErrChatNotFound`
- `directmessage.Thread`/`.Message`/`.Repository`/`.ErrNotFound`/`.ParticipantState`/
  `.StateActive` etc. → `models.DMThread`/`DMMessage`/`DMRepository`/`ErrDMNotFound`/
  `DMParticipantState`/`DMStateActive` etc.
- `moderation.Repository`/`.ErrNotFound` → `models.ModerationRepository`/
  `ErrModerationNotFound` — moderation's own vocabulary (`Report`, `Removal`,
  `Dismissal`, `Dispute`, `Actor`, `ReportReason`, `RemovalReason`,
  `DisputeState`, `ErrAlreadyDecided`, `ErrReviewerIsRemover`,
  `ErrAlreadyRemoved`) had no collisions and ports unchanged.

Storage types follow `<Domain>Store` (`ChatStore`, `DMStore`,
`ModerationStore`) — one GORM-backed implementation per domain, replacing
the old `<Domain>PostgresStore`/`<Domain>MemoryStore` pair.

## Why moderation doesn't import custody

`internal/domain/moderation/act.go` used to import the gateway's
`internal/domain/custody` for `custody.ChapterActLogEntry`/`ActKind`, because
`CreateRemoval`/`DismissReports`/`DecideDispute` write the chapter's act-log
row in the **same Postgres transaction** as the moderation row. This module
cannot import a gateway-internal package (Go `internal/` visibility forbids
it even across modules, and the gateway importing this module rules out the
reverse). Unaffected by the 2026-09-04 GORM move above — this dependency
inversion predates it and still does the same job, just from `models/`
instead of the old root-package `moderation_act.go`.

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
  equivalent (no transaction to pass through) — kept even though this
  repo's own stores no longer use a separate memory backend, since the
  gateway's own in-memory wiring (`cmd/server/comm_act_writer.go`) still
  needs it. On this repo's side, `moderation_impl.go`'s `sqlTxFrom` is what
  hands `ActWriter` a real `*sql.Tx` out of a GORM transaction — see above.
- The gateway wires both adapters at construction time
  (`cmd/server/stores.go`, `cmd/server/comm_act_writer.go` for memory,
  `internal/store/postgres/comm_act_writer.go` for postgres) — **always with
  the same `*sql.DB`/custody store the gateway's own custody code uses**, or
  the transaction guarantee breaks silently.

## Conventions

- Task status lives on
  [documentation/3. implementation/todo.md](documentation/3.%20implementation/todo.md)
  (this repo's actual precedent, not `dev-research.md`'s stated-but-unfollowed
  `documentation/2. design/todo.md` — see taskmanager's own board location).
- Four-phase `documentation/` layout — see
  [documentation/README.md](documentation/README.md).
- No file over 300 lines; split by responsibility (the gateway's own
  convention, [[file-length-limit]] — moderation's dismissal write is
  already split into `moderation_impl.go`/`moderation_dismissal_impl.go`
  for this reason, and DM's roster writes into their own
  `dm_roster_impl.go` alongside `dm_impl.go`/`dm_message_impl.go`, a
  three-way rather than the original module's two-way split, since one
  GORM store now does the job two backend-specific implementations used to
  split across).
