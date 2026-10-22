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

## The three addresses

| Method | Path | Action | Answers |
| --- | --- | --- | --- |
| GET | `/groups/{structure_id}/chat/threads/archived` | `comm.chat_thread.list_archived` | 200, the caller's archived threads |
| POST | `/groups/{structure_id}/chat/threads/{thread_id}/archive` | `comm.chat_thread.archive` | 200, the archive as stored |
| DELETE | `/groups/{structure_id}/chat/threads/{thread_id}/archive` | `comm.chat_thread.unarchive` | 204 |

Both take no body. Archiving again keeps the first archive and its
`archived_at`; unarchiving a thread that is not archived answers 204 as well,
since it is already where the caller asked it to be.

`GET /groups/{structure_id}/chat/threads` leaves out every thread the caller
archived, and `GET /groups/{structure_id}/chat/threads/archived` lists
exactly those, in the same `ChatThreadView` shape and behind the same
structure fence as the room's list, with the reader `{"from": "caller"}`.
One function, `threadViews`, serves both, so a thread is in exactly one of
the two lists for its reader. Nothing else reads the record: posting, reading a thread's posts,
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

## Room threads only

Archiving is for a room's threads (decision 205). All three addresses sit
under `/groups/{structure_id}/chat/threads/`, and the thread is looked up in
that structure's room: a direct-message thread, a subject thread or another
room's thread has no row there and answers the same not-found.
`TestArchivingIsDeclaredForRoomThreadsAlone` holds every archive operation
to a room path and `TestOnlyARoomsOwnThreadCanBeArchivedThere` exercises the
other-room case.

## MCP tools

`routes.ChatActions` carries all three, so `comm_chat_thread_list_archived`,
`comm_chat_thread_archive` and `comm_chat_thread_unarchive` are listed beside
the other chat tools, with no session fact in their schemas.

## Open to change

- Who may call the three actions is the mount's. On wakala-api that is WK138,
  which now also needs `comm.chat_thread.list_archived`.
