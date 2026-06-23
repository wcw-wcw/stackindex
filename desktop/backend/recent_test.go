package backend

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/wcw-wcw/stackindex/internal/models"
)

func TestRecentProjectsUpsertDedupeOrderAndCap(t *testing.T) {
	session := testSession(t)
	base := t.TempDir()

	for i := 0; i < maxRecentProjects+2; i++ {
		root := filepath.Join(base, "repo-"+strconv.Itoa(i))
		response := BuildAnalyzeResponse(root, &models.Analysis{
			RepoName:    filepath.Base(root),
			RepoPath:    root,
			GeneratedAt: time.Date(2026, 1, 1, i%24, 0, 0, 0, time.UTC),
			Files:       []models.FileInfo{{Path: "main.go"}},
		}, AnalyzeRequest{})
		if err := session.upsertRecentProject(response); err != nil {
			t.Fatal(err)
		}
	}

	first := filepath.Join(base, "repo-0")
	response := BuildAnalyzeResponse(first, &models.Analysis{
		RepoName:    "repo-0",
		RepoPath:    first,
		GeneratedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
		Routes:      []models.RouteInfo{{Method: "GET", Path: "/health"}},
	}, AnalyzeRequest{})
	if err := session.upsertRecentProject(response); err != nil {
		t.Fatal(err)
	}

	projects, err := session.GetRecentProjects()
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != maxRecentProjects {
		t.Fatalf("expected cap %d, got %d", maxRecentProjects, len(projects))
	}
	if projects[0].RepoPath != first || projects[0].Routes != 1 {
		t.Fatalf("expected de-duped project first: %#v", projects[0])
	}
	seen := map[string]bool{}
	for _, project := range projects {
		if seen[project.RepoPath] {
			t.Fatalf("duplicate recent project: %s", project.RepoPath)
		}
		seen[project.RepoPath] = true
	}
}

func TestRecentProjectsMalformedFileHandling(t *testing.T) {
	session := testSession(t)
	if err := os.MkdirAll(filepath.Dir(session.recentProjectsPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(session.recentProjectsPath, []byte("{not json"), 0644); err != nil {
		t.Fatal(err)
	}

	projects, err := session.GetRecentProjects()
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 0 {
		t.Fatalf("expected malformed file to return empty list, got %#v", projects)
	}
}

func TestRecentProjectsReadsLegacyEntries(t *testing.T) {
	session := testSession(t)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Dir(session.recentProjectsPath), 0755); err != nil {
		t.Fatal(err)
	}
	data := []byte(`[
  {
    "repoName": "legacy",
    "repoPath": "` + filepath.ToSlash(root) + `",
    "lastAnalyzed": "2026-06-15 12:00:00",
    "files": 3,
    "routes": 1,
    "tests": 2,
    "findings": {"high": 1},
    "jsonReportPath": "` + filepath.ToSlash(filepath.Join(root, ".stackindex", "analysis.json")) + `",
    "mdReportPath": "` + filepath.ToSlash(filepath.Join(root, ".stackindex", "reports", "repo-index.md")) + `"
  }
]`)
	if err := os.WriteFile(session.recentProjectsPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	projects, err := session.GetRecentProjects()
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected one legacy project, got %#v", projects)
	}
	if projects[0].RepoName != "legacy" || projects[0].SourceType != sourceTypeLocal || projects[0].Findings["medium"] != 0 {
		t.Fatalf("legacy project was not read compatibly: %#v", projects[0])
	}
}

func TestRemoveRecentProject(t *testing.T) {
	session := testSession(t)
	base := t.TempDir()
	keep := filepath.Join(base, "keep")
	remove := filepath.Join(base, "remove")
	for _, root := range []string{keep, remove} {
		response := BuildAnalyzeResponse(root, &models.Analysis{RepoName: filepath.Base(root), RepoPath: root}, AnalyzeRequest{})
		if err := session.upsertRecentProject(response); err != nil {
			t.Fatal(err)
		}
	}

	if err := session.RemoveRecentProject(remove); err != nil {
		t.Fatal(err)
	}
	projects, err := session.GetRecentProjects()
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 || projects[0].RepoPath != keep {
		t.Fatalf("unexpected recent projects after remove: %#v", projects)
	}
}

func TestOpenExistingReportLoadsAnalysisAndAskWorks(t *testing.T) {
	session := testSession(t)
	root := t.TempDir()
	analysis := &models.Analysis{
		RepoName:    "example",
		RepoPath:    root,
		GeneratedAt: time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC),
		Files:       []models.FileInfo{{Path: "README.md"}},
		Context: models.ProjectContext{
			Purpose:    "Example service",
			Confidence: "high",
			Evidence:   []string{"README describes an example service."},
		},
		Audit: &models.AuditResult{Passed: true},
	}
	writeAnalysisJSON(t, root, analysis)

	response, err := session.OpenExistingReport(root)
	if err != nil {
		t.Fatal(err)
	}
	if !response.LoadedFromDisk || response.RepoName != "example" || response.AuditStatus != "passed" {
		t.Fatalf("unexpected loaded response: %#v", response)
	}
	if response.JSONReportPath != filepath.Join(root, ".stackindex", "analysis.json") {
		t.Fatalf("unexpected report path: %s", response.JSONReportPath)
	}

	answer, err := session.AskQuestion(context.Background(), AskRequest{Question: "What is this project for?"})
	if err != nil {
		t.Fatal(err)
	}
	if answer.Answer == "" || answer.Mode != "deterministic" {
		t.Fatalf("unexpected ask response: %#v", answer)
	}
}

func TestOpenExistingReportPreservesRecentGitHubMetadata(t *testing.T) {
	session := testSession(t)
	root := t.TempDir()
	writeAnalysisJSON(t, root, &models.Analysis{RepoName: "repo", RepoPath: root})
	if err := writeRecentProjects(session.recentProjectsPath, []RecentProject{{
		RepoName:       "repo",
		RepoPath:       root,
		SourceType:     sourceTypeGitHub,
		GitHubURL:      "https://github.com/owner/repo.git",
		LocalCachePath: root,
	}}); err != nil {
		t.Fatal(err)
	}

	response, err := session.OpenExistingReport(root)
	if err != nil {
		t.Fatal(err)
	}
	if response.SourceType != sourceTypeGitHub || response.GitHubURL != "https://github.com/owner/repo.git" || response.LocalCachePath != root {
		t.Fatalf("GitHub metadata was not preserved: %#v", response)
	}
}

func TestOpenExistingReportMissingReportReturnsCleanError(t *testing.T) {
	session := testSession(t)
	root := t.TempDir()

	_, err := session.OpenExistingReport(root)
	if err == nil {
		t.Fatal("expected missing report error")
	}
	if want := "no previous StackIndex report found"; !strings.HasPrefix(err.Error(), want) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func testSession(t *testing.T) *Session {
	t.Helper()
	return &Session{recentProjectsPath: filepath.Join(t.TempDir(), "StackIndex", "recent-projects.json")}
}

func writeAnalysisJSON(t *testing.T, root string, analysis *models.Analysis) {
	t.Helper()
	outDir := filepath.Join(root, ".stackindex")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "analysis.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
}
