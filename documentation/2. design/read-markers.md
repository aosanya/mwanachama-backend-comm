# Read markers on a room's threads

CM40, 2026-10-20, for the member app's decision 86: a member's rooms show
what is new. Each reader keeps a last-read point per thread of a structure's
room; marking a thread read moves it, and listing a room's threads returns
each thread with its unread count for the caller.

## What a room is here

comm stores a structure's room as its threads (`chat_thread`), and a thread
is what a client lists, reads and posts into. The school spec calls a thread
a classroom's *board*, and wakala-api's self-test calls listing them "a
resident lists their ward's rooms". So the marker is kept **per thread**, and
`list_chat_threads` is the room list that carries the counts.

Whether a group's room is one thread or several is still open on the member
app's board (MF7). A per-thread marker serves either answer: if a room is one
thread, its thread's count is the room's; if it is several, the room's count
is the sum of its threads' and each thread still shows its own.

## The object

`chat_read_marker` (civic `chat_read_markers`, school `board_bookmarks`):

| Field | What it holds |
| --- | --- |
| `id` | storage key, `cread-<uuid>` |
| `structure_id` | the structure whose room the thread is in |
| `thread_id` | the thread |
| `reader_id` | whose read point it is, always the caller |
| `read_at` | the last-read point: every post at or before it is read |
| `marked_at` | when the reader last marked |

Unique on `(thread_id, reader_id)`: marking again moves the same row. A
reader with no marker has read nothing.

## The two addresses

| Method | Path | Action |
| --- | --- | --- |
| GET | `/groups/{structure_id}/chat/threads` | `comm.chat_thread.list` |
| POST | `/groups/{structure_id}/chat/threads/{thread_id}/read` | `comm.chat_thread.mark_read` |

**Listing** returns each thread as `ChatThreadView`, the thread's own fields
plus `unread`. The count is the thread's posts written **by somebody else**
after the caller's `read_at`, or all of them with no marker. A reader's own
posts are never unread to them, so posting does not light up one's own room.

**Each listed thread also carries `last_message_at`** (CM47, the member
app's decision 146): the `created_at` of the thread's latest post, by anybody,
the caller included. A thread nobody has posted in carries no
`last_message_at` at all, rather than a zero time. A client orders a room by
the latest of its threads' `last_message_at`, so a room moves up when any
thread it can read hears something, not only its first one.

**Marking** takes a JSON object body. `{"message_id": "<post>"}` moves the
point to that post's `created_at`, which is what a client should send: the
last post it actually showed, so a post that arrives between reading and
marking stays unread. `{}` moves the point to the server's clock. The answer
is the marker as stored.

## The rules

- **The reader is the session's.** `reader_id` is `{"from": "caller"}`; a
  body naming another reader, structure or thread is ignored, so nobody marks
  a thread read for somebody else
  (`TestNobodyMarksAThreadReadForSomebodyElse`).
- **The point never moves backwards.** The upsert keeps the later of the
  stored and the offered `read_at`, in SQL, so a late, repeated or reordered
  mark cannot make read posts unread again
  (`TestAReadPointMovesToTheNamedPostAndNeverBackwards`, mutation-checked by
  replacing the comparison with a plain overwrite).
- **The count stops at 100 (HS302, the member app's decision 97).**
  `UnreadCap` is 100: a thread with a hundred or more unread posts answers
  `unread: 100`, and a client that prints "99+" for anything above 99 needs
  no other signal. `UnreadCounts` asks, for each thread of the room, for at
  most `UnreadCap` posts by somebody else after the reader's point, walking
  the existing `thread_created` index (`thread_id`, `created_at`) and
  stopping at the cap, so opening the list costs the same in a thread of ten
  posts or ten thousand. `LatestPosts` reads the newest post of each thread
  from the same index, one step per thread, rather than grouping every post
  of the room. No table or index was added. The reader's own posts after the
  point are walked past without being counted, so a reader who has posted
  thousands of times since last marking pays for those; marking the thread
  read ends it
  (`TestTheUnreadCountStopsAtTheCap`, `TestOwnPostsDoNotSpendTheCap`,
  `TestARoomListShowsAHundredForARoomWithMoreUnreadThanThat`).
- **The structure fence holds.** Marking goes through `inThread`, the same
  check reading and posting use: a caller who does not belong, an invented
  structure, an invented thread and a thread of another structure all answer
  the same `ErrChatNotFound`. A `message_id` that is not a post of this
  thread answers exactly as an invented one
  (`TestAPostOfAnotherThreadAnswersAsAnInventedOne`). The fence tests in
  `routes/chat_fence_security_test.go` cover the new address alongside the
  other three.
- **A room's latest activity is the caller's.** `last_message_at` is set
  only on the threads the list returns, and the list returns only what
  `ReadableBy` admits. A post in a `leads` thread, or in somebody else's
  request, never reaches a member it is hidden from, not even as a time:
  maxing across the threads a member is given cannot reveal that the leads
  spoke (`TestALeadsThreadsPostDoesNotMoveARoomForSomebodyWhoCannotReadIt`).
  It is a per-thread `max(created_at)` in one grouped query beside the
  unread counts, and the fence is applied in Go to both alike.
- **Read and unread are compared as stored text.** Every timestamp is
  written in `specstore.TimeLayout`, a fixed-width UTC layout, so
  `created_at > read_at` orders correctly on SQLite and Postgres alike. Two
  posts with exactly the same instant as the named one count as read with
  it; at nanosecond resolution that is a tie, not a race worth a second key.

## What it does not do

- Room-level posts with no thread are not counted: no declared address
  writes one, and a marker names a thread.
- A withheld post still counts until read, the same way listing a thread
  still returns it.
- Who may call the mark-read action is the mount's. On wakala-api a resident
  is not admitted to it yet; that is WK114 there.
