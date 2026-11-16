package mwanachamacomm

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aosanya/mwanachama-backend-comm/models"
)

func TestCover13LabelBecomesATagPathOfItsWords(t *testing.T) {
	cases := map[string]string{
		"Water Committee":      "/water-committee",
		"  Roads & Bridges!! ": "/roads-bridges",
		"Ünï 2026":             "/ünï-2026",
	}
	for label, want := range cases {
		got, ok := labelTagPath(label)
		if !ok || got != want {
			t.Fatalf("labelTagPath(%q) = %q, %v; want %q", label, got, ok, want)
		}
	}
	if _, ok := labelTagPath(" !!! "); ok {
		t.Fatal("a label of only punctuation has no tag path")
	}
}

func TestCover13StartThreadNeedsALabelAndStartsWithTheLeads(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	if _, err := f.m.StartThread(ctx, "s-1", "   "); !errors.Is(err, models.ErrChatInvalid) {
		t.Fatalf("blank label = %v, want ErrChatInvalid", err)
	}
	th, err := f.m.StartThread(ctx, "s-1", "  Water   Committee ")
	if err != nil {
		t.Fatalf("StartThread: %v", err)
	}
	if th.Label != "Water Committee" || th.TagPath != "/water-committee" || th.Audience != models.ChatAudienceLeads {
		t.Fatalf("thread = %+v", th)
	}
}

func TestCover13OpenThreadMovesALeadsThreadToTheGroupAndClearsTheDecline(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	th, err := f.m.StartThread(ctx, "s-1", "Roads")
	if err != nil {
		t.Fatalf("StartThread: %v", err)
	}
	opened, err := f.m.OpenThread(ctx, "s-1", th.ID)
	if err != nil {
		t.Fatalf("OpenThread: %v", err)
	}
	if opened.Audience != models.ChatAudienceGroup {
		t.Fatalf("audience = %q, want group", opened.Audience)
	}
	again, err := f.m.OpenThread(ctx, "s-1", th.ID)
	if err != nil || again.ID != th.ID || again.Audience != models.ChatAudienceGroup {
		t.Fatalf("opening an open thread = %+v, %v", again, err)
	}
	if _, err := f.m.OpenThread(ctx, "s-2", th.ID); !errors.Is(err, models.ErrChatNotFound) {
		t.Fatalf("opening in another structure = %v, want ErrChatNotFound", err)
	}
}

