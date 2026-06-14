package qa

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/will/stackmap/internal/models"
)

func TestProjectPurposeQuestion(t *testing.T) {
	result := AnswerDeterministically(fixtureAnalysis(), "What is this project for?")
	assertAnswerContains(t, result, "Stock monitoring")
	assertMode(t, result, ModeDeterministic)
	assertEvidenceKind(t, result, "context")
}

func TestStackQuestion(t *testing.T) {
	result := AnswerDeterministically(fixtureAnalysis(), "What stack does this project use?")
	assertAnswerContains(t, result, "Next.js")
	assertAnswerContains(t, result, "PostgreSQL")
	assertEvidenceKind(t, result, "context")
}

func TestAPIRoutesQuestion(t *testing.T) {
	result := AnswerDeterministically(fixtureAnalysis(), "Where are the API routes?")
	assertAnswerContains(t, result, "3 detected API routes")
	assertEvidenceKind(t, result, "route")
}

func TestImportantFilesQuestion(t *testing.T) {
	result := AnswerDeterministically(fixtureAnalysis(), "What are the most important files?")
	assertAnswerContains(t, result, "Good starting files")
	assertEvidenceKind(t, result, "file")
}

func TestDependencyGraphQuestion(t *testing.T) {
	result := AnswerDeterministically(fixtureAnalysis(), "What are the most connected files?")
	assertAnswerContains(t, result, "most connected files")
	assertEvidenceKind(t, result, "graph")
}

func TestDeploymentReadinessQuestion(t *testing.T) {
	result := AnswerDeterministically(fixtureAnalysis(), "What should I review before deployment?")
	assertAnswerContains(t, result, "Deployment-readiness")
	assertEvidenceKind(t, result, "audit")
}

func TestTestsQuestion(t *testing.T) {
	result := AnswerDeterministically(fixtureAnalysis(), "Does this project have tests?")
	assertAnswerContains(t, result, "Tests were detected")
	assertAnswerContains(t, result, "Vitest")
	assertEvidenceKind(t, result, "script")
}

func TestEnvironmentQuestion(t *testing.T) {
	result := AnswerDeterministically(fixtureAnalysis(), "What env vars are configured?")
	assertAnswerContains(t, result, "Environment variables are used")
	assertEvidenceKind(t, result, "env")
}

func TestUnsupportedQuestionFallback(t *testing.T) {
	result := AnswerDeterministically(fixtureAnalysis(), "Which cloud region is best?")
	assertAnswerContains(t, result, "StackMap ask can answer")
	if result.Confidence != ConfidenceLow {
		t.Fatalf("confidence = %q, want low", result.Confidence)
	}
	if len(result.Warnings) == 0 {
		t.Fatal("unsupported result should include a warning")
	}
}

func TestJSONOutputShape(t *testing.T) {
	result := AnswerDeterministically(fixtureAnalysis(), "Where are the API routes?")
	data, err := MarshalJSON(result)
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v", err)
	}
	var decoded models.QAResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal QAResult: %v", err)
	}
	if decoded.Question == "" || decoded.Answer == "" || len(decoded.Evidence) == 0 {
		t.Fatalf("decoded QAResult missing expected fields: %+v", decoded)
	}
}

func TestAIFallbackWhenLocalAIFails(t *testing.T) {
	result := Ask(context.Background(), fixtureAnalysis(), "What is this project for?", Options{
		UseAI: true,
		Model: "missing:model",
		Generate: func(context.Context, string, string) (string, error) {
			return "", errors.New("model unavailable")
		},
	})
	assertMode(t, result, ModeDeterministic)
	if len(result.AttemptedModels) != 1 || result.AttemptedModels[0] != "missing:model" {
		t.Fatalf("attempted models = %v, want missing:model", result.AttemptedModels)
	}
	if len(result.Warnings) == 0 {
		t.Fatal("AI fallback should include a warning")
	}
}

func TestAIPromptFactsheetBoundedness(t *testing.T) {
	deterministic := AnswerDeterministically(fixtureAnalysis(), "What env vars are configured?")
	factsheet := BuildFactsheet(deterministic)
	prompt := PromptFor(deterministic, factsheet)
	if !strings.Contains(prompt, "using only the deterministic StackMap factsheet") {
		t.Fatalf("prompt missing boundedness instruction:\n%s", prompt)
	}
	if strings.Contains(prompt, "DATABASE_URL=super-secret") {
		t.Fatalf("prompt leaked raw env value:\n%s", prompt)
	}
	if !strings.Contains(factsheet, "Missing required count") {
		t.Fatalf("factsheet missing bounded env evidence:\n%s", factsheet)
	}
}

