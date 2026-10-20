package routes_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
)

type standings map[string]map[string]mwanachamacomm.SubjectStanding

func standingOf(table standings) mwanachamacomm.Standing {
	return func(_ context.Context, actorID, kind, id string) (mwanachamacomm.SubjectStanding, error) {
		return table[kind+"/"+id][actorID], nil
	}
}

func formStandings() standings {
	return standings{
		"doc/d-1": {
			"actor-a": mwanachamacomm.StandingAudience,
			"actor-b": mwanachamacomm.StandingAudience,
			"desk-1":  mwanachamacomm.StandingAnswerer,
		},
		"doc/d-2": {
			"actor-c": mwanachamacomm.StandingAudience,
		},
	}
}

type openedThread struct {
	ID           string `json:"id"`
	Audience     string `json:"audience"`
	OwnerID      string `json:"owner_id"`
	MessageCount int    `json:"message_count"`
}

func openSubjectThread(t *testing.T, srvCall func(method, path, caller string, body any) reply, kind, id, caller, audience string) openedThread {
	t.Helper()
	got := srvCall(http.MethodPost, "/subjects/"+kind+"/"+id+"/threads", caller, map[string]string{"audience": audience})
	if got.code != http.StatusOK {
		t.Fatalf("open %s thread on %s/%s as %s = %d: %s", audience, kind, id, caller, got.code, got.body)
	}
	var out openedThread
	got.decode(t, &out)
	return out
}

func TestASubjectThreadAnswersAnOutsiderExactlyAsAnInventedOne(t *testing.T) {
	srv, m := newCommServer(t)
	m.UseStanding(standingOf(formStandings()))
	c := func(method, path, caller string, body any) reply { return call(t, srv, method, path, caller, body) }

	private := openSubjectThread(t, c, "doc", "d-1", "actor-a", "private")
	group := openSubjectThread(t, c, "doc", "d-1", "actor-a", "group")

	cases := []struct {
		name, method, path, caller string
		body                       any
	}{
		{"another actor's private thread", http.MethodGet, "/subject-threads/" + private.ID + "/messages", "actor-b", nil},
		{"a post to another actor's private thread", http.MethodPost, "/subject-threads/" + private.ID + "/messages", "actor-b", map[string]string{"body": "hi"}},
		{"a group thread the subject never reached", http.MethodGet, "/subject-threads/" + group.ID + "/messages", "actor-c", nil},
		{"a post the subject never reached", http.MethodPost, "/subject-threads/" + group.ID + "/messages", "actor-c", map[string]string{"body": "hi"}},
		{"a stranger listing threads", http.MethodGet, "/subjects/doc/d-1/threads", "stranger", nil},
		{"a stranger opening a thread", http.MethodPost, "/subjects/doc/d-1/threads", "stranger", map[string]string{"audience": "group"}},
		{"an empty caller", http.MethodGet, "/subject-threads/" + group.ID + "/messages", "", nil},
	}
	invented := call(t, srv, http.MethodGet, "/subject-threads/sthread-never/messages", "actor-a", nil)
	if invented.code != http.StatusNotFound {
		t.Fatalf("an invented thread = %d, want 404: %s", invented.code, invented.body)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := call(t, srv, tc.method, tc.path, tc.caller, tc.body)
			if got.code != http.StatusNotFound {
				t.Fatalf("%s = %d, want 404 and never 403: %s", tc.name, got.code, got.body)
			}
			if got.message(t) != invented.message(t) {
				t.Fatalf("%s says %q, an invented thread says %q — the difference is an enumeration oracle",
					tc.name, got.message(t), invented.message(t))
			}
		})
	}
}

