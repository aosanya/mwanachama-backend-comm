# Hub import drift

Before the hub imports this repository as a tagged module and deletes its in-tree copy (`projects/hub-steward/code/wakala-api/modules/comm`), this repository has to be the union of both sides. This page lists every difference found on 2026-11-19 beyond the import path, with what was done about each. The command is the one of [hub-copy-diff.md](hub-copy-diff.md): `scripts/hub-copy-diff.sh`, which exits 1 while either side holds a file or line the other lacks.

The hub copy was compared at its commit `98c0711a`. The hub changed the module after the last comparison (2026-11-16) in ten commits; the second column names them by hub hash.

## Only in the hub

| What | Hub commit | Disposition |
|---|---|---|
| Setting `question_rooms` (a list of structure ids, default none) in `comm.blueprint.json` | d49c96ce | Merged. |
| A member's post into the open group thread of a question room is refused with `ErrChatQuestionRoom`; leaders still post there (`manager_chat.go`: `refuseOpenThreadQuestion`) | d49c96ce | Merged by hand beside this repository's `messageDraws` check; the two checks are independent, the blank-or-undrawn refusal runs first. |
| `QuestionRooms`, `SetQuestionRooms`, `isQuestionRoom` and `SetChatQuestionRoomFor` (`manager_settings.go`) | d49c96ce, 49bda544, 6a57d839 | Merged. The method takes structure, caller, on in that order, which is the order the dispatch binds arguments. |
| Operation `set_chat_question_room` (`PUT /groups/{structure_id}/chat/question-room`, action `comm.chat_room.set_question`, leaders only, one not-found for anybody else) | 49bda544 | Merged into `comm.operations.json`. |
| Error `ErrChatQuestionRoom` answering 409: the `errors` entry of `comm.operations.json`, the alias in `doc.go`, the sentinel in `routes/routes.go`, the variable in `models/chat.go` | 1e80fa4b | Merged. |
| Type `models.ChatQuestionRoom` and the fields `question_room`, `takes_questions` and `leader` on `ChatThreadView`; the thread list sets them (`manager_chat.go`) | 49bda544, 98c0711a | Merged. |
| Tool `comm.chat_room.set_question` in `ChatActions` (`routes/tools.go`) | 3453495d | Merged. |
| Tool `comm.notification.raise` in `ChatActions`; the operation exists on both sides | 0a54ec11 | Merged: the module now offers the tool. Which events a mount lets through is the hub's wiring, see below. |
| Events `payment_late` and `payment_list_handed_over`, both under the task category, in `models/notification_category.go` and as raisable events in `civic.comm.json` | 40c5776a, 8a6e1fa5 | Merged. |
| Unique index `one_late_reminder_per_bill` on the notification table, one `payment_late` per person and bill (`provision.go`) | 40c5776a | Merged. |
| Test `chat_question_room_cover_test.go` | 7afc77bb, 98c0711a | Moved with the code. |
| Test `routes/question_room_switch_test.go` | 6a57d839, 1e80fa4b | Moved with the code. |
| Test `models/address_hours_cover_test.go` (the opening-hours window, zone, midnight and validation rules) | not carried before | Moved with the code. This repository covers the same rules in `models/address_cover_test.go`; both stay, they pin different cases. |
| Extra cases in `models/address_code_cover_test.go` (a 14-character derived address, and the refused inputs `MKU48271YUT339`, `MKU 4827 YUT 33x1`, `MK$4827YUT3391`, `4827MKUYUT3391`) | not carried before | Merged into this repository's test of the same name, which keeps its own cases. |

## Only in this repository

| What | Disposition |
|---|---|
| Operation `get_chat_thread` (`GET /chat/threads/{thread_id}`), `GetChatThreadFor` in `manager_read_by_id.go`, and the tool `comm.chat_thread.read` in `ChatActions` | Kept. The hub has no such tool yet, so the hub's tool list and its pinned roster grow by one when it imports this module. |
| `message_text.go`, `refuseUndrawnMessage` in `validate.go`, and the `messageDraws` checks in `manager_chat.go`, `manager_dm.go` and `manager_subject.go`: a message or reaction that draws nothing is refused like a blank one | Kept. In the hub the same three places test only for a blank text. The hub's behaviour is the older, weaker rule, not a competing one, so nothing is asked. |
| `chat_thread_read.go` and its tests | Kept. |
| `message_text.go` and `chat_thread_read.go` are the only non-test files; 89 test files the hub copy never carried: 48 in the root package, 35 in `routes` and 6 in `models` | Kept. They test code that is identical on both sides, bar the two features above. |
| `mcp/`, `spec/`, `cmd/ddl` and `scripts/` | Kept. The hub never carried them and the comparison leaves them out. |

## Differing beyond the import path

| What | Disposition |
|---|---|
| `models/address_state_cover_test.go`: one test is named `TestAddressOpenNeedsLiveEnabledAndWithinHours` in the hub and `TestAddressOpenFollowsLiveEnabledAndHoursState` here | This repository's name stays. The hub's name is already taken by a test in `models/address_cover_test.go` here, so taking it would not compile. The body is identical. |
| `models/address_code_cover_test.go`: the same behaviours under different test names and different input lists | This repository's tests stay; the hub's extra cases are merged in (see above). |

## Hub wiring that is not part of the module, not moved

These live in the hub and stay there. They change only when the hub imports this module.

| Where in the hub | What |
|---|---|
| `internal/agencies/registry.go` | Mounts `comm_notification_raise` and allows only `report_handed_over`, `payment_late` and `payment_list_handed_over` through it, behind the `notification.raise` permission. |
| `internal/agencies/late_bills_sweep.go` and `schedule_sweep.go` | The schedule sweep that raises `payment_late` to the role a charge's late rule names. |
| `internal/api/http/resident_comm.go` | The HTTP layer for the question-room switch. |
| `modules/agency/models/permissions_catalog.json` | The permissions for the switch and for `notification.raise`. |
| `internal/api/http/mcp_roster_pin_test.go` and `testdata/mcp_roster`, `testdata/mcp_lazy` | The pinned tool roster and counts: they grow by `comm_chat_room_set_question`, `comm_notification_raise` and, once the hub imports this module, `comm_chat_thread_read`. |
| `internal/api/http/notification_raise_mcp_test.go` | The test of the raise mount. |
| `app/` screens | The question-room switch, the thread rows and the notice titles. |

## Pinned counts

Three pins moved, each by exactly what the union adds and none loosened: the table of addresses from 53 to 54 (`set_chat_question_room`) in `routes/routes_test.go`, `CLAUDE.md` and `documentation/2. design/routes.md`; and the chat tools the MCP server lists from 13 to 15 in `mcp/mcp_test.go` (`comm_chat_room_set_question` and `comm_notification_raise`). The 23 declared roles did not change.

## Owner questions

None. No operation has two different behaviours on the two sides.
