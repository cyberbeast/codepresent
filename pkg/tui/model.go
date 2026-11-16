package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cyberbeast/codepresent/pkg/presentation"
)

// Controller abstracts local Engine or remote WebSocket control for the TUI.
type Controller interface {
	State() presentation.State
	Subscribe() (<-chan presentation.Event, func())
	Play() error
	Pause() error
	Next() (bool, error)
	Prev() (bool, error)
	GoTo(index int) error
	Stop()
}

type engineEventMsg struct {
	event presentation.Event
}

type engineClosedMsg struct{}

// Model is the Bubble Tea presentation model rendering the storyboard and spotlighted code.
type Model struct {
	ctrl       Controller
	events     <-chan presentation.Event
	unsub      func()
	state      presentation.State
	lines      []string
	loadedFile string
	loadErr    string
	scrollTop  int
	width      int
	height     int
}

// NewModel initializes a Bubble Tea Model backed by ctrl.
func NewModel(ctrl Controller) Model {
	events, unsub := ctrl.Subscribe()
	m := Model{
		ctrl:   ctrl,
		events: events,
		unsub:  unsub,
		state:  ctrl.State(),
		width:  100,
		height: 30,
	}
	m.syncFileAndScroll()
	return m
}

// Run starts the Bubble Tea TUI in the alternate screen buffer until the user quits or ctx cancels.
func Run(ctx context.Context, ctrl Controller) error {
	m := NewModel(ctrl)
	defer func() {
		if m.unsub != nil {
			m.unsub()
		}
	}()

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithContext(ctx))
	_, err := p.Run()
	return err
}

func (m Model) Init() tea.Cmd {
	return waitForEvent(m.events)
}

func waitForEvent(ch <-chan presentation.Event) tea.Cmd {
	return func() tea.Msg {
		evt, ok := <-ch
		if !ok {
			return engineClosedMsg{}
		}
		return engineEventMsg{event: evt}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.centerSpotlight()
		return m, nil

	case engineEventMsg:
		m.state = msg.event.State
		if msg.event.Type == presentation.EventStoryLoaded || msg.event.Type == presentation.EventStepChanged {
			m.syncFileAndScroll()
		} else if msg.event.Type == presentation.EventStopped {
			m.lines = nil
			m.loadedFile = ""
			m.loadErr = ""
			m.scrollTop = 0
		}
		return m, waitForEvent(m.events)

	case engineClosedMsg:
		return m, tea.Quit

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case " ", "p":
			if m.state.IsPlaying {
				_ = m.ctrl.Pause()
			} else {
				_ = m.ctrl.Play()
			}
		case "right", "l", "n":
			_, _ = m.ctrl.Next()
		case "left", "h", "b":
			_, _ = m.ctrl.Prev()
		case "down", "j":
			m.scrollBy(1)
		case "up", "k":
			m.scrollBy(-1)
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			idx := int(msg.String()[0] - '1')
			if idx >= 0 && idx < m.state.TotalSteps {
				_ = m.ctrl.GoTo(idx)
			}
		}
	}
	return m, nil
}

func (m *Model) syncFileAndScroll() {
	step := m.state.CurrentStep
	if step == nil {
		m.lines = nil
		m.loadedFile = ""
		m.loadErr = ""
		return
	}

	resolved := ResolveFilePath(step.FilePath, m.state.WorkspaceRoot)
	if resolved != m.loadedFile {
		lines, err := ReadAndHighlight(step.FilePath, m.state.WorkspaceRoot)
		if err != nil {
			m.lines = nil
			m.loadErr = err.Error()
		} else {
			m.lines = lines
			m.loadErr = ""
			m.loadedFile = resolved
		}
	}
	m.centerSpotlight()
}

func (m *Model) centerSpotlight() {
	step := m.state.CurrentStep
	if step == nil || len(m.lines) == 0 {
		m.scrollTop = 0
		return
	}
	viewportHeight := m.codeViewportHeight()
	midLine := ((step.StartLine - 1) + (step.EndLine - 1)) / 2
	top := midLine - viewportHeight/2
	maxTop := max(0, len(m.lines)-viewportHeight)
	if top < 0 {
		top = 0
	}
	if top > maxTop {
		top = maxTop
	}
	m.scrollTop = top
}