func TestWithNoStandingSuppliedNobodyReadsAThread(t *testing.T) {
	srv, m := newCommServer(t)
	m.UseStanding(standingOf(formStandings()))
	c := func(method, path, caller string, body any) reply { return call(t, srv, method, path, caller, body) }
	group := openSubjectThread(t, c, "doc", "d-1", "actor-a", "group")

	m.UseStanding(nil)
	for _, caller := range []string{"actor-a", "desk-1"} {
		got := call(t, srv, http.MethodGet, "/subject-threads/"+group.ID+"/messages", caller, nil)
		if got.code != http.StatusNotFound {
			t.Fatalf("%s with no standing resolver = %d, want 404: %s", caller, got.code, got.body)
		}
	}
	got := call(t, srv, http.MethodPost, "/subjects/doc/d-1/threads", "actor-a", map[string]string{"audience": "group"})
	if got.code != http.StatusNotFound {
		t.Fatalf("opening with no standing resolver = %d, want 404: %s", got.code, got.body)
	}
}

func TestAStandingFailureIsNeitherNotFoundNorForbidden(t *testing.T) {
	srv, m := newCommServer(t)
	m.UseStanding(func(context.Context, string, string, string) (mwanachamacomm.SubjectStanding, error) {
		return mwanachamacomm.StandingNone, errors.New("directory unreachable at 10.0.0.7")
	})
	got := call(t, srv, http.MethodGet, "/subjects/doc/d-1/threads", "actor-a", nil)
	if got.code != http.StatusInternalServerError {
		t.Fatalf("a failing standing lookup = %d, want 500: %s", got.code, got.body)
	}
	if msg := got.message(t); strings.Contains(msg, "10.0.0.7") || strings.Contains(msg, "directory") {
		t.Fatalf("the host's reason reached the caller: %q", msg)
	}
}

func TestAnAnswerersReplyCarriesNoAuthorForTheAudience(t *testing.T) {
	srv, m := newCommServer(t)
	m.UseStanding(standingOf(formStandings()))
	c := func(method, path, caller string, body any) reply { return call(t, srv, method, path, caller, body) }
	private := openSubjectThread(t, c, "doc", "d-1", "actor-a", "private")

	posted := call(t, srv, http.MethodPost, "/subject-threads/"+private.ID+"/messages", "desk-1", map[string]string{"body": "We will look at it."})
	if posted.code != http.StatusCreated {
		t.Fatalf("the answerer's reply = %d: %s", posted.code, posted.body)
	}

	type message struct {
		AuthorID   string `json:"author_id"`
		ByAnswerer bool   `json:"by_answerer"`
	}
	var forAudience []message
	call(t, srv, http.MethodGet, "/subject-threads/"+private.ID+"/messages", "actor-a", nil).decode(t, &forAudience)
	if len(forAudience) != 1 || forAudience[0].AuthorID != "" || !forAudience[0].ByAnswerer {
		t.Fatalf("the audience read %+v, want one reply by the answering side with no author", forAudience)
	}
	var forAnswerer []message
	call(t, srv, http.MethodGet, "/subject-threads/"+private.ID+"/messages", "desk-1", nil).decode(t, &forAnswerer)
	if len(forAnswerer) != 1 || forAnswerer[0].AuthorID != "desk-1" {
		t.Fatalf("the answering side read %+v, want its own author kept", forAnswerer)
	}
}

func TestAPostsAuthorAndStandingAreTheSessionsNotTheBody(t *testing.T) {
	srv, m := newCommServer(t)
	m.UseStanding(standingOf(formStandings()))
	c := func(method, path, caller string, body any) reply { return call(t, srv, method, path, caller, body) }
	group := openSubjectThread(t, c, "doc", "d-1", "actor-a", "group")

	got := call(t, srv, http.MethodPost, "/subject-threads/"+group.ID+"/messages", "actor-b", map[string]any{
		"body": "me too", "author_id": "actor-a", "by_answerer": true, "thread_id": "elsewhere",
	})
	if got.code != http.StatusCreated {
		t.Fatalf("post = %d: %s", got.code, got.body)
	}
	var out struct {
		AuthorID   string `json:"author_id"`
		ThreadID   string `json:"thread_id"`
		ByAnswerer bool   `json:"by_answerer"`
	}
	got.decode(t, &out)
	if out.AuthorID != "actor-b" || out.ThreadID != group.ID || out.ByAnswerer {
		t.Fatalf("stored %+v, want the session's author on the addressed thread, not answering", out)
	}
}
