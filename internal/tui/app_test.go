package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/wcw-wcw/stackindex/internal/models"
)

func TestOverviewIncludesPurpose(t *testing.T) {
	model := testModel(t, fixtureAnalysis(), t.TempDir())
	model.cursor = sectionIndex(t, "Overview")

	out := model.detail(80)
	assertContains(t, out, "Purpose: Go CLI/TUI repository analysis tool")
	assertContains(t, out, "Files scanned: 4")
}

func TestProjectContextSectionRenders(t *testing.T) {
	model := testModel(t, fixtureAnalysis(), t.TempDir())
	model.cursor = sectionIndex(t, "Context")

	out := model.detail(80)
	assertContains(t, out, "Project Context")
	assertContains(t, out, "README summary")
	assertContains(t, out, "README/package metadata")
}

func TestStructureSectionRenders(t *testing.T) {
	model := testModel(t, fixtureAnalysis(), t.TempDir())
	model.cursor = sectionIndex(t, "Structure")

	out := model.detail(80)
	assertContains(t, out, "Structure")
	assertContains(t, out, "cmd/stackindex")
	assertContains(t, out, "Key files")
}

func TestFileConnectionsSectionRenders(t *testing.T) {
	model := testModel(t, fixtureAnalysis(), t.TempDir())
	model.cursor = sectionIndex(t, "Connections")

	out := model.detail(80)
	assertContains(t, out, "File Connections")
	assertContains(t, out, "internal/tui/app.go")
	assertContains(t, out, "Architecture hints")
}

func TestAuditSectionRendersWhenPresent(t *testing.T) {
	model := testModel(t, fixtureAnalysis(), t.TempDir())
	model.cursor = sectionIndex(t, "Audit")

	out := stripANSI(model.detail(80))
	assertContains(t, out, "Audit")
	assertContains(t, out, "Status: passed")
	assertContains(t, out, "Exit code: 0")
	assertContains(t, out, "Flags: allow-medium=false  allow-missing-tests=false  fail-on-low=false")
	assertContains(t, out, "Warnings")
	assertContains(t, out, "Review generated reports before release.")
}

func TestOverviewRendersAuditStatusWhenPresent(t *testing.T) {
	model := testModel(t, fixtureAnalysis(), t.TempDir())
	model.cursor = sectionIndex(t, "Overview")

	out := stripANSI(model.detail(80))
	assertContains(t, out, "Audit: passed (exit 0)")
}

func TestReportsHideAIParseWarningForGeneratedText(t *testing.T) {
	analysis := fixtureAnalysis()
	analysis.AI = &models.AISummary{
		Enabled:    true,
		Status:     "generated_text",
		Model:      "llama3.2:3b",
		RawText:    "Plain text summary.",
		ParseError: "response did not contain a JSON object",
	}
	model := testModel(t, analysis, t.TempDir())
	model.cursor = sectionIndex(t, "Reports")

	out := stripANSI(model.detail(80))
	assertContains(t, out, "AI: summary generated with llama3.2:3b")
	if strings.Contains(out, "AI parse warning") || strings.Contains(out, "response did not contain a JSON object") {
		t.Fatalf("reports view showed parse internals for generated_text AI:\n%s", out)
	}
}

func TestAINotesRenderLocalNotesForGeneratedTextWithParseError(t *testing.T) {
	analysis := fixtureAnalysis()
	analysis.AI = &models.AISummary{
		Enabled:    true,
		Status:     "generated_text",
		Model:      "llama3.2:3b",
		LocalNotes: "This repository is a local-first Go CLI with a Bubble Tea TUI.",
		RawText:    "This repository is a local-first Go CLI with a Bubble Tea TUI.",
		ParseError: "response did not contain a JSON object",
	}
	model := testModel(t, analysis, t.TempDir())
	model.cursor = sectionIndex(t, "AI Notes")

	out := stripANSI(model.detail(80))
	assertContains(t, out, "Local AI Notes")
	assertContains(t, out, "local-first Go CLI")
	if strings.Contains(out, "Local AI notes unavailable") || strings.Contains(out, "response did not contain a JSON object") {
		t.Fatalf("AI notes view hid usable notes or showed parse internals:\n%s", out)
	}
}

