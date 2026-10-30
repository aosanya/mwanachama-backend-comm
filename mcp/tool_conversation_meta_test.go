package mcp_test

import (
	"context"
	"sync"
	"testing"
	"time"

	gosdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"

	commmcp "github.com/aosanya/mwanachama-backend-comm/mcp"
	"github.com/aosanya/mwanachama-backend-comm/routes"
)

func conversationsSeenBy(t *testing.T, meta gosdkmcp.Meta) []string {
	t.Helper()
	m := newManager(t)
	var mu sync.Mutex
	var seen []string
	caller := func(ctx context.Context) string {
		mu.Lock()
		seen = append(seen, dispatch.ConversationOf(ctx))
		mu.Unlock()
		return "actor-a"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	server := gosdkmcp.NewServer(&gosdkmcp.Implementation{Name: "comm", Version: "0.0.0"}, nil)
	commmcp.RegisterChatTools(server, m, routes.Mount{Caller: caller})
	st, ct := gosdkmcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, st, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	cs, err := gosdkmcp.NewClient(&gosdkmcp.Implementation{Name: "test-client", Version: "0.0.0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	if _, err := cs.CallTool(ctx, &gosdkmcp.CallToolParams{
		Meta:      meta,
		Name:      "comm_chat_thread_list",
		Arguments: map[string]any{"structure_id": "s-1"},
	}); err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	return append([]string(nil), seen...)
}

func TestAToolCallCarryingMetaConversationIdReachesTheCallerOnItsContext(t *testing.T) {
	seen := conversationsSeenBy(t, gosdkmcp.Meta{dispatch.ConversationMetaKey: "chat-42"})
	if len(seen) == 0 {
		t.Fatal("the caller was never asked")
	}
	for _, id := range seen {
		if id != "chat-42" {
			t.Fatalf("the caller saw conversations %q, want only chat-42", seen)
		}
	}
}

func TestAToolCallCarryingNoMetaReachesTheCallerWithNoConversation(t *testing.T) {
	seen := conversationsSeenBy(t, nil)
	if len(seen) == 0 {
		t.Fatal("the caller was never asked")
	}
	for _, id := range seen {
		if id != "" {
			t.Fatalf("the caller saw conversations %q, want an empty one", seen)
		}
	}
}
