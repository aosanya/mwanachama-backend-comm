# MCP tools for chat (HS25)

`routes.ChatTools(m, mount)` builds five MCP tools from `comm.operations.json`
through `mwanachama-backend-shared/dispatch.Tools`, the same engine the REST
table uses, and keeps only the actions in `routes.ChatActions`:

| Tool | Action |
| --- | --- |
| `comm_chat_thread_list` | `comm.chat_thread.list` |
| `comm_chat_message_list` | `comm.chat_message.list` |
| `comm_chat_message_post` | `comm.chat_message.post` |
| `comm_chat_thread_mark_read` | `comm.chat_thread.mark_read` |
| `comm_chat_thread_create` | `comm.chat_thread.create` |

`mcp.RegisterChatTools(server, m, mount)` adds them to a go-sdk server.

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