func TestCover13MintThreadReusesTheThreadAtThePath(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	first, err := f.m.MintThread(ctx, "s-1", "/general")
	if err != nil {
		t.Fatalf("MintThread: %v", err)
	}
	second, err := f.m.MintThread(ctx, "s-1", "/general")
	if err != nil {
		t.Fatalf("MintThread again: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("a second mint at the same path made %q, want the same thread %q", second.ID, first.ID)
	}
	got, err := f.m.ResolveThread(ctx, "s-1", "/general")
	if err != nil || got.ID != first.ID {
		t.Fatalf("ResolveThread = %+v, %v", got, err)
	}
	if _, err := f.m.ResolveThread(ctx, "s-1", "/nowhere"); !errors.Is(err, models.ErrChatNotFound) {
		t.Fatalf("resolving an unknown path = %v, want ErrChatNotFound", err)
	}
}

func TestCover13RequestThreadReusesOnlyWhatTheCallerAccepts(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	if _, _, err := f.m.RequestThread(ctx, "s-1", "", "mem-1", nil); !errors.Is(err, models.ErrChatInvalid) {
		t.Fatalf("blank label = %v, want ErrChatInvalid", err)
	}
	if _, _, err := f.m.RequestThread(ctx, "s-1", "Clinic", "", nil); !errors.Is(err, models.ErrChatNotFound) {
		t.Fatalf("no requester = %v, want ErrChatNotFound", err)
	}
	accept := func(models.ChatThread) bool { return true }
	refuse := func(models.ChatThread) bool { return false }

	first, made, err := f.m.RequestThread(ctx, "s-1", "Clinic", "mem-1", accept)
	if err != nil || !made || first.RequestedBy != "mem-1" || first.Audience != models.ChatAudienceLeads {
		t.Fatalf("first request = %+v made=%v err=%v", first, made, err)
	}
	reused, made, err := f.m.RequestThread(ctx, "s-1", "Clinic", "mem-2", accept)
	if err != nil || made || reused.ID != first.ID {
		t.Fatalf("an accepted match must be reused: %+v made=%v err=%v", reused, made, err)
	}
	other, made, err := f.m.RequestThread(ctx, "s-1", "Clinic", "mem-2", refuse)
	if err != nil || !made || other.ID == first.ID {
		t.Fatalf("a refused match must make a new thread: %+v made=%v err=%v", other, made, err)
	}
	if other.TagPath == first.TagPath || !strings.HasPrefix(other.TagPath, "/clinic-") {
		t.Fatalf("the new thread must sit beside the first at its own path, got %q and %q", first.TagPath, other.TagPath)
	}
}

func TestCover13DeclineThreadRulesAndAside(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	asked, _, err := f.m.RequestThread(ctx, "s-1", "Fence", "mem-1", func(models.ChatThread) bool { return true })
	if err != nil {
		t.Fatalf("RequestThread: %v", err)
	}
	declined, changed, err := f.m.DeclineThread(ctx, "s-1", asked.ID, "lead-1")
	if err != nil || !changed || !declined.Declined() || declined.DeclinedBy != "lead-1" {
		t.Fatalf("DeclineThread = %+v changed=%v err=%v", declined, changed, err)
	}
	if declined.TagPath == "/fence" || !strings.HasPrefix(declined.TagPath, "/fence-") {
		t.Fatalf("a declined thread is put aside at its own path, got %q", declined.TagPath)
	}
	again, changed, err := f.m.DeclineThread(ctx, "s-1", asked.ID, "lead-2")
	if err != nil || changed || again.DeclinedBy != "lead-1" {
		t.Fatalf("declining twice = %+v changed=%v err=%v; the first decline stands", again, changed, err)
	}
	fresh, made, err := f.m.RequestThread(ctx, "s-1", "Fence", "mem-3", func(models.ChatThread) bool { return true })
	if err != nil || !made || fresh.ID == asked.ID || fresh.TagPath != "/fence" {
		t.Fatalf("asking again after a decline must start afresh at the free path: %+v made=%v err=%v", fresh, made, err)
	}

	group, err := f.m.MintThread(ctx, "s-1", "/general")
	if err != nil {
		t.Fatalf("MintThread: %v", err)
	}
	if _, _, err := f.m.DeclineThread(ctx, "s-1", group.ID, "lead-1"); !errors.Is(err, models.ErrChatNotDeclinable) {
		t.Fatalf("declining a group thread = %v, want ErrChatNotDeclinable", err)
	}
	led, err := f.m.StartThread(ctx, "s-1", "Leads only")
	if err != nil {
		t.Fatalf("StartThread: %v", err)
	}
	if _, _, err := f.m.DeclineThread(ctx, "s-1", led.ID, "lead-1"); !errors.Is(err, models.ErrChatNotDeclinable) {
		t.Fatalf("declining a thread nobody asked for = %v, want ErrChatNotDeclinable", err)
	}
	if _, _, err := f.m.DeclineThread(ctx, "s-1", "cthread-missing", "lead-1"); !errors.Is(err, models.ErrChatNotFound) {
		t.Fatalf("declining a missing thread = %v, want ErrChatNotFound", err)
	}
}

func TestCover13ListThreadsIsPerStructureInCreationOrder(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	a, _ := f.m.MintThread(ctx, "s-1", "/a")
	b, _ := f.m.MintThread(ctx, "s-1", "/b")
	if _, err := f.m.MintThread(ctx, "s-2", "/c"); err != nil {
		t.Fatalf("MintThread: %v", err)
	}
	got, err := f.m.ListThreads(ctx, "s-1")
	if err != nil || len(got) != 2 || got[0].ID != a.ID || got[1].ID != b.ID {
		t.Fatalf("ListThreads = %+v, %v", got, err)
	}
}

func TestCover13PostNeedsAWholeAttachmentAndAValidMessage(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	half := models.ChatMessage{StructureID: "s-1", AuthorID: "mem-1", Body: "see this", AttachmentKind: "photo"}
	if _, err := f.m.ChatStore.Post(ctx, half); !errors.Is(err, models.ErrChatInvalid) {
		t.Fatalf("an attachment kind with no id = %v, want ErrChatInvalid", err)
	}
	whole := models.ChatMessage{StructureID: "s-1", AuthorID: "mem-1", Body: "see this", AttachmentKind: "photo", AttachmentID: "ph-1"}
	got, err := f.m.ChatStore.Post(ctx, whole)
	if err != nil || got.ID == "" || !strings.HasPrefix(got.ID, prefixChatMessage+"-") {
		t.Fatalf("Post = %+v, %v", got, err)
	}
	back, err := f.m.ChatStore.GetMessage(ctx, got.ID)
	if err != nil || back.AttachmentID != "ph-1" || back.Body != "see this" {
		t.Fatalf("GetMessage = %+v, %v", back, err)
	}
	if _, err := f.m.ChatStore.GetMessage(ctx, "msg-missing"); !errors.Is(err, models.ErrChatNotFound) {
		t.Fatalf("GetMessage of a missing id = %v, want ErrChatNotFound", err)
	}
}

func TestCover13ListMessagesHidesWithdrawnAndFiltersByThread(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	f.message(t, "s-1", "t-1", "mem-1", "one")
	hidden := f.message(t, "s-1", "t-1", "mem-1", "two")
	f.message(t, "s-1", "t-2", "mem-1", "three")
	if err := f.db.Table(f.m.ChatStore.store.Table(roleChatMessage)).Where("id = ?", hidden.ID).Update("withdrawn_at", "2026-11-10T09:00:00Z").Error; err != nil {
		t.Fatalf("withdraw: %v", err)
	}
	all, err := f.m.ChatStore.ListMessages(ctx, "s-1", "")
	if err != nil || len(all) != 2 {
		t.Fatalf("all shown = %d (%v), want 2", len(all), err)
	}
	one, err := f.m.ChatStore.ListMessages(ctx, "s-1", "t-1")
	if err != nil || len(one) != 1 || one[0].Body != "one" {
		t.Fatalf("thread t-1 = %+v, %v", one, err)
	}
}

func TestCover13ListMessagesWindowPagesByTimeAndId(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	var posts []models.ChatMessage
	for _, body := range []string{"m1", "m2", "m3", "m4", "m5"} {
		posts = append(posts, f.message(t, "s-1", "t-1", "mem-1", body))
	}
	bodies := func(ms []models.ChatMessage) string {
		var out []string
		for _, m := range ms {
			out = append(out, m.Body)
		}
		return strings.Join(out, ",")
	}
	newest, err := f.m.ListMessagesWindow(ctx, "s-1", "t-1", chatMessageWindow{Limit: 2})
	if err != nil || bodies(newest) != "m4,m5" {
		t.Fatalf("newest two = %q, %v; want m4,m5 oldest first", bodies(newest), err)
	}
	before, err := f.m.ListMessagesWindow(ctx, "s-1", "t-1", chatMessageWindow{Before: &posts[3], Limit: 2})
	if err != nil || bodies(before) != "m2,m3" {
		t.Fatalf("two before m4 = %q, %v; want m2,m3", bodies(before), err)
	}
	after, err := f.m.ListMessagesWindow(ctx, "s-1", "t-1", chatMessageWindow{After: &posts[1]})
	if err != nil || bodies(after) != "m3,m4,m5" {
		t.Fatalf("after m2 with no limit = %q, %v; want m3,m4,m5", bodies(after), err)
	}
}

func TestCover13MarkReadOnlyMovesForwardAndCountsUnread(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	th, _ := f.m.MintThread(ctx, "s-1", "/general")
	f.message(t, "s-1", th.ID, "mem-2", "a")
	second := f.message(t, "s-1", th.ID, "mem-2", "b")
	f.message(t, "s-1", th.ID, "mem-1", "mine")

	counts, err := f.m.UnreadCounts(ctx, "s-1", "mem-1")
	if err != nil || counts[th.ID] != 2 {
		t.Fatalf("unread for mem-1 = %v, %v; want 2 (own posts do not count)", counts, err)
	}
	marker, err := f.m.ChatStore.MarkRead(ctx, "s-1", th.ID, "mem-1", second.CreatedAt)
	if err != nil || !marker.ReadAt.Equal(second.CreatedAt) {
		t.Fatalf("MarkRead = %+v, %v", marker, err)
	}
	counts, _ = f.m.UnreadCounts(ctx, "s-1", "mem-1")
	if len(counts) != 0 {
		t.Fatalf("unread after reading up to b = %v, want none", counts)
	}
	older, err := f.m.ChatStore.MarkRead(ctx, "s-1", th.ID, "mem-1", second.CreatedAt.Add(-time.Hour))
	if err != nil || !older.ReadAt.Equal(second.CreatedAt) {
		t.Fatalf("an older marker must not move the mark back: %+v, %v", older, err)
	}
	f.message(t, "s-1", th.ID, "mem-2", "c")
	counts, _ = f.m.UnreadCounts(ctx, "s-1", "mem-1")
	if counts[th.ID] != 1 {
		t.Fatalf("unread after a new post = %v, want 1", counts)
	}
}

func TestCover13UnreadIsCappedAtOneHundred(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	th, _ := f.m.MintThread(ctx, "s-1", "/busy")
	for i := 0; i < UnreadCap+5; i++ {
		f.message(t, "s-1", th.ID, "mem-2", "x")
	}
	counts, err := f.m.UnreadCounts(ctx, "s-1", "mem-1")
	if err != nil || counts[th.ID] != UnreadCap {
		t.Fatalf("unread = %v, %v; want it capped at %d", counts, err, UnreadCap)
	}
}

func TestCover13LatestPostsIsTheNewestShownPostOfEachThread(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	quiet, _ := f.m.MintThread(ctx, "s-1", "/quiet")
	busy, _ := f.m.MintThread(ctx, "s-1", "/busy")
	f.message(t, "s-1", busy.ID, "mem-1", "first")
	last := f.message(t, "s-1", busy.ID, "mem-1", "second")
	latest, err := f.m.LatestPosts(ctx, "s-1")
	if err != nil {
		t.Fatalf("LatestPosts: %v", err)
	}
	if got := latest[busy.ID]; got.ID != last.ID || got.Body != "second" {
		t.Fatalf("latest of busy = %+v, want %q", got, last.ID)
	}
	if _, ok := latest[quiet.ID]; ok {
		t.Fatal("a thread with no posts has no latest post")
	}
	empty, err := f.m.LatestPosts(ctx, "s-9")
	if err != nil || len(empty) != 0 {
		t.Fatalf("LatestPosts of an empty structure = %v, %v", empty, err)
	}
}

func TestCover13ArchiveIsPerReaderAndIdempotent(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	th, _ := f.m.MintThread(ctx, "s-1", "/general")
	first, err := f.m.ChatStore.Archive(ctx, "s-1", th.ID, "mem-1")
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}
	second, err := f.m.ChatStore.Archive(ctx, "s-1", th.ID, "mem-1")
	if err != nil || second.ID != first.ID {
		t.Fatalf("archiving twice = %+v, %v; want the same archive row", second, err)
	}
	mine, err := f.m.ArchivedThreads(ctx, "s-1", "mem-1")
	if err != nil || !mine[th.ID] {
		t.Fatalf("mem-1 archived = %v, %v", mine, err)
	}
	theirs, err := f.m.ArchivedThreads(ctx, "s-1", "mem-2")
	if err != nil || len(theirs) != 0 {
		t.Fatalf("mem-2 archived = %v, %v; archiving is per reader", theirs, err)
	}
	if err := f.m.ChatStore.Unarchive(ctx, th.ID, "mem-1"); err != nil {
		t.Fatalf("Unarchive: %v", err)
	}
	mine, _ = f.m.ArchivedThreads(ctx, "s-1", "mem-1")
	if len(mine) != 0 {
		t.Fatalf("archived after unarchive = %v", mine)
	}
}

