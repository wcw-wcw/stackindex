package ai

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/will/stackmap/internal/models"
)

func TestBuildAIFactsheetIncludesStackProjectFactsAndCaps(t *testing.T) {
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
		RepoPath: "/work/demo",
		Files: []models.FileInfo{
			{Path: "package.json", Kind: models.FileKindConfig, Hash: "abc123", SizeBytes: 100},
			{Path: ".env", Kind: models.FileKindConfig, Hash: "secret-hash", SizeBytes: 50},
			{Path: "src/main.ts", Kind: models.FileKindSource, Hash: "def456", SizeBytes: 200},
			{Path: "src/main.test.ts", Kind: models.FileKindTest, SizeBytes: 80},
			{Path: "docs/roadmap.md", Kind: models.FileKindDoc, SizeBytes: 200},
		},
		Stack: models.StackInfo{
			Languages:  []string{"TypeScript", "JavaScript"},
			Frameworks: []string{"Next.js", "React", "Node.js"},
			Databases:  []string{"PostgreSQL"},
			Testing:    []string{"Vitest"},
			Deployment: []string{"Vercel"},
		},
		PackageInfo: &models.PackageInfo{
			Scripts: map[string]string{"build": "next build", "dev": "next dev", "test": "vitest"},
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

	input := BuildAIFactsheet(analysis)
	if input.RepositoryName != "demo" || input.ScannedPath != "/work/demo" {
		t.Fatalf("unexpected repo identity: %#v", input)
	}
	if input.FilesScanned != 5 {
		t.Fatalf("FilesScanned = %d, want 5", input.FilesScanned)
	}
	if input.FileCounts[string(models.FileKindConfig)] != 2 || input.FileCounts[string(models.FileKindSource)] != 1 || input.FileCounts[string(models.FileKindTest)] != 1 {
		t.Fatalf("unexpected file counts: %#v", input.FileCounts)
	}
	if input.DetectedStack.Frameworks[0] != "Next.js" || input.DetectedStack.Databases[0] != "PostgreSQL" || input.DetectedStack.DeploymentTargets[0] != "Vercel" {
		t.Fatalf("stack facts were not preserved: %#v", input.DetectedStack)
	}
	if len(input.APIRoutes) != routeLimit {
		t.Fatalf("routes length = %d, want %d", len(input.APIRoutes), routeLimit)
	}
	if input.APIRoutesTotal != routeLimit+5 {
		t.Fatalf("APIRoutesTotal = %d, want %d", input.APIRoutesTotal, routeLimit+5)
	}
	if input.PackageScripts["build"] != "next build" {
		t.Fatalf("package scripts were not included")
	}
	if input.Environment.Classifications["required_app_config"] != 1 || input.Environment.Classifications["platform_provided"] != 1 {
		t.Fatalf("unexpected env classifications: %#v", input.Environment.Classifications)
	}
	if input.FindingCounts[string(models.SeverityMedium)] != 1 {
		t.Fatalf("finding counts missing medium finding: %#v", input.FindingCounts)
	}
}

func TestBuildAIFactsheetExcludesArbitraryDocsAndSourceSnippets(t *testing.T) {
	analysis := &models.Analysis{
		RepoName: "demo",
		RepoPath: "/work/demo",
		Files: []models.FileInfo{
			{Path: "src/roadmap.md", Kind: models.FileKindDoc},
			{Path: "src/main.ts", Kind: models.FileKindSource},
		},
		Stack: models.StackInfo{Languages: []string{"TypeScript"}},
	}
	data, err := json.Marshal(BuildAIFactsheet(analysis))
	if err != nil {
		t.Fatalf("Marshal factsheet: %v", err)
	}
	text := string(data)
	for _, unwanted := range []string{"src/roadmap.md", "src/main.ts", "roadmap snippet", "source snippet"} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("factsheet included arbitrary docs/source content %q:\n%s", unwanted, text)
		}
	}
}

func TestModelCandidatesDefaultOrderAndOverrides(t *testing.T) {
	defaults := modelCandidates("", nil)
	if len(defaults) != 2 || defaults[0] != DefaultModel || defaults[1] != FallbackModel {
		t.Fatalf("default candidates = %#v, want default then fallback", defaults)
	}

	explicit := modelCandidates("qwen:7b", []string{"llama3.2:3b", "qwen:7b"})
	if len(explicit) != 2 || explicit[0] != "qwen:7b" || explicit[1] != "llama3.2:3b" {
		t.Fatalf("explicit candidates = %#v, want explicit then unique fallback", explicit)
	}
}