func (m *Model) scrollBy(delta int) {
	if len(m.lines) == 0 {
		return
	}
	viewportHeight := m.codeViewportHeight()
	maxTop := max(0, len(m.lines)-viewportHeight)
	m.scrollTop = max(0, min(maxTop, m.scrollTop+delta))
}

func (m Model) codeViewportHeight() int {
	// Reserve 4 lines for top header/footer and code pane borders.
	return max(6, m.height-6)
}

func (m Model) View() string {
	if m.state.TotalSteps == 0 || m.state.CurrentStep == nil {
		waiting := lipgloss.NewStyle().
			Padding(2, 4).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#ffd54f")).
			Render("codepresent TUI\n\nWaiting for an AI agent presentation...\n\nPress q to quit.")
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, waiting)
	}

	accent := StepColor(m.state.CurrentStepIndex)
	step := m.state.CurrentStep

	// Header bar
	statusBadge := lipgloss.NewStyle().
		Bold(true).
		Padding(0, 1).
		Foreground(lipgloss.Color("#1e1e1e")).
		Background(accent)
	playText := "▶ PAUSED"
	if m.state.IsPlaying {
		playText = "⏸ PLAYING"
	}

	headerLeft := fmt.Sprintf(" %s  %s (%d/%d)",
		statusBadge.Render(playText),
		lipgloss.NewStyle().Bold(true).Render(m.state.StoryTitle),
		m.state.CurrentStepIndex+1,
		m.state.TotalSteps,
	)
	headerRight := lipgloss.NewStyle().
		Foreground(lipgloss.Color("244")).
		Render("[space] play/pause  [←/→] step  [↑/↓] scroll  [1-9] jump  [q] quit ")

	header := lipgloss.NewStyle().
		Width(m.width).
		BorderBottom(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("238")).
		Render(lipgloss.JoinHorizontal(lipgloss.Top,
			headerLeft,
			lipgloss.PlaceHorizontal(max(0, m.width-lipgloss.Width(headerLeft)), lipgloss.Right, headerRight),
		))

	// Split widths
	gutterWidth := 5
	sidebarWidth := max(30, min(44, m.width*35/100))
	codeWidth := max(20, m.width-sidebarWidth-gutterWidth-1)
	bodyHeight := max(8, m.height-3)

	sidebar, y1 := m.renderSidebar(sidebarWidth, bodyHeight)
	codePane, y2 := m.renderCodePane(step, accent, codeWidth, bodyHeight)
	gutter := renderConnectorGutter(bodyHeight, y1, y2, accent)

	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, gutter, codePane)
	return lipgloss.JoinVertical(lipgloss.Left, header, body)
}

func (m Model) renderSidebar(width, height int) (string, int) {
	steps := m.state.Steps
	if len(steps) == 0 && m.state.CurrentStep != nil {
		steps = []presentation.Step{*m.state.CurrentStep}
	}

	var items []string
	innerWidth := max(16, width-4)
	activeMidRow := 2
	currentRow := 1 // top padding = 1

	for i, s := range steps {
		c := StepColor(i)
		isActive := i == m.state.CurrentStepIndex
		isDone := i < m.state.CurrentStepIndex

		marker := fmt.Sprintf("○ %d", i+1)
		if isDone {
			marker = fmt.Sprintf("✓ %d", i+1)
		} else if isActive {
			marker = fmt.Sprintf("● %d", i+1)
		}

		titleLine := lipgloss.NewStyle().
			Bold(isActive).
			Foreground(c).
			Render(fmt.Sprintf("%s  %s", marker, s.Title))

		metaLine := lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Render(fmt.Sprintf("    %s:%d-%d", s.FilePath, s.StartLine, s.EndLine))

		if isActive {
			narration := lipgloss.NewStyle().
				Width(innerWidth - 4).
				Foreground(lipgloss.Color("252")).
				MarginTop(1).
				Render(s.Narration)

			cardContent := titleLine + "\n" + metaLine
			if s.FocusTarget != "" {
				badge := lipgloss.NewStyle().
					Foreground(c).
					Render("    [" + s.FocusTarget + "]")
				cardContent += "\n" + badge
			}
			cardContent += "\n" + narration

			card := lipgloss.NewStyle().
				Width(innerWidth).
				Padding(0, 1).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(c).
				Render(cardContent)

			cardHeight := strings.Count(card, "\n") + 1
			activeMidRow = currentRow + cardHeight/2
			currentRow += cardHeight + 2 // +2 for blank separator line
			items = append(items, card)
		} else {
			block := titleLine + "\n" + metaLine
			blockHeight := strings.Count(block, "\n") + 1
			currentRow += blockHeight + 2
			items = append(items, block)
		}
	}

	if activeMidRow >= height-1 {
		activeMidRow = max(1, height-2)
	}

	content := strings.Join(items, "\n\n")
	rendered := lipgloss.NewStyle().
		Width(width).
		Height(height).
		Padding(1, 1).
		BorderRight(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("238")).
		Render(content)
	return rendered, activeMidRow
}