func fixtureAnalysis() *models.Analysis {
	return &models.Analysis{
		RepoPath: "/tmp/stkapp",
		RepoName: "stkapp",
		Stack: models.StackInfo{
			Languages:  []string{"TypeScript", "JavaScript"},
			Frameworks: []string{"Next.js", "React", "Node.js"},
			Databases:  []string{"PostgreSQL"},
			Testing:    []string{"Vitest"},
			Deployment: []string{"Vercel"},
		},
		PackageInfo: &models.PackageInfo{
			Name:        "stkapp",
			Description: "Stock monitoring and alerting app",
			Scripts: map[string]string{
				"test": "vitest run",
				"dev":  "next dev",
			},
		},
		Context: models.ProjectContext{
			Purpose:       "Stock monitoring and alerting application",
			Confidence:    "high",
			ReadmeTitle:   "stkapp",
			ReadmeSummary: "Tracks watchlists, market data, and stock alerts.",
			Evidence:      []string{"README mentions stock alerts", "package metadata mentions monitoring"},
		},
		Structure: models.StructureMap{
			Directories: []models.DirectoryRole{
				{Path: "src/app", Role: "Next.js app routes and pages", FileCount: 20},
				{Path: "src/lib", Role: "Shared application services", FileCount: 12},
			},
			KeyFiles: []models.FileRole{
				{Path: "src/app/page.tsx", Role: "Frontend entry page", Importance: "high"},
				{Path: "src/lib/db.ts", Role: "Database helper", Importance: "high"},
			},
		},
		Dependencies: models.DependencyGraph{
			Entrypoints: []string{"src/app/page.tsx", "src/app/api/health/route.ts"},
			TopConnectedFiles: []models.ConnectedFileSummary{
				{Path: "src/lib/db.ts", Role: "Database helper", ImportsCount: 1, ImportedByCount: 8, WhyItMatters: "Shared database access"},
			},
			ArchitectureHints: []string{"API route handlers import shared services from src/lib."},
			Nodes:             []models.DependencyNode{{Path: "src/lib/db.ts"}},
			Edges:             []models.DependencyEdge{{From: "src/app/api/rules/route.ts", To: "src/lib/db.ts"}},
		},
		Routes: []models.RouteInfo{
			{Method: "GET", Path: "/api/health", SourceFile: "src/app/api/health/route.ts", Confidence: "high"},
			{Method: "POST", Path: "/api/auth/login", SourceFile: "src/app/api/auth/login/route.ts", Confidence: "high"},
			{Method: "GET", Path: "/api/rules", SourceFile: "src/app/api/rules/route.ts", Confidence: "high"},
		},
		Env: models.EnvAnalysis{
			UsesEnvVars:                true,
			ExampleFile:                ".env.example",
			ExampleVars:                []string{"DATABASE_URL"},
			UsedVars:                   []models.EnvVar{{Name: "DATABASE_URL", Files: []string{"src/lib/db.ts"}, Classification: "required_app_config"}},
			MissingRequiredFromExample: []string{"ALPACA_API_KEY"},
			EnvFilePresent:             true,
		},
		Tests: models.TestAnalysis{
			HasTestFiles:  true,
			HasTestScript: true,
			Frameworks:    []string{"Vitest"},
			TestFiles:     []string{"src/lib/db.test.ts"},
			TestScript:    "vitest run",
		},
		Deployment: models.DeploymentAnalysis{
			HasReadme:            true,
			HasEnvExample:        true,
			HasVercelConfig:      true,
			HasHealthEndpoint:    true,
			DeploymentFiles:      []string{"vercel.json"},
			HasMigrationFiles:    true,
			MigrationFiles:       []string{"database/migrations/001_init.sql"},
			ReadmeMentionsDeploy: true,
		},
	}
}

func assertAnswerContains(t *testing.T, result *models.QAResult, want string) {
	t.Helper()
	if !strings.Contains(result.Answer, want) {
		t.Fatalf("answer = %q, want to contain %q", result.Answer, want)
	}
}

func assertEvidenceKind(t *testing.T, result *models.QAResult, kind string) {
	t.Helper()
	for _, ev := range result.Evidence {
		if ev.Kind == kind {
			return
		}
	}
	t.Fatalf("evidence = %+v, want kind %q", result.Evidence, kind)
}

func assertMode(t *testing.T, result *models.QAResult, mode string) {
	t.Helper()
	if result.Mode != mode {
		t.Fatalf("mode = %q, want %q", result.Mode, mode)
	}
}
