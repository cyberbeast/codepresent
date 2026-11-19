package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/cyberbeast/codepresent/pkg/bridge"
	"github.com/cyberbeast/codepresent/pkg/mcpserver"
	"github.com/cyberbeast/codepresent/pkg/presentation"
	"github.com/cyberbeast/codepresent/pkg/registry"
	"github.com/cyberbeast/codepresent/pkg/tui"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	wsAddr := flag.String("ws-addr", "", "WebSocket address for display drivers (default: auto-bind 127.0.0.1:0, or discover via session registry in --tui mode)")
	demoMode := flag.Bool("demo", false, "Run a built-in demo presentation without starting MCP stdio")
	tuiMode := flag.Bool("tui", false, "Launch the Bubble Tea terminal presentation viewer")
	flag.Parse()

	log.SetOutput(os.Stderr)
	log.SetPrefix("[codepresent] ")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *tuiMode && !*demoMode {
		wsURL := ""
		if *wsAddr != "" {
			wsURL = "ws://" + *wsAddr + "/ws"
		} else {
			var err error
			wsURL, err = discoverWSURL(ctx)
			if err != nil {
				log.Fatalf("Session discovery failed: %v", err)
			}
		}
		if err := tui.RunWS(ctx, wsURL); err != nil {
			log.Fatalf("TUI exited with error: %v", err)
		}
		return
	}

	engine := presentation.NewEngine()
	wsBridge := bridge.NewWSBridge(engine)

	bindAddr := *wsAddr
	if bindAddr == "" {
		bindAddr = "127.0.0.1:0"
	}
	ln, err := net.Listen("tcp", bindAddr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", bindAddr, err)
	}
	wsURL := "ws://" + ln.Addr().String() + "/ws"

	wd, _ := os.Getwd()
	startedAt := time.Now().UTC()
	regDir, regErr := registry.DefaultDir()
	if regErr == nil {
		session := registry.Session{
			PID:           os.Getpid(),
			WSURL:         wsURL,
			WorkspaceRoot: wd,
			StartedAt:     startedAt,
		}
		if err := registry.Write(regDir, session); err != nil {
			log.Printf("Warning: failed to write session registry: %v", err)
		} else {
			defer func() { _ = registry.Remove(regDir, os.Getpid()) }()
			go syncSessionUpdates(ctx, engine, regDir, wsURL, wd, startedAt)
		}
	}

	go func() {
		log.Printf("Starting WebSocket display bridge on %s", wsURL)
		if err := wsBridge.Serve(ctx, ln); err != nil {
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

func syncSessionUpdates(ctx context.Context, engine *presentation.Engine, regDir, wsURL, fallbackWD string, startedAt time.Time) {
	events, unsub := engine.Subscribe()
	defer unsub()

	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-events:
			if !ok {
				return
			}
			if evt.Type != presentation.EventStoryLoaded {
				continue
			}
			root := evt.State.WorkspaceRoot
			if root == "" {
				root = fallbackWD
			}
			_ = registry.Write(regDir, registry.Session{
				PID:           os.Getpid(),
				WSURL:         wsURL,
				WorkspaceRoot: root,
				StoryTitle:    evt.State.StoryTitle,
				StartedAt:     startedAt,
			})
		}
	}
}

func discoverWSURL(ctx context.Context) (string, error) {
	regDir, err := registry.DefaultDir()
	if err != nil {
		return "", err
	}
	wd, _ := os.Getwd()

	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()

	waited := false
	for {
		sessions, err := registry.List(regDir)
		if err != nil {
			return "", err
		}
		if len(sessions) > 0 {
			if match, ok := registry.SelectForWorkspace(sessions, wd); ok {
				return match.WSURL, nil
			}
			return promptSessionChoice(sessions), nil
		}

		if !waited {
			fmt.Fprintln(os.Stderr, "Waiting for an active codepresent session...")
			waited = true
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
		}
	}
}

func promptSessionChoice(sessions []registry.Session) string {
	fmt.Fprintln(os.Stderr, "Multiple active codepresent sessions found:")
	for i, s := range sessions {
		label := s.WorkspaceRoot
		if label == "" {
			label = "(unknown workspace)"
		}
		if s.StoryTitle != "" {
			label = fmt.Sprintf("%s — %s", label, s.StoryTitle)
		}
		fmt.Fprintf(os.Stderr, "  [%d] %s (pid %d)\n", i+1, label, s.PID)
	}
	fmt.Fprintf(os.Stderr, "Select session [1-%d] (default 1): ", len(sessions))

	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		choice, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
		if err == nil && choice >= 1 && choice <= len(sessions) {
			return sessions[choice-1].WSURL
		}
	}
	return sessions[0].WSURL
}

func demoStory() presentation.Story {
	wd, _ := os.Getwd()
	return presentation.Story{
		Title:         "How codepresent Works",
		WorkspaceRoot: wd,
		Steps: []presentation.Step{
			{
				FilePath:        "cmd/codepresent/main.go",
				StartLine:       85,
				EndLine:         90,
				FocusTarget:     "wsBridge.Serve",
				Title:           "1. Engine & WebSocket Bridge",
				Narration:       "The entrypoint creates a presentation.Engine, binds an ephemeral local port, registers the session for discovery, and serves a WebSocket bridge.",
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
