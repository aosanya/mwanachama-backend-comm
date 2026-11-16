package mwanachamacomm

import (
	"context"
	"errors"
	"testing"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

func (f *cover13Fixture) wire(members map[string]bool, leads map[string]bool) {
	f.m.UseBelonging(func(_ context.Context, actorID, structureID string) (bool, error) {
		return structureID == "s-1" && members[actorID], nil
	})
	f.m.UseLeading(func(_ context.Context, actorID, structureID string) (bool, error) {
		return structureID == "s-1" && leads[actorID], nil
	})
}

func TestCover13WhoIsNotInTheStructureLearnsNothingOfIt(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	th, _ := f.m.MintThread(ctx, "s-1", "/general")
	f.wire(map[string]bool{"mem-1": true}, nil)

	if _, err := f.m.ListChatThreadsFor(ctx, "s-1", "outsider"); !errors.Is(err, models.ErrChatNotFound) {
		t.Fatalf("threads of an outsider = %v, want ErrChatNotFound", err)
	}
	if _, err := f.m.PostChatMessageFor(ctx, "s-1", th.ID, "outsider", "hi", "", ""); !errors.Is(err, models.ErrChatNotFound) {
		t.Fatalf("a post by an outsider = %v, want ErrChatNotFound", err)
	}
	if _, err := f.m.ListChatMessagesFor(ctx, "s-1", th.ID, "outsider", "", "", 0); !errors.Is(err, models.ErrChatNotFound) {
		t.Fatalf("messages for an outsider = %v, want ErrChatNotFound", err)
	}
	if _, err := f.m.ListChatThreadsFor(ctx, "s-1", ""); !errors.Is(err, models.ErrChatNotFound) {
		t.Fatalf("threads for nobody = %v, want ErrChatNotFound", err)
	}
	bare := cover13Setup(t)
	if _, err := bare.m.ListChatThreadsFor(ctx, "s-1", "mem-1"); !errors.Is(err, models.ErrChatNotFound) {
		t.Fatalf("with no belonging check wired, nobody belongs: %v", err)
	}
}

func TestCover13ALeadsThreadIsSeenOnlyByLeadsAndWhoAskedForIt(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	f.wire(map[string]bool{"mem-1": true, "mem-2": true, "lead-1": true}, map[string]bool{"lead-1": true})
	general, _ := f.m.MintThread(ctx, "s-1", "/general")
	asked, _, err := f.m.RequestThread(ctx, "s-1", "Clinic", "mem-1", func(models.ChatThread) bool { return true })
	if err != nil {
		t.Fatalf("RequestThread: %v", err)
	}
	ids := func(views []models.ChatThreadView) map[string]bool {
		out := map[string]bool{}
		for _, v := range views {
			out[v.ID] = true
		}
		return out
	}
	for caller, wantAsked := range map[string]bool{"mem-1": true, "mem-2": false, "lead-1": true} {
		views, err := f.m.ListChatThreadsFor(ctx, "s-1", caller)
		if err != nil {
			t.Fatalf("ListChatThreadsFor(%s): %v", caller, err)
		}
		got := ids(views)
		if !got[general.ID] || got[asked.ID] != wantAsked {
			t.Fatalf("%s sees %v; the general thread always, the clinic thread %v", caller, got, wantAsked)
		}
	}
	if _, err := f.m.PostChatMessageFor(ctx, "s-1", asked.ID, "mem-2", "hello", "", ""); !errors.Is(err, models.ErrChatNotFound) {
		t.Fatalf("a member posting in a leads thread they cannot see = %v, want ErrChatNotFound", err)
	}
	if _, err := f.m.PostChatMessageFor(ctx, "s-1", asked.ID, "mem-1", "hello", "", ""); err != nil {
		t.Fatalf("the asker posting in their thread: %v", err)
	}
}

func TestCover13PostingNeedsABodyAWholeAttachmentAndAThreadNotDeclined(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	f.wire(map[string]bool{"mem-1": true, "lead-1": true}, map[string]bool{"lead-1": true})
	th, _ := f.m.MintThread(ctx, "s-1", "/general")
	if _, err := f.m.PostChatMessageFor(ctx, "s-1", th.ID, "mem-1", "   ", "", ""); !errors.Is(err, models.ErrChatInvalid) {
		t.Fatalf("a blank body = %v, want ErrChatInvalid", err)
	}
	if _, err := f.m.PostChatMessageFor(ctx, "s-1", th.ID, "mem-1", "look", "photo", ""); !errors.Is(err, models.ErrChatInvalid) {
		t.Fatalf("half an attachment = %v, want ErrChatInvalid", err)
	}
	posted, err := f.m.PostChatMessageFor(ctx, "s-1", th.ID, "mem-1", "look", "photo", "ph-1")
	if err != nil || posted.AuthorID != "mem-1" || posted.AttachmentID != "ph-1" {
		t.Fatalf("PostChatMessageFor = %+v, %v", posted, err)
	}

	asked, _, _ := f.m.RequestThread(ctx, "s-1", "Fence", "mem-1", func(models.ChatThread) bool { return true })
	if _, _, err := f.m.DeclineThread(ctx, "s-1", asked.ID, "lead-1"); err != nil {
		t.Fatalf("DeclineThread: %v", err)
	}
	if _, err := f.m.PostChatMessageFor(ctx, "s-1", asked.ID, "lead-1", "sorry", "", ""); !errors.Is(err, models.ErrChatThreadDeclined) {
		t.Fatalf("posting in a declined thread = %v, want ErrChatThreadDeclined", err)
	}
}

func TestCover13AnAuthorKindIsStampedOnThePostAndMustFitTheKindPattern(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	f.wire(map[string]bool{"mem-1": true, "mem-2": true}, nil)
	th, _ := f.m.MintThread(ctx, "s-1", "/general")
	f.m.UseAuthorKindOf(func(_ context.Context, actorID string) (string, error) {
		if actorID == "mem-2" {
			return "Not A Kind!", nil
		}
		return "steward", nil
	})
	posted, err := f.m.PostChatMessageFor(ctx, "s-1", th.ID, "mem-1", "hi", "", "")
	if err != nil || posted.AuthorKind != "steward" {
		t.Fatalf("post = %+v, %v; the host's kind is stamped", posted, err)
	}
	if _, err := f.m.PostChatMessageFor(ctx, "s-1", th.ID, "mem-2", "hi", "", ""); !errors.Is(err, errAuthorKindNotASubjectKind) {
		t.Fatalf("a kind outside the pattern = %v, want errAuthorKindNotASubjectKind", err)
	}
	f.m.UseAuthorKindOf(func(context.Context, string) (string, error) { return "", errors.New("lookup down") })
	if _, err := f.m.PostChatMessageFor(ctx, "s-1", th.ID, "mem-1", "hi", "", ""); err == nil {
		t.Fatal("a failing kind lookup must fail the post, not post unlabelled")
	}
}

func TestCover13OnlyALeadOpensAThreadToTheGroup(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	f.wire(map[string]bool{"mem-1": true, "lead-1": true}, map[string]bool{"lead-1": true})
	led, _ := f.m.StartThread(ctx, "s-1", "Planning")
	if _, err := f.m.OpenChatThreadFor(ctx, "s-1", led.ID, "mem-1"); !errors.Is(err, models.ErrChatNotFound) {
		t.Fatalf("a non-lead opening = %v, want ErrChatNotFound", err)
	}
	opened, err := f.m.OpenChatThreadFor(ctx, "s-1", led.ID, "lead-1")
	if err != nil || opened.Audience != models.ChatAudienceGroup {
		t.Fatalf("OpenChatThreadFor = %+v, %v", opened, err)
	}
}

func TestCover13MessagesPageAroundAMessageOfThisThreadOnly(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	f.wire(map[string]bool{"mem-1": true}, nil)
	th, _ := f.m.MintThread(ctx, "s-1", "/general")
	other, _ := f.m.MintThread(ctx, "s-1", "/other")
	first := f.message(t, "s-1", th.ID, "mem-1", "one")
	f.message(t, "s-1", th.ID, "mem-1", "two")
	stray := f.message(t, "s-1", other.ID, "mem-1", "elsewhere")

	got, err := f.m.ListChatMessagesFor(ctx, "s-1", th.ID, "mem-1", first.ID, "", 0)
	if err != nil || len(got) != 1 || got[0].Body != "two" {
		t.Fatalf("after the first = %+v, %v", got, err)
	}
	if _, err := f.m.ListChatMessagesFor(ctx, "s-1", th.ID, "mem-1", stray.ID, "", 0); !errors.Is(err, models.ErrChatNotFound) {
		t.Fatalf("paging from a message of another thread = %v, want ErrChatNotFound", err)
	}
	if _, err := f.m.ListChatMessagesFor(ctx, "s-1", th.ID, "mem-1", "", "msg-missing", 0); !errors.Is(err, models.ErrChatNotFound) {
		t.Fatalf("paging before a missing message = %v, want ErrChatNotFound", err)
	}
	if _, err := f.m.ListChatMessagesFor(ctx, "s-1", th.ID, "mem-1", "", "", -1); !errors.Is(err, models.ErrChatInvalid) {
		t.Fatalf("a negative limit = %v, want ErrChatInvalid", err)
	}
}

func TestCover13ReadingAThreadMarksItUpToThePostSeenAndArchivingHidesItFromTheMainList(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	f.wire(map[string]bool{"mem-1": true, "mem-2": true}, nil)
	th, _ := f.m.MintThread(ctx, "s-1", "/general")
	one := f.message(t, "s-1", th.ID, "mem-2", "one")
	f.message(t, "s-1", th.ID, "mem-2", "two")

	views, err := f.m.ListChatThreadsFor(ctx, "s-1", "mem-1")
	if err != nil || len(views) != 1 || views[0].Unread != 2 || views[0].LastMessage == nil || views[0].LastMessage.Body != "two" || views[0].LastMessageAt == nil {
		t.Fatalf("thread view = %+v, %v; two unread and the last post carried", views, err)
	}
	if _, err := f.m.MarkChatThreadReadFor(ctx, "s-1", th.ID, "mem-1", one.ID); err != nil {
		t.Fatalf("MarkChatThreadReadFor: %v", err)
	}
	views, _ = f.m.ListChatThreadsFor(ctx, "s-1", "mem-1")
	if views[0].Unread != 1 {
		t.Fatalf("unread after reading the first = %d, want 1", views[0].Unread)
	}
	stray := f.message(t, "s-1", "cthread-other", "mem-2", "elsewhere")
	if _, err := f.m.MarkChatThreadReadFor(ctx, "s-1", th.ID, "mem-1", stray.ID); !errors.Is(err, models.ErrChatNotFound) {
		t.Fatalf("marking read up to a message of another thread = %v, want ErrChatNotFound", err)
	}
	if _, err := f.m.MarkChatThreadReadFor(ctx, "s-1", th.ID, "mem-1", ""); err != nil {
		t.Fatalf("marking read up to now: %v", err)
	}
	views, _ = f.m.ListChatThreadsFor(ctx, "s-1", "mem-1")
	if views[0].Unread != 0 {
		t.Fatalf("unread after reading to now = %d", views[0].Unread)
	}

	if _, err := f.m.ArchiveChatThreadFor(ctx, "s-1", th.ID, "mem-1"); err != nil {
		t.Fatalf("ArchiveChatThreadFor: %v", err)
	}
	main, _ := f.m.ListChatThreadsFor(ctx, "s-1", "mem-1")
	archived, _ := f.m.ListArchivedChatThreadsFor(ctx, "s-1", "mem-1")
	theirs, _ := f.m.ListChatThreadsFor(ctx, "s-1", "mem-2")
	if len(main) != 0 || len(archived) != 1 || len(theirs) != 1 {
		t.Fatalf("main=%d archived=%d other reader=%d; archiving is per reader", len(main), len(archived), len(theirs))
	}
	if err := f.m.UnarchiveChatThreadFor(ctx, "s-1", th.ID, "mem-1"); err != nil {
		t.Fatalf("UnarchiveChatThreadFor: %v", err)
	}
	if main, _ = f.m.ListChatThreadsFor(ctx, "s-1", "mem-1"); len(main) != 1 {
		t.Fatalf("main list after unarchiving = %d", len(main))
	}
	if _, err := f.m.ArchiveChatThreadFor(ctx, "s-1", th.ID, "outsider"); !errors.Is(err, models.ErrChatNotFound) {
		t.Fatalf("an outsider archiving = %v, want ErrChatNotFound", err)
	}
}
