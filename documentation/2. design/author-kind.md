# A chat message carries the kind of its author (HS693)

The hub's phone app marks an agent's message with an Agent chip. The chip must
come from the message itself: the only other staff signal a host has,
`author_admin`, is also true for an owner who holds no seat in the group, so it
cannot say who is an agent. Decision 151 of the hub project settles it: the
message service sends `author_kind`, and the app reads it.

## What comm knows

comm knows the caller's id, from the session, and nothing about what kind of
actor that id is. The mount supplies two session facts only, the caller and the
device. So the kind is a fact the host supplies, the way `Belonging` and
`Leading` are.

## The design

- `UseAuthorKindOf` gives the manager an `AuthorKindOf` function,
  `(ctx, actorID) -> (kind, error)`.
- `PostChatMessageFor` asks it once, after the room fence and every other
  check, with the session's caller, and stores the answer on the message as
  `author_kind`. The message carries it from then on; an author whose kind
  changes later does not rewrite what they posted.
- `author_kind` is a `chat_message` field held by the existing `subject_kind`
  pattern. It is opaque to comm, like `attachment_kind`: the host says `agent`,
  comm never learns what that means.
- It is returned by every read that returns a message: the thread's messages,
  a message by id, and the `last_message` of each listed thread
  (`ChatLastMessage.AuthorKind`). It is omitted when empty, so a person's
  message carries no field at all.
- A manager given no function names no kind for anybody.
- A function that fails refuses the post as an internal error and stores
  nothing.

## Why a client cannot forge it

`comm.chat_message.post` declares no `author_kind` argument. A body that sends
one is ignored, as a body naming another `author_id` is. The only writer is the
host's function. Routes test: `routes/chat_author_kind_test.go`.

## What the host does

The host calls `UseAuthorKindOf` where it builds the manager and returns
`agent` for an author whose membership role is agent, and an empty string for
anyone else. An owner with no seat in the group gets an empty string, so no
chip.

## Not done

- Messages posted before this change have no `author_kind`; an old agent
  message shows no chip. There is no backfill.
- Subject-thread and direct messages carry no `author_kind`.