func TestAskHelpSectionRendersLatestQuestion(t *testing.T) {
	root := t.TempDir()
	qaDir := filepath.Join(root, ".stackindex", "qa")
	if err := os.MkdirAll(qaDir, 0755); err != nil {
		t.Fatalf("mkdir qa dir: %v", err)
	}
	data := []byte(`{"question":"Where are the API routes?","answer":"Routes are in cmd/stackindex.","confidence":"high","mode":"deterministic"}`)
	if err := os.WriteFile(filepath.Join(qaDir, "latest-question.json"), data, 0644); err != nil {
		t.Fatalf("write latest qa: %v", err)
	}
	model := testModel(t, fixtureAnalysis(), root)
	model.cursor = sectionIndex(t, "Ask Help")

	out := model.detail(80)
	assertContains(t, out, "Ask / Q&A Help")
	assertContains(t, out, "Latest saved Q&A")
	assertContains(t, out, "Where are the API routes?")
}

func TestAskHelpSectionRendersInputPrompt(t *testing.T) {
	model := testModel(t, fixtureAnalysis(), t.TempDir())
	model.cursor = sectionIndex(t, "Ask Help")

	out := stripANSI(model.detail(80))
	assertContains(t, out, "Ask:")
	assertContains(t, out, ">")
	assertContains(t, out, "Ask a question about this analysis")
	if strings.Contains(out, "38;5;") || strings.Contains(out, "\x1b") {
		t.Fatalf("ask input leaked ANSI styling text:\n%s", out)
	}
}

func TestAskHelpInputPromptDoesNotLeakANSIWhenFitted(t *testing.T) {
	model := testModel(t, fixtureAnalysis(), t.TempDir())
	model.cursor = sectionIndex(t, "Ask Help")

	out := stripANSI(model.detailWithHeight(80, 24))
	assertContains(t, out, "Ask: > Ask a question about this analysis")
	if strings.Contains(out, "38;5;") || strings.Contains(out, "\x1b") {
		t.Fatalf("fitted ask input leaked ANSI styling text:\n%s", out)
	}
}

func TestStripANSIConsumesCSIParameters(t *testing.T) {
	got := stripANSI("\x1b[38;5;240mAsk a question\x1b[0m")
	if got != "Ask a question" {
		t.Fatalf("stripANSI = %q", got)
	}
}

func TestAskHelpSubmitUpdatesDisplayedQA(t *testing.T) {
	model := testModel(t, fixtureAnalysis(), t.TempDir())
	model.cursor = sectionIndex(t, "Ask Help")

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("What is this project for?")})
	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})

	out := stripANSI(model.detail(100))
	assertContains(t, out, "Current Q&A")
	assertContains(t, out, "What is this project for?")
	assertContains(t, out, "Go CLI/TUI repository analysis tool")
	assertContains(t, out, "Confidence: high")
	assertContains(t, out, "Evidence:")
}

func TestAskHelpSubmitWritesLatestQuestion(t *testing.T) {
	root := t.TempDir()
	model := testModel(t, fixtureAnalysis(), root)
	model.cursor = sectionIndex(t, "Ask Help")

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Where are the API routes?")})
	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})

	data, err := os.ReadFile(filepath.Join(root, ".stackindex", "qa", "latest-question.json"))
	if err != nil {
		t.Fatalf("read latest qa: %v", err)
	}
	var result models.QAResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal latest qa: %v", err)
	}
	if result.Question != "Where are the API routes?" {
		t.Fatalf("latest question = %q", result.Question)
	}
	if strings.TrimSpace(result.Answer) == "" {
		t.Fatalf("latest answer was empty: %s", string(data))
	}
}

