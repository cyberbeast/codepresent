package mcpserver

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cyberbeast/codepresent/pkg/presentation"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPServerTools(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	engine := presentation.NewEngine()
	server := NewServer(engine)

	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	go func() {
		_ = server.Run(ctx, serverTransport)
	}()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect failed: %v", err)
	}
	defer session.Close()

	// 1. Call present_code_story
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "present_code_story",
		Arguments: map[string]any{
			"auto_play": false,
			"story": map[string]any{
				"story_title": "MCP Tool Test",
				"steps": []map[string]any{
					{
						"file_path":  "main.go",
						"start_line": 1,
						"end_line":   5,
						"title":      "Step One",
						"narration":  "First step narration.",
					},
					{
						"file_path":  "main.go",
						"start_line": 6,
						"end_line":   12,
						"title":      "Step Two",
						"narration":  "Second step narration.",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("present_code_story failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("present_code_story returned error result: %s", res.Content[0].(*mcp.TextContent).Text)
	}

	// 2. Call control_presentation (goto step 1)
	_, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "control_presentation",
		Arguments: map[string]any{
			"action":     "goto",
			"step_index": 1,
		},
	})
	if err != nil {
		t.Fatalf("control_presentation goto failed: %v", err)
	}

	// 3. Call get_presentation_status and verify JSON state
	statusRes, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_presentation_status",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("get_presentation_status failed: %v", err)
	}
	if len(statusRes.Content) == 0 {
		t.Fatal("expected non-empty status content")
	}
	tc, ok := statusRes.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected *mcp.TextContent, got %T", statusRes.Content[0])
	}

	var st presentation.State
	if err := json.Unmarshal([]byte(tc.Text), &st); err != nil {
		t.Fatalf("failed to unmarshal status JSON: %v", err)
	}
	if st.StoryTitle != "MCP Tool Test" || st.CurrentStepIndex != 1 || st.TotalSteps != 2 || st.IsPlaying {
		t.Fatalf("unexpected presentation state: %+v", st)
	}

	// 4. Stop presentation
	_, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "control_presentation",
		Arguments: map[string]any{
			"action": "stop",
		},
	})
	if err != nil {
		t.Fatalf("control_presentation stop failed: %v", err)
	}
	if engine.State().TotalSteps != 0 {
		t.Fatalf("expected stopped engine, got %+v", engine.State())
	}
}