func TestCover13ActivityCountsThreadsAndMessagesPerStructure(t *testing.T) {
	f := cover13Setup(t)
	ctx := context.Background()
	a, _ := f.m.MintThread(ctx, "s-1", "/a")
	f.m.MintThread(ctx, "s-1", "/b")
	f.m.MintThread(ctx, "s-2", "/c")
	f.message(t, "s-1", a.ID, "mem-1", "hello")
	f.message(t, "s-1", a.ID, "mem-1", "again")

	page, err := f.m.Activity(ctx, models.ChatActivityQuery{})
	if err != nil {
		t.Fatalf("Activity: %v", err)
	}
	if page.Total != 2 || page.Threads != 3 || page.Messages != 2 || len(page.Rows) != 2 {
		t.Fatalf("page = %+v", page)
	}
	if page.Rows[0].StructureID != "s-1" || page.Rows[0].Messages != 2 || page.Rows[0].LastPostAt == nil {
		t.Fatalf("the structure with posts comes first: %+v", page.Rows)
	}
	if page.LastPostAt == nil {
		t.Fatal("the page carries the latest post time")
	}
	one, err := f.m.Activity(ctx, models.ChatActivityQuery{StructureID: "s-2"})
	if err != nil || one.Total != 1 || one.Threads != 1 || one.Messages != 0 || one.Rows[0].LastPostAt != nil {
		t.Fatalf("activity of s-2 = %+v, %v", one, err)
	}
	limit := 1
	paged, err := f.m.Activity(ctx, models.ChatActivityQuery{Limit: &limit, Offset: 1})
	if err != nil || paged.Total != 2 || len(paged.Rows) != 1 || paged.Rows[0].StructureID != "s-2" {
		t.Fatalf("second page of one = %+v, %v", paged, err)
	}
}