func TestAskHelpSubmitAppendsHistory(t *testing.T) {
	root := t.TempDir()
	model := testModel(t, fixtureAnalysis(), root)
	model.cursor = sectionIndex(t, "Ask Help")

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("What is this project for?")})
	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})

	data, err := os.ReadFile(filepath.Join(root, ".stackindex", "qa", "history.jsonl"))
	if err != nil {
		t.Fatalf("read history qa: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("history line count = %d, want 1:\n%s", len(lines), data)
	}
	var result models.QAResult
	if err := json.Unmarshal([]byte(lines[0]), &result); err != nil {
		t.Fatalf("unmarshal history qa: %v", err)
	}
	if result.Question != "What is this project for?" {
		t.Fatalf("history question = %q", result.Question)
	}
}

func TestAskHelpSectionRendersRecentQuestions(t *testing.T) {
	root := t.TempDir()
	qaDir := filepath.Join(root, ".stackindex", "qa")
	if err := os.MkdirAll(qaDir, 0755); err != nil {
		t.Fatalf("mkdir qa dir: %v", err)
	}
	history := strings.Join([]string{
		`{"question":"What is this project for?","answer":"A tool.","confidence":"high","mode":"deterministic"}`,
		`malformed`,
		`{"question":"Where are the API routes?","answer":"Routes.","confidence":"high","mode":"deterministic"}`,
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(qaDir, "history.jsonl"), []byte(history), 0644); err != nil {
		t.Fatalf("write history qa: %v", err)
	}
	model := testModel(t, fixtureAnalysis(), root)
	model.cursor = sectionIndex(t, "Ask Help")

	out := stripANSI(model.detail(100))
	assertContains(t, out, "Recent questions:")
	assertContains(t, out, "Where are the API routes?")
	assertContains(t, out, "What is this project for?")
}

func TestLongDetailContentCanScroll(t *testing.T) {
	analysis := fixtureAnalysis()
	analysis.AI = &models.AISummary{
		Enabled:    true,
		Status:     "generated_text",
		Model:      "llama3.2:3b",
		LocalNotes: numberedLines("note line", 40),
	}
	model := testModel(t, analysis, t.TempDir())
	model.width = 92
	model.height = 32
	model.cursor = sectionIndex(t, "AI Notes")

	before := stripANSI(model.View())
	assertContains(t, before, "note line 01")
	if strings.Contains(before, "note line 20") {
		t.Fatalf("unscrolled view unexpectedly showed later content:\n%s", before)
	}

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyPgDown})
	after := stripANSI(model.View())
	assertContains(t, after, "note line 20")
	if strings.Contains(after, "note line 01") {
		t.Fatalf("scrolled view still showed top content:\n%s", after)
	}
}

func TestAPIRoutesRenderAllRoutesWithoutCountCap(t *testing.T) {
	analysis := fixtureAnalysis()
	analysis.Routes = manyRoutes(30)
	model := testModel(t, analysis, t.TempDir())
	model.cursor = sectionIndex(t, "API Routes")

	out := stripANSI(model.detail(100))
	assertContains(t, out, "/api/items/01")
	assertContains(t, out, "/api/items/30")
	if strings.Contains(out, "more routes") {
		t.Fatalf("routes detail still capped route list:\n%s", out)
	}
}

func TestTestsRenderAllTestFilesWithoutCountCap(t *testing.T) {
	analysis := fixtureAnalysis()
	analysis.Tests.TestFiles = manyTestFiles(18)
	model := testModel(t, analysis, t.TempDir())
	model.cursor = sectionIndex(t, "Tests")

	out := stripANSI(model.detail(100))
	assertContains(t, out, "src/features/feature_01_test.go")
	assertContains(t, out, "src/features/feature_18_test.go")
	if strings.Contains(out, "more") {
		t.Fatalf("tests detail still capped test files:\n%s", out)
	}
}

