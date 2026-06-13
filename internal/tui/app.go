package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/will/stackmap/internal/models"
	"github.com/will/stackmap/internal/report"
)

var sections = []string{"Overview", "Findings", "Routes", "Env Vars", "Tests", "Deployment", "Reports"}

type Model struct {
	analysis *models.Analysis
	root     string
	cursor   int
	width    int
	height   int
	status   string
	err      error
}

func New(analysis *models.Analysis, root string) Model {
	return Model{analysis: analysis, root: root, width: 100, height: 32}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "esc", "backspace":
			m.cursor = 0
		case "r":
			err := report.ExportAll(m.root, m.analysis)
			m.cursor = len(sections) - 1
			if err != nil {
				m.err = err
				m.status = "Export failed"
			} else {
				m.err = nil
				m.status = "Reports exported"
			}
		case "enter":
			// The detail panel follows the selected section, so enter is intentionally calm.
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(sections)-1 {
				m.cursor++
			}
		}
	}
	return m, nil
}

func (m Model) View() string {
	width := m.width
	if width <= 0 {
		width = 100
	}
	if width < 64 {
		return m.narrowView(width)
	}

	header := m.header(width)
	navWidth := 16
	detailWidth := width - navWidth - 14
	if detailWidth < 40 {
		detailWidth = 40
	}
	left := panelStyle.Width(navWidth).Render(m.nav())
	right := panelStyle.Width(detailWidth).Render(m.detail(detailWidth - 4))
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right)
	footer := mutedStyle.Width(width).Render("up/down or j/k navigate  enter select  r export reports  q quit")
	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

func (m Model) narrowView(width int) string {
	if width < 32 {
		width = 32
	}
	header := m.header(width)
	nav := panelStyle.Width(width - 2).Render(m.compactNav())
	detail := panelStyle.Width(width - 2).Render(m.detail(width - 6))
	footer := mutedStyle.Width(width).Render("j/k move  r export  q quit")
	return lipgloss.JoinVertical(lipgloss.Left, header, nav, detail, footer)
}

func (m Model) header(width int) string {
	counts := severityCounts(m.analysis.Findings)
	stack := stackLine(m.analysis.Stack)
	if width < 88 {
		stack = truncate(stack, width-34)
	}
	lines := []string{
		titleStyle.Render("StackMap") + "  " + mutedStyle.Render(m.analysis.RepoName),
		fmt.Sprintf("Stack: %s", stack),
		fmt.Sprintf("Files %d   Routes %d   Tests %d   Findings %s",
			len(m.analysis.Files),
			len(m.analysis.Routes),
			len(m.analysis.Tests.TestFiles),
			severityLine(counts, width < 96),
		),
		aiStatus(m.analysis.AI),
	}
	return headerStyle.Width(width - 2).Render(strings.Join(lines, "\n"))
}

