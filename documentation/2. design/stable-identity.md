# A thing's stable identity

`CommManager.Identity()` answers the shared `identity.Owner` (shared S56), so a host that resolves a manager per instance and mount holds the owner for it.

It is an accessor and not an embedded interface, as agency, market and sources do, because `CommManager` already promotes `AddressStore.Resolve(ctx, hash)` and a second `Resolve` on the manager would hide it from every caller of the address lookup.

`StableIdentity(ctx, object, id)` answers the raw `comm/<instance>/<mount>/<object>/<id>`, for example `comm/mwanachama/main/notification/3f0c...`. The mount and the object ride in the identity because a uuid does not say which table it is in, and because a raw minted under one mount is `identity.ErrNotOwned` under another rather than a confirmed miss that would mark a live thing stale.

The objects that have one are the conversation and notice records: `chat_thread`, `chat_message`, `subject_thread`, `subject_message`, `report`, `removal`, `dismissal`, `dispute`, `notification`, `notification_acknowledgement` and `standing_message`. Direct messages and their roster, keys and reactions, addresses, push subscriptions and deliveries, notification preferences and the per-reader chat markers answer `ErrNoStableIdentity`: they are private to one person or keyed on key material, and a context graph has no business naming them.

`Resolve(ctx, raw)` answers:

- `identity.ErrInvalidRaw` for a string that does not parse;
- `identity.ErrNotOwned` for another module's raw, another instance's or another mount's;
- `identity.ErrNotFound` for an object comm does not identify, an id with no row, or an id that belongs to another object.

Tests: `identity_owner_test.go`.
