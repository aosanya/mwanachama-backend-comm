package mwanachamacomm_test

import (
	"context"
	"errors"
	"testing"

	comm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-shared/consent"
)

func TestChatPost_MemberWithoutConsentCannotDoPostChatMessage(t *testing.T) {
	gate := gateFor{"mem-2": consent.ErrRequired}
	m := gatedManager(t, gate)
	m.UseBelonging(func(context.Context, string, string) (bool, error) { return true, nil })
	ctx := context.Background()
	thread, err := m.ChatStore.MintThread(ctx, "s-1", "general")
	if err != nil {
		t.Fatalf("MintThread: %v", err)
	}
	if _, err := m.MarkChatThreadReadFor(ctx, "s-1", thread.ID, "mem-2", ""); !errors.Is(err, comm.ErrConsentRequired) {
		t.Fatalf("read marker without consent = %v, want ErrConsentRequired", err)
	}
	if _, err := m.PostChatMessageFor(ctx, "s-1", thread.ID, "mem-2", "hello", "", ""); !errors.Is(err, comm.ErrConsentRequired) {
		t.Errorf("route chat post without consent = %v, want ErrConsentRequired", err)
	}
	delete(gate, "mem-2")
	if _, err := m.PostChatMessageFor(ctx, "s-1", thread.ID, "mem-2", "hello", "", ""); err != nil {
		t.Fatalf("route chat post after consent = %v, want nil", err)
	}
}

func TestChatThread_MemberWithoutConsentCannotStartOne(t *testing.T) {
	gate := gateFor{"mem-2": consent.ErrRequired, "mem-3": consent.ErrWithdrawn}
	m := gatedManager(t, gate)
	m.UseBelonging(func(context.Context, string, string) (bool, error) { return true, nil })
	ctx := context.Background()
	if _, err := m.StartChatThreadFor(ctx, "s-1", "mem-2", "ideas"); !errors.Is(err, comm.ErrConsentRequired) {
		t.Errorf("start thread without consent = %v, want ErrConsentRequired", err)
	}
	if _, err := m.StartChatThreadFor(ctx, "s-1", "mem-3", "ideas"); !errors.Is(err, comm.ErrConsentWithdrawn) {
		t.Errorf("start thread after withdrawal = %v, want ErrConsentWithdrawn", err)
	}
	delete(gate, "mem-2")
	if _, err := m.StartChatThreadFor(ctx, "s-1", "mem-2", "ideas"); errors.Is(err, comm.ErrConsentRequired) {
		t.Fatalf("start thread after consent refused for consent: %v", err)
	}
}
