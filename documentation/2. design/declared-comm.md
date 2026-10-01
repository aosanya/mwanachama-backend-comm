# Converting comm to a declared domain

The scope decision CM17 asked for, and what the conversion found. The format
itself is described once, in
[declared-domains.md](../../../mwanachama-backend-shared/documentation/2.%20design/declared-domains.md)
— this page records only what is true of **comm**.

comm is the third repo converted, after `mwanachama-backend-catalog` (which
proved the shape) and `mwanachama-backend-agency` (AGD-007). It is the
largest of the three candidates that were open: fifteen stored objects
across six domains, ten route files, and three consumers rather than one.

## What has landed, as of 2026-10-01

The conversion is done. CM18–CM21 and CM25 all landed; what is left open is
listed at the bottom of this page and on the board.

| Step | Row | State |
| --- | --- | --- |
| Blueprint — `comm.blueprint.json`, fifteen objects | CM18 | landed |
| Domain spec — `civic.comm.json` | CM18 | landed |
| Second domain spec — `spec/examples/school.comm.json` | CM20 | landed 2026-09-30 |
| `Provision`, and the legacy table/column adoption | CM19 | landed |
| Spec-driven store; `gormstore/` and `tables.go` deleted | CM20 | landed 2026-09-30 |
| The civic vocabulary moves into the domain specs | CM25 | landed 2026-09-30 |
| Declared rules — `validate.go`, `patterns.go`, `Check` | CM21 | landed 2026-10-01 |
| Declared operations; `routes/` becomes an adapter | CM21 | landed 2026-10-01 |
| `cmd/ddl` and the six guard tests | CM21 | landed 2026-10-01 |
| Consumer — `mwanachama-wakala-api` mounts it | CM22 | **not started** |
| Documentation rewritten | CM23 | landed 2026-10-01 |

Net across CM20 and CM21: about 2,900 lines deleted against 2,900 written,
and the deletions are the acceptance — `gormstore/`'s nine files, `tables.go`,
ten route files and `routes/wire.go` all went, and the tests still pass, so
the declarations carry what the Go did.

Four shared changes came out of it, because a conversion that leaves generic
code in the module has not finished: `specstore` learned `time.Time` and
started refusing a carrier it cannot read (S38), `spec` let a domain widen a
declared enum (S37) and allowed a `bytes` primary key (S46), and `dispatch`
gained a second session fact (S45).

## The decision

Taken 2026-09-28 by the owner, against the three questions CM17 put.

**Scope: complete, not data-layer only.** CM18–CM23 in one pass — blueprint,
domain spec, spec-driven store, declared operations, consumers, docs. The
alternative considered was stopping after the store (CM20) and leaving
`routes/` hand-written, which would have kept the three consumers still.

**All six domains at once**, not one at a time. chat, DM, moderation,
notification, address and address-directory share one physical database, one
`Provision` and one operations table; converting one domain at a time would
mean the repo holding a `gormstore/` and a spec store side by side for as
long as it took, which is the state agency's AG39 specifically avoided.

**Table naming: one move, not two.** See below — this is the question that
had the most wrong answers available.

**The consumer is `mwanachama-wakala-api`.** Added to this record later the
same day, because it changes what "the conversion is done" means. CM22 was
written against the three repos that import this module today — the gateway,
api-kazi and api-shared — and the owner retargeted it to wakala-api, which is
where `mwanachama-backend-catalog`, `mwanachama-backend-forms` and
`mwanachama-backend-accounting` are all mounted. So the acceptance is
wakala-api's own route tests, not the gateway's Postman gate, and the mount
follows catalog's shape: a `cmd/server/comm.go` that builds the manager and
returns nil when its environment is unset, and an
`internal/api/http/comm_routes.go` that registers the declared routes under a
prefix, gated through `mwanachama-backend-permissions`.

Two consequences worth stating plainly. The gateway keeps importing this
module and is not part of the conversion's acceptance any more, which means
nothing in this set proves the gateway still works — that is what is left of
CM9. And the gateway's SQL migration mirror stops being comm's provisioning
story, because `Provision` is: that is why CM24 dropped to P3 rather than
being done, and forms made the same call about its own migration `000003`.

## CM8 was wasted work, and why

CM8 asked for `000065_extract_comm_tables` to be applied to a real database
as its own deploy step. It was retired rather than done, because the
migration it names **is not in the active migration set**. It is a pure
`ALTER TABLE … RENAME` from the gateway's original bare names
(`chat_thread`, `dm_thread`, `message_report`) to the `comm_`-prefixed ones,
and it now lives only under `dump/`, in
`mwanachama-backend-api-gateway/internal/store/postgres/migrations_archive/`.

