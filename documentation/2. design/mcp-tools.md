# MCP tools for chat (HS25)

`routes.ChatTools(m, mount)` builds eleven MCP tools from `comm.operations.json`
through `mwanachama-backend-shared/dispatch.Tools`, the same engine the REST
table uses, and keeps only the actions in `routes.ChatActions`:

| Tool | Action |
| --- | --- |
| `comm_chat_thread_list` | `comm.chat_thread.list` |
| `comm_chat_message_list` | `comm.chat_message.list` |
| `comm_chat_message_post` | `comm.chat_message.post` |
| `comm_chat_thread_mark_read` | `comm.chat_thread.mark_read` |
| `comm_chat_thread_create` | `comm.chat_thread.create` |
| `comm_chat_thread_list_archived` | `comm.chat_thread.list_archived` |
| `comm_chat_thread_archive` | `comm.chat_thread.archive` |
| `comm_chat_thread_unarchive` | `comm.chat_thread.unarchive` |
| `comm_notification_list` | `comm.notification.list` |
| `comm_notification_unread` | `comm.notification.unread` |
| `comm_notification_mark_read` | `comm.notification.mark_read` |

`mcp.RegisterChatTools(server, m, mount)` adds them to a go-sdk server.

The three notification tools are what the member app's Inbox and tab-bar badge
call (HS535). They answer the caller's own notifications only, the recipient
being bound from the session. `comm_notification_list` answers `items`,
`comm_notification_unread` answers `unread`, and `comm_notification_mark_read`
takes `ids` and answers `marked`. `routes/notification_tools_test.go` holds the
names and arguments; the wakala-api test `comm_notification_tools_test.go`
holds them against the real `/mcp`.

- The caller is `{"from": "caller"}` on every one, so it is bound from the
  mount's `Caller` and never appears in a tool's input schema.
- The structure fence, the leads fence and the read markers are the manager's,
  so a tool answers an outsider with the same not-found the REST address does.
- Every other operation is left out on purpose; widening the set is an entry
  in `ChatActions`.
- Registering the tools on wakala-api's `/mcp` is HS26, not this change.

`mcp/mcp_test.go` drives them through a real MCP client over the go-sdk's
in-memory transport.

## The call's `_meta` (S58)

A tool call hands the handler both its arguments and the call's MCP `_meta`, through `Tool.Handle`. A `conversation_id` in `_meta` therefore reaches the mount's `Caller` on the context, readable with `dispatch.ConversationOf`, the same as the `X-Conversation-Id` header does on REST. A malformed id is refused before the manager is reached. `mcp/tool_conversation_meta_test.go` holds both the carried and the absent case.
