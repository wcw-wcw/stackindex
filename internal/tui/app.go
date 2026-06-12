package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/will/stackmap/internal/models"
	"github.com/will/stackmap/internal/report"
	"github.com/will/stackmap/internal/tui/screens"
)

type screen int

const (
	screenHome screen = iota
	screenFindings
	screenRoutes
	screenReport
)

type Model struct {
	analysis *models.Analysis
	root     string
	screen   screen
	cursor   int
	status   string
	err      error
}

func New(analysis *models.Analysis, root string) Model {
	return Model{analysis: analysis, root: root}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "esc", "backspace":
			m.screen = screenHome
			m.cursor = 0
		case "1", "h":
			m.screen = screenHome
			m.cursor = 0
		case "2", "f":
			m.screen = screenFindings
			m.cursor = 0
		case "3":
			m.screen = screenRoutes
			m.cursor = 0
		case "4":
			m.screen = screenReport
			m.cursor = 0
		case "r":
			err := report.ExportAll(m.root, m.analysis)
			m.screen = screenReport
			if err != nil {
				m.err = err
				m.status = "Export failed"
			} else {
				m.err = nil
				m.status = "Reports exported"
			}
		case "enter":
			if m.screen == screenHome {
				m.screen = screenFindings
				m.cursor = 0
			}
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < m.maxCursor() {
				m.cursor++
			}
		}
	}
	return m, nil
}

func (m Model) View() string {
	var b strings.Builder
	fmt.Fprintln(&b, titleStyle.Render("StackMap - Local Codebase Assistant"))
	fmt.Fprintln(&b)
	switch m.screen {
	case screenHome:
		b.WriteString(screens.Home(m.analysis))
	case screenFindings:
		b.WriteString(screens.Findings(m.analysis, m.cursor))
	case screenRoutes:
		b.WriteString(screens.Routes(m.analysis, m.cursor))
	case screenReport:
		b.WriteString(m.reportView())
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, nav())
	return b.String()
}

func (m Model) maxCursor() int {
	switch m.screen {
	case screenFindings:
		if len(m.analysis.Findings) == 0 {
			return 0
		}
		return len(m.analysis.Findings) - 1
	case screenRoutes:
		if len(m.analysis.Routes) == 0 {
			return 0
		}
		return len(m.analysis.Routes) - 1
	default:
		return 0
	}
}

func (m Model) reportView() string {
	var b strings.Builder
	fmt.Fprintln(&b, badgeStyle.Render("Export Status"))
	fmt.Fprintln(&b)
	if m.status == "" {
		fmt.Fprintln(&b, "Press r to export reports.")
	} else {
		fmt.Fprintln(&b, m.status)
	}
	if m.err != nil {
		fmt.Fprintf(&b, "\nError: %v\n", m.err)
	} else {
		fmt.Fprintf(&b, "\nJSON: %s\n", filepath.Join(m.root, ".stackmap", "analysis.json"))
		fmt.Fprintf(&b, "Markdown: %s\n", filepath.Join(m.root, ".stackmap", "reports", "repo-report.md"))
	}
	if m.analysis.AI != nil && m.analysis.AI.Warning != "" {
		fmt.Fprintf(&b, "\nAI warning: %s\n", m.analysis.AI.Warning)
	}
	return b.String()
}

func nav() string {
	return fmt.Sprintf("%s Navigate  %s Findings  %s Routes  %s Export  %s Quit",
		keyStyle.Render("[up/down]"),
		keyStyle.Render("[f]"),
		keyStyle.Render("[3]"),
		keyStyle.Render("[r]"),
		keyStyle.Render("[q]"),
	)
}

func Run(analysis *models.Analysis, root string) error {
	_, err := tea.NewProgram(New(analysis, root)).Run()
	return err
}
