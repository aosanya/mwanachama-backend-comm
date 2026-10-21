package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	mwanachamacomm "github.com/aosanya/mwanachama-backend-comm"
	"github.com/aosanya/mwanachama-backend-comm/routes"
)

func RegisterChatTools(server *mcp.Server, m *mwanachamacomm.CommManager, mount routes.Mount) {
	for _, t := range routes.ChatTools(m, mount) {
		var schema any
		if err := json.Unmarshal(t.InputSchema, &schema); err != nil {
			panic(fmt.Sprintf("comm mcp: %s input schema: %v", t.Name, err))
		}
		server.AddTool(&mcp.Tool{
			Name:        t.Name,
			Title:       t.Title,
			Description: t.Description,
			InputSchema: schema,
		}, handlerFor(t))
	}
}

func handlerFor(t routes.Tool) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		out, err := t.Invoke(ctx, req.Params.Arguments)
		if err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
			}, nil
		}
		structured, text := envelope(t.Name, out)
		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: text}},
			StructuredContent: structured,
		}, nil
	}
}

func envelope(name string, out any) (any, string) {
	rv := reflect.ValueOf(out)
	if rv.IsValid() && (rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array) {
		return map[string]any{"items": out}, fmt.Sprintf("Found %d item(s)", rv.Len())
	}
	return out, fmt.Sprintf("%s completed", name)
}
