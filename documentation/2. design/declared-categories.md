# A domain-declared notification category (CM60)

Asked for by the municipality template (member app decision 283): a council
consultation should be its own category, so a resident can mute it apart from
`survey` and `results`.

## What a domain spec can already do

A domain widens `notification.category` and `notification_preference.category`
by listing the new value in its own spec (shared's S37), and `Check` accepts it.

- **Muting works.** `SetPreference` checks the category against the spec, then
  applies the Go exemption rule (`survey` and `security` cannot be muted). A
  widened category is not exempt, so a member can mute it and `IsMuted` reads
  it back. A spec that does not list it refuses the preference with
  `ErrNotificationInvalid`.
- **The declared check on a raise works.** `Check` accepts a notification in
  the widened category and the narrower spec refuses it.

Pinned by `notification_widened_category_test.go`.

## What blocks it: CM26

`Raise` also calls `Notification.Validate`, which looks the event up in
`notificationEventCategory`, a Go map in `models/notification_category.go`,
and refuses any notification whose category is not the one that map pairs
with its event. No event pairs with a category the domain declared, so a
`consultation` notification cannot be raised whatever the spec says.
`TestAWidenedCategoryCannotYetBeRaisedBecauseNoEventPairsWithIt` pins this,
and fails once an event pairs with `consultation`.

This is CM26 (the event-to-category pairing and the muting exemption are civic
knowledge in Go). Until the pairing is declared by the domain, or the host
supplies it the way `UseMutable` supplies mutability, CM60 cannot be finished.
So `consultation` is not added to `civic.comm.json`: a value the store
accepts for muting but cannot raise would give residents a switch that
controls nothing.

## What is left

1. CM26: move the event-to-category pairing out of Go, into the domain spec
   or a host function.
2. Declare `consultation` in `civic.comm.json` with the events that pair with
   it, and turn the pin above into a raise-and-mute test through a real mux.

## CM26: what the engine lacks, checked 2026-10-30

The event-to-category pairing cannot move into a declaration with the format as it stands. Three things are missing in `mwanachama-backend-shared/spec`, each of which would have to be built there before comm can drop `notificationEventCategory`:

1. **No value-to-value construct.** `Field.Values` is a flat list of strings, and `extend` in `spec/blueprint.go` lets a domain override a field's `Default` and, on an enum, append `Values`, nothing more. A domain that adds the event `consultation_opened` has no way to say that it is muted under `consultation`.
2. **A domain file carries no settings.** `Spec.Settings` is tagged `json:"-"` and is copied from the blueprint by `Apply`, so a setting such as a category-per-event list can be declared by the module with a neutral default but never filled by `civic.comm.json`. The only other layer is the definition layer, which the host writes at publish with `spec.PutDefinitionSettings`; that moves the pairing from Go into the host's publish step, not into a declaration.
3. **A domain file carries no rows.** The second option on the board, category-per-event as rows of a declared object, has nothing to seed those rows from, so every instance would start with no event comm accepts.

The three shapes left are therefore an owner choice: add to `spec` a pairing on an enum value (an entry in `values` that names the value of another field it implies) and let `extend` carry it; let a domain file declare setting values; or make the caller of `Raise` supply the category and have comm check only that it is declared, giving up the check that an event is raised under its own category. Until one is chosen the Go map stays, and the rows that wait on it (CM60, CM66, CM69, CM76, CM80, CM90, CM93, CM95) stay waiting.

## The same wish from seven more templates

Checked 2026-10-24 (run c922ce). Seven more templates list the same wish in
their comm module's `unenforceable`, and each is blocked by CM26 the same way
CM60 is:

| Row | Template | Category asked for |
| --- | --- | --- |
| CM66 | ava | a procurement stage (award publication) |
| CM69 | international-advisory-services | a deadline, due date first-class |
| CM76 | law-firm | cite-check |
| CM80 | party-mobilization | a mobilization instruction (cascade) |
| CM90 | software-venture | cohort milestones |
| CM93 | tour-guide-collective | booking confirmed (category or event) |
| CM95 | wakala | launch ready, launch failed, new agency in your sector |

None was added to its template. A template's `notification_categories` list
names the categories a module raises, not a widened enum, and a category
listed there could be muted but never raised. Each wish stays in
`unenforceable` until CM26 lands, as decision 283 says.
