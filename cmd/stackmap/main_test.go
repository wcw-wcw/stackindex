package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/will/stackmap/internal/models"
)

func TestAuditResultExistsInAnalysisOutputDuringAuditMode(t *testing.T) {
	root := healthyProject(t)

	err := analyze([]string{root, "--audit"}, false)
	if err != nil {
		t.Fatalf("analyze audit returned error: %v", err)
	}

	analysis := readAnalysisJSON(t, root)
	if analysis.Audit == nil {
		t.Fatal("analysis.json did not include audit result")
	}
	if !analysis.Audit.Passed || analysis.Audit.ExitCode != 0 {
		t.Fatalf("audit result = %+v, want passed with exit code 0", analysis.Audit)
	}
}

func TestAuditResultAbsentOutsideAuditMode(t *testing.T) {
	root := healthyProject(t)

	err := analyze([]string{root, "--no-tui"}, false)
	if err != nil {
		t.Fatalf("analyze returned error: %v", err)
	}

	analysis := readAnalysisJSON(t, root)
	if analysis.Audit != nil {
		t.Fatalf("analysis.json included audit result outside audit mode: %+v", analysis.Audit)
	}
}

func TestHealthyProjectPassesAudit(t *testing.T) {
	result := EvaluateAudit(healthyAnalysis(), AuditOptions{})
	if !result.Passed || result.ExitCode != 0 || len(result.Reasons) != 0 {
		t.Fatalf("EvaluateAudit() = %+v, want pass", result)
	}
}

func TestHighFindingFailsAudit(t *testing.T) {
	analysis := healthyAnalysis()
	analysis.Findings = []models.Finding{{Severity: models.SeverityHigh, Category: "env", Message: "Secret-looking value found."}}

	result := EvaluateAudit(analysis, AuditOptions{})
	assertAuditFailsWith(t, result, "1 high finding detected.")
}

func TestMediumFindingFailsByDefault(t *testing.T) {
	analysis := healthyAnalysis()
	analysis.Findings = []models.Finding{{Severity: models.SeverityMedium, Category: "env", Message: "Missing env example."}}

	result := EvaluateAudit(analysis, AuditOptions{})
	assertAuditFailsWith(t, result, "1 medium finding detected.")
}

func TestMediumFindingPassesWithAllowMedium(t *testing.T) {
	analysis := healthyAnalysis()
	analysis.Findings = []models.Finding{{Severity: models.SeverityMedium, Category: "env", Message: "Missing env example."}}

	result := EvaluateAudit(analysis, AuditOptions{AllowMedium: true})
	if !result.Passed {
		t.Fatalf("EvaluateAudit() = %+v, want pass", result)
	}
	if !contains(result.Warnings, "1 medium finding detected.") {
		t.Fatalf("warnings = %v, want medium warning", result.Warnings)
	}
}

func TestLowFindingPassesByDefault(t *testing.T) {
	analysis := healthyAnalysis()
	analysis.Findings = []models.Finding{{Severity: models.SeverityLow, Category: "docs", Message: "README missing."}}

	result := EvaluateAudit(analysis, AuditOptions{})
	if !result.Passed {
		t.Fatalf("EvaluateAudit() = %+v, want pass", result)
	}
	if !contains(result.Warnings, "1 low finding detected.") {
		t.Fatalf("warnings = %v, want low warning", result.Warnings)
	}
}

func TestLowFindingFailsWithFailOnLow(t *testing.T) {
	analysis := healthyAnalysis()
	analysis.Findings = []models.Finding{{Severity: models.SeverityLow, Category: "docs", Message: "README missing."}}

	result := EvaluateAudit(analysis, AuditOptions{FailOnLow: true})
	assertAuditFailsWith(t, result, "1 low finding detected.")
}

func TestMissingTestsFailsByDefault(t *testing.T) {
	analysis := healthyAnalysis()
	analysis.Tests = models.TestAnalysis{}

	result := EvaluateAudit(analysis, AuditOptions{})
	assertAuditFailsWith(t, result, "Tests were not detected.")
}

func TestMissingTestsPassesWithAllowMissingTests(t *testing.T) {
	analysis := healthyAnalysis()
	analysis.Tests = models.TestAnalysis{}

	result := EvaluateAudit(analysis, AuditOptions{AllowMissingTests: true})
	if !result.Passed {
		t.Fatalf("EvaluateAudit() = %+v, want pass", result)
	}
	if !contains(result.Warnings, "Tests were not detected.") {
		t.Fatalf("warnings = %v, want missing tests warning", result.Warnings)
	}
}

