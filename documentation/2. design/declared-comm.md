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
| Consumer — `mwanachama-wakala-api` mounts it | CM22 | landed 2026-10-19, under `/comm` at `module:comm`; no handset is bound and moderation gets a refusing act writer, see wakala-api's `comm-mount.md` |
| Documentation rewritten | CM23 | landed 2026-10-01 |
| `chat_thread.label` and `comm.chat_thread.create` — starting a labelled thread, see [routes.md](routes.md#starting-a-thread) | CM41 | landed 2026-10-20 |
| `chat_thread.audience`, `chat_thread.requested_by`, `UseLeading` and `comm.chat_thread.open` — a thread with the leads until it is opened, see [routes.md](routes.md#threads-with-the-leads) | CM42 | landed 2026-10-20 |
| `chat_thread.declined_at`/`declined_by`, `UseLeads`, `comm.chat_thread.request` (folded into `comm.chat_thread.create` by CM48) and `comm.chat_thread.decline`, raising `thread_requested` and `thread_declined` on subject `chat_thread` — a member asks for a thread and a lead declines it, see [routes.md](routes.md#a-thread-a-member-starts) | CM43 | landed 2026-10-20 |
| `comm.chat_thread.create` open to every member: a lead's start has no requester, anybody else's is a requested thread that tells the leads, and the request address is gone — see [routes.md](routes.md#a-thread-a-member-starts) | CM48 | landed 2026-10-21 |

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

## The civic notification events, and who raises them

`models/notification_category.go` keeps the civic vocabulary in Go until
CM26 moves the event-to-category pairing into a declaration; `civic.comm.json`
widens the blueprint's enums with the same values, and
`TestVocabularyMatchesTheBlueprint` holds the two together. What each value
means lives here rather than beside the constant.

| Event | Category | Meaning |
| --- | --- | --- |
| `survey_reached` | `survey` | "New survey reached you" |
| `survey_reminder` | `survey` | the reminder; capped one per actor per subject |
| `survey_nudge` | `survey` | capped one per structure per subject, so it names a structure |
| `results_published` | `results` | "You said, we heard" |
| `contribution_matched` | `contribution` | "Contribution matched" |
| `contribution_orphaned` | `contribution` | an orphaned contribution, told once |
| `contribution_report_refused` | `contribution` | "Report closed — not found"; the treasurer's note is a column on the report and is quoted from the subject row, never stored on the notification |
| `merchandise_recorded` | `merchandise` | "Merchandise recorded · Cap · issued by …" |
| `message_removed` | `chat` | comm's own removal receipt, which never names the moderator |
| `thread_reply` | `chat` | comm's own: a reply on a thread about a subject (CM34); subject `subject_thread`, never an author — see [subject-threads.md](subject-threads.md) |
| `thread_requested` | `chat` | comm's own: a member who does not lead the room started a thread in it (CM43, CM48); subject `chat_thread`, the starter as author, the room as `structure_id` — see [routes.md](routes.md#a-thread-a-member-starts) |
| `thread_declined` | `chat` | comm's own: a lead declined the thread the recipient asked for (CM43); subject `chat_thread`, the lead as author, the room as `structure_id` |
| `identity_case_opened`, `identity_case_closed` | `security` | a duplicate-ID case notice, and the same case ended |
| `enrollment_key_rotated` | `membership` | a rotation notice, the one notice with an author; its subject is the *retired* key, never the successor, whose identifier may not cross into a notice |
| `actor_transferred` | `membership` | |
| `consent_version_published` | `membership` | a new consent version |
| `number_changed` | `membership` | a number-on-file change |
| `device_signed_out` | `security` | a bare fact: no link, no control, no person, no place |
| `approval_requested` | `approval` | an agent-proposed write named this actor Accountable and waits on their decision (kazi's K6); subject `pending_approval`, a `mwanachama-backend-api-shared` PendingApproval id |
| `group_join_approved` | `membership` | a request to join a confirmed group was approved (CM33); subject `group` |
| `group_join_declined` | `membership` | the same request, declined (CM33); subject `group` |
| `group_invited` | `membership` | a leader invited the recipient to a group (CM44); subject `group` |
| `join_requested` | `membership` | a member asked to join, or appealed a decline, at a group the recipient leads (CM46); subject `group_join_request` |

`survey` and `security` are exempt from muting, and `security` is the only
category with an SMS channel. No event is exempt on its own: muting a
category silences every event in it (CM45).

### A decided group join (CM33, 2026-10-20)

The member app's Join Group (frontend-member MF22, MF23) tells a member in
the Inbox when a leader decides their request. `mwanachama-wakala-api`
raises the notification (its WK99) through `NotificationRepository.Raise`;
comm only carries it.

- **Two events, not one with an outcome.** `group_join_approved` and
  `group_join_declined` are separate values, because a notification has no
  stored body and the renderer picks its wording (frontend-member MF36) from
  the event alone.
- **The category is `membership`.** It is the nearest existing opt-out unit.
  Muting `membership` silences them like every other `membership` event —
  see below.
- **The subject is the group, `subject_kind: group`.** Not the request: the
  Inbox row opens that group's room, so it needs the group's id, and the
  request row is the raiser's, not something a member's client reads.
- **A silent decline is the raiser's decision, not comm's.** Member app
  decision 58 lets a leader decline without telling the member; wakala-api
  simply does not raise `group_join_declined` then. Nothing here knows the
  difference.
- `group` is not on `domain_agnostic_test.go`'s word list, so `SubjectGroup`
  needs no exemption; the two event constants live in the CM26-exempt file
  like every other civic event.

### An issued group invitation (CM44, 2026-10-21)

Member app decision 106: an invitation reaches the invitee as an Inbox row,
answered Join or Decline. `mwanachama-wakala-api` raises it (its WK122) to
the invitee when `actor.invitation` is issued; comm only carries it, as it
does the join decisions.

- **One event, `group_invited`, category `membership`.** The same opt-out
  unit as the join decisions, and like them it follows the category's mute
  (decision 120): a member who muted `membership` is not told.
- **The subject is the group, `subject_kind: group`, subject id the group.**
  Not the invitation row, which is `mwanachama-backend-actor`'s; the client
  finds the caller's open invitation to that group through the actor mount.
- Declared in `civic.comm.json` and in the Go map CM26 will retire, the way
  CM33 declared the join decisions. Pinned by
  `TestNotificationRaisesAnIssuedGroupInvitationUnderMembershipAboutTheGroup`,
  `TestNotificationRefusesAnIssuedGroupInvitationOutsideMembership` and
  `TestAMemberWhoMutedMembershipGetsNoGroupInvitation`.

### A join request told to the leaders (CM46, 2026-10-21)

Member app decision 134: each new join request or appeal puts a notice in
every leader's Inbox, under Membership, and tapping it opens that request on
Join requests. `mwanachama-wakala-api` raises it (its WK131) to each of the
group's leaders after a request or an appeal on the actor mount; comm only
carries it.

- **One event, `join_requested`, category `membership`, for a request and an
  appeal alike.** An appeal is the same request asked again, and the leader's
  next step is the same, so the renderer needs no second event to word it.
- **Mutable like the rest of Membership** (decision 120): a leader who muted
  `membership` is not told, and nothing bypasses that.
- **The subject is the request, `subject_kind: group_join_request`, subject
  id the request.** Unlike the join decisions, whose Inbox row opens the
  group, this row opens the request itself on Join requests. The name is
  `mwanachama-backend-actor`'s role for the row; comm never reads it.
- Declared in `civic.comm.json` (event and subject kind) and in the Go maps
  CM26 will retire. Pinned by
  `TestNotificationRaisesAJoinRequestUnderMembershipAboutTheRequest`,
  `TestNotificationRefusesAJoinRequestOutsideMembership` and
  `TestALeaderWhoMutedMembershipGetsNoJoinRequest`.

### Mute is mute (CM45, 2026-10-20)

Member app decision 120, superseding decision 73: a member who mutes
`membership` hears nothing from it, a leader's decision on their join
request included. Exemption is by category only: `survey` and `security`
stay exempt, and no event inside a mutable category bypasses its mute.

- **CM37 is undone.** It had named `group_join_approved` and
  `group_join_declined` in `models.NotificationEventIgnoresMute` and had
  `NotificationStore.refuseIfMuted` skip the send-time mute read for them.
  Both are gone; `refuseIfMuted` asks `IsMuted` for every event, and a muted
  member's join decision is refused with `ErrNotificationMuted`, neither
  stored, listed nor pushed.
- **No new event gets a bypass.** A declined thread, an issued group
  invitation, a join request and any later event follow their category's
  mute.
- **The preference is untouched.** `membership` can still be muted, and
  `SetPreference` does not refuse it.
- Pinned by `TestAMemberWhoMutedMembershipGetsNoJoinDecision` and
  `TestMutingMembershipSilencesEveryMembershipEvent`, through `Raise` on a
  real provisioned store.

## Whether an event can be muted is the host's to say (CM50)

`UseMutable` gives the notification store a `Mutable` function answering, for
one event, whether a member's mute may hold it back and whether the host knows
the event at all. `Raise` asks it before reading the member's preference:

- **known and unmutable** — the notification is raised whatever the member
  muted. This is how a host keeps an event its own rules fix as unmutable (a
  new form, member app decision 128) reaching people whatever an agency sets.
- **known and mutable** — the member's mute for the event's category holds,
  as before.
- **not known, or no function given** — exactly the behaviour before CM50:
  the mute holds, and the fixed `survey`/`security` exemption still applies,
  because `SetPreference` refuses to store a mute for those two at all.

A host cannot make a `survey` or `security` event mutable through this: no
muted preference for those categories can exist, so there is nothing for a
mutable answer to honour. Which events an agency may mute is the host's
notification-rules data, per mount; comm only honours the answer.
