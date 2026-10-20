# Web Push for raised notifications

CM35, 2026-10-20. The owner's member app decision 61 put Web Push in comm:
comm stores each member's browser push subscriptions and sends a push when it
raises a notification for them. Decision 68 made the deployment's VAPID pair.
The member app's MF12 is the client.

## What is stored

One new role, `push_subscription`, declared in `comm.blueprint.json` and
named by both shipped domain specs (`push_subscription` in civic,
`browser_alert` in the school example).

| Field | Why |
| --- | --- |
| `id` | Storage key. Never shown to a push service. |
| `actor_id` | The subscriber. Bound `{"from": "caller"}`, never read from a body. |
| `device_id` | The handset, `{"from": "device"}`, when the session carries one. Empty is legitimate and is what wakala-api serves today, since its mount binds no handset. |
| `endpoint` | The push service URL the browser minted. **Unique.** |
| `public_key` | `keys.p256dh`, the browser's P-256 point, base64url, 65 bytes decoded. |
| `auth_secret` | `keys.auth`, 16 bytes decoded. Both keys are `json:"-"`: no address ever returns them. |
| `created_at` | When the browser last subscribed this endpoint. |

The keys are `text`, not `bytes`, unlike the address hashes: they are the
browser's own base64url strings, handed back to the encryption verbatim, and
are never hashed or compared.

**The endpoint is unique, and subscribing an endpoint somebody else held
moves it.** One browser subscription has one owner at a time. When a second
person signs in on the same browser and subscribes, the upsert hands the
endpoint to them, so the first person's notifications stop arriving on a
browser they no longer use. Re-subscribing the same endpoint keeps its id
and replaces its keys.

## The two addresses

| Method | Path | Action | Status |
| --- | --- | --- | --- |
| POST | `/push-subscriptions` | `comm.push_subscription.subscribe` | 201 |
| POST | `/push-subscriptions/unsubscribe` | `comm.push_subscription.unsubscribe` | 204 |

The subscribe body is what a browser's `PushSubscription.toJSON()` produces:
`{"endpoint": "...", "expirationTime": null, "keys": {"p256dh": "...",
"auth": "..."}}`. Named body arguments rather than a whole-body decode, so
`expirationTime` and anything else a browser adds are ignored rather than
refused, and so a body naming `actor_id` or `device_id` cannot reach the
row.

Unsubscribe deletes the caller's own row for that endpoint and answers 204
whether or not there was one, so a client may repeat it, and an endpoint
somebody else holds is left alone and answers exactly the same.

A subscription comm could not send to is `ErrPushSubscriptionInvalid`, a 400:
an endpoint that is not `https`, carries credentials, or names an IP literal,
`localhost` or a dotless host; a `p256dh` that is not 65 bytes; an `auth`
that is not 16. A missing required field is the declared `required` rule's
400, as everywhere else in comm.

## Sending

`CommManager.UseWebPush(sender, onFailure)` turns delivery on. With no sender
nothing is pushed and nothing else changes, which is what every existing
mount gets. The seam is set after construction, like `UseBelonging`, so
`NewCommManager`'s signature did not change.

When `NotificationStore.Raise` has stored a notification, it hands it to the
delivery, which:

1. **returns at once and sends in the background.** One raise can be one of
   hundreds (a form reaching a whole group), and a push service round trip
   is a network call per browser; a caller of `Raise` never waits for it.
   The background work keeps the request's values but not its cancellation,
   is bounded to 30 seconds, and at most sixteen notifications are being
   sent at a time.
2. lists the recipient's subscriptions and sends each one the same payload;
3. **drops a subscription the push service answers 404 or 410 to** — the
   two statuses RFC 8030 uses for a subscription that no longer exists;
4. reports any other failure to `onFailure` and keeps the subscription,
   because a 429 or a 5xx is the push service's trouble, not the browser's.

A muted category is refused by `Raise` before it is stored, so it is never
pushed either. A push failure never fails the raise: the notification is in
the Inbox whether or not a browser was woken.

`DrainPushes()` waits for every send in flight. Tests use it in place of
sleeping, and a host can call it on shutdown.

### The payload

```json
{"id": "...", "category": "results", "event": "results_published",
 "subject_kind": "survey", "subject_id": "...", "structure_id": "...",
 "created_at": "2026-10-20T08:00:00Z"}
```

The same thing the Inbox row says, and nothing more: there is no stored
title or body to send, so the service worker renders both from `event` and
the subject exactly as the Inbox does, and opens the row `id` names. The
recipient is not in it, because the browser it reaches is theirs.

### The sender