func TestKeyFilesRenderAllItemsWithoutCountCap(t *testing.T) {
	analysis := fixtureAnalysis()
	analysis.Structure.KeyFiles = manyKeyFiles(20)
	model := testModel(t, analysis, t.TempDir())
	model.cursor = sectionIndex(t, "Key Files")

	out := stripANSI(model.detail(100))
	assertContains(t, out, "src/key/file_01.go")
	assertContains(t, out, "src/key/file_20.go")
	if strings.Contains(out, "more") {
		t.Fatalf("key files detail still capped key files:\n%s", out)
	}
}

func TestExpandedRoutesStillScroll(t *testing.T) {
	analysis := fixtureAnalysis()
	analysis.Routes = manyRoutes(40)
	model := testModel(t, analysis, t.TempDir())
	model.width = 92
	model.height = 24
	model.cursor = sectionIndex(t, "API Routes")

	before := stripANSI(model.View())
	assertContains(t, before, "/api/items/01")
	if strings.Contains(before, "/api/items/40") {
		t.Fatalf("unscrolled routes view unexpectedly showed final route:\n%s", before)
	}

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyPgDown})
	after := stripANSI(model.View())
	if before == after {
		t.Fatalf("scrolling expanded routes did not change view:\n%s", after)
	}
	assertContains(t, after, "scroll")
}

func TestExpandedRoutesFrameHeightWidthRemainStable(t *testing.T) {
	analysis := fixtureAnalysis()
	analysis.Routes = manyRoutes(40)
	model := testModel(t, analysis, t.TempDir())
	model.width = 82
	model.height = 18
	model.cursor = sectionIndex(t, "API Routes")
	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyPgDown})

	out := model.View()
	if got := lineCount(out); got != model.height {
		t.Fatalf("View line count = %d, want %d:\n%s", got, model.height, out)
	}
	for i, line := range strings.Split(out, "\n") {
		if got := displayWidth(line); got != model.width {
			t.Fatalf("line %d width = %d, want %d:\n%s", i+1, got, model.width, out)
		}
	}
}

func TestChangingSectionsResetsDetailScroll(t *testing.T) {
	analysis := fixtureAnalysis()
	analysis.AI = &models.AISummary{
		Enabled:    true,
		Status:     "generated_text",
		Model:      "llama3.2:3b",
		LocalNotes: numberedLines("note line", 40),
	}
	model := testModel(t, analysis, t.TempDir())
	model.width = 92
	model.height = 32
	model.cursor = sectionIndex(t, "AI Notes")

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyPgDown})
	if model.detailScroll == 0 {
		t.Fatal("detailScroll was not advanced")
	}
	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyDown})
	if model.detailScroll != 0 {
		t.Fatalf("detailScroll = %d, want reset to 0", model.detailScroll)
	}
}

func TestAskHelpTypingStillAcceptsScrollLetters(t *testing.T) {
	model := testModel(t, fixtureAnalysis(), t.TempDir())
	model.cursor = sectionIndex(t, "Ask Help")
	model.askTyping = true
	model.askInput.Focus()
	model.askInput.SetValue("Where")

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if got := model.askInput.Value(); got != "Whered" {
		t.Fatalf("ask input value = %q, want Whered", got)
	}
	if model.detailScroll != 0 {
		t.Fatalf("detailScroll = %d, want 0 while typing", model.detailScroll)
	}
}

func TestScrolledFrameHeightWidthRemainStable(t *testing.T) {
	analysis := fixtureAnalysis()
	analysis.AI = &models.AISummary{
		Enabled:    true,
		Status:     "generated_text",
		Model:      "llama3.2:3b",
		LocalNotes: numberedLines("note line", 40),
	}
	model := testModel(t, analysis, t.TempDir())
	model.width = 82
	model.height = 18
	model.cursor = sectionIndex(t, "AI Notes")
	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyPgDown})

	out := model.View()
	if got := lineCount(out); got != model.height {
		t.Fatalf("View line count = %d, want %d:\n%s", got, model.height, out)
	}
	for i, line := range strings.Split(out, "\n") {
		if got := displayWidth(line); got != model.width {
			t.Fatalf("line %d width = %d, want %d:\n%s", i+1, got, model.width, out)
		}
	}
}