func TestWriteDebugFilesWritesExpectedFilesOnlyWhenEnabled(t *testing.T) {
	tmp := t.TempDir()
	artifacts := DebugArtifacts{
		Factsheet: BuildAIFactsheet(&models.Analysis{
			RepoName: "demo",
			RepoPath: "/work/demo",
			Stack:    models.StackInfo{Languages: []string{"Go"}},
		}),
		Prompt:          "Prompt text",
		RawResponse:     "Raw response",
		RetryResponse:   "Retry response",
		ParseError:      "response did not contain a JSON object",
		Relevance:       relevanceLowConfidence,
		RelevanceReason: "No detected stack term matched.",
	}

	if err := WriteDebugFiles("", artifacts); err != nil {
		t.Fatalf("WriteDebugFiles disabled returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "ai-debug")); !os.IsNotExist(err) {
		t.Fatalf("disabled debug write created files or returned unexpected stat error: %v", err)
	}

	debugDir := filepath.Join(tmp, "ai-debug")
	if err := WriteDebugFiles(debugDir, artifacts); err != nil {
		t.Fatalf("WriteDebugFiles returned error: %v", err)
	}
	for _, name := range []string{"factsheet.json", "factsheet.txt", "prompt.txt", "raw-response.txt", "retry-response.txt", "parse-error.txt", "relevance-result.json"} {
		if _, err := os.Stat(filepath.Join(debugDir, name)); err != nil {
			t.Fatalf("expected debug file %s: %v", name, err)
		}
	}
}

func TestWriteDebugFilesRedactsEnvStyleValues(t *testing.T) {
	debugDir := filepath.Join(t.TempDir(), "ai-debug")
	artifacts := DebugArtifacts{
		Factsheet:   BuildAIFactsheet(&models.Analysis{RepoName: "demo"}),
		Prompt:      "DATABASE_URL=postgres://user:password@localhost/db\nSAFE_NAME",
		RawResponse: "API_TOKEN=super-secret-token",
	}
	if err := WriteDebugFiles(debugDir, artifacts); err != nil {
		t.Fatalf("WriteDebugFiles returned error: %v", err)
	}
	for _, name := range []string{"prompt.txt", "raw-response.txt"} {
		data, err := os.ReadFile(filepath.Join(debugDir, name))
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", name, err)
		}
		text := string(data)
		if strings.Contains(text, "postgres://user:password") || strings.Contains(text, "super-secret-token") {
			t.Fatalf("%s contained an env-style value:\n%s", name, text)
		}
		if !strings.Contains(text, "[redacted]") {
			t.Fatalf("%s did not include redaction marker:\n%s", name, text)
		}
	}
}

