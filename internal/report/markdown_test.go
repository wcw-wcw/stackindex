package report

import (
	"strings"
	"testing"
	"time"

	"github.com/will/stackmap/internal/models"
)

func TestMarkdownRendersStructuredAISummary(t *testing.T) {
	analysis := baseAnalysis()
	analysis.AI = &models.AISummary{
		Enabled:              true,
		Model:                "qwen:7b",
		Relevance:            "passed",
		ProjectSummary:       "A local-first analyzer.",
		ArchitectureOverview: "A Go CLI runs analyzers and writes reports.",
		KeyStrengths:         []string{"Deterministic static analysis"},
		PotentialRisks:       []string{"Limited framework coverage"},
		RecommendedNextSteps: []string{"Add a smoke test"},
	}

	out := Markdown(analysis)
	for _, want := range []string{
		"## AI Project Summary",
		"StackMap detected this as a Go application using Go.",
		"### Local AI Notes",
		"### Summary",
		"A local-first analyzer.",
		"### Architecture Overview",
		"### Key Strengths",
		"- Deterministic static analysis",
		"### Potential Risks",
		"### Recommended Next Steps",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("Markdown did not contain %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "could not parse") {
		t.Fatalf("structured summary rendered parse fallback:\n%s", out)
	}
}

func TestMarkdownRendersGracefulAIFallback(t *testing.T) {
	analysis := baseAnalysis()
	analysis.AI = &models.AISummary{
		Enabled:    true,
		Model:      "qwen:7b",
		RawText:    "Helpful prose, but not JSON.\n```json\n{}\n```",
		ParseError: "invalid character",
		Relevance:  "passed",
		Status:     "fallback_parse_failed",
	}

	out := Markdown(analysis)
	for _, want := range []string{
		"StackMap detected this as a Go application using Go.",
		"Local AI summary unavailable: `qwen:7b` did not return usable project-summary text.",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("Markdown did not contain %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "could not parse structured JSON") || strings.Contains(out, "### Raw Model Summary") || strings.Contains(out, "```") {
		t.Fatalf("fallback rendered parse/debug text:\n%s", out)
	}
}

func TestMarkdownDoesNotRenderIrrelevantUnixPathExplanationAsMainAISummary(t *testing.T) {
	analysis := richAnalysis()
	analysis.AI = &models.AISummary{
		Enabled:         true,
		Model:           "qwen:7b",
		RawText:         "Yes, that's a valid path in Unix-like systems. src/roadmap.md is a relative path.",
		ParseError:      "response did not contain a JSON object",
		Relevance:       "low_confidence",
		RelevanceReason: "Model output did not mention detected stack terms.",
	}

	out := Markdown(analysis)
	for _, want := range []string{
		"StackMap detected this as a Next.js/React application",
		"TypeScript",
		"PostgreSQL",
		"Local AI summary unavailable: `qwen:7b` did not return usable project-summary text.",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("Markdown did not contain %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "valid path in Unix-like systems") || strings.Contains(out, "### Raw Model Summary") {
		t.Fatalf("Markdown rendered irrelevant raw model rambling:\n%s", out)
	}
}

func TestMarkdownUnavailableMessageListsAttemptedModels(t *testing.T) {
	analysis := richAnalysis()
	analysis.AI = &models.AISummary{
		Enabled:         true,
		Model:           "qwen:7b",
		AttemptedModels: []string{"llama3.2:3b", "qwen:7b"},
		Relevance:       "low_confidence",
		Status:          "fallback_irrelevant",
	}

	out := Markdown(analysis)
	want := "Local AI summary unavailable: `llama3.2:3b` and `qwen:7b` did not return usable project-summary text."
	if !strings.Contains(out, want) {
		t.Fatalf("Markdown did not list attempted models; want %q:\n%s", want, out)
	}
}

func TestMarkdownRendersRelevantRawFallback(t *testing.T) {
	analysis := baseAnalysis()
	analysis.AI = &models.AISummary{
		Enabled:    true,
		Model:      "qwen:7b",
		LocalNotes: "This Go CLI analyzes repositories and writes local StackMap reports.",
		RawText:    "This Go CLI analyzes repositories and writes local StackMap reports.",
		ParseError: "response did not contain a JSON object",
		Relevance:  "passed",
		Status:     "generated_text",
	}

	out := Markdown(analysis)
	if !strings.Contains(out, "### Local AI Notes") || !strings.Contains(out, "This Go CLI analyzes repositories") {
		t.Fatalf("Markdown did not render relevant local AI notes:\n%s", out)
	}
	if !strings.Contains(out, "StackMap detected this as a Go application using Go.") {
		t.Fatalf("Markdown did not render deterministic summary before local notes:\n%s", out)
	}
}

func TestMarkdownRendersRelevantMarkdownBulletAINotes(t *testing.T) {
	analysis := richAnalysis()
	analysis.AI = &models.AISummary{
		Enabled: true,
		Model:   "llama3.2:3b",
		LocalNotes: "This TypeScript Next.js/React app has PostgreSQL and Vercel signals in the StackMap factsheet.\n\n" +
			"- Vitest is detected for testing.\n" +
			"- Migration files and an env example are present.",
		RawText:   "same",
		Relevance: "passed",
		Status:    "generated_text",
	}

	out := Markdown(analysis)
	for _, want := range []string{
		"StackMap detected this as a Next.js/React application",
		"### Local AI Notes",
		"This TypeScript Next.js/React app has PostgreSQL and Vercel signals",
		"- Vitest is detected for testing.",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("Markdown did not contain %q:\n%s", want, out)
		}
	}
}

func TestMarkdownSuppressesUnsupportedOverclaimText(t *testing.T) {
	analysis := baseAnalysis()
	analysis.AI = &models.AISummary{
		Enabled:         true,
		Model:           "llama3.2:3b",
		LocalNotes:      "",
		RawText:         "This Go app has a PostgreSQL database and microservices architecture.",
		ParseError:      "response did not contain a JSON object",
		Relevance:       "low_confidence",
		RelevanceReason: "Model output described service topology, but StackMap does not detect service topology.",
		Status:          "fallback_irrelevant",
	}

	out := Markdown(analysis)
	if !strings.Contains(out, "Local AI summary unavailable: `llama3.2:3b` did not return usable project-summary text.") {
		t.Fatalf("Markdown did not render unavailable text:\n%s", out)
	}
	if strings.Contains(out, "PostgreSQL database and microservices") {
		t.Fatalf("Markdown rendered unsupported overclaim text:\n%s", out)
	}
}

func TestMarkdownDoesNotEmitEmptyCodeFenceForEmptyAIRawText(t *testing.T) {
	analysis := richAnalysis()
	analysis.AI = &models.AISummary{
		Enabled:    true,
		Model:      "qwen:7b",
		ParseError: "response did not contain a JSON object",
	}

	out := Markdown(analysis)
	if !strings.Contains(out, "StackMap detected this as a Next.js/React application") || !strings.Contains(out, "Local AI summary unavailable") {
		t.Fatalf("Markdown did not render deterministic fallback:\n%s", out)
	}
	if strings.Contains(out, "```") || strings.Contains(out, "### Raw Model Summary") {
		t.Fatalf("empty raw fallback rendered a code fence or raw section:\n%s", out)
	}
}

func TestDeterministicAISummaryIncludesDetectedStackTerms(t *testing.T) {
	out := DeterministicAISummary(richAnalysis())
	for _, want := range []string{"Next.js", "React", "TypeScript", "PostgreSQL", "Vitest", "Vercel", "health endpoints", "migration files", "No actionable findings"} {
		if !strings.Contains(out, want) {
			t.Fatalf("DeterministicAISummary did not contain %q:\n%s", want, out)
		}
	}
}

func TestDeterministicAISummaryLabelsViteReactWithoutNext(t *testing.T) {
	analysis := baseAnalysis()
	analysis.Stack = models.StackInfo{
		Languages:  []string{"JavaScript", "TypeScript"},
		Frameworks: []string{"Vite", "React", "Node.js"},
		Databases:  []string{"Neon Postgres"},
		Deployment: []string{"Vercel"},
	}

	out := DeterministicAISummary(analysis)
	if !strings.Contains(out, "Vite/React application") {
		t.Fatalf("DeterministicAISummary did not label Vite/React app:\n%s", out)
	}
	if strings.Contains(out, "Next.js/React") {
		t.Fatalf("DeterministicAISummary incorrectly mentioned Next.js:\n%s", out)
	}
}

func TestStructuredAISummaryTakesPrecedenceOverDeterministicFallback(t *testing.T) {
	analysis := richAnalysis()
	analysis.AI = &models.AISummary{
		Enabled:              true,
		Model:                "llama3.2:3b",
		Relevance:            "passed",
		ProjectSummary:       "The model summary wins.",
		ArchitectureOverview: "Model architecture.",
	}

	out := Markdown(analysis)
	if !strings.Contains(out, "The model summary wins.") {
		t.Fatalf("structured summary was not rendered:\n%s", out)
	}
	if strings.Contains(out, "Local AI summary unavailable") {
		t.Fatalf("deterministic fallback rendered over structured summary:\n%s", out)
	}
}

func TestMarkdownOnlyListsMissingRequiredEnvVars(t *testing.T) {
	analysis := baseAnalysis()
	analysis.Env = models.EnvAnalysis{
		UsesEnvVars:                true,
		MissingFromExample:         []string{"NODE_ENV", "BUILD_TIME", "DATABASE_URL"},
		MissingRequiredFromExample: []string{"DATABASE_URL"},
		UsedVars: []models.EnvVar{
			{Name: "NODE_ENV", Classification: "platform_provided", MissingExample: true},
			{Name: "BUILD_TIME", Classification: "build_metadata", MissingExample: true},
			{Name: "DATABASE_URL", Classification: "required_app_config", MissingExample: true},
		},
	}

	out := Markdown(analysis)
	if !strings.Contains(out, "- Missing required from .env.example: `DATABASE_URL`") {
		t.Fatalf("Markdown did not list required missing env var:\n%s", out)
	}
	if strings.Contains(out, "Missing from .env.example") {
		t.Fatalf("Markdown used old noisy missing env label:\n%s", out)
	}
}

func TestMarkdownAuditResultRendersPassingState(t *testing.T) {
	analysis := baseAnalysis()
	analysis.Audit = &models.AuditResult{
		Passed:   true,
		ExitCode: 0,
		Mode:     "deployment-readiness",
	}

	out := Markdown(analysis)
	for _, want := range []string{
		"## Audit Result",
		"- Status: passed",
		"- Exit code: 0",
		"- Blocking issues: none",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("Markdown did not contain %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "## AI Project Summary") && strings.Index(out, "## Audit Result") > strings.Index(out, "## AI Project Summary") {
		t.Fatalf("Audit Result rendered after AI Project Summary:\n%s", out)
	}
}

func TestMarkdownAuditResultRendersFailingState(t *testing.T) {
	analysis := baseAnalysis()
	analysis.Audit = &models.AuditResult{
		Passed:   false,
		ExitCode: 1,
		Mode:     "deployment-readiness",
		Reasons: []string{
			"Tests were not detected.",
			"Deployment target detected but no health endpoint was found.",
		},
	}

	out := Markdown(analysis)
	for _, want := range []string{
		"## Audit Result",
		"- Status: failed",
		"- Exit code: 1",
		"- Blocking issues:",
		"  - Tests were not detected.",
		"  - Deployment target detected but no health endpoint was found.",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("Markdown did not contain %q:\n%s", want, out)
		}
	}
}

func TestMarkdownDoesNotRenderAuditResultWithoutAuditMode(t *testing.T) {
	out := Markdown(baseAnalysis())
	if strings.Contains(out, "## Audit Result") {
		t.Fatalf("Markdown rendered audit result without audit mode:\n%s", out)
	}
}

func baseAnalysis() *models.Analysis {
	return &models.Analysis{
		RepoPath:    "/tmp/demo",
		RepoName:    "demo",
		GeneratedAt: time.Date(2026, 6, 12, 9, 0, 0, 0, time.UTC),
		Files: []models.FileInfo{
			{Path: "main.go", Kind: models.FileKindSource},
		},
		Stack: models.StackInfo{Languages: []string{"Go"}},
	}
}

func richAnalysis() *models.Analysis {
	analysis := baseAnalysis()
	analysis.Stack = models.StackInfo{
		Languages:  []string{"TypeScript", "JavaScript"},
		Frameworks: []string{"Next.js", "React"},
		Databases:  []string{"PostgreSQL"},
		Testing:    []string{"Vitest"},
		Deployment: []string{"Vercel"},
	}
	analysis.Tests = models.TestAnalysis{HasTestFiles: true, HasTestScript: true, Frameworks: []string{"Vitest"}}
	analysis.Deployment = models.DeploymentAnalysis{
		HasReadme:            true,
		ReadmeMentionsDeploy: true,
		HasEnvExample:        true,
		HasHealthEndpoint:    true,
		HasMigrationFiles:    true,
		HasVercelConfig:      true,
	}
	return analysis
}