func TestFooterMentionsScrollOnlyWhenUseful(t *testing.T) {
	model := testModel(t, fixtureAnalysis(), t.TempDir())
	model.cursor = sectionIndex(t, "Overview")
	if strings.Contains(stripANSI(model.footer(100, false)), "scroll") {
		t.Fatalf("footer mentioned scroll for short content: %q", model.footer(100, false))
	}

	analysis := fixtureAnalysis()
	analysis.AI = &models.AISummary{Enabled: true, Status: "generated_text", LocalNotes: numberedLines("note line", 40)}
	model = testModel(t, analysis, t.TempDir())
	model.cursor = sectionIndex(t, "AI Notes")
	assertContains(t, stripANSI(model.footer(120, false)), "pgup/pgdn or u/d scroll")
}

func TestStatusFeedbackForQASubmitAndAudit(t *testing.T) {
	model := testModel(t, fixtureAnalysis(), t.TempDir())
	assertContains(t, stripANSI(model.footer(120, false)), "Audit passed")
	model.cursor = sectionIndex(t, "Ask Help")

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("What is this project for?")})
	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})

	assertContains(t, stripANSI(model.footer(120, false)), "Q&A done and saved")
	assertContains(t, stripANSI(model.detail(100)), "history.jsonl")
}

func TestEmptyStatesDoNotPanic(t *testing.T) {
	model := testModel(t, &models.Analysis{}, t.TempDir())
	for _, section := range sections {
		model.cursor = sectionIndex(t, section)
		_ = model.detail(60)
	}
}

func TestViewRendersStableFrameHeight(t *testing.T) {
	model := testModel(t, fixtureAnalysis(), t.TempDir())
	model.width = 82
	model.height = 18

	out := model.View()
	if got := lineCount(out); got != model.height {
		t.Fatalf("View line count = %d, want %d:\n%s", got, model.height, out)
	}
	for i, line := range strings.Split(out, "\n") {
		if got := displayWidth(line); got != model.width {
			t.Fatalf("line %d width = %d, want %d:\n%s", i+1, got, model.width, out)
		}
	}
}

func TestNavRendersSectionsOnlyOnce(t *testing.T) {
	model := testModel(t, fixtureAnalysis(), t.TempDir())
	nav := stripANSI(model.nav(18, 8))

	if got := strings.Count(nav, "Sections"); got != 1 {
		t.Fatalf("Sections count = %d, want 1:\n%s", got, nav)
	}
	for _, section := range sections {
		if got := strings.Count(nav, section); got > 1 {
			t.Fatalf("section %q appeared %d times:\n%s", section, got, nav)
		}
	}
}

func TestNavScrollKeepsSelectedSectionVisible(t *testing.T) {
	model := testModel(t, fixtureAnalysis(), t.TempDir())
	model.cursor = sectionIndex(t, "Reports")

	nav := stripANSI(model.nav(18, 6))
	assertContains(t, nav, "Reports")
	if strings.Contains(nav, "Overview") && !strings.Contains(nav, "...") {
		t.Fatalf("short nav did not appear to scroll:\n%s", nav)
	}
}

func TestShortViewKeepsSelectedNavVisible(t *testing.T) {
	model := testModel(t, fixtureAnalysis(), t.TempDir())
	model.width = 80
	model.height = 14
	model.cursor = sectionIndex(t, "Reports")

	out := stripANSI(model.View())
	assertContains(t, out, "> Reports")
	if got := strings.Count(out, "Sections"); got != 1 {
		t.Fatalf("Sections count = %d, want 1:\n%s", got, out)
	}
	if got := lineCount(out); got != model.height {
		t.Fatalf("short view line count = %d, want %d:\n%s", got, model.height, out)
	}
}

