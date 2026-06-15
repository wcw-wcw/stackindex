package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/will/stackmap/internal/models"
	"github.com/will/stackmap/internal/report"
)

var sections = []string{
	"Overview",
	"Audit",
	"Context",
	"Structure",
	"Key Files",
	"Connections",
	"API Routes",
	"Environment",
	"Tests",
	"Deployment",
	"Findings",
	"AI Notes",
	"Ask Help",
	"Reports",
}

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
	height := m.height
	if height <= 0 {
		height = 32
	}
	if width < 64 {
		return m.frame(width, height, true)
	}
	return m.frame(width, height, false)
}

func (m Model) frame(width, height int, narrow bool) string {
	width = maxInt(32, width)
	height = maxInt(10, height)

	headerHeight := 8
	if height < 18 {
		headerHeight = 5
	}
	footerHeight := 1
	bodyHeight := height - headerHeight - footerHeight
	if bodyHeight < 3 {
		headerHeight = 4
		bodyHeight = maxInt(1, height-headerHeight-footerHeight)
	}

	header := normalizeBlock(m.header(width, headerHeight < 8), width, headerHeight)
	body := m.body(width, bodyHeight, narrow)
	footer := normalizeBlock(m.footer(width, narrow), width, footerHeight)
	return strings.Join([]string{header, body, footer}, "\n")
}

func (m Model) body(width, height int, narrow bool) string {
	if narrow {
		return m.narrowBody(width, height)
	}

	gap := 2
	navWidth := minInt(26, maxInt(20, width/4))
	if width-navWidth-gap < 36 {
		return m.narrowBody(width, height)
	}
	detailWidth := width - navWidth - gap
	left := renderPanel(m.nav(panelContentWidth(navWidth), panelContentHeight(height)), navWidth, height)
	right := renderPanel(m.detail(panelContentWidth(detailWidth)), detailWidth, height)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat(" ", gap), right)
	return normalizeBlock(body, width, height)
}

func (m Model) narrowBody(width, height int) string {
	navHeight := minInt(6, maxInt(3, height/4))
	if height-navHeight < 4 {
		navHeight = maxInt(1, height/3)
	}
	detailHeight := maxInt(1, height-navHeight)
	nav := renderPanel(m.nav(panelContentWidth(width), panelContentHeight(navHeight)), width, navHeight)
	detail := renderPanel(m.detail(panelContentWidth(width)), width, detailHeight)
	return normalizeBlock(lipgloss.JoinVertical(lipgloss.Left, nav, detail), width, height)
}

func (m Model) header(width int, compact bool) string {
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
	style := headerStyle.MarginBottom(0).Width(maxInt(1, width-2))
	if compact {
		lines = lines[:3]
		style = style.Padding(0, 2)
	}
	return style.Render(strings.Join(lines, "\n"))
}

func (m Model) nav(width, height int) string {
	width = maxInt(8, width)
	height = maxInt(1, height)
	lines := []string{mutedStyle.Render("Sections")}
	if height == 1 {
		return lines[0]
	}

	visible := maxInt(1, height-1)
	offset := navScrollOffset(m.cursor, visible)
	end := minInt(len(sections), offset+visible)
	if offset > 0 && visible > 1 {
		lines = append(lines, mutedStyle.Render("  ..."))
		offset++
	}
	if end < len(sections) && visible > 1 {
		end--
	}
	for i := offset; i < end; i++ {
		line := "  " + sections[i]
		if i == m.cursor {
			line = selectedStyle.Render("> " + sections[i])
		} else {
			line = mutedStyle.Render("  " + sections[i])
		}
		lines = append(lines, line)
	}
	if end < len(sections) {
		lines = append(lines, mutedStyle.Render("  ..."))
	}
	return fitContent(strings.Join(lines, "\n"), width, height, false)
}

func navScrollOffset(cursor, visible int) int {
	if visible >= len(sections) {
		return 0
	}
	if cursor < 0 {
		return 0
	}
	if cursor >= len(sections) {
		cursor = len(sections) - 1
	}
	offset := cursor - visible/2
	if offset < 0 {
		return 0
	}
	maxOffset := len(sections) - visible
	if offset > maxOffset {
		return maxOffset
	}
	return offset
}

