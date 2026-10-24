# Notification settings (CM57)

Member app decisions 280, 281 and 398: each module stores and checks its own
settings, through `mwanachama-backend-shared`'s declared settings (S64), with
a definition layer and an override layer.

## What comm declares

`comm.blueprint.json` declares one setting, `notification_rules`, of type
`json`. Its value is a list of rules, each shaped as wakala-api's WK146
`notifyrules.Rule` resolves today, minus the host's own fixed facts:

| Field | Meaning |
| --- | --- |
| `event` | The event the rule is about. Free text the host names; comm does not hold it to its own event vocabulary, since most ruled events are raised by other modules. |
| `told` | Audiences told, in order. Names the host resolves (its leads, its owners, a thread). comm never learns who they are. |
| `fallback` | Audiences told only when nobody in `told` resolves. |
| `mutable` | Whether a recipient may mute the event. |

The module default is `[]`: no rule, so every event keeps the host's own
defaults and nothing behaves differently from before the setting existed.
WK146's per-event defaults, which audiences an event may tell, and which
events cannot have their mute changed are the host's knowledge and stay in
wakala-api; declaring them here would put the host's vocabulary in comm.

`models.ParseNotificationRules` holds a stored value to the shape: every rule
names an event, an event appears once, and no audience is empty.

## How it resolves

`spec.Migrate` provisions the three settings tables in comm's own table set
(`Provision` already calls it). `CommManager.NotificationRules(ctx)` reads
through `spec.ResolveStored`: an owner's override, else the value the agency
definition handed comm on publish (`spec.PutDefinitionSettings`), else the
module default. The answer carries the layer and, for an override, who set it
and when. `CommManager.NotificationRule(ctx, event)` answers one event's rule
and whether any layer rules on it.

## Not built here

- **Rewiring wakala-api** to publish its rules into the definition layer and
  read them back from comm is a later row; `agency_notification_rules` is
  still where WK146 reads.
- **A categories setting.** The categories that cannot be muted are still Go
  in `models` (CM26). A setting carrying them would declare a domain's
  category names as comm's default, which `domain_agnostic_test.go` forbids;
  no clean neutral shape was found, so it is left open.
- **A stale override after a republish** — an owner's override surviving a new
  definition — is an open question for the owner and is not decided here.
  Today the engine keeps the override until it is cleared.
- No route or tool exposes the setting yet.
