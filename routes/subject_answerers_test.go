package routes_test

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"sync"
	"testing"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/models"
)

func requestStandings() standings {
	return standings{
		"request/r-1": {
			"applicant-1": mwanachamacomm.StandingAudience,
			"decider-1":   mwanachamacomm.StandingAnswerer,
			"decider-2":   mwanachamacomm.StandingAnswerer,
		},
	}
}

func answerersOf(table map[string][]string) mwanachamacomm.Answerers {
	return func(_ context.Context, kind, id string) ([]string, error) {
		return table[kind+"/"+id], nil
	}
}

func requestServer(t *testing.T) (func(method, path, caller string, body any) reply, *mwanachamacomm.CommManager) {
	t.Helper()
	srv, m := newCommServer(t)
	m.UseStanding(standingOf(requestStandings()))
	return func(method, path, caller string, body any) reply { return call(t, srv, method, path, caller, body) }, m
}

func TestAnAudiencePostOnAPrivateThreadTellsEveryAnswererTheHostNames(t *testing.T) {
	c, m := requestServer(t)
	m.UseAnswerers(answerersOf(map[string][]string{
		"request/r-1": {"decider-1", "decider-2", "decider-1", "applicant-1", "", "stranger"},
	}))
	private := openSubjectThread(t, c, "request", "r-1", "applicant-1", "private")

	post(t, c, private.ID, "applicant-1", "may I join?")
	for _, who := range []string{"decider-1", "decider-2"} {
		if got := repliesTo(t, m, who); len(got) != 1 || got[0] != private.ID {
			t.Fatalf("%s was told %v, want one reply naming %s", who, got, private.ID)
		}
	}
	for _, who := range []string{"applicant-1", "stranger"} {
		if got := repliesTo(t, m, who); len(got) != 0 {
			t.Fatalf("%s was told %v, want nothing: the poster is never told and a name without answerer standing is skipped", who, got)
		}
	}

	post(t, c, private.ID, "decider-1", "tell us more")
	if got := repliesTo(t, m, "applicant-1"); len(got) != 1 || got[0] != private.ID {
		t.Fatalf("the applicant was told %v, want one reply naming %s", got, private.ID)
	}
	if got := repliesTo(t, m, "decider-2"); len(got) != 1 {
		t.Fatalf("another answerer was told of an answerer's reply: %v", got)
	}
}

func TestWithNoAnswerersSuppliedAnAudiencePostTellsNobody(t *testing.T) {
	c, m := requestServer(t)
	private := openSubjectThread(t, c, "request", "r-1", "applicant-1", "private")
	post(t, c, private.ID, "applicant-1", "may I join?")
	for _, who := range []string{"decider-1", "decider-2"} {
		if got := repliesTo(t, m, who); len(got) != 0 {
			t.Fatalf("%s was told %v with no answerers supplied", who, got)
		}
	}
}

func TestAGroupPostNeverReachesTheAnswerersInbox(t *testing.T) {
	c, m := requestServer(t)
	m.UseAnswerers(answerersOf(map[string][]string{"request/r-1": {"decider-1"}}))
	group := openSubjectThread(t, c, "request", "r-1", "applicant-1", "group")
	post(t, c, group.ID, "applicant-1", "hello")
	if got := repliesTo(t, m, "decider-1"); len(got) != 0 {
		t.Fatalf("an answerer was told of a group post: %v", got)
	}
}

func TestAnAnswerersFailureRefusesThePostAndStoresNothing(t *testing.T) {
	c, m := requestServer(t)
	private := openSubjectThread(t, c, "request", "r-1", "applicant-1", "private")
	m.UseAnswerers(func(context.Context, string, string) ([]string, error) {
		return nil, errors.New("roster unreachable")
	})
	got := c(http.MethodPost, "/subject-threads/"+private.ID+"/messages", "applicant-1", map[string]string{"body": "hi"})
	if got.code != http.StatusInternalServerError {
		t.Fatalf("a failing answerers lookup = %d, want 500: %s", got.code, got.body)
	}
	var msgs []map[string]any
	c(http.MethodGet, "/subject-threads/"+private.ID+"/messages", "applicant-1", nil).decode(t, &msgs)
	if len(msgs) != 0 {
		t.Fatalf("the refused post was stored: %v", msgs)
	}
}

type pushRecorder struct {
	mu        sync.Mutex
	endpoints []string
}

func (p *pushRecorder) Send(_ context.Context, sub models.PushSubscription, _ []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.endpoints = append(p.endpoints, sub.Endpoint)
	return nil
}

func throwawayKeys(t *testing.T) models.PushKeys {
	t.Helper()
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	auth := make([]byte, 16)
	if _, err := rand.Read(auth); err != nil {
		t.Fatal(err)
	}
	return models.PushKeys{
		P256dh: base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()),
		Auth:   base64.RawURLEncoding.EncodeToString(auth),
	}
}

func TestAThreadReplyIsPushedLikeAnyRaisedNotification(t *testing.T) {
	c, m := requestServer(t)
	m.UseAnswerers(answerersOf(map[string][]string{"request/r-1": {"decider-1"}}))
	sender := &pushRecorder{}
	m.UseWebPush(sender, nil)
	for actor, endpoint := range map[string]string{
		"decider-1":   "https://push.example.org/decider",
		"applicant-1": "https://push.example.org/applicant",
	} {
		if _, err := m.SubscribePushFor(context.Background(), actor, "", endpoint, throwawayKeys(t)); err != nil {
			t.Fatalf("subscribe %s: %v", actor, err)
		}
	}
	private := openSubjectThread(t, c, "request", "r-1", "applicant-1", "private")
	post(t, c, private.ID, "applicant-1", "may I join?")
	post(t, c, private.ID, "decider-1", "tell us more")
	m.DrainPushes()

	sender.mu.Lock()
	defer sender.mu.Unlock()
	if len(sender.endpoints) != 2 {
		t.Fatalf("pushed to %v, want the decider's and the applicant's one each", sender.endpoints)
	}
}
