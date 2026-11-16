package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cyberbeast/codepresent/pkg/presentation"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// PresentStoryInput is the input schema for the present_code_story tool.
type PresentStoryInput struct {
	Story    presentation.Story `json:"story" jsonschema:"Structured multi-step code walkthrough"`
	AutoPlay *bool              `json:"auto_play,omitempty" jsonschema:"Whether to start auto-advancing immediately (defaults to true)"`
}

// ControlInput is the input schema for the control_presentation tool.
type ControlInput struct {
	Action    string `json:"action" jsonschema:"Playback action: play, pause, next, prev, stop, or goto"`
	StepIndex *int   `json:"step_index,omitempty" jsonschema:"0-indexed step to jump to when action is goto"`
}

// EmptyInput represents a tool with no required arguments.
type EmptyInput struct{}

// NewServer creates and configures an MCP server backed by the given presentation engine.
func NewServer(engine *presentation.Engine) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "codepresent",
		Version: "0.1.0",
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "present_code_story",
		Description: "Launch an interactive, step-by-step highlighted code presentation in the connected editor or terminal. Prefer focused 5-15 line steps (breaking long functions into multiple sub-steps) for clearer visual spotlighting.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in PresentStoryInput) (*mcp.CallToolResult, any, error) {
		autoPlay := true
		if in.AutoPlay != nil {
			autoPlay = *in.AutoPlay
		}
		if err := engine.LoadStory(in.Story, autoPlay); err != nil {
			return nil, nil, err
		}
		state := engine.State()
		return textResult(fmt.Sprintf("Loaded presentation %q (%d steps). Active step 1: %s (%s:%d-%d)",
			state.StoryTitle,
			state.TotalSteps,
			state.CurrentStep.Title,
			state.CurrentStep.FilePath,
			state.CurrentStep.StartLine,
			state.CurrentStep.EndLine,
		)), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "control_presentation",
		Description: "Control playback of the active code presentation (play, pause, next, prev, stop, goto).",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in ControlInput) (*mcp.CallToolResult, any, error) {
		switch in.Action {
		case "play":
			if err := engine.Play(); err != nil {
				return nil, nil, err
			}
		case "pause":
			if err := engine.Pause(); err != nil {
				return nil, nil, err
			}
		case "next":
			moved, err := engine.Next()
			if err != nil {
				return nil, nil, err
			}
			if !moved {
				return textResult("Already at the last step."), nil, nil
			}
		case "prev":
			moved, err := engine.Prev()
			if err != nil {
				return nil, nil, err
			}
			if !moved {
				return textResult("Already at the first step."), nil, nil
			}
		case "goto":
			if in.StepIndex == nil {
				return nil, nil, fmt.Errorf("step_index is required when action is goto")
			}
			if err := engine.GoTo(*in.StepIndex); err != nil {
				return nil, nil, err
			}
		case "stop":
			engine.Stop()
			return textResult("Presentation stopped."), nil, nil
		default:
			return nil, nil, fmt.Errorf("unknown action %q: expected play, pause, next, prev, stop, or goto", in.Action)
		}

		state := engine.State()
		return textResult(fmt.Sprintf("Action %q applied. Now at step %d/%d (playing=%v).",
			in.Action, state.CurrentStepIndex+1, state.TotalSteps, state.IsPlaying)), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_presentation_status",
		Description: "Get the current state of the active code presentation.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, any, error) {
		data, err := json.MarshalIndent(engine.State(), "", "  ")
		if err != nil {
			return nil, nil, err
		}
		return textResult(string(data)), nil, nil
	})

	return server
}

func textResult(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: msg},
		},
	}
}
