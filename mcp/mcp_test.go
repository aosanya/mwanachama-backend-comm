package mcp_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	gosdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
	commmcp "github.com/aosanya/mwanachama-backend-comm/mcp"
	"github.com/aosanya/mwanachama-backend-comm/routes"
)

type noopActWriter struct{}

func (noopActWriter) WriteAct(context.Context, *sql.Tx, mwanachamacomm.ActEntry) error { return nil }

func newManager(t *testing.T) *mwanachamacomm.CommManager {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	s, err := mwanachamacomm.SpecFor("mwanachama")
	if err != nil {
		t.Fatalf("SpecFor: %v", err)
	}
	if err := mwanachamacomm.Provision(db, s); err != nil {
		t.Fatalf("Provision: %v", err)
	}
	m, err := mwanachamacomm.NewCommManager(db, s, nil, noopActWriter{}, "")
	if err != nil {
		t.Fatalf("NewCommManager: %v", err)
	}
	members := map[string]bool{"actor-a": true, "actor-b": true}
	m.UseBelonging(func(_ context.Context, actorID, structureID string) (bool, error) {
		return structureID == "s-1" && members[actorID], nil
	})
	m.UseLeading(func(_ context.Context, actorID, structureID string) (bool, error) {
		return structureID == "s-1" && actorID == "actor-a", nil
	})
	return m
}

