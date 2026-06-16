package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/will/stackmap/internal/models"
)

func TestListOllamaModelsSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[{"name":"llama3.2:3b","modified_at":"2026-06-01T12:00:00Z","size":2019393189},{"name":"qwen:7b","size":4500000000}]}`))
	}))
	defer server.Close()

	models, err := listOllamaModels(context.Background(), server.URL, server.Client())
	if err != nil {
		t.Fatalf("listOllamaModels returned error: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("models length = %d, want 2: %#v", len(models), models)
	}
	if models[0].Name != "llama3.2:3b" || models[0].ModifiedAt != "2026-06-01T12:00:00Z" || models[0].Size != 2019393189 {
		t.Fatalf("first model not mapped cleanly: %#v", models[0])
	}
	if models[1].Name != "qwen:7b" || models[1].Size != 4500000000 {
		t.Fatalf("second model not mapped cleanly: %#v", models[1])
	}
}

func TestListOllamaModelsEmptyList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[]}`))
	}))
	defer server.Close()

	models, err := listOllamaModels(context.Background(), server.URL, server.Client())
	if err != nil {
		t.Fatalf("listOllamaModels returned error: %v", err)
	}
	if len(models) != 0 {
		t.Fatalf("models length = %d, want 0: %#v", len(models), models)
	}
}

func TestListOllamaModelsInvalidResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer server.Close()

	_, err := listOllamaModels(context.Background(), server.URL, server.Client())
	if err == nil {
		t.Fatal("listOllamaModels returned nil error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "invalid model list") {
		t.Fatalf("error = %q, want invalid model list", err.Error())
	}
}

func TestListOllamaModelsUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	_, err := listOllamaModels(context.Background(), server.URL, server.Client())
	if err == nil {
		t.Fatal("listOllamaModels returned nil error for unavailable Ollama")
	}
	if !strings.Contains(err.Error(), "HTTP 503") {
		t.Fatalf("error = %q, want HTTP 503", err.Error())
	}
}

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
		Context: models.ProjectContext{
			Purpose:            "Stock monitoring and alerting application",
			Confidence:         "high",
			Evidence:           []string{"README/package metadata mentions stock monitoring.", "API routes include market and watchlist endpoints."},
			ReadmeTitle:        "Stock Watcher",
			PackageName:        "stkapp",
			PackageDescription: "Stock monitoring alerts",
		},
		Structure: models.StructureMap{
			Directories: []models.DirectoryRole{
				{Path: "src/app/api/", Role: "Next.js API route handlers"},
				{Path: "scripts/", Role: "Operational scripts/tooling"},
			},
			KeyFiles: []models.FileRole{
				{Path: "package.json", Role: "Node package manifest and scripts", Importance: "high"},
				{Path: "src/app/api/health/route.ts", Role: "Health endpoint implementation", Importance: "high"},
			},
		},
		Dependencies: models.DependencyGraph{
			Entrypoints: []string{"src/app/api/health/route.ts", "scripts/worker.mjs"},
			TopConnectedFiles: []models.ConnectedFileSummary{
				{Path: "src/app/api/health/route.ts", Role: "Health endpoint implementation", ImportsCount: 1, ImportedByCount: 0, WhyItMatters: "API route handler connected to shared application code."},
				{Path: "src/lib/db.ts", Role: "Source file", ImportsCount: 0, ImportedByCount: 3, WhyItMatters: "Shared module imported by multiple files."},
			},
			ArchitectureHints: []string{"API route files import shared library or database-related code."},
			UnresolvedImports: []models.UnresolvedImport{{From: "src/app/api/health/route.ts", ImportPath: "./missing", Reason: "relative import did not match a file or index file"}},
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
	if input.ProjectContext.Purpose != "Stock monitoring and alerting application" || input.ProjectContext.Confidence != "high" {
		t.Fatalf("project context missing from factsheet: %#v", input.ProjectContext)
	}
	if len(input.StructureSummary.Directories) != 2 || input.StructureSummary.KeyFiles[0].Path != "package.json" {
		t.Fatalf("structure summary missing from factsheet: %#v", input.StructureSummary)
	}
	if len(input.DependencySummary.Entrypoints) != 2 || input.DependencySummary.TopConnectedFiles[1].Path != "src/lib/db.ts" || input.DependencySummary.UnresolvedCount != 1 {
		t.Fatalf("dependency summary missing from factsheet: %#v", input.DependencySummary)
	}
	if input.DependencySummary.ArchitectureHints[0] != "API route files import shared library or database-related code." {
		t.Fatalf("architecture hints missing from factsheet: %#v", input.DependencySummary.ArchitectureHints)
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

func TestPromptRequestsParagraphBulletsAndGroundedGraphSummary(t *testing.T) {
	analysis := &models.Analysis{
		RepoName: "demo",
		Stack:    models.StackInfo{Languages: []string{"TypeScript"}, Frameworks: []string{"Next.js"}},
		Dependencies: models.DependencyGraph{
			Entrypoints:       []string{"src/app/api/rules/route.ts"},
			ArchitectureHints: []string{"API route files import shared library or database-related code."},
		},
	}
	prompt := promptFor(analysis)
	for _, want := range []string{
		"One short paragraph, then 2 to 4 Markdown bullets.",
		"Use dependencySummary facts to explain how main pieces fit together",
		"Do not mention a connection, entrypoint, database, migration, worker, route, or shared module unless it appears in the factsheet.",
		`"dependencySummary"`,
		"src/app/api/rules/route.ts",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt did not contain %q:\n%s", want, prompt)
		}
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

func TestPlainTextModelResponseRejectsEnvVarNameListing(t *testing.T) {
	analysis := &models.Analysis{
		Stack: models.StackInfo{Languages: []string{"TypeScript"}},
	}
	text := "This TypeScript project uses environment variables such as DATABASE_URL and API_TOKEN."
	summary := &models.AISummary{Enabled: true, Model: "llama3.2:3b"}
	applyModelResponse(summary, text, analysis)

	if summary.Relevance != relevanceLowConfidence {
		t.Fatalf("Relevance = %q, want low confidence; reason=%q", summary.Relevance, summary.RelevanceReason)
	}
	if !strings.Contains(summary.RelevanceReason, "environment variable names") {
		t.Fatalf("RelevanceReason = %q, want env var listing warning", summary.RelevanceReason)
	}
	if summary.LocalNotes != "" {
		t.Fatalf("LocalNotes = %q, want empty for env var name listing", summary.LocalNotes)
	}
}

func TestPlainTextModelResponseRejectsFactsheetMetaSummary(t *testing.T) {
	analysis := &models.Analysis{
		Stack: models.StackInfo{Languages: []string{"TypeScript"}, Deployment: []string{"Vercel"}},
	}
	text := "The provided information includes StackDetected and HealthSummary fields for a TypeScript project on Vercel."
	summary := &models.AISummary{Enabled: true, Model: "llama3.2:3b"}
	applyModelResponse(summary, text, analysis)

	if summary.Relevance != relevanceLowConfidence {
		t.Fatalf("Relevance = %q, want low confidence; reason=%q", summary.Relevance, summary.RelevanceReason)
	}
	if !strings.Contains(summary.RelevanceReason, "factsheet field names") {
		t.Fatalf("RelevanceReason = %q, want factsheet meta warning", summary.RelevanceReason)
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
