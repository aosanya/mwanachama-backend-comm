package mwanachamacomm

import (
	"strings"
	"testing"
)

// The chat summary's predicate goes on BOTH sides of the full outer join.
// Applied to one only, it would count one chapter's threads against another
// chapter's messages — a wrong answer rather than an error. Checked without
// a database, since a mistake in the string-building is invisible until
// somebody stands a cluster up (the postgres_scratch_test.go suite proves
// the statement runs; this proves its shape).
func TestChatActivityFiltersBothSidesOfTheJoin(t *testing.T) {
	filtered := strings.ReplaceAll(chatActivitySQL, chatRoomFilter, "WHERE chapter_id = $1")
	if n := strings.Count(filtered, "WHERE chapter_id = $1"); n != 2 {
		t.Fatalf("the room filter belongs on both sides of the join, found %d: %s", n, filtered)
	}
	if strings.Contains(filtered, chatRoomFilter) {
		t.Fatalf("a placeholder survived substitution: %s", filtered)
	}

	// And the unfiltered form leaves no stray placeholder behind either — the
	// network-wide read is the common case.
	unfiltered := strings.ReplaceAll(chatActivitySQL, chatRoomFilter, "")
	if strings.Contains(unfiltered, chatRoomFilter) || strings.Contains(unfiltered, "$1") {
		t.Fatalf("the unfiltered summary must bind nothing: %s", unfiltered)
	}
	// A room with threads and no posts, and a room with posts and no threads,
	// are both chapters with chat. An inner join drops the first and a left
	// join drops the second, silently in both directions.
	if !strings.Contains(chatActivitySQL, "FULL OUTER JOIN") {
		t.Fatal("the chapter summary needs a full outer join to see both kinds of room")
	}
}