The gateway's active mirror — `migrations/000002_comm_tables.up.sql`, one of
the seven files `cmd/migrate up` runs — creates the `comm_`-prefixed tables
directly. A fresh database therefore arrives already named the way the Go
code expects, and there is nothing for a rename to do. The DEV-256 re-lay is
what made this true; CM8 simply outlived it.

So the declared rename in CM19 is the **only** table move comm makes:
`comm_chat_thread` becomes `<instance>_<hashOf(mount)>_<hashOf(comm_chat_thread)>`
— shared's S29/S30 shape, where only the instance segment stays readable and
the per-instance `<instance>_spec_table_names` registry records which raw name
each hash belongs to. Applying CM8 first would have moved the same tables twice
for no gain, which is what CM17 was written to prevent.

### The gap found while retiring it

That same active mirror carries **eleven of comm's fifteen tables**.
`comm_notification`, `comm_notification_preference`, `comm_member_address`
and `comm_member_address_block` are absent from it, so `cmd/migrate up`
alone cannot provision a fresh database for this module — the four
address/notification tables only ever existed because `gormstore.Migrate`
created them at startup. comm's own CLAUDE.md flagged this in prose in
September and no row tracked it; it is now **CM24**.

This matters more after the conversion than before it: `spec.Migrate`
replaces `gormstore.Migrate` as what creates comm's tables, so the mirror
and the spec are the only two things that describe comm's schema, and a
mirror missing four tables is the difference between a provisioned database
and a broken one.

## What the format could not carry, and what happened instead

Written as a plan, and two of its three rows turned out to be wrong, which is
worth keeping rather than tidying away: the cost of a conversion is not
knowable from reading the engine, only from running it.

`specstore` joins a declared field to a Go field **by name**, and encodes it
by the declared type. Three carrier shapes were expected not to fit:

| Was expected to move | Why | What actually happened |
| --- | --- | --- |
| `time.Time` on every object, ~23 fields | `cell` had no arm for a struct | **Stayed `time.Time`.** `specstore` learned the type instead (shared's S38), which also uncovered a silent-corruption bug: `cell` ended in `reflect.Value.String()`, which answers `"<time.Time Value>"` rather than panicking, so an unreadable carrier was written as that placeholder text |
| `[]byte` — `Address.Hash` (a primary key), `AddressBlock.Hash`, `DMThread.OpenedViaAddressHash` | `Decode`'s slice arm unmarshals JSON, which a raw hash is not | **Stayed `[]byte`.** Hex text was tried first and was wrong twice over — lossy for key material, and a `text` column where the live data is `bytea`, needing every row converted on adoption. `spec.TypeBytes` landed the same morning and is exactly the live type |
| `map[string]string` — `DMMessage.PerRecipientKeys` | same arm | **Unchanged.** It round-trips through the json arm as it stood, as does `SentFromAddressSealed` |

So the only carrier changes comm actually made were `models.Address` giving
up its embedded `AddressSettings` for eight flat fields behind
`Settings()`/`WithSettings()`, `Address.Index` becoming `AddressIndex` to
match its column, and `DMThread`'s two derived fields taking a `spec:"-"`
tag rather than being removed. The public API moved far less than the row
predicted.

The pointer fields comm is full of — `MessageTTLSeconds`, `SentFromAddressIndex`,
`ReadAt`, `RetiredAt`, `ExpiresAt`, `ListedAt`, `DisabledAt`, `DecidedAt`,
`Note` — stay pointers, and are declared `nullable`. That flag
and `specstore`'s pointer support arrived in shared on 2026-09-28 for
`mwanachama-backend-forms`' own conversion; comm is its second consumer. The
agreement is enforced in both directions at construction: a pointer carrier
whose field is not declared nullable, and a nullable field carried by a
plain value, both refuse to build rather than silently reading a stored NULL
back as a zero.

**`nil` and the zero value are genuinely different here**, which is why
flattening the pointers away was not an option: `SentFromAddressIndex` is an
index into a actor's own addresses, and `0` is a real address, not an absent
one. A conversion that dropped the distinction would have quietly rewritten
which address every affected thread was sent from.

## Columns the rename also has to fix

comm's domain types say `StructureID` and `ActorID`; its row structs said
`ChapterID` and `MemberID`, and the conversion from one to the other lived
in `gormstore`'s `…ToRow`/`…FromRow` pairs. With those pairs deleted there
is nothing left to do the translating, and the declared column name is the
Go field's name — so the physical columns are renamed with the tables:

- `chapter_id` → `structure_id` (chat threads and messages, reports,
  removals, dismissals)
- `member_id` → `actor_id` (DM participants, reactions, device keys,
  addresses, address blocks, notifications, preferences)
- `review_chapter_id` → `review_structure_id`, `seat_chapter_id` →
  `seat_structure_id`, `author_member_id` → `author_actor_id`

`address_index` was listed here as becoming `index` and does **not**: the
declared column is `address_index`, and the Go field moved to `AddressIndex`
to match it. `index` would have been a poor choice anyway — it is a reserved
word in SQLite.

This is not cosmetic. Left alone, `specstore.New` would refuse to build
comm's manager at all, naming each column the spec declares that no carrier
holds — which is the check doing its job.

## The exclusion list is gone — CM21

Landed 2026-10-01, and the detail now lives in
[routes.md](routes.md) beside the rest of the route table.

`routes/doc.go` carried a long, named list of which of the gateway's
37 chat/DM/moderation/address/notification routes this module's `routes/`
could portably serve, and which reached a gateway-internal domain package
(`chapter`, `member`, `role`, `address`, `phonesalt`, `orgpolicy`) from the
handler body and had to stay behind. Nineteen of 37 qualified.

`comm.operations.json` declares the whole surface, and the split that list
encoded is now `AnonymousActions` plus the mount's own authorizer: every
declared operation carries an `action`, so an operation nobody names
arrives gated. A route that needs a gateway-internal fact still composes it
in the gateway — but it composes it around a declared operation rather than
instead of one.

## What stays in Go

Everything a spec cannot state, beside the type it is about:

- the DM roster rules — self-kick, the last admin, and the re-enable
  exclusivity CM15 was filed against
- `Notification.Validate`'s derived category, its seat-address pairing, and
  the nudge's scope requirement
- `Removal.Room()` and `Dismissal.Reporter()`, the two projections that
  enforce G47's anonymity by omission
- `AddressHours.OpenAt`, which errs open
- the two notification caps, which are partial unique indexes rather than
  counters

The notification vocabulary is the one piece that moved *into* the
declaration. `category`, `event` and `subject_kind` are declared `enum`
fields with their `values`, and `TestVocabularyMatchesTheBlueprint` holds
`models/notification_category.go`'s constants and the blueprint to each
other in both directions, the way catalog's own test does — a stored value
outlives a rename, which makes a drifted constant a data bug rather than a
compile error.

## The vocabulary is declared in the wrong place — corrected 2026-09-30

This page and CM16 both claimed that declaring the vocabulary as `enum`
`values` made it domain-extensible: *"an archetype-specific category is now
a value set a domain spec can widen, not a Go change."* **That is not
true**, and was not true when it was written.

`spec`'s `extend` lets a domain override exactly one thing on a field the
module declares — its `default`. `values`, `type`, `description` and every
rule are the module's, and a domain naming any of them is refused at load
by name. `TestADomainCannotRedeclareAFieldTheModuleOwns` pins the refusal.

So the vocabulary is not merely undeclared-by-the-domain; it is **closed
against the domain**, and the words in it are mostly not comm's:

- `category` declares `survey`, `contribution`, `merchandise` — three
  civic-programme words a school, a clinic or a union has no use for.
- `event` declares `survey_reached`, `survey_reminder`, `survey_nudge`,
  `results_published`, `contribution_matched`, `contribution_orphaned`,
  `contribution_report_refused`, `merchandise_recorded`,
  `enrollment_key_rotated`.
- `subject_kind` declares `survey`, `contribution`, `contribution_report`,
  `handout`, `enrollment_key`.

comm itself raises exactly **one** of those events. `message_removed` is
the removal receipt its own moderation writes; every other value in the set
is written by a different module through `NotificationRepository.Raise` —
survey and results by the gateway's own programme code, `approval_requested`
by `mwanachama-backend-api-kazi`. comm is the carrier, and the vocabulary
belongs to whoever raises, not to whoever stores.

That makes this the choice comm's conversion turns on, the same way
`AgencyID`/`DraftID` is the one `mwanachama-backend-insights` turns on. The
owner's direction on 2026-09-30 is to **move the civic words out of the
module**: the blueprint keeps only what comm's own acts produce, and each
domain declares the rest.

Two things follow that are not just a list of strings moving:

1. **A domain has to be able to declare `values` at all**, which needs a
   change in `mwanachama-backend-shared`'s `extend` — widening the
   module's declared set rather than replacing it, so a domain that forgets
   `message_removed` does not break comm's own removal receipt.
2. **`notificationEventCategory` is the harder half.** The event-to-category
   map in `models/notification_category.go` is what `Notification.Validate`
   derives a row's category from, and it is domain knowledge in Go. If the
   events move to the domain, the pairing has to move with them — a declared
   field whose value constrains another field's value, which the format has
   no concept for today.

Neither is done. The scope of (2) is tracked as its own row rather than
folded into CM20, because it is a format question, not a storage one.