func (m Model) footer(width int, narrow bool) string {
	text := "up/down or j/k navigate  r export reports  q quit"
	if narrow {
		text = "j/k move  r export  q quit"
	}
	return mutedStyle.Width(width).Render(truncate(text, width))
}

func (m Model) detail(width int) string {
	switch sections[m.cursor] {
	case "Audit":
		return m.auditDetail(width)
	case "Context":
		return m.contextDetail(width)
	case "Structure":
		return m.structureDetail(width)
	case "Key Files":
		return m.keyFilesDetail(width)
	case "Connections":
		return m.connectionsDetail(width)
	case "API Routes":
		return m.routesDetail(width)
	case "Environment":
		return m.envDetail(width)
	case "AI Notes":
		return m.aiNotesDetail(width)
	case "Ask Help":
		return m.askHelpDetail(width)
	case "Findings":
		return m.findingsDetail(width)
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
	counts := fileKindCounts(m.analysis.Files)
	fmt.Fprintf(&b, "Repository: %s\n", firstNonEmpty(m.analysis.RepoName, filepath.Base(m.analysis.RepoPath)))
	fmt.Fprintf(&b, "Purpose: %s\n", fallback(m.analysis.Context.Purpose, "unknown"))
	fmt.Fprintf(&b, "Confidence: %s\n", fallback(m.analysis.Context.Confidence, "unknown"))
	fmt.Fprintf(&b, "Stack: %s\n", truncate(stackLine(m.analysis.Stack), width-7))
	fmt.Fprintf(&b, "Findings: %d (%s)\n", len(m.analysis.Findings), plainSeverityLine(severityCounts(m.analysis.Findings)))
	if m.analysis.Audit != nil {
		fmt.Fprintf(&b, "Audit: %s (exit %d)\n", auditStatus(m.analysis.Audit), m.analysis.Audit.ExitCode)
	} else {
		fmt.Fprintln(&b, "Audit: not run")
	}
	fmt.Fprintf(&b, "Files scanned: %d\n", len(m.analysis.Files))
	fmt.Fprintf(&b, "Kinds: source %d, config %d, tests %d, docs %d\n",
		counts[models.FileKindSource],
		counts[models.FileKindConfig],
		counts[models.FileKindTest],
		counts[models.FileKindDoc],
	)
	fmt.Fprintf(&b, "AI: %s\n", strings.TrimPrefix(aiStatus(m.analysis.AI), "AI: "))
	fmt.Fprintf(&b, "Generated: %s\n\n", m.analysis.GeneratedAt.Format("2006-01-02 15:04"))
	writeStatusLine(&b, "Health endpoint", m.analysis.Deployment.HasHealthEndpoint)
	writeStatusLine(&b, ".env.example", m.analysis.Deployment.HasEnvExample)
	writeStatusLine(&b, "Tests", m.analysis.Tests.HasTestFiles || m.analysis.Tests.HasTestScript)
	fmt.Fprintln(&b)
	writeTopFindings(&b, m.analysis.Findings, 4, width)
	return b.String()
}

func (m Model) auditDetail(width int) string {
	var b strings.Builder
	fmt.Fprintln(&b, sectionTitleStyle.Render("Audit"))
	if m.analysis.Audit == nil {
		fmt.Fprintln(&b, "Audit was not run. Use `stackmap audit .` or `stackmap analyze . --audit`.")
		return b.String()
	}
	audit := m.analysis.Audit
	fmt.Fprintf(&b, "Status: %s\n", auditStatus(audit))
	fmt.Fprintf(&b, "Exit code: %d\n", audit.ExitCode)
	fmt.Fprintf(&b, "Mode: %s\n", fallback(audit.Mode, "deployment-readiness"))
	fmt.Fprintf(&b, "Flags: allow-medium=%t  allow-missing-tests=%t  fail-on-low=%t\n\n", audit.AllowMedium, audit.AllowMissingTests, audit.FailOnLow)
	writeTextList(&b, "Blocking issues", audit.Reasons, 6, width)
	writeTextList(&b, "Warnings", audit.Warnings, 6, width)
	if len(audit.Reasons) == 0 && len(audit.Warnings) == 0 {
		fmt.Fprintln(&b, okStyle.Render("No audit blockers or warnings."))
	}
	return b.String()
}

func (m Model) contextDetail(width int) string {
	var b strings.Builder
	fmt.Fprintln(&b, sectionTitleStyle.Render("Project Context"))
	ctx := m.analysis.Context
	if strings.TrimSpace(ctx.Purpose) == "" {
		fmt.Fprintln(&b, "No project context was inferred.")
		return b.String()
	}
	fmt.Fprintf(&b, "Likely purpose: %s\n", ctx.Purpose)
	fmt.Fprintf(&b, "Confidence: %s\n", fallback(ctx.Confidence, "unknown"))
	if ctx.ReadmeTitle != "" {
		fmt.Fprintf(&b, "README title: %s\n", ctx.ReadmeTitle)
	}
	if ctx.PackageName != "" || ctx.PackageDescription != "" {
		fmt.Fprintf(&b, "Package: %s", fallback(ctx.PackageName, "unknown"))
		if ctx.PackageDescription != "" {
			fmt.Fprintf(&b, " - %s", ctx.PackageDescription)
		}
		fmt.Fprintln(&b)
	}
	if ctx.ReadmeSummary != "" {
		fmt.Fprintln(&b, "\nREADME summary:")
		writeWrapped(&b, ctx.ReadmeSummary, width, "  ")
	}
	writeTextList(&b, "Evidence", ctx.Evidence, 5, width)
	writeTextList(&b, "Script signals", ctx.ScriptSignals, 4, width)
	writeTextList(&b, "Env signals", ctx.EnvSignals, 4, width)
	writeTextList(&b, "Doc signals", ctx.DocSignals, 4, width)
	return b.String()
}

func (m Model) structureDetail(width int) string {
	var b strings.Builder
	fmt.Fprintln(&b, sectionTitleStyle.Render("Structure"))
	if len(m.analysis.Structure.Directories) == 0 {
		fmt.Fprintln(&b, "No directory roles were detected.")
		return b.String()
	}
	dirs := append([]models.DirectoryRole(nil), m.analysis.Structure.Directories...)
	sort.SliceStable(dirs, func(i, j int) bool {
		if directoryRank(dirs[i]) == directoryRank(dirs[j]) {
			return dirs[i].Path < dirs[j].Path
		}
		return directoryRank(dirs[i]) < directoryRank(dirs[j])
	})
	limit := minInt(len(dirs), 8)
	for i := 0; i < limit; i++ {
		dir := dirs[i]
		fmt.Fprintf(&b, "- %s  %s\n", truncate(dir.Path, width-8), mutedStyle.Render(fmt.Sprintf("(%d files)", dir.FileCount)))
		writeWrapped(&b, dir.Role, width, "  ")
	}
	if len(dirs) > limit {
		fmt.Fprintf(&b, "%s %d more directories\n", mutedStyle.Render("..."), len(dirs)-limit)
	}
	if len(m.analysis.Structure.KeyFiles) > 0 {
		fmt.Fprintln(&b, "\nKey files:")
		for i, file := range m.analysis.Structure.KeyFiles {
			if i >= 5 {
				fmt.Fprintf(&b, "%s %d more key files\n", mutedStyle.Render("..."), len(m.analysis.Structure.KeyFiles)-i)
				break
			}
			fmt.Fprintf(&b, "- %s - %s\n", truncate(file.Path, width-8), truncate(fallback(file.Role, file.Importance), width-8))
		}
	}
	return b.String()
}

func (m Model) keyFilesDetail(width int) string {
	var b strings.Builder
	fmt.Fprintln(&b, sectionTitleStyle.Render("Key Files"))
	if len(m.analysis.Structure.KeyFiles) == 0 {
		fmt.Fprintln(&b, "No key files were identified.")
		return b.String()
	}
	for i, file := range m.analysis.Structure.KeyFiles {
		if i >= 12 {
			fmt.Fprintf(&b, "%s %d more\n", mutedStyle.Render("..."), len(m.analysis.Structure.KeyFiles)-i)
			break
		}
		role := fallback(file.Role, "Key project file")
		if file.Importance != "" {
			role += " (" + file.Importance + ")"
		}
		fmt.Fprintf(&b, "- %s\n", truncate(file.Path, width-2))
		writeWrapped(&b, role, width, "  ")
	}
	return b.String()
}

func (m Model) connectionsDetail(width int) string {
	var b strings.Builder
	fmt.Fprintln(&b, sectionTitleStyle.Render("File Connections"))
	if len(m.analysis.Dependencies.TopConnectedFiles) == 0 {
		fmt.Fprintln(&b, "No internal file connection summary was detected.")
		return b.String()
	}
	for i, file := range m.analysis.Dependencies.TopConnectedFiles {
		if i >= 8 {
			fmt.Fprintf(&b, "%s %d more connected files\n", mutedStyle.Render("..."), len(m.analysis.Dependencies.TopConnectedFiles)-i)
			break
		}
		fmt.Fprintf(&b, "- %s\n", truncate(file.Path, width-2))
		meta := fmt.Sprintf("%s; imports %d, imported by %d", fallback(file.Role, "connected file"), file.ImportsCount, file.ImportedByCount)
		writeWrapped(&b, meta, width, "  ")
		if file.WhyItMatters != "" {
			writeWrapped(&b, file.WhyItMatters, width, "  ")
		}
	}
	writeTextList(&b, "Architecture hints", m.analysis.Dependencies.ArchitectureHints, 5, width)
	return b.String()
}

func (m Model) findingsDetail(width int) string {
	var b strings.Builder
	fmt.Fprintln(&b, sectionTitleStyle.Render("Findings"))
	if len(m.analysis.Findings) == 0 {
		fmt.Fprintln(&b, okStyle.Render("No findings. Nice and quiet."))
		return b.String()
	}
	written := 0
	for _, severity := range []models.Severity{models.SeverityHigh, models.SeverityMedium, models.SeverityLow, models.SeverityInfo} {
		group := findingsBySeverity(m.analysis.Findings, severity)
		if len(group) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n%s %d\n", severityBadge(severity), len(group))
		for _, f := range group {
			if written >= 10 {
				fmt.Fprintf(&b, "%s %d more findings\n", mutedStyle.Render("..."), len(m.analysis.Findings)-written)
				return b.String()
			}
			fmt.Fprintf(&b, "- %s: %s\n", fallback(f.Category, "general"), truncate(f.Message, width-4))
			if f.File != "" {
				fmt.Fprintf(&b, "  file: %s\n", truncate(f.File, width-8))
			}
			if f.Recommendation != "" {
				writeWrapped(&b, "recommendation: "+f.Recommendation, width, "  ")
			}
			written++
		}
	}
	return b.String()
}

func (m Model) routesDetail(width int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s\n\n", sectionTitleStyle.Render("API Routes"), mutedStyle.Render(fmt.Sprintf("%d detected", len(m.analysis.Routes))))
	if len(m.analysis.Routes) == 0 {
		fmt.Fprintln(&b, "No API routes detected.")
		return b.String()
	}
	health := healthRoutes(m.analysis.Routes)
	if len(health) > 0 {
		fmt.Fprintln(&b, okStyle.Render("Health routes detected:"))
		for _, route := range health {
			fmt.Fprintf(&b, "  %s %s\n", route.Method, route.Path)
		}
		fmt.Fprintln(&b)
	}
	grouped := routesByPrefix(m.analysis.Routes)
	written := 0
	for _, prefix := range sortedRouteGroups(grouped) {
		fmt.Fprintf(&b, "%s\n", mutedStyle.Render(prefix))
		for _, route := range grouped[prefix] {
			if written >= 12 {
				fmt.Fprintf(&b, "%s %d more routes\n", mutedStyle.Render("..."), len(m.analysis.Routes)-written)
				return b.String()
			}
			fmt.Fprintf(&b, "  %-7s %s\n", methodStyle.Render(route.Method), truncate(route.Path, width-12))
			fmt.Fprintf(&b, "          %s\n", mutedStyle.Render(truncate(route.SourceFile+" · "+route.Confidence, width-10)))
			if route.Note != "" {
				fmt.Fprintf(&b, "          %s\n", mutedStyle.Render(truncate(route.Note, width-10)))
			}
			written++
		}
	}
	return b.String()
}

func (m Model) envDetail(width int) string {
	var b strings.Builder
	fmt.Fprintln(&b, sectionTitleStyle.Render("Environment"))
	if !m.analysis.Env.UsesEnvVars {
		fmt.Fprintln(&b, "No environment variable usage detected.")
		fmt.Fprintf(&b, ".env.example: %s\n", presentWord(m.analysis.Env.ExampleFile != "" || m.analysis.Deployment.HasEnvExample))
		return b.String()
	}
	fmt.Fprintf(&b, ".env.example: %s\n", presentWord(m.analysis.Env.ExampleFile != "" || m.analysis.Deployment.HasEnvExample))
	fmt.Fprintf(&b, ".env file present: %s\n\n", presentWord(m.analysis.Env.EnvFilePresent))
	writeEnvClass(&b, "Required app config", m.analysis.Env.UsedVars, width, "required_app_config")
	writeEnvClass(&b, "Optional app config", m.analysis.Env.UsedVars, width, "optional_app_config")
	writeEnvClass(&b, "Platform/build vars", m.analysis.Env.UsedVars, width, "platform_provided", "build_metadata")
	writeEnvClass(&b, "Script-only vars", m.analysis.Env.UsedVars, width, "test_or_script_only")
	writeTextList(&b, "Missing required vars", m.analysis.Env.MissingRequiredFromExample, 8, width)
	return b.String()
}

func (m Model) testsDetail(width int) string {
	var b strings.Builder
	fmt.Fprintln(&b, sectionTitleStyle.Render("Tests & Deployment"))
	writeStatusLine(&b, "Test files", m.analysis.Tests.HasTestFiles)
	writeStatusLine(&b, "Test script", m.analysis.Tests.HasTestScript)
	writeStatusLine(&b, "Playwright", m.analysis.Tests.PlaywrightDetected)
	writeStatusLine(&b, "Health endpoint", m.analysis.Deployment.HasHealthEndpoint)
	writeStatusLine(&b, "Migrations", m.analysis.Deployment.HasMigrationFiles || m.analysis.Deployment.ReadmeMentionsMigrations)
	writeStatusLine(&b, "Deployment docs", m.analysis.Deployment.ReadmeMentionsDeploy)
	writeStatusLine(&b, "Dockerfile", m.analysis.Deployment.HasDockerfile)
	writeStatusLine(&b, "Vercel config", m.analysis.Deployment.HasVercelConfig)
	if len(m.analysis.Stack.Deployment) > 0 {
		fmt.Fprintf(&b, "\nDeployment target: %s\n", strings.Join(m.analysis.Stack.Deployment, ", "))
	}
	if len(m.analysis.Tests.Frameworks) > 0 {
		fmt.Fprintf(&b, "Frameworks: %s\n", strings.Join(m.analysis.Tests.Frameworks, ", "))
	}
	if m.analysis.Tests.TestScript != "" {
		fmt.Fprintf(&b, "Test script: %s\n", truncate(m.analysis.Tests.TestScript, width-13))
	}
	if len(m.analysis.Tests.TestFiles) > 0 {
		fmt.Fprintln(&b, "\nTest files:")
		for i, file := range m.analysis.Tests.TestFiles {
			if i >= 6 {
				fmt.Fprintf(&b, "%s %d more\n", mutedStyle.Render("..."), len(m.analysis.Tests.TestFiles)-i)
				break
			}
			fmt.Fprintf(&b, "- %s\n", truncate(file, width-2))
		}
	}
	return b.String()
}

func (m Model) deploymentDetail(width int) string {
	var b strings.Builder
	fmt.Fprintln(&b, sectionTitleStyle.Render("Deployment"))
	writeStatusLine(&b, "README", m.analysis.Deployment.HasReadme)
	writeStatusLine(&b, "Setup docs", m.analysis.Deployment.ReadmeMentionsSetup)
	writeStatusLine(&b, "Deployment docs", m.analysis.Deployment.ReadmeMentionsDeploy)
	writeStatusLine(&b, ".env.example", m.analysis.Deployment.HasEnvExample)
	writeStatusLine(&b, "Dockerfile", m.analysis.Deployment.HasDockerfile)
	writeStatusLine(&b, "Vercel config", m.analysis.Deployment.HasVercelConfig)
	writeStatusLine(&b, "Health endpoint", m.analysis.Deployment.HasHealthEndpoint)
	writeStatusLine(&b, "Migration files", m.analysis.Deployment.HasMigrationFiles)
	writeTextList(&b, "Deployment files", m.analysis.Deployment.DeploymentFiles, 6, width)
	writeTextList(&b, "Migration files", m.analysis.Deployment.MigrationFiles, 6, width)
	return b.String()
}

func (m Model) aiNotesDetail(width int) string {
	var b strings.Builder
	fmt.Fprintln(&b, sectionTitleStyle.Render("AI Notes"))
	if m.analysis.AI == nil {
		fmt.Fprintln(&b, "AI was not requested. Use `--ai` to generate local AI notes.")
		return b.String()
	}
	ai := m.analysis.AI
	fmt.Fprintln(&b, "Deterministic summary:")
	writeWrapped(&b, report.DeterministicAISummary(m.analysis), width, "  ")
	fmt.Fprintf(&b, "\nStatus: %s\n", strings.TrimPrefix(aiStatus(ai), "AI: "))
	if ai.Model != "" {
		fmt.Fprintf(&b, "Model: %s\n", ai.Model)
	}
	if len(ai.AttemptedModels) > 0 {
		fmt.Fprintf(&b, "Attempted: %s\n", strings.Join(ai.AttemptedModels, ", "))
	}
	if ai.Warning != "" || ai.ParseError != "" || ai.Relevance == "low_confidence" {
		fmt.Fprintln(&b, "\nLocal AI notes unavailable or not useful for this repository.")
		return b.String()
	}
	switch {
	case strings.TrimSpace(ai.LocalNotes) != "":
		fmt.Fprintln(&b, "\nLocal AI Notes:")
		writeWrapped(&b, strings.TrimSpace(ai.LocalNotes), width, "  ")
	case ai.ProjectSummary != "" || ai.ArchitectureOverview != "" || len(ai.KeyStrengths)+len(ai.PotentialRisks)+len(ai.RecommendedNextSteps) > 0:
		writeAIStructuredSections(&b, ai, width)
	default:
		fmt.Fprintln(&b, "\nLocal AI notes unavailable or not useful for this repository.")
	}
	return b.String()
}

func (m Model) askHelpDetail(width int) string {
	var b strings.Builder
	fmt.Fprintln(&b, sectionTitleStyle.Render("Ask / Q&A Help"))
	fmt.Fprintln(&b, "Examples:")
	examples := []string{
		`stackmap ask . "What is this project for?"`,
		`stackmap ask . "Where are the API routes?"`,
		`stackmap ask . "What should I review before deployment?"`,
		`stackmap ask . "How does this project use Postgres?"`,
	}
	for _, example := range examples {
		fmt.Fprintf(&b, "- %s\n", example)
	}
	fmt.Fprintln(&b, "\nSupported categories:")
	writeInlineList(&b, []string{"purpose", "stack", "structure", "file connections", "API routes", "environment", "database/storage", "tests", "deployment readiness"}, width)
	if result, ok := m.latestQA(); ok {
		fmt.Fprintln(&b, "\nLatest saved Q&A:")
		fmt.Fprintf(&b, "Q: %s\n", truncate(result.Question, width-3))
		writeWrapped(&b, "A: "+result.Answer, width, "")
		if result.Confidence != "" {
			fmt.Fprintf(&b, "Confidence: %s\n", result.Confidence)
		}
	} else {
		fmt.Fprintln(&b, "\nNo saved Q&A result found at .stackmap/qa/latest-question.json.")
	}
	return b.String()
}

func (m Model) latestQA() (*models.QAResult, bool) {
	data, err := os.ReadFile(filepath.Join(m.root, ".stackmap", "qa", "latest-question.json"))
	if err != nil {
		return nil, false
	}
	var result models.QAResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, false
	}
	if strings.TrimSpace(result.Question) == "" && strings.TrimSpace(result.Answer) == "" {
		return nil, false
	}
	return &result, true
}

