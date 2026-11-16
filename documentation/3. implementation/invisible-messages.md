# Messages that draw nothing (MUN686)

A message made only of characters that draw nothing was accepted and stored: U+200B zero-width space, U+2060 word joiner, U+200E left-to-right mark, U+FEFF byte order mark, the soft hyphen, the Hangul and halfwidth Hangul fillers, the braille blank, private-use code points, variation selectors, a lone combining mark. The old blank test was `strings.TrimSpace`, which trims only what Go counts as white space, so a thread showed an empty-looking line.

## Where it is refused

| Surface | Route | Answer |
|---|---|---|
| Chat post | `POST /groups/{structure_id}/chat/threads/{thread_id}/messages` | 400 `body is required`, the same answer a blank post gets |
| Subject-thread post | `POST /subject-threads/{thread_id}/messages` | 400 `body is required` |
| Standing message | `POST /groups/{structure_id}/chat/standing` | 400 `body is required` |

`PostChatMessageFor` and `PostSubjectMessageFor` call `messageDraws` where they called `TrimSpace`. `Check` also calls `refuseUndrawnMessage` for the `body` field of the `chat_message`, `subject_message` and `standing_message` roles, so a host that writes through a store instead of a route meets the same rule. A direct message payload is ciphertext and cannot be read here.

## What counts

`messageDraws` is true when at least one character is not `plaintext.IsUnseenOrLineBreaking` and is a letter, digit, punctuation or symbol (the classes `plaintext.Problem` uses for a name). It composes the shared plaintext helpers and adds no rule to shared.

It is more lenient than `plaintext.CheckLine` on purpose. `CheckLine` refuses a line break and any invisible character anywhere, which is right for a name and wrong for a message: a message may run over several lines, and an emoji family is joined by zero-width joiners. So a message is refused only when nothing in it draws. A zero-width space between two real words stays, as does a decomposed accent on a letter.

## Not in this module

The same refusal is still missing on two catalog routes, which comm does not own: `POST /catalog/raised-items/{id}/replies` and `POST /catalog/suggestions/{id}/comments`. They can compose the same shared helpers.

## Tests

`routes/invisible_message_test.go` drives all three routes through the mux with thirteen messages that draw nothing and eight that draw something (a plain message, an accented Hungarian one, a decomposed accent, several lines, a joined emoji, a zero-width space between words, punctuation, a digit).

## Hub copy

The hub copy in `wakala-api/modules/comm` needs the same change until ISS-105 deletes it; until then `scripts/hub-copy-diff.sh` shows `message_text.go`, the `validate.go` call and the two manager lines as differences.
