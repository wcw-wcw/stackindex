package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/will/stackmap/internal/models"
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
	assertContains(t, out, "cmd/stackmap")
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

func TestAskHelpSectionRendersLatestQuestion(t *testing.T) {
	root := t.TempDir()
	qaDir := filepath.Join(root, ".stackmap", "qa")
	if err := os.MkdirAll(qaDir, 0755); err != nil {
		t.Fatalf("mkdir qa dir: %v", err)
	}
	data := []byte(`{"question":"Where are the API routes?","answer":"Routes are in cmd/stackmap.","confidence":"high","mode":"deterministic"}`)
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
		RepoPath:    "/tmp/stackmap",
		RepoName:    "stackmap",
		GeneratedAt: time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC),
		Files: []models.FileInfo{
			{Path: "cmd/stackmap/main.go", Kind: models.FileKindSource},
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
			ReadmeTitle:   "StackMap",
			ReadmeSummary: "StackMap analyzes a local repository and presents concise reports for developers.",
			Evidence:      []string{"README/package metadata points to go cli/tui repository analysis tool."},
			ScriptSignals: []string{"go test ./..."},
			EnvSignals:    []string{"DATABASE_URL"},
		},
		Structure: models.StructureMap{
			Directories: []models.DirectoryRole{
				{Path: "cmd/stackmap", Role: "CLI entrypoint", FileCount: 1},
				{Path: "internal/tui", Role: "Bubble Tea terminal UI", FileCount: 2},
			},
			KeyFiles: []models.FileRole{
				{Path: "cmd/stackmap/main.go", Role: "CLI command routing", Importance: "high"},
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
			{Method: "GET", Path: "/api/health", SourceFile: "cmd/stackmap/main.go", Confidence: "medium"},
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
			ProjectSummary: "StackMap is a local developer analysis tool.",
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
