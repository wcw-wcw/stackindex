package backend

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

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

func TestBuildAnalyzeResponseIncludesChangeSummary(t *testing.T) {
	analysis := &models.Analysis{
		RepoName: "example",
		RepoPath: "example",
		Changes: &models.ChangeSummary{
			HasPrevious:       true,
			PreviousSnapshot:  "20260616-120000",
			SummaryBullets:    []string{"Routes changed: 1 added, 0 removed."},
			AddedRoutes:       []string{"GET /api/new"},
			AddedEnvVars:      []string{"NEW_SECRET"},
			AddedFindings:     []string{"medium | env | Missing NEW_SECRET | .env.example"},
			AuditStatusBefore: "failed",
			AuditStatusAfter:  "passed",
		},
	}

	response := BuildAnalyzeResponse("example", analysis, AnalyzeRequest{})
	changes := response.Reports.Changes
	if !changes.HasPrevious || changes.PreviousSnapshot != "20260616-120000" {
		t.Fatalf("unexpected change metadata: %#v", changes)
	}
	if len(changes.AddedRoutes) != 1 || changes.AddedRoutes[0] != "GET /api/new" {
		t.Fatalf("unexpected added routes: %#v", changes)
	}
	if changes.AuditStatusBefore != "failed" || changes.AuditStatusAfter != "passed" {
		t.Fatalf("unexpected audit transition: %#v", changes)
	}
}

func TestAskQuestionRequiresLoadedAnalysis(t *testing.T) {
	session := NewSession()

	_, err := session.AskQuestion(context.Background(), AskRequest{Question: "What is this project for?"})
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "Analyze a project before asking questions." {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAskQuestionAnswersAndWritesArtifacts(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Example service\n\nA tiny API service.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	session := NewSession()
	if _, err := session.AnalyzeProject(context.Background(), AnalyzeRequest{Path: root}); err != nil {
		t.Fatal(err)
	}

	response, err := session.AskQuestion(context.Background(), AskRequest{Question: "What is this project for?"})
	if err != nil {
		t.Fatal(err)
	}

	if response.Question != "What is this project for?" || response.Answer == "" {
		t.Fatalf("unexpected ask response: %#v", response)
	}
	if response.Mode != "deterministic" {
		t.Fatalf("expected deterministic mode, got %q", response.Mode)
	}
	if response.Confidence == "" {
		t.Fatalf("expected confidence: %#v", response)
	}
	if len(response.Evidence) == 0 {
		t.Fatalf("expected evidence: %#v", response)
	}
	for _, path := range []string{
		filepath.Join(root, ".stackmap", "qa", "latest-question.json"),
		filepath.Join(root, ".stackmap", "qa", "history.jsonl"),
	} {
		if info, err := os.Stat(path); err != nil || info.Size() == 0 {
			t.Fatalf("expected written qa artifact at %s: info=%#v err=%v", path, info, err)
		}
	}
}

func TestOpenExistingReportDoesNotCreateSnapshot(t *testing.T) {
	root := t.TempDir()
	analysisDir := filepath.Join(root, ".stackmap")
	if err := os.MkdirAll(analysisDir, 0755); err != nil {
		t.Fatal(err)
	}
	data := []byte(`{"repoName":"example","generatedAt":"2026-06-16T12:00:00Z"}`)
	if err := os.WriteFile(filepath.Join(analysisDir, "analysis.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	session := NewSession()
	if _, err := session.OpenExistingReport(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".stackmap", "history")); !os.IsNotExist(err) {
		t.Fatalf("OpenExistingReport created history directory, err=%v", err)
	}
}

func TestBuildAskResponseCopiesEvidence(t *testing.T) {
	response := BuildAskResponse(&models.QAResult{
		Question:   "Where are the API routes?",
		Answer:     "Review handlers.",
		Confidence: "high",
		Mode:       "deterministic",
		Evidence: []models.QAEvidence{{
			Kind:  "route",
			Label: "GET /health",
			Value: "high",
			Path:  "main.go",
		}},
		Warnings: []string{"example warning"},
	})

	if len(response.Evidence) != 1 || response.Evidence[0].Path != "main.go" {
		t.Fatalf("unexpected evidence view: %#v", response.Evidence)
	}
	if len(response.Warnings) != 1 {
		t.Fatalf("unexpected warnings: %#v", response.Warnings)
	}
}

func TestBuildAnalyzeResponseHandlesAskFixture(t *testing.T) {
	root := t.TempDir()
	analysis := &models.Analysis{
		RepoName:    "example",
		RepoPath:    root,
		GeneratedAt: time.Now(),
		Context: models.ProjectContext{
			Purpose:    "Example service",
			Confidence: "high",
		},
	}

	response := BuildAnalyzeResponse(root, analysis, AnalyzeRequest{})
	if response.RepoPath != root {
		t.Fatalf("unexpected repo path: %s", response.RepoPath)
	}
}
