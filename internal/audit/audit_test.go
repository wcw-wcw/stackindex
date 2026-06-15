package audit

import (
	"strings"
	"testing"

	"github.com/will/stackmap/internal/models"
)

func TestHealthyProjectPassesAudit(t *testing.T) {
	result := Evaluate(healthyAnalysis(), Options{})
	if !result.Passed || result.ExitCode != 0 || len(result.Reasons) != 0 {
		t.Fatalf("Evaluate() = %+v, want pass", result)
	}
}

func TestHighFindingFailsAudit(t *testing.T) {
	analysis := healthyAnalysis()
	analysis.Findings = []models.Finding{{Severity: models.SeverityHigh, Category: "env", Message: "Secret-looking value found."}}

	result := Evaluate(analysis, Options{})
	assertAuditFailsWith(t, result, "1 high finding detected.")
}

func TestMediumFindingFailsByDefault(t *testing.T) {
	analysis := healthyAnalysis()
	analysis.Findings = []models.Finding{{Severity: models.SeverityMedium, Category: "env", Message: "Missing env example."}}

	result := Evaluate(analysis, Options{})
	assertAuditFailsWith(t, result, "1 medium finding detected.")
}

func TestMediumFindingPassesWithAllowMedium(t *testing.T) {
	analysis := healthyAnalysis()
	analysis.Findings = []models.Finding{{Severity: models.SeverityMedium, Category: "env", Message: "Missing env example."}}

	result := Evaluate(analysis, Options{AllowMedium: true})
	if !result.Passed {
		t.Fatalf("Evaluate() = %+v, want pass", result)
	}
	if !contains(result.Warnings, "1 medium finding detected.") {
		t.Fatalf("warnings = %v, want medium warning", result.Warnings)
	}
}

func TestLowFindingPassesByDefault(t *testing.T) {
	analysis := healthyAnalysis()
	analysis.Findings = []models.Finding{{Severity: models.SeverityLow, Category: "docs", Message: "README missing."}}

	result := Evaluate(analysis, Options{})
	if !result.Passed {
		t.Fatalf("Evaluate() = %+v, want pass", result)
	}
	if !contains(result.Warnings, "1 low finding detected.") {
		t.Fatalf("warnings = %v, want low warning", result.Warnings)
	}
}

func TestLowFindingFailsWithFailOnLow(t *testing.T) {
	analysis := healthyAnalysis()
	analysis.Findings = []models.Finding{{Severity: models.SeverityLow, Category: "docs", Message: "README missing."}}

	result := Evaluate(analysis, Options{FailOnLow: true})
	assertAuditFailsWith(t, result, "1 low finding detected.")
}

func TestMissingTestsFailsByDefault(t *testing.T) {
	analysis := healthyAnalysis()
	analysis.Tests = models.TestAnalysis{}

	result := Evaluate(analysis, Options{})
	assertAuditFailsWith(t, result, "Tests were not detected.")
}

func TestMissingTestsPassesWithAllowMissingTests(t *testing.T) {
	analysis := healthyAnalysis()
	analysis.Tests = models.TestAnalysis{}

	result := Evaluate(analysis, Options{AllowMissingTests: true})
	if !result.Passed {
		t.Fatalf("Evaluate() = %+v, want pass", result)
	}
	if !contains(result.Warnings, "Tests were not detected.") {
		t.Fatalf("warnings = %v, want missing tests warning", result.Warnings)
	}
}

func TestEnvVarsWithoutExampleFailAudit(t *testing.T) {
	analysis := healthyAnalysis()
	analysis.Env = models.EnvAnalysis{UsesEnvVars: true}

	result := Evaluate(analysis, Options{})
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

	result := Evaluate(analysis, Options{})
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

	result := Evaluate(analysis, Options{})
	if !result.Passed {
		t.Fatalf("Evaluate() = %+v, want pass", result)
	}
	if !result.HasBackendSurface || !result.RequiresHealthEndpoint {
		t.Fatalf("audit backend fields = hasBackendSurface:%v requiresHealthEndpoint:%v, want true/true", result.HasBackendSurface, result.RequiresHealthEndpoint)
	}
}

func TestStaticViteDeploymentWithoutHealthEndpointWarns(t *testing.T) {
	analysis := staticViteAnalysis()

	result := Evaluate(analysis, Options{})
	if !result.Passed {
		t.Fatalf("Evaluate() = %+v, want pass with warning", result)
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

	result := Evaluate(analysis, Options{AllowMissingTests: true})
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

	result := Evaluate(analysis, Options{})
	assertAuditFailsWith(t, result, "Tests were not detected.")
	if contains(result.Reasons, "Backend/API deployment surface detected but no health endpoint was found.") {
		t.Fatalf("static app should not fail on missing health endpoint: %v", result.Reasons)
	}

	allowed := Evaluate(analysis, Options{AllowMissingTests: true})
	if !allowed.Passed {
		t.Fatalf("Evaluate(allow missing tests) = %+v, want pass", allowed)
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

	result := Evaluate(analysis, Options{})
	if !result.Passed {
		t.Fatalf("Evaluate() = %+v, want pass", result)
	}
}

func TestAnimerecLikeAuditFailsOnlyForMissingTestsByDefault(t *testing.T) {
	analysis := animerecLikeAnalysis()

	result := Evaluate(analysis, Options{})
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

	result := Evaluate(analysis, Options{AllowMissingTests: true})
	if !result.Passed {
		t.Fatalf("Evaluate() = %+v, want pass", result)
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

	result := Evaluate(analysis, Options{AllowMissingTests: true})
	if !result.Passed {
		t.Fatalf("Evaluate() = %+v, want pass", result)
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

	result := Evaluate(analysis, Options{})
	if !result.Passed {
		t.Fatalf("Evaluate() = %+v, want pass despite AI failure", result)
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

func assertAuditFailsWith(t *testing.T, result *models.AuditResult, reason string) {
	t.Helper()
	if result.Passed || result.ExitCode == 0 {
		t.Fatalf("Evaluate() = %+v, want failure", result)
	}
	if !contains(result.Reasons, reason) {
		t.Fatalf("reasons = %v, want %q", result.Reasons, reason)
	}
}

func contains(items []string, want string) bool {
	return strings.Contains("\x00"+strings.Join(items, "\x00")+"\x00", "\x00"+want+"\x00")
}
