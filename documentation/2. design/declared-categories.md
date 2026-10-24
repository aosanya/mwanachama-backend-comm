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
