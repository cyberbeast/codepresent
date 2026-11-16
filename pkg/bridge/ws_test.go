package bridge

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/cyberbeast/codepresent/pkg/presentation"
	"github.com/gorilla/websocket"
)

func TestWSBridgeRoundTrip(t *testing.T) {
	eng := presentation.NewEngine()
	br := NewWSBridge(eng)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = br.Serve(ctx, ln)
	}()

	wsURL := "ws://" + ln.Addr().String() + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("websocket.Dial failed: %v", err)
	}
	defer conn.Close()

	story := presentation.Story{
		Title: "Bridge Test",
		Steps: []presentation.Step{
			{FilePath: "main.go", StartLine: 1, EndLine: 5, Title: "Step 1", Narration: "First", DurationSeconds: 10},
			{FilePath: "main.go", StartLine: 6, EndLine: 10, Title: "Step 2", Narration: "Second", DurationSeconds: 10},
		},
	}

	if err := eng.LoadStory(story, true); err != nil {
		t.Fatalf("LoadStory failed: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var evt presentation.Event
	if err := conn.ReadJSON(&evt); err != nil {
		t.Fatalf("ReadJSON failed: %v", err)
	}
	if evt.Type != presentation.EventStoryLoaded || evt.State.CurrentStepIndex != 0 {
		t.Fatalf("unexpected initial event: %+v", evt)
	}

	// Send "next" from display driver
	if err := conn.WriteJSON(ClientCommand{Action: "next"}); err != nil {
		t.Fatalf("WriteJSON next failed: %v", err)
	}

	if err := conn.ReadJSON(&evt); err != nil {
		t.Fatalf("ReadJSON after next failed: %v", err)
	}
	if evt.Type != presentation.EventStepChanged || evt.State.CurrentStepIndex != 1 {
		t.Fatalf("expected step 1 after next, got %+v", evt)
	}

	// Send "pause" from display driver
	if err := conn.WriteJSON(ClientCommand{Action: "pause"}); err != nil {
		t.Fatalf("WriteJSON pause failed: %v", err)
	}

	if err := conn.ReadJSON(&evt); err != nil {
		t.Fatalf("ReadJSON after pause failed: %v", err)
	}
	if evt.Type != presentation.EventPaused || evt.State.IsPlaying {
		t.Fatalf("expected paused state, got %+v", evt)
	}
}
