package ai

import (
	"strings"
	"testing"

	"github.com/will/stackmap/internal/models"
)

func TestBuildCompactInputIsAISafeAndCapped(t *testing.T) {
	var routes []models.RouteInfo
	for i := 0; i < routeLimit+5; i++ {
		routes = append(routes, models.RouteInfo{
			Method:     "GET",
			Path:       "/api/item",
			SourceFile: "app/routes/item.ts",
			Confidence: "high",
		})
	}
	analysis := &models.Analysis{
		RepoName: "demo",
		Files: []models.FileInfo{
			{Path: "package.json", Kind: models.FileKindConfig, Hash: "abc123", SizeBytes: 100},
			{Path: ".env", Kind: models.FileKindConfig, Hash: "secret-hash", SizeBytes: 50},
			{Path: "src/main.ts", Kind: models.FileKindSource, Hash: "def456", SizeBytes: 200},
			{Path: "src/main.test.ts", Kind: models.FileKindTest, SizeBytes: 80},
		},
		PackageInfo: &models.PackageInfo{
			Scripts: map[string]string{"build": "vite build", "test": "vitest"},
			Dependencies: map[string]string{
				"huge": "not included",
			},
		},
		Routes: routes,
		Tests: models.TestAnalysis{
			HasTestFiles:  true,
			HasTestScript: true,
			TestFiles:     []string{"src/main.test.ts"},
			TestScript:    "vitest",
		},
		Env: models.EnvAnalysis{
			UsesEnvVars:                true,
			ExampleFile:                ".env.example",
			ExampleVars:                []string{"API_URL"},
			MissingFromExample:         []string{"DATABASE_URL"},
			MissingRequiredFromExample: []string{"DATABASE_URL"},
			UsedVars: []models.EnvVar{
				{Name: "DATABASE_URL", Classification: "required_app_config", MissingExample: true},
				{Name: "VERCEL", Classification: "platform_provided"},
			},
			EnvFilePresent: true,
		},
		Findings: []models.Finding{
			{Severity: models.SeverityMedium, Category: "env", Message: "missing env example"},
		},
	}

	input := BuildCompactInput(analysis)
	if input.RepoName != "demo" {
		t.Fatalf("RepoName = %q, want demo", input.RepoName)
	}
	if input.FileCounts[models.FileKindConfig] != 2 || input.FileCounts[models.FileKindSource] != 1 || input.FileCounts[models.FileKindTest] != 1 {
		t.Fatalf("unexpected file counts: %#v", input.FileCounts)
	}
	if len(input.Routes) != routeLimit {
		t.Fatalf("routes length = %d, want %d", len(input.Routes), routeLimit)
	}
	if input.RoutesTotal != routeLimit+5 {
		t.Fatalf("RoutesTotal = %d, want %d", input.RoutesTotal, routeLimit+5)
	}
	if input.PackageScripts["build"] != "vite build" {
		t.Fatalf("package scripts were not included")
	}
	if input.Env.Classifications["required_app_config"] != 1 || input.Env.Classifications["platform_provided"] != 1 {
		t.Fatalf("unexpected env classifications: %#v", input.Env.Classifications)
	}
	for _, path := range input.TopImportantFiles {
		if strings.Contains(path, ".env") {
			t.Fatalf("important files included env file path: %q", path)
		}
	}
}

func TestParseModelResponseValidJSON(t *testing.T) {
	text := `{
		"projectSummary": "Local-first analyzer.",
		"architectureOverview": "CLI runs analyzers then writes reports.",
		"keyStrengths": ["Deterministic analysis"],
		"potentialRisks": ["Limited language coverage"],
		"recommendedNextSteps": ["Add smoke tests"]
	}`

	summary, err := ParseModelResponse(text)
	if err != nil {
		t.Fatalf("ParseModelResponse returned error: %v", err)
	}
	if summary.ProjectSummary != "Local-first analyzer." {
		t.Fatalf("ProjectSummary = %q", summary.ProjectSummary)
	}
	if got := summary.RecommendedNextSteps[0]; got != "Add smoke tests" {
		t.Fatalf("RecommendedNextSteps[0] = %q", got)
	}
}

func TestApplyModelResponseFallsBackOnInvalidJSON(t *testing.T) {
	summary := &models.AISummary{Enabled: true, Model: "qwen:7b"}
	applyModelResponse(summary, "Helpful prose, but not JSON.")

	if summary.RawText != "Helpful prose, but not JSON." {
		t.Fatalf("RawText = %q", summary.RawText)
	}
	if summary.ProjectSummary != "" || len(summary.RecommendedNextSteps) != 0 {
		t.Fatalf("invalid JSON should not populate structured fields: %#v", summary)
	}
}
