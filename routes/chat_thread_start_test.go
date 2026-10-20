package routes_test

import (
	"context"
	"net/http"
	"testing"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/models"
)

func TestStartingAThreadAddsItToTheRoomWithItsLabel(t *testing.T) {
	srv, m := newCommServer(t)
	m.UseBelonging(belongingOf(map[string][]string{"s-1": {"actor-a"}}))

	started := call(t, srv, http.MethodPost, "/groups/s-1/chat/threads", "actor-a", map[string]string{"label": "  Road   repairs "})
	if started.code != http.StatusCreated {
		t.Fatalf("start = %d, want 201: %s", started.code, started.body)
	}
	var th models.ChatThread
	started.decode(t, &th)
	if th.ID == "" || th.StructureID != "s-1" || th.Label != "Road repairs" {
		t.Fatalf("started thread = %+v, want s-1's with the label tidied to %q", th, "Road repairs")
	}

	listed := call(t, srv, http.MethodGet, "/groups/s-1/chat/threads", "actor-a", nil)
	if listed.code != http.StatusOK {
		t.Fatalf("list = %d: %s", listed.code, listed.body)
	}
	var views []mwanachamacomm.ChatThreadView
	listed.decode(t, &views)
	if len(views) != 1 || views[0].ID != th.ID || views[0].Label != "Road repairs" {
		t.Fatalf("the room lists %+v, want the one started thread with its label", views)
	}
}

func TestStartingAnExistingLabelAnswersWithThatThread(t *testing.T) {
	srv, m := newCommServer(t)
	m.UseBelonging(belongingOf(map[string][]string{"s-1": {"actor-a", "actor-b"}, "s-2": {"actor-a"}}))

	start := func(structureID, caller, label string) models.ChatThread {
		t.Helper()
		got := call(t, srv, http.MethodPost, "/groups/"+structureID+"/chat/threads", caller, map[string]string{"label": label})
		if got.code != http.StatusCreated {
			t.Fatalf("start %q = %d: %s", label, got.code, got.body)
		}
		var th models.ChatThread
		got.decode(t, &th)
		return th
	}

	first := start("s-1", "actor-a", "General")
	for _, again := range []string{"General", "general", " GENERAL "} {
		if th := start("s-1", "actor-b", again); th.ID != first.ID {
			t.Fatalf("starting %q again minted %s, want the existing %s", again, th.ID, first.ID)
		}
	}
	if other := start("s-2", "actor-a", "General"); other.ID == first.ID {
		t.Fatalf("another structure's General is s-1's thread: %+v", other)
	}

	threads, err := m.ListThreads(context.Background(), "s-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(threads) != 1 || threads[0].Label != "General" {
		t.Fatalf("s-1 holds %+v, want one General thread", threads)
	}
}

func TestAThreadNeedsALabel(t *testing.T) {
	srv, m := newCommServer(t)
	m.UseBelonging(belongingOf(map[string][]string{"s-1": {"actor-a"}}))

	for _, body := range []any{map[string]string{}, map[string]string{"label": "   "}, map[string]string{"label": "?!"}} {
		got := call(t, srv, http.MethodPost, "/groups/s-1/chat/threads", "actor-a", body)
		if got.code != http.StatusBadRequest {
			t.Errorf("start %v = %d, want 400: %s", body, got.code, got.body)
		}
	}
	threads, err := m.ListThreads(context.Background(), "s-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(threads) != 0 {
		t.Fatalf("a refused start minted %+v", threads)
	}
}

func TestStartingAThreadIsFencedLikeEveryOtherRoomAddress(t *testing.T) {
	srv, m := newCommServer(t)
	m.UseBelonging(belongingOf(map[string][]string{"s-1": {"actor-a"}}))
	body := map[string]string{"label": "General"}

	outsider := call(t, srv, http.MethodPost, "/groups/s-1/chat/threads", "outsider-9", body)
	if outsider.code != http.StatusNotFound {
		t.Fatalf("an outsider starting a thread = %d, want 404: %s", outsider.code, outsider.body)
	}
	invented := call(t, srv, http.MethodPost, "/groups/s-never/chat/threads", "outsider-9", body)
	if invented.code != outsider.code || invented.message(t) != outsider.message(t) {
		t.Fatalf("an outsider answers %d %q and an invented structure %d %q; the difference is an enumeration oracle",
			outsider.code, outsider.message(t), invented.code, invented.message(t))
	}
	threads, err := m.ListThreads(context.Background(), "s-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(threads) != 0 {
		t.Fatalf("a refused start minted %+v", threads)
	}
}

func TestWithNoBelongingSuppliedNobodyStartsAThread(t *testing.T) {
	srv, m := newCommServer(t)
	got := call(t, srv, http.MethodPost, "/groups/s-1/chat/threads", "actor-a", map[string]string{"label": "General"})
	if got.code != http.StatusNotFound {
		t.Fatalf("start with no belonging mounted = %d, want 404: %s", got.code, got.body)
	}
	threads, err := m.ListThreads(context.Background(), "s-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(threads) != 0 {
		t.Fatalf("a refused start minted %+v", threads)
	}
}