func writeAIStructuredSections(b *strings.Builder, ai *models.AISummary, width int) {
	if ai.ProjectSummary != "" {
		fmt.Fprintln(b, "\nSummary:")
		writeWrapped(b, ai.ProjectSummary, width, "  ")
	}
	if ai.ArchitectureOverview != "" {
		fmt.Fprintln(b, "\nArchitecture:")
		writeWrapped(b, ai.ArchitectureOverview, width, "  ")
	}
	writeTextList(b, "Strengths", ai.KeyStrengths, 4, width)
	writeTextList(b, "Risks", ai.PotentialRisks, 4, width)
	writeTextList(b, "Next steps", ai.RecommendedNextSteps, 4, width)
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

func plainSeverityLine(counts map[models.Severity]int) string {
	return fmt.Sprintf("high %d, medium %d, low %d, info %d",
		counts[models.SeverityHigh],
		counts[models.SeverityMedium],
		counts[models.SeverityLow],
		counts[models.SeverityInfo],
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

func auditStatus(audit *models.AuditResult) string {
	if audit == nil {
		return "not run"
	}
	if audit.Passed {
		return okStyle.Render("passed")
	}
	return highStyle.Render("failed")
}

func fileKindCounts(files []models.FileInfo) map[models.FileKind]int {
	counts := map[models.FileKind]int{}
	for _, file := range files {
		counts[file.Kind]++
	}
	return counts
}

func findingsBySeverity(findings []models.Finding, severity models.Severity) []models.Finding {
	var out []models.Finding
	for _, finding := range findings {
		if finding.Severity == severity {
			out = append(out, finding)
		}
	}
	return out
}

func directoryRank(dir models.DirectoryRole) int {
	text := strings.ToLower(dir.Path + " " + dir.Role)
	switch {
	case strings.Contains(text, "entry") || strings.Contains(text, "api") || strings.Contains(text, "server") || strings.Contains(text, "app"):
		return 0
	case strings.Contains(text, "source") || strings.Contains(text, "frontend") || strings.Contains(text, "backend") || strings.Contains(text, "component"):
		return 1
	case strings.Contains(text, "config") || strings.Contains(text, "database") || strings.Contains(text, "migration"):
		return 2
	case strings.Contains(text, "test"):
		return 3
	case strings.Contains(text, "doc"):
		return 4
	default:
		return 5
	}
}

func routesByPrefix(routes []models.RouteInfo) map[string][]models.RouteInfo {
	groups := map[string][]models.RouteInfo{}
	for _, route := range routes {
		prefix := routePrefix(route.Path)
		groups[prefix] = append(groups[prefix], route)
	}
	return groups
}

func routePrefix(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "(unknown)"
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return "/"
	}
	if parts[0] == "api" && len(parts) > 1 {
		return "/api/" + parts[1]
	}
	return "/" + parts[0]
}

func sortedRouteGroups(groups map[string][]models.RouteInfo) []string {
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		iHealth := strings.Contains(keys[i], "health")
		jHealth := strings.Contains(keys[j], "health")
		if iHealth != jHealth {
			return iHealth
		}
		return keys[i] < keys[j]
	})
	return keys
}

