package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDemoStoryLineBounds(t *testing.T) {
	story := demoStory()
	if err := story.Validate(); err != nil {
		t.Fatalf("demoStory() failed validation: %v", err)
	}

	// Tests in cmd/codepresent run with cwd = cmd/codepresent; resolve repo root.
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("failed to resolve repo root: %v", err)
	}

	for _, step := range story.Steps {
		fullPath := filepath.Join(repoRoot, step.FilePath)
		raw, err := os.ReadFile(fullPath)
		if err != nil {
			t.Fatalf("step %d (%s): failed to read %s: %v", step.StepNumber, step.Title, fullPath, err)
		}

		lines := strings.Split(string(raw), "\n")
		if step.EndLine > len(lines) {
			t.Fatalf("step %d (%s): EndLine %d exceeds file line count %d", step.StepNumber, step.Title, step.EndLine, len(lines))
		}

		startText := strings.TrimSpace(lines[step.StartLine-1])
		endText := strings.TrimSpace(lines[step.EndLine-1])

		if startText == "" {
			t.Errorf("step %d (%s): StartLine %d is blank", step.StepNumber, step.Title, step.StartLine)
		}
		if endText != "}" && endText != "}()" {
			t.Errorf("step %d (%s): expected EndLine %d to close its block with '}' or '}()', got %q", step.StepNumber, step.Title, step.EndLine, endText)
		}
	}
}