func TestWriteDebugFilesRemovesStaleOptionalArtifacts(t *testing.T) {
	debugDir := filepath.Join(t.TempDir(), "ai-debug")
	first := DebugArtifacts{
		Factsheet:     BuildAIFactsheet(&models.Analysis{RepoName: "demo"}),
		Prompt:        "Prompt text",
		RawResponse:   "Raw response",
		RetryResponse: "Retry response",
		ParseError:    "parse failed",
	}
	if err := WriteDebugFiles(debugDir, first); err != nil {
		t.Fatalf("initial WriteDebugFiles returned error: %v", err)
	}
	second := DebugArtifacts{
		Factsheet:   first.Factsheet,
		Prompt:      "Prompt text",
		RawResponse: "Raw response",
	}
	if err := WriteDebugFiles(debugDir, second); err != nil {
		t.Fatalf("second WriteDebugFiles returned error: %v", err)
	}
	for _, name := range []string{"retry-response.txt", "parse-error.txt"} {
		if _, err := os.Stat(filepath.Join(debugDir, name)); !os.IsNotExist(err) {
			t.Fatalf("stale optional artifact %s was not removed; stat err=%v", name, err)
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

func TestParseModelResponseFencedJSON(t *testing.T) {
	text := "```json\n" + `{
		"projectSummary": " Local-first analyzer. ",
		"architectureOverview": "CLI runs analyzers.",
		"keyStrengths": [" Deterministic analysis ", ""],
		"potentialRisks": [],
		"recommendedNextSteps": ["Add smoke tests"]
	}` + "\n```"

	summary, err := ParseModelResponse(text)
	if err != nil {
		t.Fatalf("ParseModelResponse returned error: %v", err)
	}
	if summary.ProjectSummary != "Local-first analyzer." {
		t.Fatalf("ProjectSummary = %q", summary.ProjectSummary)
	}
	if len(summary.KeyStrengths) != 1 || summary.KeyStrengths[0] != "Deterministic analysis" {
		t.Fatalf("KeyStrengths = %#v", summary.KeyStrengths)
	}
}

func TestParseModelResponseGenericFence(t *testing.T) {
	text := "```\n" + `{
		"projectSummary": "Summary.",
		"architectureOverview": "Architecture.",
		"keyStrengths": ["Strength"],
		"potentialRisks": ["Risk"],
		"recommendedNextSteps": ["Step"]
	}` + "\n```"

	if _, err := ParseModelResponse(text); err != nil {
		t.Fatalf("ParseModelResponse returned error: %v", err)
	}
}

func TestParseModelResponseWithProseBeforeAfter(t *testing.T) {
	text := `Here is the summary:
{
	"projectSummary": "Summary.",
	"architectureOverview": "Architecture.",
	"keyStrengths": ["Strength"],
	"potentialRisks": ["Risk"],
	"recommendedNextSteps": ["Step"]
}
Hope this helps.`

	summary, err := ParseModelResponse(text)
	if err != nil {
		t.Fatalf("ParseModelResponse returned error: %v", err)
	}
	if summary.ArchitectureOverview != "Architecture." {
		t.Fatalf("ArchitectureOverview = %q", summary.ArchitectureOverview)
	}
}

func TestParseModelResponseSkipsInvalidObjectBeforeValidJSON(t *testing.T) {
	text := `This {is not json}
{
	"projectSummary": "Summary.",
	"architectureOverview": "",
	"keyStrengths": [],
	"potentialRisks": [],
	"recommendedNextSteps": []
}`

	summary, err := ParseModelResponse(text)
	if err != nil {
		t.Fatalf("ParseModelResponse returned error: %v", err)
	}
	if summary.ArchitectureOverview != missingSectionFallback {
		t.Fatalf("ArchitectureOverview = %q", summary.ArchitectureOverview)
	}
}

func TestParseModelResponseRepairsCommonJSONIssues(t *testing.T) {
	text := `{
		"projectSummary": "Summary.",
		"architectureOverview": "Architecture.",
		"keyStrengths": ["Strength",],
		"potentialRisks": ["Risk"],
		"recommendedNextSteps": ["Step"],
	}`

	if _, err := ParseModelResponse(text); err != nil {
		t.Fatalf("ParseModelResponse returned error: %v", err)
	}
}

func TestParseModelResponseEmptyFencedJSON(t *testing.T) {
	if _, err := ParseModelResponse("```json\n```"); err == nil {
		t.Fatal("ParseModelResponse succeeded for empty fenced JSON")
	}
}

func TestParseModelResponseAllEmptyJSONIsNotUsable(t *testing.T) {
	text := `{
		"projectSummary": "",
		"architectureOverview": "",
		"keyStrengths": [],
		"potentialRisks": [],
		"recommendedNextSteps": []
	}`

	if _, err := ParseModelResponse(text); err == nil {
		t.Fatal("ParseModelResponse succeeded for all-empty JSON")
	}
}

func TestApplyModelResponseFallsBackOnInvalidJSON(t *testing.T) {
	summary := &models.AISummary{Enabled: true, Model: "qwen:7b"}
	applyModelResponse(summary, "Helpful Go prose, but not JSON.", &models.Analysis{Stack: models.StackInfo{Languages: []string{"Go"}}})

	if summary.RawText != "Helpful Go prose, but not JSON." {
		t.Fatalf("RawText = %q", summary.RawText)
	}
	if summary.ParseError == "" {
		t.Fatal("ParseError was empty")
	}
	if summary.LocalNotes != "Helpful Go prose, but not JSON." {
		t.Fatalf("LocalNotes = %q", summary.LocalNotes)
	}
	if summary.ProjectSummary != "" || len(summary.RecommendedNextSteps) != 0 {
		t.Fatalf("invalid JSON should not populate structured fields: %#v", summary)
	}
	if summary.Relevance != relevancePassed {
		t.Fatalf("Relevance = %q, want passed", summary.Relevance)
	}
	if summary.Status != statusGeneratedText {
		t.Fatalf("Status = %q, want %q", summary.Status, statusGeneratedText)
	}
}

func TestApplyModelResponseDropsEmptyFenceRawText(t *testing.T) {
	summary := &models.AISummary{Enabled: true, Model: "qwen:7b"}
	applyModelResponse(summary, "```json\n```", &models.Analysis{Stack: models.StackInfo{Languages: []string{"Go"}}})

	if summary.RawText != "" {
		t.Fatalf("RawText = %q, want empty", summary.RawText)
	}
	if summary.ParseError == "" {
		t.Fatal("ParseError was empty")
	}
	if summary.Status != statusFallbackEmpty {
		t.Fatalf("Status = %q, want %q", summary.Status, statusFallbackEmpty)
	}
}

func TestApplyModelResponseDropsAllEmptyJSONRawText(t *testing.T) {
	summary := &models.AISummary{Enabled: true, Model: "qwen:7b"}
	applyModelResponse(summary, `{"projectSummary":"","architectureOverview":"","keyStrengths":[],"potentialRisks":[],"recommendedNextSteps":[]}`, &models.Analysis{Stack: models.StackInfo{Languages: []string{"Go"}}})

	if summary.RawText != "" {
		t.Fatalf("RawText = %q, want empty", summary.RawText)
	}
	if summary.ParseError == "" {
		t.Fatal("ParseError was empty")
	}
	if summary.Status != statusFallbackEmpty {
		t.Fatalf("Status = %q, want %q", summary.Status, statusFallbackEmpty)
	}
}

func TestQwenStylePathResponseIsLowConfidence(t *testing.T) {
	analysis := &models.Analysis{
		Stack: models.StackInfo{
			Languages:  []string{"TypeScript"},
			Frameworks: []string{"Next.js"},
			Databases:  []string{"PostgreSQL"},
			Deployment: []string{"Vercel"},
		},
	}
	summary := &models.AISummary{Enabled: true, Model: "qwen:7b"}
	applyModelResponse(summary, "Yes, that's a valid path in Unix-like systems. src/roadmap.md is a relative path.", analysis)

	if summary.Relevance != relevanceLowConfidence {
		t.Fatalf("Relevance = %q, want low confidence; reason=%q", summary.Relevance, summary.RelevanceReason)
	}
	if summary.RawText == "" {
		t.Fatal("RawText should retain diagnostic text for analysis.json")
	}
	if summary.LocalNotes != "" {
		t.Fatalf("LocalNotes = %q, want empty for irrelevant text", summary.LocalNotes)
	}
	if summary.Status != statusFallbackIrrelevant {
		t.Fatalf("Status = %q, want %q", summary.Status, statusFallbackIrrelevant)
	}
}

func TestPlainTextModelResponsePassesRelevanceValidation(t *testing.T) {
	analysis := &models.Analysis{
		Stack: models.StackInfo{
			Languages:  []string{"Go"},
			Frameworks: []string{"Bubble Tea"},
		},
	}
	text := "This Go CLI uses Bubble Tea for a local terminal workflow.\n\n- StackMap detected Go source files.\n- Keep reports current before handoff."
	summary := &models.AISummary{Enabled: true, Model: "llama3.2:3b"}
	applyModelResponse(summary, text, analysis)

	if summary.ParseError == "" {
		t.Fatal("ParseError should record the optional structured parse failure")
	}
	if summary.LocalNotes != text {
		t.Fatalf("LocalNotes = %q", summary.LocalNotes)
	}
	if summary.Status != statusGeneratedText {
		t.Fatalf("Status = %q, want %q", summary.Status, statusGeneratedText)
	}
	if summary.Relevance != relevancePassed {
		t.Fatalf("Relevance = %q, want passed; reason=%q", summary.Relevance, summary.RelevanceReason)
	}
}

func TestPlainTextModelResponseRejectsUnsupportedOverclaim(t *testing.T) {
	analysis := &models.Analysis{
		Stack: models.StackInfo{
			Languages:  []string{"TypeScript"},
			Frameworks: []string{"React"},
		},
	}
	text := "This TypeScript React project uses a PostgreSQL database and microservices architecture."
	summary := &models.AISummary{Enabled: true, Model: "llama3.2:3b"}
	applyModelResponse(summary, text, analysis)

	if summary.LocalNotes != "" {
		t.Fatalf("LocalNotes = %q, want empty for unsupported claims", summary.LocalNotes)
	}
	if summary.Relevance != relevanceLowConfidence {
		t.Fatalf("Relevance = %q, want low confidence; reason=%q", summary.Relevance, summary.RelevanceReason)
	}
	if summary.Status != statusFallbackIrrelevant {
		t.Fatalf("Status = %q, want %q", summary.Status, statusFallbackIrrelevant)
	}
}

func TestPlainTextModelResponseRejectsUnsupportedSecurityRiskClaim(t *testing.T) {
	analysis := &models.Analysis{
		Stack:    models.StackInfo{Languages: []string{"TypeScript"}},
		Findings: []models.Finding{{Severity: models.SeverityLow, Category: "tests", Message: "No test files found."}},
	}
	text := "This TypeScript project has detected security risks: no obvious test files were found."
	summary := &models.AISummary{Enabled: true, Model: "llama3.2:3b"}
	applyModelResponse(summary, text, analysis)

	if summary.Relevance != relevanceLowConfidence {
		t.Fatalf("Relevance = %q, want low confidence; reason=%q", summary.Relevance, summary.RelevanceReason)
	}
	if !strings.Contains(summary.RelevanceReason, "security findings") {
		t.Fatalf("RelevanceReason = %q, want security support warning", summary.RelevanceReason)
	}
}

func TestPlainTextModelResponseRejectsUnsupportedTestingCoverageClaim(t *testing.T) {
	analysis := &models.Analysis{
		Stack: models.StackInfo{
			Frameworks: []string{"Next.js"},
			Testing:    []string{"Vitest"},
		},
		Tests: models.TestAnalysis{HasTestFiles: true, HasTestScript: true},
	}
	text := "This Next.js application has API routes that are tested by Vitest."
	summary := &models.AISummary{Enabled: true, Model: "llama3.2:3b"}
	applyModelResponse(summary, text, analysis)

	if summary.Relevance != relevanceLowConfidence {
		t.Fatalf("Relevance = %q, want low confidence; reason=%q", summary.Relevance, summary.RelevanceReason)
	}
	if !strings.Contains(summary.RelevanceReason, "test coverage") {
		t.Fatalf("RelevanceReason = %q, want test coverage support warning", summary.RelevanceReason)
	}
}

func TestPlainTextModelResponseRejectsDeploymentReadyClaim(t *testing.T) {
	analysis := &models.Analysis{
		Stack:      models.StackInfo{Frameworks: []string{"Next.js"}, Deployment: []string{"Vercel"}},
		Deployment: models.DeploymentAnalysis{HasReadme: true, HasEnvExample: true},
	}
	text := "This Next.js project is deployment-ready for Vercel."
	summary := &models.AISummary{Enabled: true, Model: "llama3.2:3b"}
	applyModelResponse(summary, text, analysis)

	if summary.Relevance != relevanceLowConfidence {
		t.Fatalf("Relevance = %q, want low confidence; reason=%q", summary.Relevance, summary.RelevanceReason)
	}
	if !strings.Contains(summary.RelevanceReason, "readiness signals") {
		t.Fatalf("RelevanceReason = %q, want readiness signal support warning", summary.RelevanceReason)
	}
}

func TestPlainTextModelResponseRejectsUnsupportedRequiredEnvClaim(t *testing.T) {
	analysis := &models.Analysis{
		Stack: models.StackInfo{Languages: []string{"TypeScript"}},
		Env:   models.EnvAnalysis{UsesEnvVars: true},
	}
	text := "This TypeScript project has at least one missing required variable in example files."
	summary := &models.AISummary{Enabled: true, Model: "llama3.2:3b"}
	applyModelResponse(summary, text, analysis)

	if summary.Relevance != relevanceLowConfidence {
		t.Fatalf("Relevance = %q, want low confidence; reason=%q", summary.Relevance, summary.RelevanceReason)
	}
	if !strings.Contains(summary.RelevanceReason, "required environment variables") {
		t.Fatalf("RelevanceReason = %q, want required env support warning", summary.RelevanceReason)
	}
}

func TestPlainTextModelResponseRejectsCurrentDeploymentClaim(t *testing.T) {
	analysis := &models.Analysis{
		Stack: models.StackInfo{Frameworks: []string{"React"}, Deployment: []string{"Vercel"}},
	}
	text := "This React project is currently deployed on Vercel."
	summary := &models.AISummary{Enabled: true, Model: "llama3.2:3b"}
	applyModelResponse(summary, text, analysis)

	if summary.Relevance != relevanceLowConfidence {
		t.Fatalf("Relevance = %q, want low confidence; reason=%q", summary.Relevance, summary.RelevanceReason)
	}
	if !strings.Contains(summary.RelevanceReason, "current deployment state") {
		t.Fatalf("RelevanceReason = %q, want current deployment support warning", summary.RelevanceReason)
	}
}

func TestPlainTextModelResponseRejectsUnsupportedCriticalityClaim(t *testing.T) {
	analysis := &models.Analysis{
		Stack: models.StackInfo{Languages: []string{"JavaScript"}},
	}
	text := "This JavaScript project needs tests given the critical nature of the application."
	summary := &models.AISummary{Enabled: true, Model: "llama3.2:3b"}
	applyModelResponse(summary, text, analysis)

	if summary.Relevance != relevanceLowConfidence {
		t.Fatalf("Relevance = %q, want low confidence; reason=%q", summary.Relevance, summary.RelevanceReason)
	}
	if !strings.Contains(summary.RelevanceReason, "criticality") {
		t.Fatalf("RelevanceReason = %q, want criticality support warning", summary.RelevanceReason)
	}
}

func TestPlainTextModelResponseRejectsRuntimeReachabilityClaim(t *testing.T) {
	analysis := &models.Analysis{
		Stack:      models.StackInfo{Frameworks: []string{"Next.js"}},
		Deployment: models.DeploymentAnalysis{HasHealthEndpoint: true},
	}
	text := "This Next.js project has a health endpoint that is reachable from the root URL."
	summary := &models.AISummary{Enabled: true, Model: "llama3.2:3b"}
	applyModelResponse(summary, text, analysis)

	if summary.Relevance != relevanceLowConfidence {
		t.Fatalf("Relevance = %q, want low confidence; reason=%q", summary.Relevance, summary.RelevanceReason)
	}
	if !strings.Contains(summary.RelevanceReason, "runtime reachability") {
		t.Fatalf("RelevanceReason = %q, want reachability support warning", summary.RelevanceReason)
	}
}

func TestPlainTextModelResponseRejectsUnsupportedRequirementsClaim(t *testing.T) {
	analysis := &models.Analysis{
		Stack: models.StackInfo{Deployment: []string{"Vercel"}},
	}
	text := "Deployment target is Vercel, which requires an env.example file to be present."
	summary := &models.AISummary{Enabled: true, Model: "llama3.2:3b"}
	applyModelResponse(summary, text, analysis)

	if summary.Relevance != relevanceLowConfidence {
		t.Fatalf("Relevance = %q, want low confidence; reason=%q", summary.Relevance, summary.RelevanceReason)
	}
	if !strings.Contains(summary.RelevanceReason, "project requirements") {
		t.Fatalf("RelevanceReason = %q, want requirements support warning", summary.RelevanceReason)
	}
}

func TestStructuredParsedSummaryPassesRelevanceValidation(t *testing.T) {
	analysis := &models.Analysis{
		Stack: models.StackInfo{
			Frameworks: []string{"Next.js"},
			Databases:  []string{"PostgreSQL"},
			Deployment: []string{"Vercel"},
		},
	}
	text := `{
		"projectSummary": "A Next.js application backed by PostgreSQL with Vercel deployment signals.",
		"architectureOverview": "API routes and package scripts support the web app workflow.",
		"keyStrengths": ["Health endpoint detected"],
		"potentialRisks": ["Keep environment variables documented"],
		"recommendedNextSteps": ["Run Vitest before deployment"]
	}`
	summary := &models.AISummary{Enabled: true, Model: "qwen:7b"}
	applyModelResponse(summary, text, analysis)

	if summary.ParseError != "" {
		t.Fatalf("ParseError = %q", summary.ParseError)
	}
	if summary.Relevance != relevancePassed {
		t.Fatalf("Relevance = %q, want passed; reason=%q", summary.Relevance, summary.RelevanceReason)
	}
}

func TestStructuredParsedSummaryRejectsUnsupportedArchitectureClaims(t *testing.T) {
	analysis := &models.Analysis{
		Stack: models.StackInfo{
			Frameworks: []string{"Next.js"},
			Databases:  []string{"PostgreSQL"},
		},
		Tests: models.TestAnalysis{HasTestFiles: true, HasTestScript: true},
	}
	text := `{
		"projectSummary": "Next.js app with PostgreSQL.",
		"architectureOverview": "Microservices architecture with a single endpoint for health checks.",
		"keyStrengths": ["Next.js routes are detected"],
		"potentialRisks": ["Missing security headers in API routes", "Insufficient testing for migration scripts"],
		"recommendedNextSteps": ["Implement additional security measures"]
	}`
	summary := &models.AISummary{Enabled: true, Model: "llama3.2:3b"}
	applyModelResponse(summary, text, analysis)

	if summary.ParseError != "" {
		t.Fatalf("ParseError = %q", summary.ParseError)
	}
	if summary.Relevance != relevanceLowConfidence {
		t.Fatalf("Relevance = %q, want low confidence; reason=%q", summary.Relevance, summary.RelevanceReason)
	}
	if !strings.Contains(summary.RelevanceReason, "service topology") {
		t.Fatalf("RelevanceReason = %q, want service topology support warning", summary.RelevanceReason)
	}
}

func TestStructuredParsedSummaryRejectsUnsupportedTestingPraise(t *testing.T) {
	analysis := &models.Analysis{
		Stack: models.StackInfo{
			Frameworks: []string{"Vite", "React"},
		},
	}
	text := `{
		"projectSummary": "A Vite React app.",
		"architectureOverview": "The app uses Vite.",
		"keyStrengths": ["Strong testing focus"],
		"potentialRisks": [],
		"recommendedNextSteps": ["Add deployment docs"]
	}`
	summary := &models.AISummary{Enabled: true, Model: "llama3.2:3b"}
	applyModelResponse(summary, text, analysis)

	if summary.Relevance != relevanceLowConfidence {
		t.Fatalf("Relevance = %q, want low confidence; reason=%q", summary.Relevance, summary.RelevanceReason)
	}
	if !strings.Contains(summary.RelevanceReason, "strong testing") {
		t.Fatalf("RelevanceReason = %q, want testing support warning", summary.RelevanceReason)
	}
}

func TestStructuredParsedSummaryRejectsMissingMigrationContradiction(t *testing.T) {
	analysis := &models.Analysis{
		Stack: models.StackInfo{
			Frameworks: []string{"Vite"},
			Databases:  []string{"Neon Postgres"},
		},
		Deployment: models.DeploymentAnalysis{HasMigrationFiles: true},
	}
	text := `{
		"projectSummary": "A Vite app using Neon Postgres.",
		"architectureOverview": "Build scripts are present.",
		"keyStrengths": ["Vite is detected"],
		"potentialRisks": ["Missing migration files could cause data inconsistencies"],
		"recommendedNextSteps": ["Document migrations"]
	}`
	summary := &models.AISummary{Enabled: true, Model: "llama3.2:3b"}
	applyModelResponse(summary, text, analysis)

	if summary.Relevance != relevanceLowConfidence {
		t.Fatalf("Relevance = %q, want low confidence; reason=%q", summary.Relevance, summary.RelevanceReason)
	}
	if !strings.Contains(summary.RelevanceReason, "migration files were missing") {
		t.Fatalf("RelevanceReason = %q, want missing migration contradiction", summary.RelevanceReason)
	}
}

func TestStructuredParsedSummaryRejectsUnsupportedDatabaseClaims(t *testing.T) {
	analysis := &models.Analysis{
		Stack: models.StackInfo{
			Languages:  []string{"TypeScript"},
			Frameworks: []string{"Vite", "React"},
		},
	}
	text := `{
		"projectSummary": "Vite React TypeScript project with database and deployment targets.",
		"architectureOverview": "The app uses Vite.",
		"keyStrengths": ["React is detected"],
		"potentialRisks": [],
		"recommendedNextSteps": []
	}`
	summary := &models.AISummary{Enabled: true, Model: "llama3.2:3b"}
	applyModelResponse(summary, text, analysis)

	if summary.Relevance != relevanceLowConfidence {
		t.Fatalf("Relevance = %q, want low confidence; reason=%q", summary.Relevance, summary.RelevanceReason)
	}
	if !strings.Contains(summary.RelevanceReason, "database/storage") {
		t.Fatalf("RelevanceReason = %q, want database support warning", summary.RelevanceReason)
	}
}