func healthRoutes(routes []models.RouteInfo) []models.RouteInfo {
	var out []models.RouteInfo
	for _, route := range routes {
		if strings.Contains(strings.ToLower(route.Path), "health") {
			out = append(out, route)
		}
	}
	return out
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

func renderPanel(content string, width, height int) string {
	width = maxInt(8, width)
	height = maxInt(1, height)
	innerWidth := panelContentWidth(width)
	innerHeight := panelContentHeight(height)
	fitted := fitContent(content, innerWidth, innerHeight, true)
	rendered := panelStyle.MarginBottom(0).Width(maxInt(1, width-2)).Height(maxInt(1, height-2)).Render(fitted)
	return normalizeBlock(rendered, width, height)
}

func panelContentWidth(width int) int {
	return maxInt(1, width-6)
}

func panelContentHeight(height int) int {
	return maxInt(1, height-4)
}

func normalizeBlock(block string, width, height int) string {
	width = maxInt(1, width)
	height = maxInt(1, height)
	lines := strings.Split(strings.ReplaceAll(block, "\r\n", "\n"), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	out := make([]string, 0, height)
	for i := 0; i < height; i++ {
		line := ""
		if i < len(lines) {
			line = lines[i]
		}
		out = append(out, fitLine(line, width))
	}
	return strings.Join(out, "\n")
}

func fitContent(content string, width, height int, moreHint bool) string {
	width = maxInt(1, width)
	height = maxInt(1, height)
	raw := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	if len(raw) > 0 && raw[len(raw)-1] == "" {
		raw = raw[:len(raw)-1]
	}
	var lines []string
	for _, line := range raw {
		lines = append(lines, wrapOrClipLine(line, width)...)
	}
	if len(lines) > height {
		if moreHint && height > 0 {
			hidden := len(lines) - height + 1
			lines = append(lines[:height-1], mutedStyle.Render(fmt.Sprintf("... and %d more", hidden)))
		} else {
			lines = lines[:height]
		}
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	for i, line := range lines {
		lines[i] = fitLine(line, width)
	}
	return strings.Join(lines, "\n")
}

func wrapOrClipLine(line string, width int) []string {
	if lipgloss.Width(line) <= width {
		return []string{line}
	}
	if strings.Contains(line, "\x1b") {
		return []string{truncate(stripANSI(line), width)}
	}
	return wrapPlainLine(line, width)
}

func wrapPlainLine(line string, width int) []string {
	width = maxInt(1, width)
	leading := lineIndent(line)
	available := maxInt(8, width-lipgloss.Width(leading))
	var out []string
	current := ""
	for _, word := range strings.Fields(strings.TrimSpace(line)) {
		next := word
		if current != "" {
			next = current + " " + word
		}
		if lipgloss.Width(next) > available && current != "" {
			out = append(out, leading+current)
			current = word
			continue
		}
		current = next
	}
	if current != "" {
		out = append(out, leading+current)
	}
	if len(out) == 0 {
		return []string{truncate(line, width)}
	}
	return out
}

func fitLine(line string, width int) string {
	width = maxInt(1, width)
	if lipgloss.Width(line) > width {
		line = truncate(stripANSI(line), width)
	}
	for lipgloss.Width(line) < width {
		line += " "
	}
	return line
}

func lineIndent(line string) string {
	var b strings.Builder
	for _, r := range line {
		if r != ' ' && r != '\t' {
			break
		}
		b.WriteRune(r)
	}
	return b.String()
}

func stripANSI(text string) string {
	var b strings.Builder
	inEscape := false
	for _, r := range text {
		if inEscape {
			if r >= '@' && r <= '~' {
				inEscape = false
			}
			continue
		}
		if r == '\x1b' {
			inEscape = true
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func writeTextList(b *strings.Builder, label string, items []string, limit, width int) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(b, "\n%s:\n", label)
	for i, item := range items {
		if i >= limit {
			fmt.Fprintf(b, "%s %d more\n", mutedStyle.Render("..."), len(items)-i)
			return
		}
		writeWrapped(b, "- "+item, width, "")
	}
}

func writeInlineList(b *strings.Builder, items []string, width int) {
	writeWrapped(b, strings.Join(items, ", "), width, "")
}

func writeWrapped(b *strings.Builder, text string, width int, indent string) {
	width = maxInt(24, width)
	available := width - lipgloss.Width(indent)
	if available < 16 {
		available = width
	}
	for _, paragraph := range strings.Split(strings.TrimSpace(text), "\n") {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			fmt.Fprintln(b)
			continue
		}
		line := ""
		for _, word := range strings.Fields(paragraph) {
			next := word
			if line != "" {
				next = line + " " + word
			}
			if lipgloss.Width(next) > available && line != "" {
				fmt.Fprintf(b, "%s%s\n", indent, line)
				line = word
				continue
			}
			line = next
		}
		if line != "" {
			fmt.Fprintf(b, "%s%s\n", indent, line)
		}
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && value != "." && value != string(filepath.Separator) {
			return value
		}
	}
	return "unknown"
}

func fallback(value, replacement string) string {
	if strings.TrimSpace(value) == "" {
		return replacement
	}
	return value
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func Run(analysis *models.Analysis, root string) error {
	_, err := tea.NewProgram(New(analysis, root)).Run()
	return err
}
