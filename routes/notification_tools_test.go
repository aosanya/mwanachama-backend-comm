package routes_test

import (
	"encoding/json"
	"testing"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/routes"
)

type schemaOf struct {
	Properties map[string]json.RawMessage `json:"properties"`
	Required   []string                   `json:"required"`
}

func chatToolsByName(t *testing.T) map[string]routes.Tool {
	t.Helper()
	tools, err := routes.BuildChatTools((*mwanachamacomm.CommManager)(nil), routes.Mount{})
	if err != nil {
		t.Fatalf("BuildChatTools: %v", err)
	}
	byName := map[string]routes.Tool{}
	for _, tool := range tools {
		byName[tool.Name] = tool
	}
	return byName
}

func TestTheNotificationActionsBuildTheToolsTheMemberAppCalls(t *testing.T) {
	byName := chatToolsByName(t)
	for name, action := range map[string]string{
		"comm_notification_list":      "comm.notification.list",
		"comm_notification_unread":    "comm.notification.unread",
		"comm_notification_mark_read": "comm.notification.mark_read",
	} {
		tool, ok := byName[name]
		if !ok {
			t.Errorf("no chat tool named %s", name)
			continue
		}
		if tool.Action != action {
			t.Errorf("%s is built from %s, want %s", name, tool.Action, action)
		}
	}
}

func TestTheNotificationToolsTakeTheArgumentsTheMemberAppSends(t *testing.T) {
	byName := chatToolsByName(t)
	markRead, ok := byName["comm_notification_mark_read"]
	if !ok {
		t.Fatal("no chat tool named comm_notification_mark_read")
	}
	var schema schemaOf
	if err := json.Unmarshal(markRead.InputSchema, &schema); err != nil {
		t.Fatal(err)
	}
	if _, ok := schema.Properties["ids"]; !ok {
		t.Errorf("comm_notification_mark_read takes no ids: %s", markRead.InputSchema)
	}
	if len(schema.Required) != 1 || schema.Required[0] != "ids" {
		t.Errorf("comm_notification_mark_read requires %v, want only ids", schema.Required)
	}
	for _, name := range []string{"comm_notification_list", "comm_notification_unread", "comm_notification_mark_read"} {
		tool, ok := byName[name]
		if !ok {
			continue
		}
		var own schemaOf
		if err := json.Unmarshal(tool.InputSchema, &own); err != nil {
			t.Fatal(err)
		}
		if _, ok := own.Properties["caller_id"]; ok {
			t.Errorf("%s asks the client for the recipient: %s", name, tool.InputSchema)
		}
	}
}
