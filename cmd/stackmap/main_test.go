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

func TestDeploymentTargetWithoutHealthEndpointFailsAudit(t *testing.T) {
	analysis := healthyAnalysis()
	analysis.Stack.Deployment = []string{"Vercel"}
	analysis.Deployment.HasHealthEndpoint = false

	result := EvaluateAudit(analysis, AuditOptions{})
	assertAuditFailsWith(t, result, "Deployment target detected but no health endpoint was found.")
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
