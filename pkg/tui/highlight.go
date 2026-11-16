package tui

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/lipgloss"
)

// StepColors defines the per-step accent palette matching the VS Code canvas.
var StepColors = []lipgloss.Color{
	lipgloss.Color("#ffd54f"), // Gold
	lipgloss.Color("#4fc3f7"), // Sky Cyan
	lipgloss.Color("#69f0ae"), // Mint Green
	lipgloss.Color("#b388ff"), // Lavender
	lipgloss.Color("#ff8a65"), // Coral
}

// StepColor returns the accent color for a 0-indexed step.
func StepColor(index int) lipgloss.Color {
	n := len(StepColors)
	return StepColors[((index%n)+n)%n]
}

// ResolveFilePath resolves a relative step path against workspaceRoot if needed.
func ResolveFilePath(filePath, workspaceRoot string) string {
	if filepath.IsAbs(filePath) || workspaceRoot == "" {
		return filePath
	}
	return filepath.Join(workspaceRoot, filePath)
}

// ReadAndHighlight reads a file from disk and returns syntax-highlighted lines.
func ReadAndHighlight(filePath, workspaceRoot string) ([]string, error) {
	resolved := ResolveFilePath(filePath, workspaceRoot)
	raw, err := os.ReadFile(resolved)
	if err != nil {
		return nil, err
	}
	return HighlightLines(string(raw), resolved), nil
}

// HighlightLines syntax-highlights source code into ANSI-styled lines using Chroma.
func HighlightLines(source, filename string) []string {
	lexer := lexers.Match(filename)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	style := styles.Get("monokai")
	if style == nil {
		style = styles.Fallback
	}

	formatter := formatters.Get("terminal256")
	if formatter == nil {
		return strings.Split(source, "\n")
	}

	iter, err := lexer.Tokenise(nil, source)
	if err != nil {
		return strings.Split(source, "\n")
	}

	var buf bytes.Buffer
	if err := formatter.Format(&buf, style, iter); err != nil {
		return strings.Split(source, "\n")
	}

	// Normalize tabs to 4 spaces so terminal column widths stay predictable.
	rendered := strings.ReplaceAll(buf.String(), "\t", "    ")
	return strings.Split(strings.TrimSuffix(rendered, "\n"), "\n")
}
