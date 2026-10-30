package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"

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
		out, err := t.Handle(ctx, dispatch.Request{Arguments: req.Params.Arguments, Meta: metaOf(req)})
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

func metaOf(req *mcp.CallToolRequest) map[string]any {
	if req == nil || req.Params == nil {
		return nil
	}
	return req.Params.Meta
}

func envelope(name string, out any) (any, string) {
	rv := reflect.ValueOf(out)
	if rv.IsValid() && (rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array) {
		return map[string]any{"items": out}, fmt.Sprintf("Found %d item(s)", rv.Len())
	}
	return out, fmt.Sprintf("%s completed", name)
}
