package presentation

import (
	"testing"
	"time"
)

func sampleStory() Story {
	return Story{
		Title: "Config Loading Walkthrough",
		Steps: []Step{
			{
				FilePath:        "pkg/config/loader.go",
				StartLine:       10,
				EndLine:         22,
				Title:           "Read YAML",
				Narration:       "Reads default configuration values from disk.",
				DurationSeconds: 1,
			},
			{
				FilePath:        "pkg/config/loader.go",
				StartLine:       25,
				EndLine:         35,
				Title:           "Env Overrides",
				Narration:       "Binds environment variables to override file settings.",
				DurationSeconds: 1,
			},
		},
	}
}

func TestStoryValidation(t *testing.T) {
	tests := []struct {
		name    string
		story   Story
		wantErr bool
	}{
		{
			name:    "valid story",
			story:   sampleStory(),
			wantErr: false,
		},
		{
			name:    "empty title",
			story:   Story{Steps: sampleStory().Steps},
			wantErr: true,
		},
		{
			name:    "no steps",
			story:   Story{Title: "Empty"},
			wantErr: true,
		},
		{
			name: "invalid line range",
			story: Story{
				Title: "Bad range",
				Steps: []Step{{FilePath: "main.go", StartLine: 20, EndLine: 10, Narration: "bad"}},
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.story.Validate()
			if (err != nil) != tc.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestEngineNavigationAndPlayPause(t *testing.T) {
	eng := NewEngine()
	events, unsub := eng.Subscribe()
	defer unsub()

	if err := eng.LoadStory(sampleStory(), false); err != nil {
		t.Fatalf("LoadStory failed: %v", err)
	}

	evt := <-events
	if evt.Type != EventStoryLoaded {
		t.Fatalf("expected EventStoryLoaded, got %s", evt.Type)
	}
	if evt.State.CurrentStepIndex != 0 || evt.State.TotalSteps != 2 || evt.State.IsPlaying {
		t.Fatalf("unexpected initial state: %+v", evt.State)
	}

	moved, err := eng.Next()
	if err != nil || !moved {
		t.Fatalf("Next() failed: moved=%v err=%v", moved, err)
	}
	evt = <-events
	if evt.Type != EventStepChanged || evt.State.CurrentStepIndex != 1 {
		t.Fatalf("expected step index 1, got %+v", evt)
	}

	// Next at end should return false
	moved, err = eng.Next()
	if err != nil || moved {
		t.Fatalf("expected Next() at end to return false, got moved=%v err=%v", moved, err)
	}

	moved, err = eng.Prev()
	if err != nil || !moved {
		t.Fatalf("Prev() failed: moved=%v err=%v", moved, err)
	}
	evt = <-events
	if evt.Type != EventStepChanged || evt.State.CurrentStepIndex != 0 {
		t.Fatalf("expected step index 0, got %+v", evt)
	}

	// Play and wait for auto-advance
	if err := eng.Play(); err != nil {
		t.Fatalf("Play() failed: %v", err)
	}
	evt = <-events
	if evt.Type != EventPlayed || !evt.State.IsPlaying {
		t.Fatalf("expected EventPlayed, got %+v", evt)
	}

	select {
	case evt = <-events:
		if evt.Type != EventStepChanged || evt.State.CurrentStepIndex != 1 {
			t.Fatalf("expected auto-advance to step 1, got %+v", evt)
		}
	case <-time.After(2500 * time.Millisecond):
		t.Fatal("timed out waiting for auto-advance")
	}

	if err := eng.Pause(); err != nil {
		t.Fatalf("Pause() failed: %v", err)
	}
	evt = <-events
	if evt.Type != EventPaused || evt.State.IsPlaying {
		t.Fatalf("expected EventPaused, got %+v", evt)
	}
}
