package backend

import (
	"path/filepath"
	"testing"

	"github.com/will/stackmap/internal/models"
)

func TestBuildAnalyzeResponseMapsDesktopSummary(t *testing.T) {
	root := filepath.Join("tmp", "example")
	analysis := &models.Analysis{
		RepoName: "example",
		RepoPath: root,
		Files:    []models.FileInfo{{Path: "main.go"}, {Path: "main_test.go"}},
		Stack: models.StackInfo{
			Languages:  []string{"Go"},
			Frameworks: []string{"Wails"},
			Databases:  []string{"SQLite"},
		},
		Routes: []models.RouteInfo{{Method: "GET", Path: "/health"}},
		Tests:  models.TestAnalysis{HasTestFiles: true, HasTestScript: true, Frameworks: []string{"go test"}, TestFiles: []string{"main_test.go"}, TestScript: "go test ./..."},
		Context: models.ProjectContext{
			Purpose:    "Example service",
			Confidence: "high",
			Evidence:   []string{"README title matched service language."},
		},
		Findings: []models.Finding{
			{Severity: models.SeverityHigh},
			{Severity: models.SeverityLow},
			{Severity: models.SeverityLow},
		},
		Audit: &models.AuditResult{Passed: false, ExitCode: 1, Reasons: []string{"1 high finding detected."}, Warnings: []string{"Review deployment docs."}},
		AI:    &models.AISummary{Model: "llama3.2:3b", Status: "generated_structured", LocalNotes: "Local-only notes."},
	}

	response := BuildAnalyzeResponse(root, analysis, AnalyzeRequest{RunAudit: true, UseAI: true})

	if response.RepoName != "example" || response.Files != 2 || response.Routes != 1 || response.Tests != 1 {
		t.Fatalf("unexpected summary: %#v", response)
	}
	if response.Findings["high"] != 1 || response.Findings["low"] != 2 || response.Findings["medium"] != 0 {
		t.Fatalf("unexpected finding counts: %#v", response.Findings)
	}
	if response.AuditStatus != "failed" || response.AuditExitCode != 1 {
		t.Fatalf("unexpected audit status: %#v", response)
	}
	if response.AIStatus != "generated" || response.AIModel != "llama3.2:3b" {
		t.Fatalf("unexpected AI status: %#v", response)
	}
	if response.JSONReportPath != filepath.Join(root, ".stackmap", "analysis.json") {
		t.Fatalf("unexpected JSON path: %s", response.JSONReportPath)
	}
	if response.Context.Purpose != "Example service" || len(response.Context.Evidence) != 1 {
		t.Fatalf("unexpected context view: %#v", response.Context)
	}
	if len(response.APIRoutes) != 1 || response.APIRoutes[0].Path != "/health" {
		t.Fatalf("unexpected route view: %#v", response.APIRoutes)
	}
	if response.TestSummary.TestScript != "go test ./..." || len(response.TestSummary.TestFiles) != 1 {
		t.Fatalf("unexpected test summary: %#v", response.TestSummary)
	}
	if response.Audit.Status != "failed" || len(response.Audit.Blockers) != 1 || len(response.Audit.Warnings) != 1 {
		t.Fatalf("unexpected audit view: %#v", response.Audit)
	}
	if response.AI.LocalNotes != "Local-only notes." || response.AI.DeterministicSummary == "" {
		t.Fatalf("unexpected AI view: %#v", response.AI)
	}
	if response.Reports.MarkdownPath != filepath.Join(root, ".stackmap", "reports", "repo-report.md") {
		t.Fatalf("unexpected reports view: %#v", response.Reports)
	}
}

func TestBuildAnalyzeResponseDefaultStatuses(t *testing.T) {
	analysis := &models.Analysis{RepoName: "example", RepoPath: "example"}

	response := BuildAnalyzeResponse("example", analysis, AnalyzeRequest{})

	if response.AuditStatus != "not run" {
		t.Fatalf("expected audit not run, got %q", response.AuditStatus)
	}
	if response.AIStatus != "not requested" {
		t.Fatalf("expected AI not requested, got %q", response.AIStatus)
	}
}
