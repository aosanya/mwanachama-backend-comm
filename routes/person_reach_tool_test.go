package routes_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/models"
	"github.com/aosanya/mwanachama-backend-comm/routes"
)

const personReachAction = "comm.person_reach.list"

func personReachTool(t *testing.T, m *mwanachamacomm.CommManager, mount routes.Mount) routes.Tool {
	t.Helper()
	tools, err := routes.BuildChatTools(m, mount)
	if err != nil {
		t.Fatalf("BuildChatTools: %v", err)
	}
	for _, tool := range tools {
		if tool.Action == personReachAction {
			return tool
		}
	}
	t.Fatalf("no chat tool is built from %s", personReachAction)
	return routes.Tool{}
}

func TestThePersonReachToolIsBuiltFromItsActionAndNamesThePerson(t *testing.T) {
	tool := personReachTool(t, nil, routes.Mount{})
	if tool.Name != "comm_person_reach_list" {
		t.Errorf("the tool is named %q, want comm_person_reach_list", tool.Name)
	}
	var schema schemaOf
	if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
		t.Fatal(err)
	}
	if _, ok := schema.Properties["actor_id"]; !ok {
		t.Errorf("the tool takes no actor_id: %s", tool.InputSchema)
	}
	if len(schema.Required) != 1 || schema.Required[0] != "actor_id" {
		t.Errorf("required = %v, want [actor_id]", schema.Required)
	}
}

func TestThePersonReachToolAnswersOnlyACallerWhoseGateHoldsItsAction(t *testing.T) {
	_, m := newCommServer(t)
	m.UseBelonging(belongingOf(map[string][]string{"s-1": {"asker-b"}}))
	m.UseLeading(leadingOf(nil))
	m.UseRoomsOf(roomsOfPeople(map[string][]string{"asker-b": {"s-1"}}))
	seedChatThread(t, m, "s-1", "general")
	args := json.RawMessage(`{"actor_id":"asker-b"}`)
	holding := func(held ...string) dispatch.Authorizer {
		return func(_ context.Context, action string) error {
			for _, h := range held {
				if h == action {
					return nil
				}
			}
			return dispatch.ErrForbidden
		}
	}

	refused := personReachTool(t, m, routes.Mount{Authorize: holding("comm.chat_thread.list")})
	if _, err := refused.Invoke(context.Background(), args); !errors.Is(err, dispatch.ErrForbidden) {
		t.Fatalf("a caller holding another comm action got %v, want ErrForbidden", err)
	}

	allowed := personReachTool(t, m, routes.Mount{Authorize: holding(personReachAction)})
	out, err := allowed.Invoke(context.Background(), args)
	if err != nil {
		t.Fatalf("a caller holding %s got %v", personReachAction, err)
	}
	rows, ok := out.([]models.ChatReach)
	if !ok || len(rows) != 1 || rows[0].Reason != models.ChatReachGroup {
		t.Fatalf("the reach of asker-b = %#v, want her one group thread", out)
	}
}