func (m Model) nav() string {
	var b strings.Builder
	fmt.Fprintln(&b, mutedStyle.Render("Sections"))
	for i, section := range sections {
		line := "  " + section
		if i == m.cursor {
			line = selectedStyle.Render("> " + section)
		}
		fmt.Fprintln(&b, line)
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m Model) compactNav() string {
	var items []string
	for i, section := range sections {
		if i == m.cursor {
			items = append(items, selectedStyle.Render(section))
		} else {
			items = append(items, mutedStyle.Render(section))
		}
	}
	return strings.Join(items, "  ")
}

func (m Model) detail(width int) string {
	switch sections[m.cursor] {
	case "Findings":
		return m.findingsDetail(width)
	case "Routes":
		return m.routesDetail(width)
	case "Env Vars":
		return m.envDetail(width)
	case "Tests":
		return m.testsDetail(width)
	case "Deployment":
		return m.deploymentDetail(width)
	case "Reports":
		return m.reportsDetail(width)
	default:
		return m.overviewDetail(width)
	}
}

func (m Model) overviewDetail(width int) string {
	var b strings.Builder
	fmt.Fprintln(&b, sectionTitleStyle.Render("Overview"))
	fmt.Fprintf(&b, "Repository: %s\n", m.analysis.RepoPath)
	fmt.Fprintf(&b, "Generated: %s\n\n", m.analysis.GeneratedAt.Format("2006-01-02 15:04"))
	writeStatusLine(&b, "Health endpoint", m.analysis.Deployment.HasHealthEndpoint)
	writeStatusLine(&b, ".env.example", m.analysis.Deployment.HasEnvExample)
	writeStatusLine(&b, "Tests", m.analysis.Tests.HasTestFiles || m.analysis.Tests.HasTestScript)
	writeStatusLine(&b, "Migration files", m.analysis.Deployment.HasMigrationFiles)
	fmt.Fprintf(&b, "AI                 %s\n", aiStatus(m.analysis.AI))
	fmt.Fprintln(&b)
	writeTopFindings(&b, m.analysis.Findings, 4, width)
	return b.String()
}

func (m Model) findingsDetail(width int) string {
	var b strings.Builder
	fmt.Fprintln(&b, sectionTitleStyle.Render("Findings"))
	if len(m.analysis.Findings) == 0 {
		fmt.Fprintln(&b, "No findings. Nice and quiet.")
		return b.String()
	}
	for i, f := range m.analysis.Findings {
		if i >= 8 {
			fmt.Fprintf(&b, "%s %d more\n", mutedStyle.Render("..."), len(m.analysis.Findings)-i)
			break
		}
		file := ""
		if f.File != "" {
			file = " · " + f.File
		}
		fmt.Fprintf(&b, "%s %s\n", severityBadge(f.Severity), truncate(f.Message+file, width-10))
		if f.Recommendation != "" && i < 3 {
			fmt.Fprintf(&b, "  %s\n", mutedStyle.Render(truncate(f.Recommendation, width-4)))
		}
	}
	return b.String()
}

func (m Model) routesDetail(width int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s\n\n", sectionTitleStyle.Render("Routes"), mutedStyle.Render(fmt.Sprintf("%d detected", len(m.analysis.Routes))))
	if len(m.analysis.Routes) == 0 {
		fmt.Fprintln(&b, "No API routes detected.")
		return b.String()
	}
	for i, route := range m.analysis.Routes {
		if i >= 10 {
			fmt.Fprintf(&b, "%s %d more\n", mutedStyle.Render("..."), len(m.analysis.Routes)-i)
			break
		}
		fmt.Fprintf(&b, "%-7s %s\n", methodStyle.Render(route.Method), truncate(route.Path, width-12))
		fmt.Fprintf(&b, "        %s\n", mutedStyle.Render(truncate(route.SourceFile+" · "+route.Confidence, width-8)))
		if route.Note != "" {
			fmt.Fprintf(&b, "        %s\n", mutedStyle.Render(truncate(route.Note, width-8)))
		}
	}
	return b.String()
}

func (m Model) envDetail(width int) string {
	var b strings.Builder
	fmt.Fprintln(&b, sectionTitleStyle.Render("Env Vars"))
	if !m.analysis.Env.UsesEnvVars {
		fmt.Fprintln(&b, "No environment variable usage detected.")
		return b.String()
	}
	fmt.Fprintf(&b, ".env.example: %s\n\n", presentWord(m.analysis.Env.ExampleFile != ""))
	writeEnvClass(&b, "Required app config", m.analysis.Env.UsedVars, width, "required_app_config")
	writeEnvClass(&b, "Optional app config", m.analysis.Env.UsedVars, width, "optional_app_config")
	writeEnvClass(&b, "Platform/build metadata", m.analysis.Env.UsedVars, width, "platform_provided", "build_metadata")
	writeEnvClass(&b, "Script-only vars", m.analysis.Env.UsedVars, width, "test_or_script_only")
	return b.String()
}

func (m Model) testsDetail(width int) string {
	var b strings.Builder
	fmt.Fprintln(&b, sectionTitleStyle.Render("Tests"))
	writeStatusLine(&b, "Test files", m.analysis.Tests.HasTestFiles)
	writeStatusLine(&b, "Test script", m.analysis.Tests.HasTestScript)
	writeStatusLine(&b, "Playwright", m.analysis.Tests.PlaywrightDetected)
	if len(m.analysis.Tests.Frameworks) > 0 {
		fmt.Fprintf(&b, "\nFrameworks: %s\n", strings.Join(m.analysis.Tests.Frameworks, ", "))
	}
	for i, file := range m.analysis.Tests.TestFiles {
		if i >= 6 {
			fmt.Fprintf(&b, "%s %d more\n", mutedStyle.Render("..."), len(m.analysis.Tests.TestFiles)-i)
			break
		}
		fmt.Fprintf(&b, "- %s\n", truncate(file, width-2))
	}
	return b.String()
}

func (m Model) deploymentDetail(width int) string {
	var b strings.Builder
	fmt.Fprintln(&b, sectionTitleStyle.Render("Deployment"))
	writeStatusLine(&b, "README", m.analysis.Deployment.HasReadme)
	writeStatusLine(&b, "Setup docs", m.analysis.Deployment.ReadmeMentionsSetup)
	writeStatusLine(&b, "Deployment docs", m.analysis.Deployment.ReadmeMentionsDeploy)
	writeStatusLine(&b, "Dockerfile", m.analysis.Deployment.HasDockerfile)
	writeStatusLine(&b, "Vercel config", m.analysis.Deployment.HasVercelConfig)
	writeStatusLine(&b, "Health endpoint", m.analysis.Deployment.HasHealthEndpoint)
	writeStatusLine(&b, "Migration files", m.analysis.Deployment.HasMigrationFiles)
	if len(m.analysis.Deployment.DeploymentFiles) > 0 {
		fmt.Fprintf(&b, "\nFiles: %s\n", truncate(strings.Join(m.analysis.Deployment.DeploymentFiles, ", "), width-7))
	}
	return b.String()
}

func (m Model) reportsDetail(width int) string {
	var b strings.Builder
	fmt.Fprintln(&b, sectionTitleStyle.Render("Reports"))
	if m.status == "" {
		fmt.Fprintln(&b, "Press r to export JSON and Markdown reports.")
	} else {
		fmt.Fprintln(&b, m.status)
	}
	if m.err != nil {
		fmt.Fprintf(&b, "\nError: %v\n", m.err)
		return b.String()
	}
	fmt.Fprintf(&b, "\nJSON: %s\n", truncate(filepath.Join(m.root, ".stackmap", "analysis.json"), width-6))
	fmt.Fprintf(&b, "Markdown: %s\n", truncate(filepath.Join(m.root, ".stackmap", "reports", "repo-report.md"), width-10))
	if m.analysis.AI != nil && m.analysis.AI.Warning != "" {
		fmt.Fprintf(&b, "\nAI warning: %s\n", truncate(m.analysis.AI.Warning, width-12))
	} else if m.analysis.AI != nil && m.analysis.AI.ParseError != "" {
		fmt.Fprintf(&b, "\nAI parse warning: %s\n", truncate(m.analysis.AI.ParseError, width-18))
	} else {
		fmt.Fprintf(&b, "\n%s\n", aiStatus(m.analysis.AI))
	}
	return b.String()
}

func writeTopFindings(b *strings.Builder, findings []models.Finding, limit, width int) {
	if len(findings) == 0 {
		fmt.Fprintln(b, "No recommended fixes.")
		return
	}
	fmt.Fprintln(b, sectionTitleStyle.Render("Top Fixes"))
	count := 0
	for _, severity := range []models.Severity{models.SeverityHigh, models.SeverityMedium, models.SeverityLow, models.SeverityInfo} {
		for _, f := range findings {
			if f.Severity != severity {
				continue
			}
			text := f.Message
			if f.Recommendation != "" {
				text = f.Recommendation
			}
			fmt.Fprintf(b, "%s %s\n", severityBadge(f.Severity), truncate(text, width-10))
			count++
			if count >= limit {
				return
			}
		}
	}
}

func writeEnvClass(b *strings.Builder, label string, vars []models.EnvVar, width int, classes ...string) {
	classSet := map[string]bool{}
	for _, class := range classes {
		classSet[class] = true
	}
	var names []string
	for _, envVar := range vars {
		if classSet[envVar.Classification] {
			name := envVar.Name
			if envVar.MissingExample {
				name += "*"
			}
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		fmt.Fprintf(b, "%s: %s\n", label, mutedStyle.Render("none"))
		return
	}
	fmt.Fprintf(b, "%s: %s\n", label, truncate(strings.Join(names, ", "), width-len(label)-2))
}

func writeStatusLine(b *strings.Builder, label string, ok bool) {
	mark := mutedStyle.Render("no")
	if ok {
		mark = okStyle.Render("yes")
	}
	fmt.Fprintf(b, "%-18s %s\n", label, mark)
}

func severityCounts(findings []models.Finding) map[models.Severity]int {
	counts := map[models.Severity]int{}
	for _, f := range findings {
		counts[f.Severity]++
	}
	return counts
}

func severityLine(counts map[models.Severity]int, compact bool) string {
	if compact {
		return fmt.Sprintf("H%d M%d L%d I%d", counts[models.SeverityHigh], counts[models.SeverityMedium], counts[models.SeverityLow], counts[models.SeverityInfo])
	}
	return fmt.Sprintf("%s %s %s %s",
		severityCount(models.SeverityHigh, counts[models.SeverityHigh]),
		severityCount(models.SeverityMedium, counts[models.SeverityMedium]),
		severityCount(models.SeverityLow, counts[models.SeverityLow]),
		severityCount(models.SeverityInfo, counts[models.SeverityInfo]),
	)
}

func severityCount(severity models.Severity, count int) string {
	return severityBadge(severity) + fmt.Sprintf(" %d", count)
}

func severityBadge(severity models.Severity) string {
	switch severity {
	case models.SeverityHigh:
		return highStyle.Render("high")
	case models.SeverityMedium:
		return mediumStyle.Render("medium")
	case models.SeverityLow:
		return lowStyle.Render("low")
	default:
		return infoStyle.Render("info")
	}
}

func stackLine(stack models.StackInfo) string {
	var parts []string
	parts = append(parts, stack.Languages...)
	parts = append(parts, stack.Frameworks...)
	parts = append(parts, stack.Databases...)
	parts = append(parts, stack.Deployment...)
	if len(parts) == 0 {
		return "none detected"
	}
	return strings.Join(parts, " / ")
}

func presentWord(ok bool) string {
	if ok {
		return okStyle.Render("yes")
	}
	return mutedStyle.Render("no")
}

func aiStatus(ai *models.AISummary) string {
	if ai == nil {
		return "AI: disabled"
	}
	switch ai.Status {
	case "generated_structured", "generated_text":
		if ai.Model != "" {
			return "AI: summary generated with " + ai.Model
		}
		return "AI: summary generated"
	case "fallback_empty":
		return "AI: returned no usable text"
	case "fallback_irrelevant":
		return "AI: returned unrelated text"
	case "fallback_model_unavailable":
		return "AI: requested but unavailable"
	}
	if ai.Warning != "" {
		if strings.Contains(strings.ToLower(ai.Warning), "empty response") {
			return "AI: returned no usable text"
		}
		return "AI: requested but unavailable"
	}
	if ai.ParseError != "" {
		if strings.TrimSpace(ai.RawText) == "" {
			return "AI: returned no usable text"
		}
		return "AI: returned text but parsing failed"
	}
	if ai.Model != "" {
		return "AI: summary generated with " + ai.Model
	}
	return "AI: summary generated"
}

func truncate(text string, max int) string {
	if max <= 0 || lipgloss.Width(text) <= max {
		return text
	}
	if max <= 1 {
		return "."
	}
	runes := []rune(text)
	if max <= 3 {
		return string(runes[:max])
	}
	if len(runes) <= max {
		return text
	}
	return string(runes[:max-3]) + "..."
}

func Run(analysis *models.Analysis, root string) error {
	_, err := tea.NewProgram(New(analysis, root)).Run()
	return err
}
