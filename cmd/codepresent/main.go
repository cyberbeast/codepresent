package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/cyberbeast/codepresent/pkg/bridge"
	"github.com/cyberbeast/codepresent/pkg/mcpserver"
	"github.com/cyberbeast/codepresent/pkg/presentation"
	"github.com/cyberbeast/codepresent/pkg/tui"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	wsAddr := flag.String("ws-addr", "127.0.0.1:9123", "WebSocket address for editor display drivers")
	demoMode := flag.Bool("demo", false, "Run a built-in demo presentation without starting MCP stdio")
	tuiMode := flag.Bool("tui", false, "Launch the Bubble Tea terminal presentation viewer")
	flag.Parse()

	log.SetOutput(os.Stderr)
	log.SetPrefix("[codepresent] ")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *tuiMode && !*demoMode {
		wsURL := "ws://" + *wsAddr + "/ws"
		if err := tui.RunWS(ctx, wsURL); err != nil {
			log.Fatalf("TUI exited with error: %v", err)
		}
		return
	}

	engine := presentation.NewEngine()
	wsBridge := bridge.NewWSBridge(engine)

	go func() {
		log.Printf("Starting WebSocket display bridge on ws://%s/ws", *wsAddr)
		if err := wsBridge.ListenAndServe(ctx, *wsAddr); err != nil {
			log.Printf("WebSocket bridge stopped: %v", err)
		}
	}()

	if *demoMode {
		log.Println("Demo mode active: loading built-in codepresent walkthrough on Step 1...")
		if err := engine.LoadStory(demoStory(), false); err != nil {
			log.Fatalf("Failed to load demo story: %v", err)
		}
		if *tuiMode {
			if err := tui.Run(ctx, engine); err != nil {
				log.Fatalf("TUI exited with error: %v", err)
			}
			return
		}
		<-ctx.Done()
		return
	}

	server := mcpserver.NewServer(engine)

	log.Println("Starting codepresent MCP server on stdio...")
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		log.Fatalf("MCP server exited with error: %v", err)
	}
}

func demoStory() presentation.Story {
	wd, _ := os.Getwd()
	return presentation.Story{
		Title:         "How codepresent Works",
		WorkspaceRoot: wd,
		Steps: []presentation.Step{
			{
				FilePath:        "cmd/codepresent/main.go",
				StartLine:       38,
				EndLine:         46,
				FocusTarget:     "presentation.NewEngine + bridge.NewWSBridge",
				Title:           "1. Engine & WebSocket Bridge",
				Narration:       "The entrypoint creates a single presentation.Engine and attaches a local WebSocket bridge on 127.0.0.1:9123 so VS Code can subscribe to live step events.",
				DurationSeconds: 6,
			},
			{
				FilePath:        "pkg/presentation/engine.go",
				StartLine:       77,
				EndLine:         97,
				FocusTarget:     "Engine.LoadStory",
				Title:           "2. Loading & Validating a Story",
				Narration:       "When an AI agent invokes present_code_story, LoadStory validates line ranges, starts the auto-advance timer if autoPlay is enabled, and broadcasts EventStoryLoaded.",
				DurationSeconds: 6,
			},
			{
				FilePath:        "pkg/bridge/ws.go",
				StartLine:       81,
				EndLine:         112,
				FocusTarget:     "WSBridge.handleWS",
				Title:           "3. Two-Way Editor Sync",
				Narration:       "When VS Code connects, the bridge immediately syncs the current step and listens for Play, Pause, Next, Prev, and Stop commands sent from the editor UI.",
				DurationSeconds: 6,
			},
		},
	}
}
