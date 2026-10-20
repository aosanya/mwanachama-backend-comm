package routes_test

import (
	"context"
	"net/http"
	"testing"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/models"
)

func subjectServer(t *testing.T) (func(method, path, caller string, body any) reply, *mwanachamacomm.CommManager) {
	t.Helper()
	srv, m := newCommServer(t)
	m.UseStanding(standingOf(formStandings()))
	return func(method, path, caller string, body any) reply { return call(t, srv, method, path, caller, body) }, m
}

func post(t *testing.T, c func(method, path, caller string, body any) reply, threadID, caller, body string) {
	t.Helper()
	got := c(http.MethodPost, "/subject-threads/"+threadID+"/messages", caller, map[string]string{"body": body})
	if got.code != http.StatusCreated {
		t.Fatalf("%s posting = %d: %s", caller, got.code, got.body)
	}
}

func repliesTo(t *testing.T, m *mwanachamacomm.CommManager, actorID string) []string {
	t.Helper()
	got, err := m.NotificationStore.List(context.Background(), actorID, 0)
	if err != nil {
		t.Fatalf("list %s's notifications: %v", actorID, err)
	}
	var out []string
	for _, n := range got {
		if n.Event != models.EventThreadReply {
			continue
		}
		if n.SubjectKind != models.SubjectSubjectThread || n.Category != models.CategoryChat || n.AuthorActorID != "" {
			t.Fatalf("a reply notification %+v, want category chat, subject_thread and no author", n)
		}
		out = append(out, n.SubjectID)
	}
	return out
}

func TestOpeningTwiceReturnsTheSameThread(t *testing.T) {
	c, _ := subjectServer(t)
	first := openSubjectThread(t, c, "doc", "d-1", "actor-a", "group")
	again := openSubjectThread(t, c, "doc", "d-1", "actor-b", "group")
	if first.ID != again.ID || again.OwnerID != "" {
		t.Fatalf("two opens of the group thread gave %+v and %+v, want one ownerless thread", first, again)
	}
	mine := openSubjectThread(t, c, "doc", "d-1", "actor-a", "private")
	theirs := openSubjectThread(t, c, "doc", "d-1", "actor-b", "private")
	if mine.ID == theirs.ID || mine.OwnerID != "actor-a" || theirs.OwnerID != "actor-b" {
		t.Fatalf("private threads %+v and %+v, want one per actor, each owned by its opener", mine, theirs)
	}
	if again := openSubjectThread(t, c, "doc", "d-1", "actor-a", "private"); again.ID != mine.ID {
		t.Fatalf("reopening a private thread gave %q, want %q", again.ID, mine.ID)
	}
}

func TestOpeningRefusesWhatIsNotAThread(t *testing.T) {
	c, m := subjectServer(t)
	cases := []struct{ name, caller, audience string }{
		{"an answerer opening a private thread", "desk-1", "private"},
		{"an audience nobody declared", "actor-a", "everyone"},
		{"no audience", "actor-a", ""},
	}
	for _, tc := range cases {
		got := c(http.MethodPost, "/subjects/doc/d-1/threads", tc.caller, map[string]string{"audience": tc.audience})
		if got.code != http.StatusBadRequest {
			t.Fatalf("%s = %d, want 400: %s", tc.name, got.code, got.body)
		}
	}
	m.UseStanding(func(context.Context, string, string, string) (mwanachamacomm.SubjectStanding, error) {
		return mwanachamacomm.StandingAudience, nil
	})
	got := c(http.MethodPost, "/subjects/Not-A-Kind/d-1/threads", "actor-a", map[string]string{"audience": "group"})
	if got.code != http.StatusBadRequest {
		t.Fatalf("a subject kind outside the pattern = %d, want 400: %s", got.code, got.body)
	}
}

func TestABlankSubjectPostIsRefused(t *testing.T) {
	c, _ := subjectServer(t)
	group := openSubjectThread(t, c, "doc", "d-1", "actor-a", "group")
	got := c(http.MethodPost, "/subject-threads/"+group.ID+"/messages", "actor-a", map[string]string{"body": "  "})
	if got.code != http.StatusBadRequest {
		t.Fatalf("a blank post = %d, want 400: %s", got.code, got.body)
	}
}

func TestTheListShowsEachReaderWhatTheyMayRead(t *testing.T) {
	c, _ := subjectServer(t)
	group := openSubjectThread(t, c, "doc", "d-1", "actor-a", "group")
	mine := openSubjectThread(t, c, "doc", "d-1", "actor-a", "private")
	theirs := openSubjectThread(t, c, "doc", "d-1", "actor-b", "private")
	post(t, c, group.ID, "actor-a", "first")
	post(t, c, group.ID, "actor-b", "second")
	post(t, c, mine.ID, "actor-a", "a question")

	ids := func(caller string) map[string]int {
		var got []openedThread
		c(http.MethodGet, "/subjects/doc/d-1/threads", caller, nil).decode(t, &got)
		out := map[string]int{}
		for _, th := range got {
			out[th.ID] = th.MessageCount
		}
		return out
	}
	forA := ids("actor-a")
	if len(forA) != 2 || forA[group.ID] != 2 || forA[mine.ID] != 1 {
		t.Fatalf("actor-a lists %v, want the group thread with 2 and their own with 1", forA)
	}
	if _, leaked := forA[theirs.ID]; leaked {
		t.Fatal("actor-a can list actor-b's private thread")
	}
	if forDesk := ids("desk-1"); len(forDesk) != 3 {
		t.Fatalf("the answering side lists %v, want every thread", forDesk)
	}
}

func TestAReplyReachesWhomItRepliesTo(t *testing.T) {
	c, m := subjectServer(t)
	private := openSubjectThread(t, c, "doc", "d-1", "actor-a", "private")

	post(t, c, private.ID, "actor-a", "a question")
	if got := repliesTo(t, m, "actor-a"); len(got) != 0 {
		t.Fatalf("the owner was told of their own message: %v", got)
	}
	post(t, c, private.ID, "desk-1", "an answer")
	if got := repliesTo(t, m, "actor-a"); len(got) != 1 || got[0] != private.ID {
		t.Fatalf("the owner's replies %v, want one naming %s", got, private.ID)
	}

	group := openSubjectThread(t, c, "doc", "d-1", "actor-a", "group")
	post(t, c, group.ID, "actor-a", "anyone?")
	post(t, c, group.ID, "actor-b", "yes")
	post(t, c, group.ID, "desk-1", "noted")
	post(t, c, group.ID, "actor-a", "thanks")

	counts := map[string]int{}
	for _, who := range []string{"actor-a", "actor-b", "desk-1"} {
		for _, id := range repliesTo(t, m, who) {
			if id == group.ID {
				counts[who]++
			}
		}
	}
	if len(counts) != 2 || counts["actor-a"] != 2 || counts["actor-b"] != 2 {
		t.Fatalf("group replies told %v, want actor-a and actor-b twice each: the poster and the answering side are never told", counts)
	}
}

func TestAMutedReplyStillPosts(t *testing.T) {
	c, m := subjectServer(t)
	if _, err := m.SetNotificationPreferenceFor(context.Background(), "actor-a", "chat", true); err != nil {
		t.Fatalf("mute chat: %v", err)
	}
	private := openSubjectThread(t, c, "doc", "d-1", "actor-a", "private")
	post(t, c, private.ID, "desk-1", "an answer")
	if got := repliesTo(t, m, "actor-a"); len(got) != 0 {
		t.Fatalf("a muted owner was told: %v", got)
	}
}
