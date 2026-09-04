# mwanachama-backend-comm (Go)

🚀 in progress · 📋 not started · ⏸️ blocked

See [todo_done.md](todo_done.md) for W1-W7 (the extraction itself).

| ID | Pri | Status | Task |
|---|---|---|---|
| W8 | P1 | 📋 | Apply `000065_extract_comm_tables` to a real (staging/demo) database as its own deploy step — explicitly deferred, never applied to the live demo DB during the extraction itself. |
| W9 | P2 | 📋 | Run the `06-1-chat-threads` and `06-2-chat-moderation` Postman journeys with newman against a locally running gateway, as an end-to-end confirmation the route surface is unchanged. |
| W10 | P2 | 📋 | Create the `aosanya/mwanachama-backend-comm` GitHub remote and push (matches taskmanager's setup). |
| W12 | P1 | 📋 | Rewire the gateway's `internal/store/{postgres,memory}` adapters (`cmd/server/stores.go`, `comm_act_writer.go`) to this module's new GORM constructors (`NewChatStore`/`NewDMStore`/`NewModerationStore`) — broken by W11's storage swap; blocks the gateway build until done. |
| W13 | P2 | 📋 | Repoint the gateway's `chat_handlers.go`/`dm_handlers.go`/`moderation_handlers.go` at the 19 routes W11's `routes/` package now covers, retiring the gateway's now-duplicate handler code for those. |
