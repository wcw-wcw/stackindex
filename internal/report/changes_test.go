package report

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/will/stackmap/internal/models"
)

func TestBuildChangeSummaryNoPreviousSnapshot(t *testing.T) {
	root := t.TempDir()
	current := &models.Analysis{RepoName: "demo", RepoPath: root, GeneratedAt: time.Now()}

	summary, err := BuildChangeSummary(root, current)
	if err != nil {
		t.Fatal(err)
	}
	if summary.HasPrevious {
		t.Fatalf("expected no previous snapshot: %#v", summary)
	}
	if summary.Message == "" {
		t.Fatalf("expected friendly no previous message: %#v", summary)
	}
}

func TestBuildChangeSummaryRoutesAddedAndRemoved(t *testing.T) {
	root := t.TempDir()
	writeSnapshotFixture(t, root, "20260616-120000", &models.Analysis{
		RepoName: "demo",
		RepoPath: root,
		Routes: []models.RouteInfo{
			{Method: "GET", Path: "/api/old"},
			{Method: "GET", Path: "/api/stable"},
		},
	})
	current := &models.Analysis{
		RepoName: "demo",
		RepoPath: root,
		Routes: []models.RouteInfo{
			{Method: "GET", Path: "/api/new"},
			{Method: "GET", Path: "/api/stable"},
		},
	}

	summary := mustChangeSummary(t, root, current)
	assertStrings(t, summary.AddedRoutes, []string{"GET /api/new"})
	assertStrings(t, summary.RemovedRoutes, []string{"GET /api/old"})
}

func TestBuildChangeSummaryEnvVarsAddedAndRemoved(t *testing.T) {
	root := t.TempDir()
	writeSnapshotFixture(t, root, "20260616-120000", &models.Analysis{
		RepoName: "demo",
		RepoPath: root,
		Env:      models.EnvAnalysis{UsedVars: []models.EnvVar{{Name: "DATABASE_URL"}, {Name: "OLD_SECRET"}}},
	})
	current := &models.Analysis{
		RepoName: "demo",
		RepoPath: root,
		Env:      models.EnvAnalysis{UsedVars: []models.EnvVar{{Name: "DATABASE_URL"}, {Name: "NEW_SECRET"}}},
	}

	summary := mustChangeSummary(t, root, current)
	assertStrings(t, summary.AddedEnvVars, []string{"NEW_SECRET"})
	assertStrings(t, summary.RemovedEnvVars, []string{"OLD_SECRET"})
}

func TestBuildChangeSummaryAuditStatusChanged(t *testing.T) {
	root := t.TempDir()
	writeSnapshotFixture(t, root, "20260616-120000", &models.Analysis{
		RepoName: "demo",
		RepoPath: root,
		Audit:    &models.AuditResult{Passed: false},
	})
	current := &models.Analysis{
		RepoName: "demo",
		RepoPath: root,
		Audit:    &models.AuditResult{Passed: true},
	}

	summary := mustChangeSummary(t, root, current)
	if summary.AuditStatusBefore != "failed" || summary.AuditStatusAfter != "passed" {
		t.Fatalf("audit status = %q -> %q, want failed -> passed", summary.AuditStatusBefore, summary.AuditStatusAfter)
	}
	assertContainsString(t, summary.SummaryBullets, "Audit status changed from failed to passed.")
}

func TestBuildChangeSummaryFindingsAddedAndResolved(t *testing.T) {
	root := t.TempDir()
	resolved := models.Finding{Severity: models.SeverityHigh, Category: "route", Message: "Old problem.", File: "old.go"}
	stable := models.Finding{Severity: models.SeverityLow, Category: "docs", Message: "Stable.", File: "README.md"}
	added := models.Finding{Severity: models.SeverityMedium, Category: "env", Message: "New problem.", File: ".env.example"}
	writeSnapshotFixture(t, root, "20260616-120000", &models.Analysis{
		RepoName: "demo",
		RepoPath: root,
		Findings: []models.Finding{resolved, stable},
	})
	current := &models.Analysis{
		RepoName: "demo",
		RepoPath: root,
		Findings: []models.Finding{stable, added},
	}

	summary := mustChangeSummary(t, root, current)
	assertContainsString(t, summary.AddedFindings, "medium | env | New problem. | .env.example")
	assertContainsString(t, summary.ResolvedFindings, "high | route | Old problem. | old.go")
}

func TestBuildChangeSummaryStackFrameworkSignalsChanged(t *testing.T) {
	root := t.TempDir()
	writeSnapshotFixture(t, root, "20260616-120000", &models.Analysis{
		RepoName: "demo",
		RepoPath: root,
		Stack: models.StackInfo{
			Languages:  []string{"JavaScript"},
			Frameworks: []string{"React"},
			Databases:  []string{"SQLite"},
			Testing:    []string{"Jest"},
			Deployment: []string{"Docker"},
		},
		Tests:      models.TestAnalysis{HasTestFiles: true},
		Deployment: models.DeploymentAnalysis{HasDockerfile: true},
		Structure:  models.StructureMap{KeyFiles: []models.FileRole{{Path: "old.go"}}},
	})
	current := &models.Analysis{
		RepoName: "demo",
		RepoPath: root,
		Stack: models.StackInfo{
			Languages:  []string{"TypeScript"},
			Frameworks: []string{"Next.js"},
			Databases:  []string{"PostgreSQL"},
			Testing:    []string{"Vitest"},
			Deployment: []string{"Vercel"},
		},
		Tests:      models.TestAnalysis{HasTestScript: true, Frameworks: []string{"Vitest"}, TestScript: "vitest run"},
		Deployment: models.DeploymentAnalysis{HasVercelConfig: true, HasHealthEndpoint: true},
		Structure:  models.StructureMap{KeyFiles: []models.FileRole{{Path: "new.ts"}}},
	}

	summary := mustChangeSummary(t, root, current)
	assertContainsString(t, summary.StackChanges, "added stack: TypeScript")
	assertContainsString(t, summary.FrameworkChanges, "added framework: Next.js")
	assertContainsString(t, summary.DatabaseChanges, "added database: PostgreSQL")
	assertContainsString(t, summary.TestSignalChanges, "added test: script: vitest run")
	assertContainsString(t, summary.DeploymentSignalChanges, "added deployment: health endpoint")
	assertContainsString(t, summary.KeyFileChanges, "added key file: new.ts")
}

func TestBuildChangeSummarySkipsUnrelatedRepoSnapshots(t *testing.T) {
	root := t.TempDir()
	writeSnapshotFixture(t, root, "20260616-120000", &models.Analysis{
		RepoName: "other",
		RepoPath: filepath.Join(root, "other"),
		Routes:   []models.RouteInfo{{Method: "GET", Path: "/api/old"}},
	})
	current := &models.Analysis{RepoName: "demo", RepoPath: root, Routes: []models.RouteInfo{{Method: "GET", Path: "/api/new"}}}

	summary := mustChangeSummary(t, root, current)
	if summary.HasPrevious {
		t.Fatalf("unrelated snapshot was compared: %#v", summary)
	}
}

func mustChangeSummary(t *testing.T, root string, current *models.Analysis) *models.ChangeSummary {
	t.Helper()
	summary, err := BuildChangeSummary(root, current)
	if err != nil {
		t.Fatal(err)
	}
	return summary
}

func assertContainsString(t *testing.T, values []string, want string) {
	t.Helper()
	for _, value := range values {
		if value == want {
			return
		}
	}
	t.Fatalf("values %#v did not contain %q", values, want)
}

func assertStrings(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("strings = %#v, want %#v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("strings = %#v, want %#v", got, want)
		}
	}
}
