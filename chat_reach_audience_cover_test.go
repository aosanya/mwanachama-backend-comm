package mwanachamacomm

import (
	"context"
	"errors"
	"testing"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

func wireReach(f *cover13Fixture, structures map[string]map[string]bool, leads map[string]bool) {
	f.m.UseBelonging(func(_ context.Context, actorID, structureID string) (bool, error) {
		return structures[structureID][actorID], nil
	})
	f.m.UseLeading(func(_ context.Context, actorID, structureID string) (bool, error) {
		return leads[actorID], nil
	})
}

func TestChatReachOfShowsAMemberGroupThreadsAndTheirOwnLeadsThreadOnly(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	wireReach(f, map[string]map[string]bool{"s-1": {"mem-1": true, "mem-2": true, "lead-1": true}}, map[string]bool{"lead-1": true})
	group, err := f.m.MintThread(ctx, "s-1", "/general")
	if err != nil {
		t.Fatalf("MintThread: %v", err)
	}
	asked, err := f.m.StartChatThreadFor(ctx, "s-1", "mem-1", "Water supply")
	if err != nil {
		t.Fatalf("StartChatThreadFor: %v", err)
	}
	if asked.Audience != models.ChatAudienceLeads {
		t.Fatalf("a member's thread audience = %q, want leads", asked.Audience)
	}

	byThread := func(reach []models.ChatReach) map[string]models.ChatReachReason {
		out := map[string]models.ChatReachReason{}
		for _, r := range reach {
			out[r.ThreadID] = r.Reason
		}
		return out
	}

	asker, err := f.m.ChatReachOf(ctx, "s-1", "mem-1")
	if err != nil {
		t.Fatalf("ChatReachOf asker: %v", err)
	}
	got := byThread(asker)
	if len(got) != 2 || got[group.ID] != models.ChatReachGroup || got[asked.ID] != models.ChatReachAsked {
		t.Fatalf("asker reach = %v, want group and asked", got)
	}

	neighbour, err := f.m.ChatReachOf(ctx, "s-1", "mem-2")
	if err != nil {
		t.Fatalf("ChatReachOf neighbour: %v", err)
	}
	got = byThread(neighbour)
	if len(got) != 1 || got[group.ID] != models.ChatReachGroup {
		t.Fatalf("a neighbour reaches %v, want only the group thread", got)
	}

	lead, err := f.m.ChatReachOf(ctx, "s-1", "lead-1")
	if err != nil {
		t.Fatalf("ChatReachOf lead: %v", err)
	}
	got = byThread(lead)
	if len(got) != 2 || got[asked.ID] != models.ChatReachLeads {
		t.Fatalf("a lead reaches %v, want the leads thread by leads", got)
	}
	for _, r := range lead {
		if r.StructureID != "s-1" {
			t.Fatalf("reach names structure %q, want s-1", r.StructureID)
		}
		if r.ThreadID == asked.ID && (r.RequestedBy != "mem-1" || r.Label != "Water supply") {
			t.Fatalf("leads thread reach = %+v", r)
		}
	}
}

func TestChatReachOfAnOutsiderIsAnEmptyListNotARefusal(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	wireReach(f, map[string]map[string]bool{"s-1": {"mem-1": true}}, nil)
	if _, err := f.m.MintThread(ctx, "s-1", "/general"); err != nil {
		t.Fatalf("MintThread: %v", err)
	}
	reach, err := f.m.ChatReachOf(ctx, "s-1", "outsider")
	if err != nil {
		t.Fatalf("outsider reach err = %v", err)
	}
	if reach == nil || len(reach) != 0 {
		t.Fatalf("outsider reach = %#v, want an empty non-nil list", reach)
	}
}

func TestChatReachOfPropagatesAFailingBelongingCheck(t *testing.T) {
	f := cover13Setup(t)
	boom := errors.New("roster unavailable")
	f.m.UseBelonging(func(context.Context, string, string) (bool, error) { return false, boom })
	if _, err := f.m.ChatReachOf(context.Background(), "s-1", "mem-1"); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the roster failure", err)
	}
}

