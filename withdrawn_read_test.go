package mwanachamacomm_test

import (
	"context"
	"errors"
	"testing"

	comm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/models"
)

func roomOfNeighbours(t *testing.T) (*comm.CommManager, string) {
	t.Helper()
	m := gatedManager(t, nil)
	members := map[string]bool{"mem-1": true, "mem-2": true}
	m.UseBelonging(func(_ context.Context, actorID, structureID string) (bool, error) {
		return structureID == "s-1" && members[actorID], nil
	})
	thread, err := m.MintThread(context.Background(), "s-1", "/general")
	if err != nil {
		t.Fatalf("MintThread: %v", err)
	}
	return m, thread.ID
}

func TestANeighbourCannotReadAWithdrawnMembersMessageByItsId(t *testing.T) {
	ctx := context.Background()
	m, threadID := roomOfNeighbours(t)
	posted, err := m.PostChatMessageFor(ctx, "s-1", threadID, "mem-1", "hello from mem-1", "", "")
	if err != nil {
		t.Fatalf("PostChatMessage: %v", err)
	}
	if got, err := m.GetChatMessageFor(ctx, posted.ID, "mem-2"); err != nil || got.Body == "" {
		t.Fatalf("before the withdrawal the neighbour reads %+v (%v), want the message", got, err)
	}
	if err := m.HideActorMessages(ctx, "mem-1"); err != nil {
		t.Fatalf("HideActorMessages: %v", err)
	}
	if got, err := m.GetChatMessageFor(ctx, posted.ID, "mem-2"); !errors.Is(err, models.ErrChatNotFound) {
		t.Fatalf("after the withdrawal the neighbour reads %+v (%v), want not found", got, err)
	}
	if got, err := m.GetChatMessageFor(ctx, posted.ID, "mem-1"); err != nil || got.Body == "" {
		t.Fatalf("the author reads %+v (%v), want their own message", got, err)
	}
	if err := m.RestoreActorMessages(ctx, "mem-1"); err != nil {
		t.Fatalf("RestoreActorMessages: %v", err)
	}
	if got, err := m.GetChatMessageFor(ctx, posted.ID, "mem-2"); err != nil || got.Body == "" {
		t.Fatalf("after the restore the neighbour reads %+v (%v), want the message", got, err)
	}
}

func TestAMessagePostedBeforeAWithdrawalIsListedToNobodyAfterIt(t *testing.T) {
	ctx := context.Background()
	m, threadID := roomOfNeighbours(t)
	for _, author := range []string{"mem-1", "mem-2"} {
		if _, err := m.PostChatMessageFor(ctx, "s-1", threadID, author, "hello from "+author, "", ""); err != nil {
			t.Fatalf("PostChatMessageFor %s: %v", author, err)
		}
	}
	if err := m.HideActorMessages(ctx, "mem-1"); err != nil {
		t.Fatalf("HideActorMessages: %v", err)
	}
	shown, err := m.ListChatMessagesFor(ctx, "s-1", threadID, "mem-2", "", "", 0)
	if err != nil {
		t.Fatalf("ListChatMessagesFor: %v", err)
	}
	if len(shown) != 1 || shown[0].AuthorID != "mem-2" {
		t.Fatalf("listed to the neighbour = %+v, want only mem-2's", shown)
	}
}
