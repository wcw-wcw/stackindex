package ai

import (
	"encoding/json"
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
	if summary.ProjectSummary != "" || len(summary.RecommendedNextSteps) != 0 {
		t.Fatalf("invalid JSON should not populate structured fields: %#v", summary)
	}
	if summary.Relevance != relevancePassed {
		t.Fatalf("Relevance = %q, want passed", summary.Relevance)
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
