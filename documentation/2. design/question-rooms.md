# Question rooms

A room can be switched into a question room. In it a member cannot post into the open group thread; the member asks in their own conversation with the room, a thread of the `leads` audience that only the asker and the room's leaders read, and the leaders answer there. Leaders still post in the open group thread.

## What declares it

- The setting `question_rooms` in `comm.blueprint.json` is a list of structure ids, empty by default. A room not listed is an ordinary group chat.
- `PUT /groups/{structure_id}/chat/question-room` (`comm.chat_room.set_question`, body `{"on": true}`) switches one room and answers `{structure_id, question_room}`. Switching a room to the state it is in changes nothing. Only a caller the mount says leads the structure may switch it; anybody else, an outsider and a structure that does not exist all answer the same not-found.

## What it changes

- A post into the open group thread of a question room by somebody who does not lead the room is refused with `ErrChatQuestionRoom`, which the wire carries as 409. The blank-message check runs first.
- A thread list marks each view: `question_room` for a member who must ask in their own thread, `takes_questions` and `leader` for a leader.
- The tool `comm.chat_room.set_question` is in `routes.ChatActions`, beside `comm.notification.raise`.
