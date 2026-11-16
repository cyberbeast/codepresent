package presentation

import (
	"fmt"
	"sync"
	"time"
)

// EventType identifies the kind of state change emitted by the presentation engine.
type EventType string

const (
	EventStoryLoaded EventType = "story_loaded"
	EventStepChanged EventType = "step_changed"
	EventPlayed      EventType = "played"
	EventPaused      EventType = "paused"
	EventStopped     EventType = "stopped"
)

// Event is broadcast to connected renderers (VS Code extension, TUI, etc.) whenever state changes.
type Event struct {
	Type  EventType `json:"type"`
	State State     `json:"state"`
}

// State is a point-in-time snapshot of the presentation engine.
type State struct {
	StoryTitle       string `json:"story_title"`
	CurrentStepIndex int    `json:"current_step_index"`
	TotalSteps       int    `json:"total_steps"`
	IsPlaying        bool   `json:"is_playing"`
	CurrentStep      *Step  `json:"current_step,omitempty"`
}

// Engine manages the lifecycle, navigation, and auto-play timer for a code presentation.
type Engine struct {
	mu           sync.RWMutex
	story        *Story
	currentIndex int
	isPlaying    bool
	timer        *time.Timer
	subscribers  map[uint64]chan Event
	nextSubID    uint64
}

// NewEngine creates a new idle presentation engine.
func NewEngine() *Engine {
	return &Engine{
		subscribers: make(map[uint64]chan Event),
	}
}

// Subscribe registers a listener for presentation events and returns a channel and an unsubscribe function.
func (e *Engine) Subscribe() (<-chan Event, func()) {
	e.mu.Lock()
	defer e.mu.Unlock()

	id := e.nextSubID
	e.nextSubID++
	ch := make(chan Event, 16)
	e.subscribers[id] = ch

	unsub := func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		if sub, ok := e.subscribers[id]; ok {
			delete(e.subscribers, id)
			close(sub)
		}
	}
	return ch, unsub
}

// LoadStory validates and loads a new presentation story, optionally starting auto-play immediately.
func (e *Engine) LoadStory(story Story, autoPlay bool) error {
	if err := story.Validate(); err != nil {
		return err
	}

	e.mu.Lock()
	e.stopTimerLocked()
	copied := story
	copied.Steps = append([]Step(nil), story.Steps...)
	e.story = &copied
	e.currentIndex = 0
	e.isPlaying = autoPlay
	if autoPlay {
		e.startTimerLocked()
	}
	state := e.stateLocked()
	e.broadcastLocked(Event{Type: EventStoryLoaded, State: state})
	e.mu.Unlock()

	return nil
}

// Play resumes or starts auto-advance playback.
func (e *Engine) Play() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.story == nil {
		return fmt.Errorf("no story is currently loaded")
	}
	if e.isPlaying {
		return nil
	}
	e.isPlaying = true
	e.startTimerLocked()
	e.broadcastLocked(Event{Type: EventPlayed, State: e.stateLocked()})
	return nil
}

// Pause pauses auto-advance playback while keeping the current highlight active.
func (e *Engine) Pause() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.story == nil {
		return fmt.Errorf("no story is currently loaded")
	}
	if !e.isPlaying {
		return nil
	}
	e.isPlaying = false
	e.stopTimerLocked()
	e.broadcastLocked(Event{Type: EventPaused, State: e.stateLocked()})
	return nil
}

// Next advances to the next step if available. Returns false if already at the last step.
func (e *Engine) Next() (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.story == nil {
		return false, fmt.Errorf("no story is currently loaded")
	}
	if e.currentIndex >= len(e.story.Steps)-1 {
		if e.isPlaying {
			e.isPlaying = false
			e.stopTimerLocked()
			e.broadcastLocked(Event{Type: EventPaused, State: e.stateLocked()})
		}
		return false, nil
	}

	e.currentIndex++
	if e.isPlaying {
		e.startTimerLocked()
	}
	e.broadcastLocked(Event{Type: EventStepChanged, State: e.stateLocked()})
	return true, nil
}

// Prev moves back to the previous step if available. Returns false if already at the first step.
func (e *Engine) Prev() (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.story == nil {
		return false, fmt.Errorf("no story is currently loaded")
	}
	if e.currentIndex <= 0 {
		return false, nil
	}

	e.currentIndex--
	if e.isPlaying {
		e.startTimerLocked()
	}
	e.broadcastLocked(Event{Type: EventStepChanged, State: e.stateLocked()})
	return true, nil
}

// GoTo jumps directly to a 0-indexed step.
func (e *Engine) GoTo(index int) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.story == nil {
		return fmt.Errorf("no story is currently loaded")
	}
	if index < 0 || index >= len(e.story.Steps) {
		return fmt.Errorf("step index %d out of range [0, %d)", index, len(e.story.Steps))
	}

	e.currentIndex = index
	if e.isPlaying {
		e.startTimerLocked()
	}
	e.broadcastLocked(Event{Type: EventStepChanged, State: e.stateLocked()})
	return nil
}

// Stop clears the active presentation and stops any running timers.
func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.stopTimerLocked()
	e.story = nil
	e.currentIndex = 0
	e.isPlaying = false
	e.broadcastLocked(Event{Type: EventStopped, State: e.stateLocked()})
}

// State returns a snapshot of the current engine state.
func (e *Engine) State() State {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.stateLocked()
}

func (e *Engine) stateLocked() State {
	if e.story == nil || len(e.story.Steps) == 0 {
		return State{}
	}
	step := e.story.Steps[e.currentIndex]
	return State{
		StoryTitle:       e.story.Title,
		CurrentStepIndex: e.currentIndex,
		TotalSteps:       len(e.story.Steps),
		IsPlaying:        e.isPlaying,
		CurrentStep:      &step,
	}
}

func (e *Engine) startTimerLocked() {
	e.stopTimerLocked()
	if e.story == nil || e.currentIndex >= len(e.story.Steps) {
		return
	}
	duration := time.Duration(e.story.Steps[e.currentIndex].DurationSeconds) * time.Second
	expectedIndex := e.currentIndex
	e.timer = time.AfterFunc(duration, func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		if !e.isPlaying || e.story == nil || e.currentIndex != expectedIndex {
			return
		}
		if e.currentIndex >= len(e.story.Steps)-1 {
			e.isPlaying = false
			e.broadcastLocked(Event{Type: EventPaused, State: e.stateLocked()})
			return
		}
		e.currentIndex++
		e.startTimerLocked()
		e.broadcastLocked(Event{Type: EventStepChanged, State: e.stateLocked()})
	})
}

func (e *Engine) stopTimerLocked() {
	if e.timer != nil {
		e.timer.Stop()
		e.timer = nil
	}
}

func (e *Engine) broadcastLocked(evt Event) {
	for _, ch := range e.subscribers {
		select {
		case ch <- evt:
		default:
			// Drop if subscriber buffer is full to prevent blocking the engine.
		}
	}
}