func TestEnvVarsWithoutExampleFailAudit(t *testing.T) {
	analysis := healthyAnalysis()
	analysis.Env = models.EnvAnalysis{UsesEnvVars: true}

	result := EvaluateAudit(analysis, AuditOptions{})
	assertAuditFailsWith(t, result, "Environment variables were detected but no `.env.example` file was found.")
}

func TestNextAPIAppWithDeploymentAndNoHealthEndpointFailsAudit(t *testing.T) {
	analysis := healthyAnalysis()
	analysis.Stack = models.StackInfo{
		Languages:  []string{"TypeScript"},
		Frameworks: []string{"Next.js", "React"},
		Deployment: []string{"Vercel"},
	}
	analysis.Deployment.HasHealthEndpoint = false
	analysis.Routes = []models.RouteInfo{{Method: "GET", Path: "/api/users", SourceFile: "src/app/api/users/route.ts", Confidence: "high"}}

	result := EvaluateAudit(analysis, AuditOptions{})
	assertAuditFailsWith(t, result, "Backend/API deployment surface detected but no health endpoint was found.")
	if !result.HasBackendSurface || !result.RequiresHealthEndpoint {
		t.Fatalf("audit backend fields = hasBackendSurface:%v requiresHealthEndpoint:%v, want true/true", result.HasBackendSurface, result.RequiresHealthEndpoint)
	}
}

func TestNextAPIAppWithHealthEndpointPassesHealthRule(t *testing.T) {
	analysis := healthyAnalysis()
	analysis.Stack = models.StackInfo{
		Languages:  []string{"TypeScript"},
		Frameworks: []string{"Next.js", "React"},
		Deployment: []string{"Vercel"},
	}
	analysis.Deployment.HasHealthEndpoint = true
	analysis.Routes = []models.RouteInfo{{Method: "GET", Path: "/api/health", SourceFile: "src/app/api/health/route.ts", Confidence: "high"}}

	result := EvaluateAudit(analysis, AuditOptions{})
	if !result.Passed {
		t.Fatalf("EvaluateAudit() = %+v, want pass", result)
	}
	if !result.HasBackendSurface || !result.RequiresHealthEndpoint {
		t.Fatalf("audit backend fields = hasBackendSurface:%v requiresHealthEndpoint:%v, want true/true", result.HasBackendSurface, result.RequiresHealthEndpoint)
	}
}

func TestStaticViteDeploymentWithoutHealthEndpointWarns(t *testing.T) {
	analysis := staticViteAnalysis()

	result := EvaluateAudit(analysis, AuditOptions{})
	if !result.Passed {
		t.Fatalf("EvaluateAudit() = %+v, want pass with warning", result)
	}
	if result.HasBackendSurface || result.RequiresHealthEndpoint {
		t.Fatalf("audit backend fields = hasBackendSurface:%v requiresHealthEndpoint:%v, want false/false", result.HasBackendSurface, result.RequiresHealthEndpoint)
	}
	if !contains(result.Warnings, "Deployment target detected without a health endpoint; this may be acceptable for static frontend apps.") {
		t.Fatalf("warnings = %v, want static health warning", result.Warnings)
	}
}

func TestVercelServerlessAPIDeploymentWithoutHealthEndpointFailsAudit(t *testing.T) {
	analysis := staticViteAnalysis()
	analysis.Routes = []models.RouteInfo{{Method: "ANY", Path: "/api/anime/lookup", SourceFile: "api/anime/lookup.js", Confidence: "medium"}}
	analysis.Deployment.HasHealthEndpoint = false

	result := EvaluateAudit(analysis, AuditOptions{AllowMissingTests: true})
	assertAuditFailsWith(t, result, "Backend/API deployment surface detected but no health endpoint was found.")
	if !result.HasBackendSurface || !result.RequiresHealthEndpoint {
		t.Fatalf("audit backend fields = hasBackendSurface:%v requiresHealthEndpoint:%v, want true/true", result.HasBackendSurface, result.RequiresHealthEndpoint)
	}
	if contains(result.Warnings, "Deployment target detected without a health endpoint; this may be acceptable for static frontend apps.") {
		t.Fatalf("serverless API app should not get static-only health warning: %v", result.Warnings)
	}
}