func TestViewDoesNotCarryPreviousSectionContent(t *testing.T) {
	model := testModel(t, fixtureAnalysis(), t.TempDir())
	model.width = 90
	model.height = 20
	model.cursor = sectionIndex(t, "Overview")
	overview := stripANSI(model.View())
	assertContains(t, overview, "Purpose:")

	model.cursor = sectionIndex(t, "Reports")
	reports := stripANSI(model.View())
	assertContains(t, reports, "Reports")
	if strings.Contains(reports, "Purpose:") {
		t.Fatalf("reports view carried overview content:\n%s", reports)
	}
	if got := lineCount(reports); got != model.height {
		t.Fatalf("reports line count = %d, want %d", got, model.height)
	}
}

func TestLongLinesAreWrappedOrClippedToFrameWidth(t *testing.T) {
	analysis := fixtureAnalysis()
	analysis.Routes = []models.RouteInfo{{
		Method:     "GET",
		Path:       "/api/" + strings.Repeat("very-long-segment/", 12),
		SourceFile: "src/" + strings.Repeat("deeply-nested-directory/", 12) + "route.ts",
		Confidence: "high",
	}}
	model := testModel(t, analysis, t.TempDir())
	model.width = 76
	model.height = 16
	model.cursor = sectionIndex(t, "API Routes")

	out := model.View()
	for i, line := range strings.Split(out, "\n") {
		if got := displayWidth(line); got > model.width {
			t.Fatalf("line %d width = %d, want <= %d:\n%s", i+1, got, model.width, out)
		}
	}
}

func fixtureAnalysis() *models.Analysis {
	return &models.Analysis{
		RepoPath:    "/tmp/stackindex",
		RepoName:    "stackindex",
		GeneratedAt: time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC),
		Files: []models.FileInfo{
			{Path: "cmd/stackindex/main.go", Kind: models.FileKindSource},
			{Path: "internal/tui/app.go", Kind: models.FileKindSource},
			{Path: "internal/tui/app_test.go", Kind: models.FileKindTest},
			{Path: "README.md", Kind: models.FileKindDoc},
		},
		Stack: models.StackInfo{
			Languages:  []string{"Go"},
			Frameworks: []string{"Bubble Tea"},
			Testing:    []string{"go test"},
		},
		Context: models.ProjectContext{
			Purpose:       "Go CLI/TUI repository analysis tool",
			Confidence:    "high",
			ReadmeTitle:   "StackIndex",
			ReadmeSummary: "StackIndex analyzes a local repository and presents concise reports for developers.",
			Evidence:      []string{"README/package metadata points to go cli/tui repository analysis tool."},
			ScriptSignals: []string{"go test ./..."},
			EnvSignals:    []string{"DATABASE_URL"},
		},
		Structure: models.StructureMap{
			Directories: []models.DirectoryRole{
				{Path: "cmd/stackindex", Role: "CLI entrypoint", FileCount: 1},
				{Path: "internal/tui", Role: "Bubble Tea terminal UI", FileCount: 2},
			},
			KeyFiles: []models.FileRole{
				{Path: "cmd/stackindex/main.go", Role: "CLI command routing", Importance: "high"},
				{Path: "internal/tui/app.go", Role: "TUI model and views", Importance: "high"},
			},
		},
		Dependencies: models.DependencyGraph{
			TopConnectedFiles: []models.ConnectedFileSummary{
				{Path: "internal/tui/app.go", Role: "TUI model", ImportsCount: 3, ImportedByCount: 1, WhyItMatters: "Centralizes terminal report browsing."},
			},
			ArchitectureHints: []string{"CLI delegates report browsing to the TUI package."},
		},
		Routes: []models.RouteInfo{
			{Method: "GET", Path: "/api/health", SourceFile: "cmd/stackindex/main.go", Confidence: "medium"},
		},
		Env: models.EnvAnalysis{
			UsesEnvVars:                true,
			ExampleFile:                ".env.example",
			UsedVars:                   []models.EnvVar{{Name: "DATABASE_URL", Classification: "required_app_config"}},
			MissingRequiredFromExample: []string{"DATABASE_URL"},
		},
		Tests: models.TestAnalysis{
			HasTestFiles:  true,
			HasTestScript: true,
			Frameworks:    []string{"go test"},
			TestFiles:     []string{"internal/tui/app_test.go"},
			TestScript:    "go test ./...",
		},
		Deployment: models.DeploymentAnalysis{
			HasReadme:            true,
			HasEnvExample:        true,
			HasHealthEndpoint:    true,
			HasMigrationFiles:    true,
			ReadmeMentionsDeploy: true,
		},
		Findings: []models.Finding{
			{Severity: models.SeverityMedium, Category: "tests", Message: "Add more coverage.", Recommendation: "Add TUI rendering tests."},
		},
		AI: &models.AISummary{
			Enabled:        true,
			Status:         "generated_structured",
			Model:          "llama3.2:3b",
			ProjectSummary: "StackIndex is a local developer analysis tool.",
		},
		Audit: &models.AuditResult{
			Passed:   true,
			ExitCode: 0,
			Mode:     "deployment-readiness",
			Warnings: []string{"Review generated reports before release."},
		},
	}
}