func TestChatReachOfPersonGathersEveryRoomOnceAndSkipsRoomsTheyLeft(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	wireReach(f, map[string]map[string]bool{"s-1": {"mem-1": true}, "s-2": {"mem-1": true}, "s-3": {}}, nil)
	f.m.UseRoomsOf(func(context.Context, string) ([]string, error) {
		return []string{"s-1", "s-2", "s-1", "s-3"}, nil
	})
	t1, _ := f.m.MintThread(ctx, "s-1", "/general")
	t2, _ := f.m.MintThread(ctx, "s-2", "/general")
	if _, err := f.m.MintThread(ctx, "s-3", "/general"); err != nil {
		t.Fatalf("MintThread: %v", err)
	}
	reach, err := f.m.ChatReachOfPerson(ctx, "mem-1")
	if err != nil {
		t.Fatalf("ChatReachOfPerson: %v", err)
	}
	if len(reach) != 2 {
		t.Fatalf("reach = %+v, want one thread each from s-1 and s-2", reach)
	}
	seen := map[string]string{}
	for _, r := range reach {
		seen[r.ThreadID] = r.StructureID
	}
	if seen[t1.ID] != "s-1" || seen[t2.ID] != "s-2" {
		t.Fatalf("reach by thread = %v", seen)
	}
}

func TestChatReachOfPersonAnswersNotFoundWhenNoRoomHoldsThem(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	if _, err := f.m.ChatReachOfPerson(ctx, "mem-1"); !errors.Is(err, models.ErrChatNotFound) {
		t.Fatalf("no rooms-of wired = %v, want ErrChatNotFound", err)
	}
	wireReach(f, map[string]map[string]bool{"s-1": {}}, nil)
	f.m.UseRoomsOf(func(context.Context, string) ([]string, error) { return []string{"s-1"}, nil })
	if _, err := f.m.ChatReachOfPerson(ctx, "mem-1"); !errors.Is(err, models.ErrChatNotFound) {
		t.Fatalf("a room they do not belong to = %v, want ErrChatNotFound", err)
	}
	if _, err := f.m.ChatReachOfPerson(ctx, ""); !errors.Is(err, models.ErrChatNotFound) {
		t.Fatalf("nobody = %v, want ErrChatNotFound", err)
	}
	boom := errors.New("rooms unavailable")
	f.m.UseRoomsOf(func(context.Context, string) ([]string, error) { return nil, boom })
	if _, err := f.m.ChatReachOfPerson(ctx, "mem-1"); !errors.Is(err, boom) {
		t.Fatalf("rooms failure = %v, want it passed through", err)
	}
}

func resultsTemplate(subjectID string) models.Notification {
	return models.Notification{
		Category:    models.CategoryResults,
		Event:       models.EventResultsPublished,
		SubjectKind: models.SubjectSurvey,
		SubjectID:   subjectID,
	}
}

func TestRaiseToAudienceRefusesATemplateThatNamesAnActorOrNoRecord(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	f.m.UseAudience(func(context.Context, models.NotificationSubjectKind, string) ([]string, error) {
		return []string{"mem-1"}, nil
	})
	named := resultsTemplate("sv-1")
	named.ActorID = "mem-1"
	if _, err := f.m.RaiseToAudience(ctx, named); !errors.Is(err, models.ErrNotificationInvalid) {
		t.Fatalf("a template naming an actor = %v, want ErrNotificationInvalid", err)
	}
	if _, err := f.m.RaiseToAudience(ctx, resultsTemplate("")); !errors.Is(err, models.ErrNotificationInvalid) {
		t.Fatalf("a template with no subject id = %v, want ErrNotificationInvalid", err)
	}
	noKind := resultsTemplate("sv-1")
	noKind.SubjectKind = ""
	if _, err := f.m.RaiseToAudience(ctx, noKind); !errors.Is(err, models.ErrNotificationInvalid) {
		t.Fatalf("a template with no subject kind = %v, want ErrNotificationInvalid", err)
	}
}