func TestStaticAppStillFailsForMissingTestsUnlessAllowed(t *testing.T) {
	analysis := staticViteAnalysis()
	analysis.Tests = models.TestAnalysis{}

	result := EvaluateAudit(analysis, AuditOptions{})
	assertAuditFailsWith(t, result, "Tests were not detected.")
	if contains(result.Reasons, "Backend/API deployment surface detected but no health endpoint was found.") {
		t.Fatalf("static app should not fail on missing health endpoint: %v", result.Reasons)
	}

	allowed := EvaluateAudit(analysis, AuditOptions{AllowMissingTests: true})
	if !allowed.Passed {
		t.Fatalf("EvaluateAudit(allow missing tests) = %+v, want pass", allowed)
	}
	if !contains(allowed.Warnings, "Tests were not detected.") {
		t.Fatalf("warnings = %v, want missing tests warning", allowed.Warnings)
	}
}

func TestStkappLikeAuditStillPasses(t *testing.T) {
	analysis := healthyAnalysis()
	analysis.Stack = models.StackInfo{
		Languages:  []string{"TypeScript", "JavaScript"},
		Frameworks: []string{"Next.js", "React", "Node.js"},
		Testing:    []string{"Vitest"},
		Deployment: []string{"Vercel"},
	}
	analysis.Deployment = models.DeploymentAnalysis{HasHealthEndpoint: true, HasEnvExample: true}
	analysis.Env = models.EnvAnalysis{UsesEnvVars: true, ExampleFile: ".env.example"}
	analysis.Routes = []models.RouteInfo{{Method: "GET", Path: "/api/health", SourceFile: "src/app/api/health/route.ts", Confidence: "high"}}

	result := EvaluateAudit(analysis, AuditOptions{})
	if !result.Passed {
		t.Fatalf("EvaluateAudit() = %+v, want pass", result)
	}
}

func TestAnimerecLikeAuditFailsOnlyForMissingTestsByDefault(t *testing.T) {
	analysis := animerecLikeAnalysis()

	result := EvaluateAudit(analysis, AuditOptions{})
	assertAuditFailsWith(t, result, "Tests were not detected.")
	if contains(result.Reasons, "Backend/API deployment surface detected but no health endpoint was found.") {
		t.Fatalf("animerec-like static app should not fail on missing health endpoint: %v", result.Reasons)
	}
	if !contains(result.Warnings, "Deployment target detected without a health endpoint; this may be acceptable for static frontend apps.") {
		t.Fatalf("warnings = %v, want static health warning", result.Warnings)
	}
}

func TestAnimerecLikeAuditPassesWithAllowMissingTests(t *testing.T) {
	analysis := animerecLikeAnalysis()

	result := EvaluateAudit(analysis, AuditOptions{AllowMissingTests: true})
	if !result.Passed {
		t.Fatalf("EvaluateAudit() = %+v, want pass", result)
	}
	if !contains(result.Warnings, "Tests were not detected.") {
		t.Fatalf("warnings = %v, want missing tests warning", result.Warnings)
	}
}

func TestMissingTestsLowFindingDoesNotDuplicateAuditWarning(t *testing.T) {
	analysis := animerecLikeAnalysis()
	analysis.Findings = append(analysis.Findings, models.Finding{
		Severity: models.SeverityLow,
		Category: "tests",
		Message:  "No obvious test files were found.",
	})

	result := EvaluateAudit(analysis, AuditOptions{AllowMissingTests: true})
	if !result.Passed {
		t.Fatalf("EvaluateAudit() = %+v, want pass", result)
	}
	if !contains(result.Warnings, "1 low finding detected.") {
		t.Fatalf("warnings = %v, want non-test low finding warning", result.Warnings)
	}
	if contains(result.Warnings, "2 low findings detected.") {
		t.Fatalf("warnings duplicated missing test finding in low count: %v", result.Warnings)
	}
	if !contains(result.Warnings, "Tests were not detected.") {
		t.Fatalf("warnings = %v, want missing tests warning", result.Warnings)
	}
}

func TestAIFailureDoesNotFailAudit(t *testing.T) {
	analysis := healthyAnalysis()
	analysis.AI = &models.AISummary{
		Enabled:         true,
		Model:           "qwen:7b",
		AttemptedModels: []string{"llama3.2:3b", "qwen:7b"},
		Status:          "fallback_model_unavailable",
		Warning:         "local model unavailable",
	}

	result := EvaluateAudit(analysis, AuditOptions{})
	if !result.Passed {
		t.Fatalf("EvaluateAudit() = %+v, want pass despite AI failure", result)
	}
}

func TestAuditErrorUsesAuditResultExitCode(t *testing.T) {
	err := auditError(&models.AuditResult{Passed: false, ExitCode: 1, Reasons: []string{"Tests were not detected."}})
	var failure auditFailure
	if !errors.As(err, &failure) {
		t.Fatalf("auditError() = %v, want auditFailure", err)
	}
	if failure.exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", failure.exitCode)
	}
}

