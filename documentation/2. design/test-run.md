# A manager for one test run (HS280)

`ForRun(runID)` on `*CommManager` returns a manager that reads and writes only the `_test` copy of every comm table under one run id. The agency's Test run plays a scenario through it: messages a coordinator sends, texts, notifications. No row reaches a live table and nothing leaves the building.

There is no manager interface in this module, so the method sits on the concrete `*CommManager` and returns `(*CommManager, error)`. An empty run id is refused with `specstore.ErrRunIDEmpty`. The parent is not changed.

## What the run manager does differently

- Every store is the shared engine's store scoped to the run: it reads and writes the `_test` table, stamps `run_id` on what it inserts, and filters every read by that run id.
- Raw SQL that the engine's scoping cannot reach carries its own run filter: the chat activity summary, unread counts and latest posts, the address directory search, the direct-thread roster and reaction lists, and the delete helpers.
- Conflict targets for upserts start with `run_id`, because the test tables' unique indexes are per run.
- Work done inside a transaction (starting or declining a requested chat thread, a subject reply with its notifications, a standing message's claim and post, a moderation decision) builds its stores from the run id too.
- Web Push is off. The copy's notification store has no raise hook, the delivery pipeline is dropped, and `UseWebPush` on a run manager does nothing, so no browser is ever contacted. Push subscriptions and attempts a run records stay in its own rows.
- Moderation acts are not written to the host's act log, because that log is a live table the module does not own. A run manager's `ActWriter` accepts and discards the entry.

## What it still shares with the parent

The hooks (belonging, leading, leads, standing, answerers, recipients, audience, mutable, the clock and the actors table) are copied when `ForRun` is called, so a host sets them first. The notification rules setting is read from the live settings table, and the member directory the address search joins is the live actors table; both are reads of configuration, not writes.

`Identity()` on a run manager still resolves against the live tables.

The host removes a run's rows with the shared engine's `RemoveRun`.