`WebPushSender` is the real `PushSender`, over
`github.com/SherClockHolmes/webpush-go` v1.4.0: RFC 8291 `aes128gcm`
encryption to the browser's keys and an RFC 8292 VAPID `Authorization`
header signed with the deployment's key. TTL is one day and urgency
`normal`. Every test runs against an `httptest` server; the encryption test
decrypts the body with the browser's own private key and verifies the VAPID
token against the public key, so the library is held to the RFCs rather than
trusted.

Two corrections to the library, both in `WebPushSender.Send`:

- it prepends `mailto:` to any subject that is not `https:`, so a configured
  `mailto:` contact would reach the push service as `mailto:mailto:…`; the
  prefix is stripped from the first contact before it is handed over;
- it builds no client of its own worth keeping, so the default client is
  comm's (below).

### Who the server will reach

The endpoint is a URL a member chose, and the server POSTs to it, so it is a
server-side request forgery surface. Two layers hold it:

- **at subscribe time**, the endpoint must be `https` and name a host by a
  dotted name — no IP literal, no `localhost`;
- **at send time**, the default client's dialer refuses any connection to a
  loopback, private, link-local, multicast or unspecified address, after DNS
  resolution, so a public name that resolves inward is refused too. It does
  not follow redirects and does not use an environment proxy, since a proxy
  address would be what the dialer checks.

A host that must reach push services through an egress proxy passes its own
`HTTPClient` to `NewWebPushSender` and takes on that check itself.

What is **not** built: an allowlist of known push service hosts
(`fcm.googleapis.com`, `updates.push.services.mozilla.com`,
`*.push.apple.com`, `*.notify.windows.com`), and a cap on how many
subscriptions one member may hold. Both are filed as open questions in the
row's report rather than decided here.

## The VAPID pair

The pair is the deployment's, made under decision 68 and kept in the
keychain under service `mwanachama-ci`, accounts `VAPID_PUBLIC_KEY` and
`VAPID_PRIVATE_KEY`. comm never holds it: the host reads it from its
environment and hands it over.

```go
v, ok := mwanachamacomm.VAPIDFromEnv(os.Getenv)
sender, err := mwanachamacomm.NewWebPushSender(v, nil)
manager.UseWebPush(sender, logPushFailure)
```

| Variable | Shape |
| --- | --- |
| `VAPID_PUBLIC_KEY` | base64url, the uncompressed P-256 point (65 bytes) — the same string the browser is given as `applicationServerKey` |
| `VAPID_PRIVATE_KEY` | base64url, the raw 32-byte P-256 scalar, as `web-push generate-vapid-keys` prints it |
| `VAPID_SUBJECT` | one or more `mailto:` or `https:` contacts the push services may use, comma-separated, in order of preference; Apple's refuses a push without one |

`VAPIDFromEnv` reports `ok` when either key is set, so a half-configured
deployment reaches `NewWebPushSender` and fails there by name rather than
silently pushing nothing. `NewWebPushSender` refuses a pair whose public key
is not the private key's, a key of the wrong length, and a missing or
malformed subject — at boot, before anything is sent.

### The contact comes from configuration (CM39)

Member app decision 84 (owner, 2026-10-20): the contact a push names is a
deployment setting, never a value built into comm or into a host. There is no
fallback address anywhere in the code, so a deployment that has not said who
the push services may reach has not configured Web Push.

- **`VAPID_SUBJECT` is a list.** Entries are separated by a comma
  (`VAPIDSubjectSeparator`), and the space around each entry is trimmed:
  `mailto:push@example.org, https://example.org/contact`.
- **Every entry is checked, not only the one that is sent.** Each must be a
  `mailto:` address with a local part and a domain, or an `https:` URL with a
  host and no credentials. An empty entry (`a,,b`, a trailing comma), an
  `http:` URL or a bare address refuses the whole setting, so a typo in a
  contact nobody is sending to yet is found at boot rather than the day it
  is promoted to first.
- **Each push carries the first entry.** The VAPID token's `sub` claim names
  one contact, so the deployment orders them: the first is the one the push
  services see. `VAPID.Contacts()` returns the list as parsed and
  `VAPID.Contact()` the one that is sent.
- **Unset or invalid behaves like a bad key.** With neither key set there is
  no sender and nothing is pushed, whatever the subject says. With a key set,
  a missing, blank or invalid `VAPID_SUBJECT` is `ErrVAPIDInvalid` from
  `NewWebPushSender`, naming the entry it refused, and the host decides
  whether that stops the boot — never a sender with a guessed contact.

Tests make a throwaway pair per run with `crypto/ecdh`; no key material is
committed.
