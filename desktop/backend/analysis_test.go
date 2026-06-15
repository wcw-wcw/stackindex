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
		Tests:  models.TestAnalysis{TestFiles: []string{"main_test.go"}},
		Findings: []models.Finding{
			{Severity: models.SeverityHigh},
			{Severity: models.SeverityLow},
			{Severity: models.SeverityLow},
		},
		Audit: &models.AuditResult{Passed: false, ExitCode: 1},
		AI:    &models.AISummary{Model: "llama3.2:3b", Status: "generated_structured"},
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
