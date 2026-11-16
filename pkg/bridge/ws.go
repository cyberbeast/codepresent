package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"sync"

	"github.com/cyberbeast/codepresent/pkg/presentation"
	"github.com/gorilla/websocket"
)

// ClientCommand represents an interactive playback command sent by a connected display driver.
type ClientCommand struct {
	Action    string `json:"action"`
	StepIndex *int   `json:"step_index,omitempty"`
}

// WSBridge exposes a local WebSocket server that streams presentation events to connected
// editor display drivers (e.g. VS Code) and accepts playback commands back from the UI.
type WSBridge struct {
	engine   *presentation.Engine
	upgrader websocket.Upgrader

	mu      sync.Mutex
	clients map[*websocket.Conn]struct{}
}

// NewWSBridge creates a new WebSocket bridge backed by the given presentation engine.
func NewWSBridge(engine *presentation.Engine) *WSBridge {
	return &WSBridge{
		engine:  engine,
		clients: make(map[*websocket.Conn]struct{}),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

// ListenAndServe starts the WebSocket HTTP server on addr and blocks until ctx is cancelled.
func (b *WSBridge) ListenAndServe(ctx context.Context, addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return b.Serve(ctx, ln)
}

// Serve serves WebSocket connections on an existing net.Listener until ctx is cancelled.
func (b *WSBridge) Serve(ctx context.Context, ln net.Listener) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", b.handleWS)
	mux.HandleFunc("/ws", b.handleWS)

	srv := &http.Server{Handler: mux}

	events, unsub := b.engine.Subscribe()
	defer unsub()

	go func() {
		for evt := range events {
			b.broadcast(evt)
		}
	}()

	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
		b.closeAll()
	}()

	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (b *WSBridge) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := b.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade error: %v", err)
		return
	}

	b.addClient(conn)
	defer b.removeClient(conn)

	// Send current state immediately if a presentation is already active.
	state := b.engine.State()
	if state.TotalSteps > 0 {
		_ = conn.WriteJSON(presentation.Event{
			Type:  presentation.EventStepChanged,
			State: state,
		})
	}

	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var cmd ClientCommand
		if err := json.Unmarshal(payload, &cmd); err != nil {
			log.Printf("invalid client command: %v", err)
			continue
		}
		b.dispatchCommand(cmd)
	}
}

func (b *WSBridge) dispatchCommand(cmd ClientCommand) {
	switch cmd.Action {
	case "play":
		_ = b.engine.Play()
	case "pause":
		_ = b.engine.Pause()
	case "next":
		_, _ = b.engine.Next()
	case "prev":
		_, _ = b.engine.Prev()
	case "stop":
		b.engine.Stop()
	case "goto":
		if cmd.StepIndex != nil {
			_ = b.engine.GoTo(*cmd.StepIndex)
		}
	default:
		log.Printf("unknown client command action: %q", cmd.Action)
	}
}

func (b *WSBridge) addClient(conn *websocket.Conn) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.clients[conn] = struct{}{}
}

func (b *WSBridge) removeClient(conn *websocket.Conn) {
	b.mu.Lock()
	delete(b.clients, conn)
	b.mu.Unlock()
	_ = conn.Close()
}

func (b *WSBridge) closeAll() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for conn := range b.clients {
		_ = conn.Close()
		delete(b.clients, conn)
	}
}

func (b *WSBridge) broadcast(evt presentation.Event) {
	b.mu.Lock()
	conns := make([]*websocket.Conn, 0, len(b.clients))
	for conn := range b.clients {
		conns = append(conns, conn)
	}
	b.mu.Unlock()

	for _, conn := range conns {
		if err := conn.WriteJSON(evt); err != nil {
			b.removeClient(conn)
		}
	}
}
