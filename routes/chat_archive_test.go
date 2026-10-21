package routes_test

import (
	"net/http"
	"testing"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
)

func (f roomFixture) listed(caller string) map[string]bool {
	f.t.Helper()
	out := map[string]bool{}
	for id := range f.unread(caller) {
		out[id] = true
	}
	return out
}

func (f roomFixture) archive(method, threadID, caller string) reply {
	f.t.Helper()
	return f.call(method, "/groups/s-1/chat/threads/"+threadID+"/archive", caller, nil)
}

func TestAnArchivedThreadLeavesOnlyItsReadersListUntilUnarchived(t *testing.T) {
	f := newRoomFixture(t)

	got := f.archive(http.MethodPost, f.threadID, "actor-a")
	if got.code != http.StatusOK {
		t.Fatalf("archive = %d: %s", got.code, got.body)
	}
	var archive mwanachamacomm.ChatArchive
	got.decode(t, &archive)
	if archive.ReaderID != "actor-a" || archive.ThreadID != f.threadID || archive.StructureID != "s-1" {
		t.Fatalf("archive = %+v", archive)
	}
	if again := f.archive(http.MethodPost, f.threadID, "actor-a"); again.code != http.StatusOK {
		t.Fatalf("archiving again = %d: %s", again.code, again.body)
	}

	if listed := f.listed("actor-a"); listed[f.threadID] || !listed[f.other] {
		t.Fatalf("actor-a's list = %v, want general gone and events kept", listed)
	}
	if listed := f.listed("actor-b"); !listed[f.threadID] || !listed[f.other] {
		t.Fatalf("actor-b's list = %v, want both threads: the archive is actor-a's alone", listed)
	}

	if back := f.archive(http.MethodDelete, f.threadID, "actor-a"); back.code != http.StatusNoContent {
		t.Fatalf("unarchive = %d: %s", back.code, back.body)
	}
	if listed := f.listed("actor-a"); !listed[f.threadID] {
		t.Fatalf("after unarchiving, actor-a's list = %v, want general back", listed)
	}
	if back := f.archive(http.MethodDelete, f.threadID, "actor-a"); back.code != http.StatusNoContent {
		t.Fatalf("unarchiving a thread that is not archived = %d: %s", back.code, back.body)
	}
}

func TestAnArchiveIsFencedLikeReadingTheRoom(t *testing.T) {
	f := newRoomFixture(t)
	for name, c := range map[string]struct{ caller, thread string }{
		"outsider":        {"actor-z", f.threadID},
		"invented thread": {"actor-a", "cthread-nope"},
	} {
		for _, method := range []string{http.MethodPost, http.MethodDelete} {
			if got := f.archive(method, c.thread, c.caller); got.code != http.StatusNotFound {
				t.Errorf("%s %s = %d: %s, want 404", name, method, got.code, got.body)
			}
		}
	}
}