func TestRaiseToAudienceNeedsAnAudienceThatResolvesToSomebody(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	if _, err := f.m.RaiseToAudience(ctx, resultsTemplate("sv-1")); !errors.Is(err, ErrNotificationNoAudience) {
		t.Fatalf("no audience wired = %v, want ErrNotificationNoAudience", err)
	}
	f.m.UseAudience(func(context.Context, models.NotificationSubjectKind, string) ([]string, error) {
		return []string{"", ""}, nil
	})
	if _, err := f.m.RaiseToAudience(ctx, resultsTemplate("sv-1")); !errors.Is(err, ErrNotificationNoAudience) {
		t.Fatalf("an audience of blanks = %v, want ErrNotificationNoAudience", err)
	}
	boom := errors.New("directory down")
	f.m.UseAudience(func(context.Context, models.NotificationSubjectKind, string) ([]string, error) { return nil, boom })
	if _, err := f.m.RaiseToAudience(ctx, resultsTemplate("sv-1")); !errors.Is(err, boom) {
		t.Fatalf("audience failure = %v, want it passed through", err)
	}
}

func TestRaiseToAudienceRaisesOncePerDistinctRecipientAndReportsTheMuted(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	f.m.UseAudience(func(_ context.Context, kind models.NotificationSubjectKind, id string) ([]string, error) {
		if kind != models.SubjectSurvey || id != "sv-1" {
			t.Fatalf("audience asked for %s/%s", kind, id)
		}
		return []string{"mem-1", "mem-2", "mem-1", "", "mem-3"}, nil
	})
	if _, err := f.m.NotificationStore.SetPreference(ctx, "mem-2", models.CategoryResults, true); err != nil {
		t.Fatalf("SetPreference: %v", err)
	}
	out, err := f.m.RaiseToAudience(ctx, resultsTemplate("sv-1"))
	if err != nil {
		t.Fatalf("RaiseToAudience: %v", err)
	}
	if out.Resolved != 3 {
		t.Fatalf("Resolved = %d, want 3 distinct recipients", out.Resolved)
	}
	raisedTo := map[string]bool{}
	for _, n := range out.Raised {
		raisedTo[n.ActorID] = true
		if n.SubjectID != "sv-1" || n.Event != models.EventResultsPublished {
			t.Fatalf("raised %+v", n)
		}
	}
	if len(out.Raised) != 2 || !raisedTo["mem-1"] || !raisedTo["mem-3"] {
		t.Fatalf("raised to %v, want mem-1 and mem-3", raisedTo)
	}
	if len(out.Missed) != 1 || out.Missed[0].ActorID != "mem-2" || !errors.Is(out.Missed[0].Reason, models.ErrNotificationMuted) {
		t.Fatalf("missed = %+v, want mem-2 muted", out.Missed)
	}
	if !out.Complete() {
		t.Fatalf("a raise with every recipient raised or missed must be complete")
	}
	if (AudienceRaise{Resolved: 3}).Complete() {
		t.Fatalf("an audience raise accounting for nobody must not be complete")
	}
}

func TestRaiseToAudienceRefusesAnInvalidTemplateBeforeRaisingToAnyone(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	f.m.UseAudience(func(context.Context, models.NotificationSubjectKind, string) ([]string, error) {
		return []string{"mem-1"}, nil
	})
	mismatched := resultsTemplate("sv-1")
	mismatched.Category = models.CategoryChat
	out, err := f.m.RaiseToAudience(ctx, mismatched)
	if !errors.Is(err, models.ErrNotificationInvalid) {
		t.Fatalf("category not matching the event = %v, want ErrNotificationInvalid", err)
	}
	if len(out.Raised) != 0 {
		t.Fatalf("raised %v despite refusal", out.Raised)
	}
}
