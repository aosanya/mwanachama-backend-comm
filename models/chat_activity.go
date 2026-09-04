package models

import (
	"context"
	"time"
)

// The **chat activity summary** — the one network-scope read of this domain,
// and the only one that is not scoped to a chapter room.
//
// # WHAT IT DELIBERATELY DOES NOT RETURN
//
// Not one word anybody wrote, and not one member's name. This is a count of
// rooms, threads and posts per chapter and nothing else, and the restraint is
// the whole design rather than a first cut waiting to be widened:
//
//   - **A network-wide list of messages would be the surveillance surface the
//     design set refused.** Chapter chat is read by walking into a chapter
//     room, where `requireChapterMember` fences it. A console route returning
//     message bodies across every ward would hand one seat the organization's
//     entire conversation, which is not a wider version of any read that
//     exists — it is a different act.
//   - **Per-member figures are refused outright (G21).** There is no author
//     breakdown here and there must never be one: *"no per-member engagement
//     anywhere"* is one `GROUP BY author_id` away from being untrue, and that
//     is exactly the line DEV-318 found already crossed one object over.
//   - **Nothing here is a named read, so nothing here needs an audit row.**
//     DSN-318's rule is that a read with a name on it is an act and must be
//     logged in the same transaction. The way to stay outside that rule is to
//     carry no names, which this does — rather than to carry them and hope the
//     log keeps up.
//
// What a national coordinator actually needs from it is which chapters are
// talking and which have gone quiet, and that is a count.
//
// # MODERATION IS NOT FOLDED IN
//
// Messages counts posts, and a post a moderator has since removed still counts
// as one. Netting removals out would make a busy, heavily-moderated ward read
// as a quiet one — the opposite of what a coordinator scanning for trouble
// needs — and a single net figure cannot be read back into either of the two
// facts it was built from.

// ChatActivityQuery is one filtered page of the chat activity summary.
type ChatActivityQuery struct {
	// ChapterID narrows to one chapter's room exactly, with no rollup. The
	// rollup needs the chapter tree, which this domain does not depend on.
	ChapterID string
	// Limit nil returns every chapter with any chat at all; Limit 0 returns
	// the totals alone and no rows — GET /v1/members' convention, kept
	// deliberately identical across every register on this gateway.
	Limit  *int
	Offset int
}

// ChatActivityRow is one chapter's room, as figures.
type ChatActivityRow struct {
	ChapterID string `json:"chapter_id"`
	// Threads is how many conversations have been opened in this room.
	Threads int `json:"threads"`
	// Messages is how many posts were made — including any since removed, for
	// the reason in the package comment above.
	Messages int `json:"messages"`
	// LastPostAt is when the room last heard anything, and is the figure the
	// "gone quiet" reading rests on. Nil for a room holding threads and no
	// posts, which is a real state and not a missing value.
	LastPostAt *time.Time `json:"last_post_at,omitempty"`
}

// ChatActivityPage is a page of rows plus the totals over the whole filtered
// set.
//
// Chapters with no chat at all are **absent rather than zero**, matching
// chapterMemberCounts: a row per silent ward would make this response the size
// of the structure rather than the size of the conversation, and on a 2,000-
// chapter network that is the difference between a summary and a dump.
// Total is therefore *chapters that have any chat*, never chapters.
type ChatActivityPage struct {
	Rows []ChatActivityRow `json:"rows"`
	// Total is how many chapters matched — chapters with chat, not the size of
	// the structure.
	Total      int        `json:"total"`
	Threads    int        `json:"threads"`
	Messages   int        `json:"messages"`
	LastPostAt *time.Time `json:"last_post_at,omitempty"`
}

// ChatActivityReader is the summary half of ChatRepository, its own interface
// so a caller that only summarises cannot reach a room's contents.
type ChatActivityReader interface {
	// Activity returns one page of the chat activity summary plus the totals
	// over the whole filtered set. Authorisation is the HTTP layer's, as it is
	// for every other read in this domain.
	Activity(ctx context.Context, q ChatActivityQuery) (ChatActivityPage, error)
}