func (m Model) renderCodePane(step *presentation.Step, accent lipgloss.Color, width, height int) (string, int) {
	fileHeader := lipgloss.NewStyle().
		Bold(true).
		Foreground(accent).
		Render(fmt.Sprintf("%s (%d-%d)", step.FilePath, step.StartLine, step.EndLine))

	if m.loadErr != "" {
		rendered := lipgloss.NewStyle().
			Width(width).
			Height(height).
			Padding(1, 2).
			Render(fileHeader + "\n\nError: " + m.loadErr)
		return rendered, 2
	}

	viewportHeight := max(4, height-3)
	endIdx := min(len(m.lines), m.scrollTop+viewportHeight)

	dimNumStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("239")).Width(4).Align(lipgloss.Right)
	dimBarStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	dimCodeStyle := lipgloss.NewStyle().Faint(true)

	spotNumStyle := lipgloss.NewStyle().Foreground(accent).Bold(true).Width(4).Align(lipgloss.Right)
	spotBarStyle := lipgloss.NewStyle().Foreground(accent).Bold(true)

	var rendered []string
	var spotRows []int
	for i := m.scrollTop; i < endIdx; i++ {
		lineNum := i + 1
		inSpotlight := lineNum >= step.StartLine && lineNum <= step.EndLine
		lineText := m.lines[i]
		screenRow := 2 + (i - m.scrollTop) // top padding (1) + fileHeader (1)

		if inSpotlight {
			spotRows = append(spotRows, screenRow)
			prefix := fmt.Sprintf("%s %s ", spotNumStyle.Render(fmt.Sprintf("%d", lineNum)), spotBarStyle.Render("┃"))
			rendered = append(rendered, prefix+lineText)
		} else {
			prefix := fmt.Sprintf("%s %s ", dimNumStyle.Render(fmt.Sprintf("%d", lineNum)), dimBarStyle.Render("│"))
			rendered = append(rendered, prefix+dimCodeStyle.Render(lineText))
		}
	}

	midRow := 2
	if len(spotRows) > 0 {
		midRow = spotRows[len(spotRows)/2]
	}

	out := lipgloss.NewStyle().
		Width(width).
		Height(height).
		Padding(1, 1).
		Render(fileHeader + "\n" + strings.Join(rendered, "\n"))
	return out, midRow
}

func renderConnectorGutter(height, y1, y2 int, accent lipgloss.Color) string {
	style := lipgloss.NewStyle().Foreground(accent).Bold(true)
	rows := make([]string, height)

	for r := 0; r < height; r++ {
		var cell string
		switch {
		case y1 == y2 && r == y1:
			cell = style.Render("●───●")
		case y1 < y2:
			switch {
			case r == y1:
				cell = style.Render("●─╮  ")
			case r > y1 && r < y2:
				cell = style.Render("  │  ")
			case r == y2:
				cell = style.Render("  ╰─●")
			default:
				cell = "     "
			}
		case y1 > y2:
			switch {
			case r == y2:
				cell = style.Render("  ╭─●")
			case r > y2 && r < y1:
				cell = style.Render("  │  ")
			case r == y1:
				cell = style.Render("●─╯  ")
			default:
				cell = "     "
			}
		default:
			cell = "     "
		}
		rows[r] = cell
	}
	return strings.Join(rows, "\n")
}
