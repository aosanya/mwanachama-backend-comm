package mwanachamacomm_test

import (
	"context"
	"errors"
	"testing"

	comm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/models"
)

func roomWithALeadAndANeighbours(t *testing.T) *comm.CommManager {
	t.Helper()
	m := gatedManager(t, nil)
	members := map[string]bool{"mem-1": true, "mem-2": true, "lead-1": true}
	leads := map[string]bool{"lead-1": true}
	m.UseBelonging(func(_ context.Context, actorID, structureID string) (bool, error) {
		return structureID == "s-1" && members[actorID], nil
	})
	m.UseLeading(func(_ context.Context, actorID, structureID string) (bool, error) {
		return structureID == "s-1" && leads[actorID], nil
	})
	return m
}

func TestAMemberReadsOneThreadByItsIdWithItsStructureAndLabel(t *testing.T) {
	ctx := context.Background()
	m := roomWithALeadAndANeighbours(t)
	thread, err := m.MintThread(ctx, "s-1", "/general")
	if err != nil {
		t.Fatalf("MintThread: %v", err)
	}
	got, err := m.GetChatThreadFor(ctx, thread.ID, "mem-2")
	if err != nil {
		t.Fatalf("GetChatThreadFor: %v", err)
	}
	if got.ID != thread.ID || got.StructureID != "s-1" || got.Label != thread.Label {
		t.Fatalf("read %+v, want thread %+v", got, thread)
	}
}

func TestAThreadReadByIdRefusesWhoDoesNotBelongAndAnInventedId(t *testing.T) {
	ctx := context.Background()
	m := roomWithALeadAndANeighbours(t)
	thread, err := m.MintThread(ctx, "s-1", "/general")
	if err != nil {
		t.Fatalf("MintThread: %v", err)
	}
	cases := map[string][2]string{
		"an outsider":                   {thread.ID, "outsider"},
		"nobody":                        {thread.ID, ""},
		"an invented id":                {"thr-invented", "mem-1"},
		"an empty id":                   {"", "mem-1"},
		"an invented id by an outsider": {"thr-invented", "outsider"},
	}
	for name, c := range cases {
		got, err := m.GetChatThreadFor(ctx, c[0], c[1])
		if !errors.Is(err, models.ErrChatNotFound) {
			t.Errorf("%s reads %+v (%v), want ErrChatNotFound", name, got, err)
		}
	}
}

func TestALeadsThreadReadByIdIsSeenOnlyByLeadsAndWhoAskedForIt(t *testing.T) {
	ctx := context.Background()
	m := roomWithALeadAndANeighbours(t)
	asked, _, err := m.RequestThread(ctx, "s-1", "Clinic", "mem-1", func(models.ChatThread) bool { return true })
	if err != nil {
		t.Fatalf("RequestThread: %v", err)
	}
	for _, caller := range []string{"mem-1", "lead-1"} {
		got, err := m.GetChatThreadFor(ctx, asked.ID, caller)
		if err != nil || got.ID != asked.ID || got.StructureID != "s-1" {
			t.Errorf("%s reads %+v (%v), want the clinic thread", caller, got, err)
		}
	}
	if got, err := m.GetChatThreadFor(ctx, asked.ID, "mem-2"); !errors.Is(err, models.ErrChatNotFound) {
		t.Errorf("a neighbour who did not ask reads %+v (%v), want ErrChatNotFound", got, err)
	}
}

func TestAThreadTheCallerArchivedStillReadsById(t *testing.T) {
	ctx := context.Background()
	m := roomWithALeadAndANeighbours(t)
	thread, err := m.MintThread(ctx, "s-1", "/general")
	if err != nil {
		t.Fatalf("MintThread: %v", err)
	}
	if _, err := m.ArchiveChatThreadFor(ctx, "s-1", thread.ID, "mem-1"); err != nil {
		t.Fatalf("ArchiveChatThreadFor: %v", err)
	}
	got, err := m.GetChatThreadFor(ctx, thread.ID, "mem-1")
	if err != nil || got.ID != thread.ID || got.StructureID != "s-1" {
		t.Fatalf("the archiver reads %+v (%v), want the thread", got, err)
	}
}
