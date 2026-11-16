package routes

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"
)

func TestEveryCommSentinelHasADeclaredStatus(t *testing.T) {
	unmapped, err := Table.UnmappedSentinels()
	if err != nil {
		t.Fatalf("spec: %v", err)
	}
	if len(unmapped) != 0 {
		t.Errorf("sentinels with no declared status: %v", unmapped)
	}
}

func TestNoCommAddressIsAnonymous(t *testing.T) {
	if len(AnonymousActions) != 0 {
		t.Fatalf("AnonymousActions = %v, want none", AnonymousActions)
	}
	if got := PublicRoutes(nil); len(got) != 0 {
		t.Errorf("PublicRoutes served %d anonymous addresses, want 0", len(got))
	}
	unknown, err := Table.UnknownAnonymousActions()
	if err != nil || len(unknown) != 0 {
		t.Errorf("unknown anonymous actions = %v, err = %v", unknown, err)
	}
}

func TestSplitPutsEveryAddressBehindTheGate(t *testing.T) {
	split := Split(nil, Mount{})
	shape, err := Shape()
	if err != nil {
		t.Fatalf("shape: %v", err)
	}
	if len(split.Anonymous) != 0 {
		t.Errorf("%d anonymous routes, want 0", len(split.Anonymous))
	}
	if len(split.Gated) != len(shape) {
		t.Errorf("gated %d routes, shape declares %d", len(split.Gated), len(shape))
	}
	if got := OperatorRoutes(nil, Mount{}); len(got) != len(shape) {
		t.Errorf("OperatorRoutes returned %d, want %d", len(got), len(shape))
	}
}

func TestShapeCarriesEveryAddressOnceWithAnAction(t *testing.T) {
	shape, err := Shape()
	if err != nil {
		t.Fatalf("shape: %v", err)
	}
	if len(shape) == 0 {
		t.Fatal("shape is empty")
	}
	seen := map[string]bool{}
	for _, route := range shape {
		if route.Action == "" {
			t.Errorf("%s %s declares no action", route.Method, route.Path)
		}
		key := route.Method + " " + route.Path
		if seen[key] {
			t.Errorf("%s declared twice", key)
		}
		seen[key] = true
	}
}

func TestBuildAndRoutesAgree(t *testing.T) {
	built, err := Build(nil)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if routes := Routes(nil); len(routes) != len(built) {
		t.Errorf("Routes %d, Build %d", len(routes), len(built))
	}
	mounted, err := BuildFor(nil, Mount{Authorize: func(context.Context, string) error { return nil }})
	if err != nil || len(mounted) != len(built) {
		t.Errorf("BuildFor = %d routes, err %v; want %d", len(mounted), err, len(built))
	}
}

func TestAnAuthorizerThatRefusesAnswers403BeforeTheManagerIsTouched(t *testing.T) {
	refuse := func(context.Context, string) error { return errors.New("not yours") }
	asked := map[string]bool{}
	mount := Mount{Authorize: func(ctx context.Context, action string) error {
		asked[action] = true
		return refuse(ctx, action)
	}}
	for _, route := range RoutesFor(nil, mount) {
		if route.Action != "comm.chat_thread.list" {
			continue
		}
		rec := httptest.NewRecorder()
		route.Handler(rec, httptest.NewRequest(route.Method, "/", nil))
		if rec.Code != http.StatusForbidden {
			t.Errorf("refused caller got %d, want 403", rec.Code)
		}
		if !asked["comm.chat_thread.list"] {
			t.Error("the authorizer was never asked about the declared action")
		}
		return
	}
	t.Fatal("no route for comm.chat_thread.list")
}

func TestChatToolsAreExactlyTheChatActions(t *testing.T) {
	tools, err := BuildChatTools(nil, Mount{})
	if err != nil {
		t.Fatalf("tools: %v", err)
	}
	var got []string
	for _, tool := range tools {
		got = append(got, tool.Action)
		if tool.Name == "" || tool.Invoke == nil && tool.Handle == nil {
			t.Errorf("tool for %s has no name or handler", tool.Action)
		}
	}
	want := append([]string(nil), ChatActions...)
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("got %d tools %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("tool %d is %s, want %s", i, got[i], want[i])
		}
	}
	if len(ChatTools(nil, Mount{})) != len(want) {
		t.Error("ChatTools disagrees with BuildChatTools")
	}
}

func TestChatToolsAreGatedByTheMountAuthorizer(t *testing.T) {
	mount := Mount{Authorize: func(context.Context, string) error { return errors.New("no") }}
	tools, err := BuildChatTools(nil, mount)
	if err != nil {
		t.Fatalf("tools: %v", err)
	}
	for _, tool := range tools {
		var gotErr error
		if tool.Invoke != nil {
			_, gotErr = tool.Invoke(context.Background(), []byte(`{}`))
		} else {
			_, gotErr = tool.Handle(context.Background(), dispatch.Request{Arguments: []byte(`{}`)})
		}
		if !errors.Is(gotErr, dispatch.ErrForbidden) {
			t.Errorf("%s ran for a refused caller: %v", tool.Action, gotErr)
		}
	}
}
