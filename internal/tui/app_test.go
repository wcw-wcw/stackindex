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

	out := model.detail(80)
	assertContains(t, out, "Audit")
	assertContains(t, out, "Status:")
	assertContains(t, out, "Warnings")
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