func connect(t *testing.T, m *mwanachamacomm.CommManager, who *string) *gosdkmcp.ClientSession {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	server := gosdkmcp.NewServer(&gosdkmcp.Implementation{Name: "comm", Version: "0.0.0"}, nil)
	commmcp.RegisterChatTools(server, m, routes.Mount{Caller: func(context.Context) string { return *who }})
	st, ct := gosdkmcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, st, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := gosdkmcp.NewClient(&gosdkmcp.Implementation{Name: "test-client", Version: "0.0.0"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func callTool(t *testing.T, cs *gosdkmcp.ClientSession, name string, args map[string]any) (*gosdkmcp.CallToolResult, map[string]any) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &gosdkmcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	out := map[string]any{}
	if res.StructuredContent != nil {
		raw, _ := json.Marshal(res.StructuredContent)
		_ = json.Unmarshal(raw, &out)
	}
	return res, out
}

func TestTheChatToolsAreListedWithNoSessionFactInTheirSchemas(t *testing.T) {
	who := "actor-a"
	cs := connect(t, newManager(t), &who)
	listed, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	want := map[string]bool{
		"comm_chat_thread_list": true, "comm_chat_message_list": true, "comm_chat_message_post": true,
		"comm_chat_thread_mark_read": true, "comm_chat_thread_create": true,
		"comm_chat_thread_list_archived": true, "comm_chat_thread_archive": true, "comm_chat_thread_unarchive": true,
	}
	if len(listed.Tools) != len(want) {
		t.Fatalf("listed %d tools, want %d", len(listed.Tools), len(want))
	}
	for _, tl := range listed.Tools {
		if !want[tl.Name] {
			t.Errorf("unexpected tool %q", tl.Name)
		}
		raw, _ := json.Marshal(tl.InputSchema)
		for _, fact := range []string{"caller", "author_id", "requested_by", "reader_id", "device"} {
			if strings.Contains(string(raw), `"`+fact+`"`) {
				t.Errorf("%s schema names %q, a session fact: %s", tl.Name, fact, raw)
			}
		}
	}
}

func TestAMemberStartsPostsListsAndMarksReadThroughTheTools(t *testing.T) {
	who := "actor-a"
	cs := connect(t, newManager(t), &who)

	res, thread := callTool(t, cs, "comm_chat_thread_create", map[string]any{"structure_id": "s-1", "label": "Road repairs"})
	if res.IsError {
		t.Fatalf("start: %+v", res.Content[0])
	}
	threadID, _ := thread["id"].(string)
	if threadID == "" {
		t.Fatalf("start returned no id: %v", thread)
	}

	res, posted := callTool(t, cs, "comm_chat_message_post", map[string]any{"structure_id": "s-1", "thread_id": threadID, "body": "hello"})
	if res.IsError {
		t.Fatalf("post: %+v", res.Content[0])
	}
	raw, _ := json.Marshal(posted)
	if !strings.Contains(string(raw), "actor-a") {
		t.Fatalf("the post is not the caller's: %s", raw)
	}

	res, threads := callTool(t, cs, "comm_chat_thread_list", map[string]any{"structure_id": "s-1"})
	if res.IsError {
		t.Fatalf("list threads: %+v", res.Content[0])
	}
	if !strings.Contains(mustJSON(threads), threadID) {
		t.Fatalf("the started thread is not listed: %s", mustJSON(threads))
	}

	res, msgs := callTool(t, cs, "comm_chat_message_list", map[string]any{"structure_id": "s-1", "thread_id": threadID})
	if res.IsError || !strings.Contains(mustJSON(msgs), "hello") {
		t.Fatalf("list messages: %v %s", res.IsError, mustJSON(msgs))
	}

	if res, _ = callTool(t, cs, "comm_chat_thread_mark_read", map[string]any{"structure_id": "s-1", "thread_id": threadID}); res.IsError {
		t.Fatalf("mark read: %+v", res.Content[0])
	}
	_, threads = callTool(t, cs, "comm_chat_thread_list", map[string]any{"structure_id": "s-1"})
	if !strings.Contains(mustJSON(threads), `"unread":0`) {
		t.Fatalf("after mark read: %s", mustJSON(threads))
	}
}

func TestAnOutsiderIsFencedOutThroughTheTools(t *testing.T) {
	who := "stranger"
	cs := connect(t, newManager(t), &who)
	res, _ := callTool(t, cs, "comm_chat_thread_list", map[string]any{"structure_id": "s-1"})
	if !res.IsError {
		t.Fatalf("an outsider listed the room's threads")
	}
	if text := res.Content[0].(*gosdkmcp.TextContent).Text; !strings.Contains(strings.ToLower(text), "not found") {
		t.Fatalf("the refusal is %q, want the same not-found an invented room gets", text)
	}
}

func mustJSON(v any) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}

func TestAMemberArchivesAndBringsBackAThreadThroughTheTools(t *testing.T) {
	who := "actor-a"
	cs := connect(t, newManager(t), &who)
	_, thread := callTool(t, cs, "comm_chat_thread_create", map[string]any{"structure_id": "s-1", "label": "Market day"})
	threadID, _ := thread["id"].(string)
	if threadID == "" {
		t.Fatalf("start returned no id: %v", thread)
	}

	if res, _ := callTool(t, cs, "comm_chat_thread_archive", map[string]any{"structure_id": "s-1", "thread_id": threadID}); res.IsError {
		t.Fatalf("archive: %+v", res.Content[0])
	}
	if _, threads := callTool(t, cs, "comm_chat_thread_list", map[string]any{"structure_id": "s-1"}); strings.Contains(mustJSON(threads), threadID) {
		t.Fatalf("an archived thread is still listed: %s", mustJSON(threads))
	}
	if _, archived := callTool(t, cs, "comm_chat_thread_list_archived", map[string]any{"structure_id": "s-1"}); !strings.Contains(mustJSON(archived), threadID) {
		t.Fatalf("the archived thread is not in the archived list: %s", mustJSON(archived))
	}

	if res, _ := callTool(t, cs, "comm_chat_thread_unarchive", map[string]any{"structure_id": "s-1", "thread_id": threadID}); res.IsError {
		t.Fatalf("unarchive: %+v", res.Content[0])
	}
	if _, threads := callTool(t, cs, "comm_chat_thread_list", map[string]any{"structure_id": "s-1"}); !strings.Contains(mustJSON(threads), threadID) {
		t.Fatalf("an unarchived thread is not back: %s", mustJSON(threads))
	}
}
