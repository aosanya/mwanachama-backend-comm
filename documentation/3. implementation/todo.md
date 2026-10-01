# mwanachama-backend-comm (Go)

🚀 in progress · 📋 not started · ⏸️ blocked

See [todo_done.md](todo_done.md) for W1-W14 (the extraction itself), and for
CM8, CM15, CM17, CM18, CM19, CM20 and CM25.

**Rows are `CM<n>`, renumbered from `W<n>` on 2026-09-28.** Five boards
numbered with a bare `W` — this one, accounting, taskmanager, website and
digitaltwin — so a bare "W22" named three different tasks and
developer's consolidated backlog could not be read. comm moved to `CM`, the
per-repo letter every other board already uses (`AG`, `ACT`, `CAT`, `WK`,
`PM`, `WS`, `F`); accounting keeps `W`. The old numbers are unchanged, so
W15 is CM15. The remaining `W`-on-`W` overlaps are noted, not touched.

| ID | Pri | Status | Task |
|---|---|---|---|
| CM9 | P3 | 📋 | Run the `06-1-chat-threads` and `06-2-chat-moderation` Postman journeys with newman against a locally running **gateway**, as an end-to-end confirmation the route surface is unchanged. **No longer the conversion's acceptance**: CM22 was retargeted to `mwanachama-wakala-api` on 2026-09-28, so what proves the conversion is wakala-api's own route tests, and these two journeys only ever covered the gateway's own chat surface. Worth keeping as a check that the gateway's surface still works while it continues to import this module, but it gates nothing — and note the gateway's Postman suite is already red on its own account (DEV-1693/DEV-1694). |
| CM16 | P2 | 📋 | ⚠️ **A notification cannot be acknowledged, so "who has received this" is not a question comm can answer.** Found 2026-09-28 building the `party-mobilization` agency template, whose O-6 is "cascade an instruction from national to every branch within a day" and whose whole value there is the centre seeing which wards acknowledged and which went quiet. `models.Notification` records that a notification was raised; there is no per-recipient state, so an acknowledgement has to be rebuilt from chat replies or not tracked at all. Needed: an acknowledgeable notification with a per-recipient received/acted state. Two smaller gaps found in the same pass, both worth deciding alongside it rather than separately: (1) `models.ChatThread` resolves by `tag_path` but nothing creates a room when an actor group is created, and that template creates ward branches in the thousands — a room template bound to a group type would close it (small); (2) `NotificationCategory` is a closed set of eight with an exhaustive `AllCategories`, so an archetype-specific category (a mobilization instruction, a cohort milestone) has to ride on `results` — either an agency-declared category or a subtype under `results` (medium). Note the closed set is being quietly ignored elsewhere: some mined agency templates' `comm.json` invent category names that this package would refuse, which is a template-side correction as much as a code one. ~~**Gap (2) is closed by CM18** — the vocabulary is declared `enum` `values`, so a domain spec widens it with no Go change.~~ **That was wrong, corrected 2026-09-30.** Declaring the vocabulary as `enum` `values` put it in the *module*, where `spec`'s `extend` lets a domain override a field's `default` and nothing else — so the vocabulary is now closed *against* the domain rather than open to it, which is the opposite of what this row claimed. `TestADomainCannotRedeclareAFieldTheModuleOwns` pins the refusal. Gap (2) is therefore still open and is now **CM25** (blueprint side) and **CM26** (the event→category pairing). The acknowledgeable notification itself and the room template are still open. |
| CM26 | P2 | 📋 | **`notificationEventCategory` is domain knowledge in Go, and the format has nowhere to put it.** Split out of CM25 deliberately: moving the event *values* to the domain is a list of strings, but `models/notification_category.go`'s event→category map is what `Notification.Validate` derives a row's category from, and once a domain can add an event it must be able to say which category the event is muted under. That is a declared field whose value constrains another field's value, which `spec` has no concept for — so this is a format question, not a storage one, and folding it into CM25 would hide that. Until it lands, a domain that declares a new `event` value has no way to declare its category, and `Notification.Validate` will refuse the row. Options to weigh: a `pairs`/`implies` construct on a declared enum value; a second declared object the domain fills (category-per-event as rows); or moving the derivation out of comm entirely and making the caller supply both. |
| CM28 | P2 | 📋 | ⚠️ **`mwanachama-backend-api-gateway` does not compile against this module — but it did not compile before CM20 either.** Verified 2026-09-30, and the order matters: `go build ./...` in the gateway fails at *module resolution*, before type-checking, on `mwanachama-backend-actor/gormstore` and `mwanachama-backend-taskmanager/gormstore` — packages those two repos deleted in their own conversions. So the gateway has not built since then, and CM20 adds comm to a list rather than starting one. This is the same pattern shared's board records as "AGD-007 is recorded complete while two of its five consumers have not built since". **comm's own share of the breakage**, once the two resolution errors are fixed: `cmd/server/stores.go:358-359` calls `mwanachamacomm.DefaultTableNames()` then `Migrate(commDB, commTables)`, and lines 362-380 pass `commTables` to all five constructors. CM20 changed those to `New<X>Store(db, *spec.Spec, clock)` and deleted `TableNames`/`DefaultTableNames`/`Migrate`. Three carrier shapes also moved: `models.Address` no longer embeds `AddressSettings` and its `Index` is `AddressIndex`; `Address.Hash`, `AddressBlock.Hash` and `DMThread.OpenedViaAddressHash` are lowercase hex strings rather than `[]byte` (`HashHex`/`HashBytes` convert at the edge — `Resolve` and `IsBlocked` still take `[]byte`, so those call sites are untouched); and `NewAddressDirectoryStore` now returns an error. **Left broken deliberately**, on the owner's call of 2026-09-30: the gateway stopped being this conversion's acceptance when CM22 was retargeted to `mwanachama-wakala-api`, and editing it is a sibling-repo change. The fix is a `SpecFor`/`Provision` pair in place of `DefaultTableNames`/`Migrate`, the shape catalog, forms and accounting already use in that file. |
| CM29 | P2 | 📋 | ⚠️ **`mwanachama-backend-api-kazi` does not compile against this module, and likewise did not before CM20.** Verified 2026-09-30: `go build ./...` there already reported `undefined: mwanachamagit.DefaultTableNames` (stores.go:115, 247) and `undefined: mwanachamataskmanager.DefaultTableNames` (stores.go:128, 256) from git's and taskmanager's own conversions, alongside comm's. **comm's own share**: `stores.go:185-189`'s `buildNotifications` calls `DefaultTableNames()` and `Migrate`, then `NewNotificationStore(db, tables, SystemClock)`. Two things make this more than a signature change, and they are the reason it is its own row rather than part of CM28: the table is now `<instance>_<hashOf(mount)>_<hashOf(module_object)>` rather than `comm_notification`, so kazi has to know which **instance** the gateway provisioned comm under — it reaches into the gateway's own database for that single table (`GATEWAY_POSTGRES_URL`, recorded in this repo's CLAUDE.md under "First outside producer of Notification") — and `Provision` must **not** be called from kazi, since it would try to adopt and rename tables kazi does not own. kazi should call `SpecFor(instance)` and construct the store without provisioning. **Left broken deliberately**, same call as CM28. |
| CM30 | P2 | 🚀 | 🐞 **BUG — an unknown notification category is answered 404 against the *civic* vocabulary, so a domain that declared its own is refused a preference it is entitled to.** Found 2026-09-30 while stripping `models/` for CM21. `routes/notification.go`'s `SetNotificationPreference` pre-checks `IsNotificationCategory(category)` and answers **404 "no such notification category"**; that function reads `models/notification_category.go`'s Go constants. Before CM25 those constants *were* the module's declared vocabulary, so the check agreed with the store. CM25 moved the civic values into `civic.comm.json` and left the Go constants describing the civic domain — so under the `school` spec, `attendance` is a declared, storable category that this route answers 404, while `SetPreference`'s own `Check` accepts it. **Why `IsNotificationCategory` was kept rather than deleted with the other membership maps**: it decides a status code rather than a refusal (404 here, 400 from the declared check), and deleting it inside the `models/` strip would have turned one into the other silently. **The fix is in CM21**: the declared route table has the spec, so the check asks `o.Fields`'s declared `values` for the mounted domain instead of a Go slice, and `IsNotificationCategory` goes. Until then the bug is live for any non-civic instance — which today is none, since only `mwanachama` is provisioned. |
| CM31 | P2 | 📋 | 🐞 **BUG — `dmStatusFor`'s default arm is 403, so a database failure on any DM route is reported to the caller as "forbidden".** `routes/dm.go`: the switch maps `ErrDMNotFound`→404, the three conflict sentinels→409 and `ErrDMNotLastToLeave`→403, then `default: return http.StatusForbidden`. The default exists to catch the package's **unexported** `errNotAdmin`, which has no sentinel name a caller or a declaration could reference — so every *other* error, including a dropped connection or a constraint nobody anticipated, takes the same arm and arrives as 403 with the driver's message in the body. That is the opposite shape to catalog's CAT7 (a 400-ish refusal arriving as an unexplained 500) and worse in one respect: it tells the caller the request was understood and denied, when it was not understood at all. **Found transcribing the status tables for CM21**, which is where it is fixed: `errNotAdmin` becomes an exported sentinel mapped to 403 in `comm.operations.json`'s `errors`, and `dispatch`'s own fallback (500) covers the rest. That is a deliberate behaviour change on the unmapped path and is recorded as one rather than carried across — the mapped statuses are all unchanged. |
| CM21 | P2 | 📋 | **`comm.operations.json` — the route table is declared, and `routes/` becomes an adapter.** Every address across the ten route files declared with method, path, manager method, gating action, status, title, prose and arguments; the hand-written handlers and per-domain error switches collapse into one sentinel table plus `Routes(m)`/`Build(m)`/`Shape()`. Every route gains a gating action for the first time — a new contract with `mwanachama-backend-permissions`, not a refactor. Depends on CM20. |
| CM22 | P2 | 📋 | **`mwanachama-wakala-api` mounts the declared operations.** **Retargeted by the owner on 2026-09-28**, from what this row used to say — "the three consumers (`mwanachama-backend-api-gateway`, `mwanachama-backend-api-kazi`, `mwanachama-backend-api-shared`) build and test against the declared shape". The consumer for this conversion is **wakala-api**, the same retarget accounting's W21 took the same day and the shape forms' F11 landed. Build it as forms did: `cmd/server/comm.go` with a `buildComm` that reads `COMM_DATABASE_URL`/`COMM_INSTANCE`, calls `SpecFor`, `Provision` and the five store constructors, and leaves the module unmounted when either variable is unset; `internal/api/http/comm_routes.go` registering the declared routes under a `/comm` prefix, gated through `mwanachama-backend-permissions` under the scope `module:comm`. `cmd/server/main.go`, `internal/api/http/deps.go`, `router.go` and `authorize.go` are shared with the forms and accounting mounts — re-read each immediately before editing, since three sessions have them open. **The gateway is no longer this row's acceptance**, so the Postman gate (DEV-1693/DEV-1694, red) and CM9's two newman journeys are no longer what proves it; wakala-api's own route tests are. Depends on CM21. |
| CM23 | P2 | 📋 | **The documentation says what the repo now is.** CLAUDE.md, root `doc.go` and the README describe a `models/` + `gormstore/` + route-builder layout that the conversion makes untrue — the same trap agency's AG43 found. Rewrite them, add the pointer to shared's declared-domains reference and to [declared-comm.md](../2.%20design/declared-comm.md), and record CM17's answered questions rather than deleting them. Depends on CM22. |
| CM24 | P3 | 📋 | **Dropped off comm's critical path on 2026-09-28, kept as a record.** Two things moved it: CM19 landed `Provision`, which is now this module's whole provisioning story — it moves a legacy `comm_*` table set onto the declared names, renames the chapter/member columns, creates whatever is missing and applies the two notification caps — and CM22 was retargeted from the gateway to `mwanachama-wakala-api`, which provisions through `Provision` and never reads the gateway's SQL mirror. forms took the same decision for the same reason and deliberately left its own migration `000003` alone. So the gap below no longer blocks anything comm does; it matters only for a fresh provision of the **gateway's** own database, which is the gateway's to decide, and it is left here rather than deleted so the finding is not lost. Original finding: **the gateway's active migration mirror is missing four of comm's fifteen tables.** `mwanachama-backend-api-gateway/internal/store/postgres/migrations/000002_comm_tables.up.sql` creates eleven tables — the chat, DM and moderation set. `comm_notification`, `comm_notification_preference`, `comm_member_address` and `comm_member_address_block` are absent: they have only ever existed because `gormstore.Migrate`'s `AutoMigrate` created them at startup, which is not a migration and leaves no mirror. This repo's own CLAUDE.md flagged it in prose on 2026-09-09 ("in fact doesn't carry a `comm_notification`/`comm_notification_preference` table at all yet — a pre-existing gap in that migration mirror, flagged here but not fixed") and no row ever tracked it. Found again 2026-09-28 while retiring CM8. **Why it gets worse after the conversion:** `spec.Migrate` replaces `gormstore.Migrate`, so the mirror is one of only two things that still describe comm's schema, and the org-wide rule is that every GORM/declared domain carries a hand-maintained SQL mirror in the gateway's active `migrations/` precisely so `cmd/migrate up` alone provisions a fresh database. Fix: add the four tables and their indexes (including the two partial unique notification caps `syncNotificationCapIndexes` creates by raw SQL) to the active mirror, under the declared `<instance>_comm_<object>` names CM19 settles. Not done inside CM19 on purpose — it is a pre-existing hole, and folding it in would hide that the conversion inherited rather than caused it. |

## Overlapping row numbers on sibling boards

Recorded while renumbering this board, and deliberately **not** acted on —
each is another repo's to decide.

| Board | Rows | Note |
|---|---|---|
| mwanachama-backend-accounting | W12, W15–W22 | Keeps `W`. Its W16–W22 is its own declared-domain conversion, in progress 2026-09-28 |
| mwanachama-backend-taskmanager | W14–W22 | Keeps `W` |
| mwanachama-website | W11–W17 | Keeps `W` |
| mwanachama-backend-digitaltwin | W9, W13 | Keeps `W` |

---

## What the accounting pilot settled (2026-09-28)

`mwanachama-backend-accounting` ran its whole conversion set (W16–W22) on
2026-09-28 and is **done**. It was the cheapest of the three candidates and
went first deliberately. Eight findings change how the rows above should be
read — none of them are guesses, all of them cost time there.

**1. The consumer step is aimed at the wrong repo.** The row below that says
"the gateway builds and tests against the declared shape" was written when
`mwanachama-backend-api-gateway` was the only consumer. The owner's
direction on 2026-09-28 is *"we are using wakala-api on local for now, not
gateway"*. For accounting that meant the real consumer work was wiring
`mwanachama-wakala-api`, which had never imported it at all, and the
gateway became a keep-it-compiling obligation rather than the acceptance.
**Re-read that row before starting it** and decide which repo it means.

**2. Keeping every consumer compiling is a gate, not a step.** §1 of the
consolidated backlog exists because AGD-007 is recorded complete while two
of its five consumers have not built since. Accounting's pass treated
"every consumer still builds" as a precondition on finishing, not as the
last item. Do the same, and note that the gateway is *currently* broken by
the forms conversion (`mwanachamaforms.DefaultTableNames`/`Migrate` are
gone but `cmd/server/stores.go` and `internal/api/http/backend_memory_test.go`
still call them) — whoever owns that should close it.

**3. `specstore` stores an absent string as `''`, not NULL.** So a column
that is *optional but unique when present* cannot be declared `unique`:
every row without a value collides on the one constraint. The workable
shape is a plain indexed column plus a partial unique index created in
`Provision` (`create unique index ... where col <> ''`), with the friendly
refusal done in Go. Accounting needed this twice.

**4. There is no `time.Time` arm in `specstore`.** Timestamps are carried as
RFC 3339 strings. If anything orders on one, use a fixed-width
nanosecond layout (`models.TimeLayout`, as agency does) — plain
`time.RFC3339` is second-precision and will not order rows created in the
same second, and `RFC3339Nano` trims trailing zeros, which breaks
lexicographic ordering. There is no `[]byte` arm either; carry raw bytes as
a hex string.

**5. Pointer carriers now need `nullable` in the blueprint.** The
uncommitted nullable/float work in `mwanachama-backend-shared` (owned by
another session, confirmed staying) made `specstore.New`'s `disagreements`
check strict in *both* directions: a pointer field fails unless the
declared field says `nullable`, and a `nullable` field fails unless the
carrier is a pointer. This refuses carrier/spec pairs that construct fine
today, so it decides whether a store builds at all.

**6. The domain spec file is `<domain>.<module>.json`.** That is what
catalog (`agency.catalog.json`) and agency (`agency.agency.json`) actually
ship, not the `<module>.<domain>.json` several of these row titles guess.
Ship an embedded default plus `SpecFor(instance)` so a consumer needs no
file path at runtime, and a second domain under `spec/examples/` that fills
the same roles with different nouns — that second file is what actually
proves the module learned no vocabulary.

**7. Delete the hand-rolled memory fake.** Catalog has no second
implementation: it runs the real spec store on in-memory SQLite. Accounting
had a `memory.go` and a `postgres.go` implementing one interface, and its
W15 double-reversal bug existed *because* there were two copies of the same
write that could drift. One store, one write path, tested on SQLite.

**8. A bug row whose fix site the conversion rewrites should be folded in,
and its pinning test inverted rather than deleted.** Accounting closed W15
inside its store step and turned
`W15_PostDoesNotRefuseADoubleReversal` into
`TestPostRefusesASecondReversalOfTheSameEntry`. A sibling copy of the same
bug in another repo is *not* closed by that, and stays its own row.

**If the mount is per instance in wakala-api**, note the two halves:
`Shape()` should return `[]Route` (agency's signature, which the
per-instance mount loop consumes) and the routes should be built with
catalog's `Mount{Authorize, Caller}`, so every declared action arrives
gated rather than merely behind `requireCaller`.
