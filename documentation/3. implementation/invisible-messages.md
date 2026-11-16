# Messages that draw nothing (MUN686)

A message made only of characters that draw nothing was accepted and stored: U+200B zero-width space, U+2060 word joiner, U+200E left-to-right mark, U+FEFF byte order mark, the soft hyphen, the Hangul and halfwidth Hangul fillers, the braille blank, private-use code points, variation selectors, a lone combining mark. The old blank test was `strings.TrimSpace`, which trims only what Go counts as white space, so a thread showed an empty-looking line.

## Where it is refused

| Surface | Route | Answer |
|---|---|---|
| Chat post | `POST /groups/{structure_id}/chat/threads/{thread_id}/messages` | 400 `body is required`, the same answer a blank post gets |
| Subject-thread post | `POST /subject-threads/{thread_id}/messages` | 400 `body is required` |
| Standing message | `POST /groups/{structure_id}/chat/standing` | 400 `body is required` |
| Direct-message reaction | `PUT /dm/messages/{message_id}/reaction` | 400 `emoji is required`, the same answer an empty emoji gets |

`PostChatMessageFor` and `PostSubjectMessageFor` call `messageDraws` where they called `TrimSpace`. `Check` also calls `refuseUndrawnMessage` for the `body` field of the `chat_message`, `subject_message` and `standing_message` roles, so a host that writes through a store instead of a route meets the same rule. A direct message payload is ciphertext and cannot be read here, but a reaction's emoji is plain text: `SetDMReactionFor` refuses one that does not draw (U+200B, U+3164, a plain space, U+00A0, a lone variation selector) with `messageDraws`, so the reaction never shows as an empty chip (hub issue CM110). `Check` has no rule for `dm_reaction.emoji`; a host that writes a reaction through a store instead of the route is not guarded.

## What counts

`messageDraws` is true when at least one character is not `plaintext.IsUnseenOrLineBreaking` and is a letter, digit, punctuation or symbol (the classes `plaintext.Problem` uses for a name). It composes the shared plaintext helpers and adds no rule to shared.

It is more lenient than `plaintext.CheckLine` on purpose. `CheckLine` refuses a line break and any invisible character anywhere, which is right for a name and wrong for a message: a message may run over several lines, and an emoji family is joined by zero-width joiners. So a message is refused only when nothing in it draws. A zero-width space between two real words stays, as does a decomposed accent on a letter.

## Toolchain independence (hub issue CM109)

`plaintext.IsUnseenOrLineBreaking` treats any rune outside the letter, mark, number, punctuation, symbol and space-separator classes as unseen, and those classes come from the Unicode tables of the Go toolchain that builds the service: go1.25 carries Unicode 15.0.0, go1.27 carries Unicode 17.0.0. An emoji added in Unicode 16 or 17 (U+1FAE9 face with bags under eyes, U+1FA89 harp, U+1FAC6 fingerprint, U+1FA8F shovel) is unassigned to a go1.25 build, so a message or reaction made only of it was refused as blank, while a go1.27 build kept it.

The rule that closes it is the smallest one: `isPictograph` counts any code point from U+1F000 to U+1FAFF as drawing, assigned or not, so `messageDraws` gives the same answer whichever toolchain builds comm. That span holds the pictograph, emoticon, transport and supplemental symbol blocks; every assigned code point in it is a symbol, number or modifier symbol, and the span holds no space or invisible code point. The cost is that a still-unassigned code point in the span also counts as drawing. The owner can reverse the rule by deleting `isPictograph`, which returns the answer to the toolchain's tables, or widen it by moving the two bounds. It lives in comm, not in shared plaintext: `CheckLine` and `Problem` keep the strict reading for names. Newer symbols outside the span (for example in Miscellaneous Symbols and Dingbats) are not covered.

`routes/invisible_message_test.go` and `routes/invisible_reaction_test.go` post U+1FAE9, U+1FA89, U+1FAC6 and U+1FA8F on the chat, subject and standing routes and the reaction route; the tests are run under `GOTOOLCHAIN=go1.25.11` and `go1.27.3`.

## Optional notes (waiting for the owner)

`report.note`, `dismissal.note` and `dispute.statement` are optional free text that only `CheckText` guards, so a note of invisible characters alone is stored as it is. Whether such a note is kept, refused or stored as empty is an open owner question (hub issue CM110); nothing changes until it is answered.

## Not in this module

The same refusal is still missing on two catalog routes, which comm does not own: `POST /catalog/raised-items/{id}/replies` and `POST /catalog/suggestions/{id}/comments`. They can compose the same shared helpers.

## Tests

`routes/invisible_message_test.go` drives all three routes through the mux with thirteen messages that draw nothing and eight that draw something (a plain message, an accented Hungarian one, a decomposed accent, several lines, a joined emoji, a zero-width space between words, punctuation, a digit).

## Hub copy

The hub copy in `wakala-api/modules/comm` needs the same change until ISS-105 deletes it; until then `scripts/hub-copy-diff.sh` shows `message_text.go`, the `validate.go` call and the two manager lines as differences.
