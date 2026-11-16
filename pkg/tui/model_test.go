package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cyberbeast/codepresent/pkg/presentation"
)

func TestStepColorCycles(t *testing.T) {
	if StepColor(0) != StepColors[0] {
		t.Fatalf("expected index 0 to match StepColors[0]")
	}
	if StepColor(len(StepColors)) != StepColors[0] {
		t.Fatalf("expected index len to wrap to StepColors[0]")
	}
	if StepColor(-1) != StepColors[len(StepColors)-1] {
		t.Fatalf("expected index -1 to wrap to last color")
	}
}

func TestHighlightLines(t *testing.T) {
	src := "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}"
	lines := HighlightLines(src, "main.go")
	if len(lines) != 5 {
		t.Fatalf("expected 5 highlighted lines, got %d", len(lines))
	}
}

func TestModelNavigationAndView(t *testing.T) {
	dir := t.TempDir()
	sampleFile := filepath.Join(dir, "main.go")
	content := "package main\n\nfunc first() {\n\tprintln(1)\n}\n\nfunc second() {\n\tprintln(2)\n}\n"
	if err := os.WriteFile(sampleFile, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	eng := presentation.NewEngine()
	story := presentation.Story{
		Title:         "TUI Walkthrough",
		WorkspaceRoot: dir,
		Steps: []presentation.Step{
			{
				FilePath:    "main.go",
				StartLine:   3,
				EndLine:     5,
				FocusTarget: "first()",
				Title:       "First Function",
				Narration:   "Prints 1 to stdout.",
			},
			{
				FilePath:    "main.go",
				StartLine:   7,
				EndLine:     9,
				FocusTarget: "second()",
				Title:       "Second Function",
				Narration:   "Prints 2 to stdout.",
			},
		},
	}
	if err := eng.LoadStory(story, false); err != nil {
		t.Fatalf("LoadStory failed: %v", err)
	}

	m := NewModel(eng)
	defer m.unsub()

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 32})
	m = updated.(Model)

	view := m.View()
	for _, want := range []string{"TUI Walkthrough", "First Function", "Prints 1 to stdout.", "main.go (3-5)", "┃"} {
		if !strings.Contains(view, want) {
			t.Errorf("expected view to contain %q, got:\n%s", want, view)
		}
	}

	// Next step via right arrow
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = updated.(Model)
	if eng.State().CurrentStepIndex != 1 {
		t.Fatalf("expected engine step 1 after right key, got %d", eng.State().CurrentStepIndex)
	}

	// Jump back to step 1 via '1' key
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	m = updated.(Model)
	if eng.State().CurrentStepIndex != 0 {
		t.Fatalf("expected engine step 0 after '1' key, got %d", eng.State().CurrentStepIndex)
	}

	// Quit via 'q'
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected quit command on 'q'")
	}
}

func TestRenderConnectorGutter(t *testing.T) {
	accent := StepColor(0)

	straight := renderConnectorGutter(5, 2, 2, accent)
	if !strings.Contains(straight, "●───●") {
		t.Errorf("expected horizontal connector '●───●', got:\n%s", straight)
	}

	down := renderConnectorGutter(6, 1, 4, accent)
	for _, token := range []string{"●─╮", "│", "╰─●"} {
		if !strings.Contains(down, token) {
			t.Errorf("expected downward connector to contain %q, got:\n%s", token, down)
		}
	}

	up := renderConnectorGutter(6, 4, 1, accent)
	for _, token := range []string{"╭─●", "│", "●─╯"} {
		if !strings.Contains(up, token) {
			t.Errorf("expected upward connector to contain %q, got:\n%s", token, up)
		}
	}
}
