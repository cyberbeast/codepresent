package tui

import (
	"context"
	"fmt"
	"sync"

	"github.com/cyberbeast/codepresent/pkg/bridge"
	"github.com/cyberbeast/codepresent/pkg/presentation"
	"github.com/gorilla/websocket"
)

// WSController implements Controller over a WebSocket connection to a running codepresent server.
type WSController struct {
	conn *websocket.Conn

	mu          sync.RWMutex
	state       presentation.State
	subscribers map[uint64]chan presentation.Event
	nextSubID   uint64
}

// RunWS connects to a running codepresent WebSocket bridge and launches the TUI.
func RunWS(ctx context.Context, wsURL string) error {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("connect to %s: %w", wsURL, err)
	}
	defer conn.Close()

	ctrl := &WSController{
		conn:        conn,
		subscribers: make(map[uint64]chan presentation.Event),
	}

	go ctrl.readLoop(ctx)
	return Run(ctx, ctrl)
}

func (c *WSController) readLoop(ctx context.Context) {
	defer c.closeSubscribers()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		var evt presentation.Event
		if err := c.conn.ReadJSON(&evt); err != nil {
			return
		}
		c.mu.Lock()
		c.state = evt.State
		for _, ch := range c.subscribers {
			select {
			case ch <- evt:
			default:
			}
		}
		c.mu.Unlock()
	}
}

func (c *WSController) closeSubscribers() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, ch := range c.subscribers {
		delete(c.subscribers, id)
		close(ch)
	}
}

func (c *WSController) State() presentation.State {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

func (c *WSController) Subscribe() (<-chan presentation.Event, func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	id := c.nextSubID
	c.nextSubID++
	ch := make(chan presentation.Event, 16)
	c.subscribers[id] = ch
	return ch, func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if sub, ok := c.subscribers[id]; ok {
			delete(c.subscribers, id)
			close(sub)
		}
	}
}

func (c *WSController) Play() error  { return c.send("play", nil) }
func (c *WSController) Pause() error { return c.send("pause", nil) }
func (c *WSController) Next() (bool, error) {
	return true, c.send("next", nil)
}
func (c *WSController) Prev() (bool, error) {
	return true, c.send("prev", nil)
}
func (c *WSController) GoTo(index int) error {
	return c.send("goto", &index)
}
func (c *WSController) Stop() {
	_ = c.send("stop", nil)
}

func (c *WSController) send(action string, stepIndex *int) error {
	return c.conn.WriteJSON(bridge.ClientCommand{
		Action:    action,
		StepIndex: stepIndex,
	})
}