func testModel(t *testing.T, analysis *models.Analysis, root string) Model {
	t.Helper()
	model := New(analysis, root)
	model.width = 100
	model.height = 32
	return model
}

func updateModel(t *testing.T, model Model, msg tea.Msg) Model {
	t.Helper()
	next, _ := model.Update(msg)
	updated, ok := next.(Model)
	if !ok {
		t.Fatalf("updated model type = %T", next)
	}
	return updated
}

func sectionIndex(t *testing.T, name string) int {
	t.Helper()
	for i, section := range sections {
		if section == name {
			return i
		}
	}
	t.Fatalf("section %q not found", name)
	return 0
}

func assertContains(t *testing.T, out, want string) {
	t.Helper()
	if !strings.Contains(out, want) {
		t.Fatalf("output did not contain %q:\n%s", want, out)
	}
}

func lineCount(out string) int {
	if out == "" {
		return 0
	}
	return len(strings.Split(out, "\n"))
}

func displayWidth(line string) int {
	return len([]rune(stripANSI(line)))
}

func numberedLines(prefix string, count int) string {
	var lines []string
	for i := 1; i <= count; i++ {
		lines = append(lines, prefix+" "+fmt.Sprintf("%02d", i))
	}
	return strings.Join(lines, "\n")
}

func manyRoutes(count int) []models.RouteInfo {
	routes := make([]models.RouteInfo, 0, count)
	for i := 1; i <= count; i++ {
		routes = append(routes, models.RouteInfo{
			Method:     "GET",
			Path:       fmt.Sprintf("/api/items/%02d", i),
			SourceFile: fmt.Sprintf("src/app/api/items/%02d/route.ts", i),
			Confidence: "high",
		})
	}
	return routes
}

func manyTestFiles(count int) []string {
	files := make([]string, 0, count)
	for i := 1; i <= count; i++ {
		files = append(files, fmt.Sprintf("src/features/feature_%02d_test.go", i))
	}
	return files
}

func manyKeyFiles(count int) []models.FileRole {
	files := make([]models.FileRole, 0, count)
	for i := 1; i <= count; i++ {
		files = append(files, models.FileRole{
			Path:       fmt.Sprintf("src/key/file_%02d.go", i),
			Role:       "Important generated fixture file",
			Importance: "high",
		})
	}
	return files
}