func TestNormalizeAnalyzeArgsKeepsAuditFlagsBeforePositionals(t *testing.T) {
	got := normalizeAnalyzeArgs([]string{".", "--audit", "--allow-medium", "--allow-missing-tests", "--fail-on-low", "--ai"})
	want := []string{"--audit", "--allow-medium", "--allow-missing-tests", "--fail-on-low", "--ai", "."}
	if len(got) != len(want) {
		t.Fatalf("normalizeAnalyzeArgs length = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("normalizeAnalyzeArgs[%d] = %q, want %q (%v)", i, got[i], want[i], got)
		}
	}
}

func TestNormalizeAnalyzeArgsKeepsAssignedAuditFlagsBeforePositionals(t *testing.T) {
	got := normalizeAnalyzeArgs([]string{".", "--audit=true", "--allow-medium=true", "--allow-missing-tests=true", "--fail-on-low=true"})
	want := []string{"--audit=true", "--allow-medium=true", "--allow-missing-tests=true", "--fail-on-low=true", "."}
	if len(got) != len(want) {
		t.Fatalf("normalizeAnalyzeArgs length = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("normalizeAnalyzeArgs[%d] = %q, want %q (%v)", i, got[i], want[i], got)
		}
	}
}

func TestNormalizeAskArgsKeepsFlagsBeforePathAndQuestion(t *testing.T) {
	got := normalizeAskArgs([]string{".", "Where are the API routes?", "--json", "--ai", "--model", "llama3.2:3b", "--no-tui"})
	want := []string{"--json", "--ai", "--model", "llama3.2:3b", "--no-tui", ".", "Where are the API routes?"}
	if len(got) != len(want) {
		t.Fatalf("normalizeAskArgs length = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("normalizeAskArgs[%d] = %q, want %q (%v)", i, got[i], want[i], got)
		}
	}
}

func TestNormalizeAskArgsKeepsAssignedFlagsBeforePositionals(t *testing.T) {
	got := normalizeAskArgs([]string{".", "What is this project for?", "--json=true", "--ai=false", "--model=qwen:7b", "--ai-debug=true"})
	want := []string{"--json=true", "--ai=false", "--model=qwen:7b", "--ai-debug=true", ".", "What is this project for?"}
	if len(got) != len(want) {
		t.Fatalf("normalizeAskArgs length = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("normalizeAskArgs[%d] = %q, want %q (%v)", i, got[i], want[i], got)
		}
	}
}

func healthyAnalysis() *models.Analysis {
	return &models.Analysis{
		Stack: models.StackInfo{
			Languages: []string{"Go"},
		},
		Tests: models.TestAnalysis{
			HasTestFiles:  true,
			HasTestScript: true,
		},
	}
}

func staticViteAnalysis() *models.Analysis {
	analysis := healthyAnalysis()
	analysis.Stack = models.StackInfo{
		Languages:  []string{"TypeScript", "JavaScript"},
		Frameworks: []string{"Vite", "React", "Node.js"},
		Deployment: []string{"Vercel"},
	}
	return analysis
}

func animerecLikeAnalysis() *models.Analysis {
	analysis := staticViteAnalysis()
	analysis.Tests = models.TestAnalysis{}
	analysis.Findings = []models.Finding{
		{Severity: models.SeverityLow, Category: "docs", Message: "README missing deployment notes."},
	}
	return analysis
}

func healthyProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "main.go"), "package main\n\nfunc main() {}\n")
	writeTestFile(t, filepath.Join(root, "main_test.go"), "package main\n\nimport \"testing\"\n\nfunc TestMain(t *testing.T) {}\n")
	return root
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readAnalysisJSON(t *testing.T, root string) *models.Analysis {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".stackmap", "analysis.json"))
	if err != nil {
		t.Fatalf("read analysis.json: %v", err)
	}
	var analysis models.Analysis
	if err := json.Unmarshal(data, &analysis); err != nil {
		t.Fatalf("unmarshal analysis.json: %v", err)
	}
	return &analysis
}

func assertAuditFailsWith(t *testing.T, result *models.AuditResult, reason string) {
	t.Helper()
	if result.Passed || result.ExitCode == 0 {
		t.Fatalf("EvaluateAudit() = %+v, want failure", result)
	}
	if !contains(result.Reasons, reason) {
		t.Fatalf("reasons = %v, want %q", result.Reasons, reason)
	}
}

func contains(items []string, want string) bool {
	return strings.Contains("\x00"+strings.Join(items, "\x00")+"\x00", "\x00"+want+"\x00")
}
