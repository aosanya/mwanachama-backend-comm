# Converting comm to a declared domain

The scope decision CM17 asked for, and what the conversion found. The format
itself is described once, in
[declared-domains.md](../../../mwanachama-backend-shared/documentation/2.%20design/declared-domains.md)
— this page records only what is true of **comm**.

comm is the third repo converted, after `mwanachama-backend-catalog` (which
proved the shape) and `mwanachama-backend-agency` (AGD-007). It is the
largest of the three candidates that were open: fourteen stored objects
across six domains, ten route files, and three consumers rather than one.

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
`comm_chat_thread` becomes `<instance>_comm_chat_thread`. Applying CM8 first
would have moved the same tables twice for no gain, which is what CM17 was
written to prevent.

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

## What the format could not carry, and what was done about it

`specstore` joins a declared field to a Go field **by name**, and encodes it
by the declared type. Three of comm's carrier shapes had no arm:

| Was | Why it did not fit | Now |
| --- | --- | --- |
| `time.Time` on every object | `assign` has no arm for a struct; catalog carries timestamps as text | RFC3339 `string`, and `*string` where the old field was `*time.Time` |
| `[]byte` — `Address.Hash` (a primary key), `AddressBlock.Hash`, `DMThread.OpenedViaAddressHash` | `Decode`'s slice arm unmarshals JSON, which a raw hash is not | lowercase hex `string`; `HashBytes`/`SetHashBytes` convert at the edge |
| `map[string]string` — `DMMessage.PerRecipientKeys` | same arm | a `json` column carried as a marshalled `string` |

The pointer fields comm is full of — `MessageTTLSeconds`, `SentFromAddressIndex`,
`ReadAt`, `RetiredAt`, `ExpiresAt`, `ListedAt`, `DisabledAt`, `DecidedAt`,
`Note` — are carried as pointers still, and declared `nullable`. That flag
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
- `address_index` → `index`

This is not cosmetic. Left alone, `specstore.New` would refuse to build
comm's manager at all, naming each column the spec declares that no carrier
holds — which is the check doing its job.

## The exclusion list is gone

`routes/doc.go` used to carry a long, named list of which of the gateway's
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

That is also the answer to CM16's third question. An archetype-specific
category is now a value set a domain spec can widen, not a Go change.
