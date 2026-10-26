# A chat message may carry one attachment reference (HS114)

A host that attaches a form to a message needs the agent to find that form
again when it reads the thread. A chat message therefore has two optional
fields, `attachment_kind` and `attachment_id`.

- Both are opaque. comm never learns what they point at; the host says
  `form` and a form id, the way a subject thread's `subject_kind` and
  `subject_id` work.
- Both default to empty, which is a post with no attachment.
- `attachment_kind` is held by the existing `subject_kind` pattern, and
  `attachment_id` by a new `attachment_id` pattern in `patterns.go`: 1 to 128
  characters from letters, digits and `_ . : -`, starting with a letter or
  digit.
- They are set together or not at all. A declaration cannot say that one
  field's presence depends on another, so `ChatMessage.AttachmentIsWhole`
  holds it, and `PostChatMessageFor` refuses a half pair with
  `ErrChatInvalid` (400).
- The `comm.chat_message.post` operation takes both as optional body
  arguments; the message is returned whole on post, read and list. The
  action id and the path are unchanged, so the hub client's
  `comm_chat_message_post` tool keeps its name.
- A message stays append-only: the attachment is written at post time and
  never edited.
