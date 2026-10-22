package routes_test

import (
	"encoding/json"
	"net/http"
	"strings"
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

func (f roomFixture) archived(caller string) (int, map[string]bool) {
	f.t.Helper()
	got := f.call(http.MethodGet, "/groups/s-1/chat/threads/archived", caller, nil)
	if got.code != http.StatusOK {
		return got.code, nil
	}
	var views []mwanachamacomm.ChatThreadView
	got.decode(f.t, &views)
	out := map[string]bool{}
	for _, v := range views {
		out[v.ID] = true
	}
	return got.code, out
}

func TestTheArchivedListHoldsOnlyTheCallersOwnArchives(t *testing.T) {
	f := newRoomFixture(t)
	if code, got := f.archived("actor-a"); code != http.StatusOK || len(got) != 0 {
		t.Fatalf("before archiving = %d %v, want 200 and nothing", code, got)
	}
	if got := f.archive(http.MethodPost, f.threadID, "actor-a"); got.code != http.StatusOK {
		t.Fatalf("archive = %d: %s", got.code, got.body)
	}
	if code, got := f.archived("actor-a"); code != http.StatusOK || !got[f.threadID] || got[f.other] {
		t.Fatalf("actor-a's archived = %d %v, want general only", code, got)
	}
	if code, got := f.archived("actor-b"); code != http.StatusOK || len(got) != 0 {
		t.Fatalf("actor-b's archived = %d %v, want nothing: the archive is actor-a's", code, got)
	}
	if back := f.archive(http.MethodDelete, f.threadID, "actor-a"); back.code != http.StatusNoContent {
		t.Fatalf("unarchive = %d: %s", back.code, back.body)
	}
	if code, got := f.archived("actor-a"); code != http.StatusOK || len(got) != 0 {
		t.Fatalf("after unarchiving = %d %v, want nothing", code, got)
	}
}

func TestTheArchivedListIsFencedLikeTheRoom(t *testing.T) {
	f := newRoomFixture(t)
	if code, _ := f.archived("actor-z"); code != http.StatusNotFound {
		t.Fatalf("outsider = %d, want 404", code)
	}
	outsider := f.call(http.MethodGet, "/groups/s-1/chat/threads/archived", "actor-z", nil)
	invented := f.call(http.MethodGet, "/groups/s-nope/chat/threads/archived", "actor-a", nil)
	if outsider.code != invented.code || string(outsider.body) != string(invented.body) {
		t.Fatalf("outsider %d %q and invented structure %d %q differ", outsider.code, outsider.body, invented.code, invented.body)
	}
}

func TestOnlyARoomsOwnThreadCanBeArchivedThere(t *testing.T) {
	f := newRoomFixture(t)
	for _, method := range []string{http.MethodPost, http.MethodDelete} {
		got := f.call(method, "/groups/s-2/chat/threads/"+f.threadID+"/archive", "actor-a", nil)
		if got.code != http.StatusNotFound {
			t.Errorf("%s another room's thread = %d: %s, want 404", method, got.code, got.body)
		}
	}
}

func TestArchivingIsDeclaredForRoomThreadsAlone(t *testing.T) {
	var ops struct {
		Operations map[string]struct {
			Path   string `json:"path"`
			Action string `json:"action"`
		} `json:"operations"`
	}
	if err := json.Unmarshal(mwanachamacomm.Operations(), &ops); err != nil {
		t.Fatal(err)
	}
	seen := 0
	for name, op := range ops.Operations {
		if !strings.Contains(op.Action, "archive") {
			continue
		}
		seen++
		if !strings.HasPrefix(op.Path, "/groups/{structure_id}/chat/threads/") || !strings.HasPrefix(op.Action, "comm.chat_thread.") {
			t.Errorf("%s archives %s at %s: archiving is for a room's threads only", name, op.Action, op.Path)
		}
	}
	if seen != 3 {
		t.Fatalf("found %d archive operations, want 3", seen)
	}
}
