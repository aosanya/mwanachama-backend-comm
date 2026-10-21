# Archiving a room's thread for oneself

CM49, 2026-10-21, for the member app's decision 153: anyone archives any
thread for themselves. An archive is per reader, the same shape as a
[read marker](read-markers.md): it takes a thread out of the caller's list of
the room's threads and nobody else's, and unarchiving puts it back.

The other half of CM49, decision 152 (a declined thread stays visible), is
not built here: what "no longer hidden from anyone" means for a room's other
members is still with the owner, and decline behaves as before.

## The object

`chat_archive` (civic `chat_archives`, school `board_shelves`):

| Field | What it holds |
| --- | --- |
| `id` | storage key, `carch-<uuid>` |
| `structure_id` | the structure whose room the thread is in |
| `thread_id` | the thread |
| `reader_id` | whose archive it is, always the caller |
| `archived_at` | when the reader archived it |

Unique on `(thread_id, reader_id)`. A reader with no archive row has archived
nothing; unarchiving deletes the row rather than flagging it.

## The two addresses

| Method | Path | Action | Answers |
| --- | --- | --- | --- |
| POST | `/groups/{structure_id}/chat/threads/{thread_id}/archive` | `comm.chat_thread.archive` | 200, the archive as stored |
| DELETE | `/groups/{structure_id}/chat/threads/{thread_id}/archive` | `comm.chat_thread.unarchive` | 204 |

Both take no body. Archiving again keeps the first archive and its
`archived_at`; unarchiving a thread that is not archived answers 204 as well,
since it is already where the caller asked it to be.

`GET /groups/{structure_id}/chat/threads` leaves out every thread the caller
archived. Nothing else reads the record: posting, reading a thread's posts,
marking it read and its unread count all carry on as before, so an archived
thread that hears something stays archived.

## The rules

- **The reader is the session's.** `reader_id` is `{"from": "caller"}`; the
  addresses take no body, so nobody archives a thread for somebody else.
- **The structure fence holds.** Both go through `inThread`, the check
  reading and posting use: a caller who does not belong, an invented
  structure, an invented thread, a thread of another structure and a `leads`
  thread the caller may not read all answer the same `ErrChatNotFound`.
  `routes/chat_fence_security_test.go` covers both addresses.

## Decided minimally, open to change

- **There is no listing of archived threads.** Unarchiving needs only the
  thread's id, which the client had when it archived. If the member app wants
  an "Archived" view, that is one more read (a query flag on the thread list,
  or its own address) and is not built.
- **Archive is not offered as a chat MCP tool.** `routes.ChatActions` is
  unchanged; adding the two actions there is a one-line change when an agent
  needs it.
- Who may call the two actions is the mount's. On wakala-api that is WK138.
