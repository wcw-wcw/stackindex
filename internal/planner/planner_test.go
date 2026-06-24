package planner

import (
	"strings"
	"testing"
	"time"

	"github.com/wcw-wcw/stackindex/internal/models"
)

func TestPlanMatchesDomainFeatures(t *testing.T) {
	analysis := plannerTestAnalysis()

	cases := []struct {
		task    string
		feature string
		files   []string
	}{
		{
			task:    "fix rule validation bug",
			feature: "Rules / alerts",
			files: []string{
				"src/lib/rules/schema.ts",
				"src/app/api/rules/route.ts",
				"src/lib/db/repositories.ts",
			},
		},
		{
			task:    "debug worker tick issue",
			feature: "Worker",
			files: []string{
				"src/app/api/worker/tick/route.ts",
				"src/lib/worker/local-worker-loop.ts",
			},
		},
		{
			task:    "fix watchlist delete edit issue",
			feature: "Watchlist",
			files: []string{
				"src/app/watchlist/page.tsx",
				"src/app/api/watchlist/[symbol]/route.ts",
				"src/lib/db/repositories.ts",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.task, func(t *testing.T) {
			plan := Plan(tc.task, analysis)
			if plan.MatchedFeature != tc.feature {
				t.Fatalf("MatchedFeature = %q, want %q", plan.MatchedFeature, tc.feature)
			}
			got := recPaths(plan.RecommendedFiles)
			for _, file := range tc.files {
				if !containsPath(got, file) {
					t.Fatalf("recommended files = %v, want %s", got, file)
				}
			}
		})
	}
}

func TestPlanRanksRuleSchemaAndRelatedTests(t *testing.T) {
	plan := Plan("fix rule validation bug", plannerTestAnalysis())

	if len(plan.RecommendedFiles) == 0 {
		t.Fatal("no recommended files")
	}
	if plan.RecommendedFiles[0].Path != "src/lib/rules/schema.ts" {
		t.Fatalf("top recommendation = %q, want src/lib/rules/schema.ts", plan.RecommendedFiles[0].Path)
	}
	if !containsPath(recPaths(plan.RelatedTests), "src/lib/rules/evaluate.test.ts") {
		t.Fatalf("related tests = %v, want rule test", recPaths(plan.RelatedTests))
	}
	if !containsPath(plan.Directories, "src/lib/rules/") {
		t.Fatalf("directories = %v, want src/lib/rules/", plan.Directories)
	}
	if !containsPath(plan.SearchTerms, "rule") {
		t.Fatalf("search terms = %v, want rule", plan.SearchTerms)
	}
}

func TestScorePlanMeasuresUsefulnessRisks(t *testing.T) {
	fixture := Fixture{
		Task:            "fix rule validation bug",
		ExpectedFeature: "Rules / alerts",
		ExpectedRelevantFiles: []string{
			"src/lib/rules/schema.ts",
			"src/app/api/rules/route.ts",
			"src/lib/db/repositories.ts",
			"src/lib/rules/evaluate.test.ts",
		},
	}
	plan := Plan(fixture.Task, plannerTestAnalysis())
	score := ScorePlan(fixture, plan)

	if score.PrecisionAt5 <= 0 {
		t.Fatalf("PrecisionAt5 = %.2f, want > 0", score.PrecisionAt5)
	}
	if score.RecallAt10 < 0.75 {
		t.Fatalf("RecallAt10 = %.2f, want >= 0.75", score.RecallAt10)
	}
	if !score.TopHit {
		t.Fatalf("TopHit = false, top recommendation %q was expected", score.TopRecommendation)
	}
	if !score.TestsIncluded {
		t.Fatal("TestsIncluded = false, want true")
	}
	if score.BroadSearchRisk {
		t.Fatalf("BroadSearchRisk = true, warnings: %v", score.Warnings)
	}
	if score.UnderSearchRisk {
		t.Fatalf("UnderSearchRisk = true, warnings: %v", score.Warnings)
	}
}

func TestBuiltInFixturesRouteToExpectedFiles(t *testing.T) {
	analysis := plannerTestAnalysis()
	fixtures := BuiltInFixtures()
	wantTasks := map[string]bool{
		"fix rule validation bug":         false,
		"debug worker tick issue":         false,
		"fix watchlist delete edit issue": false,
	}

	for _, fixture := range fixtures {
		if _, ok := wantTasks[fixture.Task]; !ok {
			continue
		}
		plan := Plan(fixture.Task, analysis)
		score := ScorePlan(fixture, plan)
		if score.RelevantFoundInTop10 == 0 {
			t.Fatalf("%s found no expected files in top 10; plan=%v", fixture.Name, recPaths(plan.RecommendedFiles))
		}
		if fixture.ExpectedFeature != "" && plan.MatchedFeature != fixture.ExpectedFeature {
			t.Fatalf("%s matched feature %q, want %q", fixture.Name, plan.MatchedFeature, fixture.ExpectedFeature)
		}
		wantTasks[fixture.Task] = true
	}
	for task, seen := range wantTasks {
		if !seen {
			t.Fatalf("built-in fixture missing task %q", task)
		}
	}
}

func plannerTestAnalysis() *models.Analysis {
	files := []models.FileInfo{
		{Path: ".env.example", Kind: models.FileKindConfig},
		{Path: "README.md", Kind: models.FileKindDoc},
		{Path: "package.json", Kind: models.FileKindConfig},
		{Path: "src/app/api/rules/route.ts", Kind: models.FileKindSource},
		{Path: "src/app/api/rules/validate/route.ts", Kind: models.FileKindSource},
		{Path: "src/lib/rules/schema.ts", Kind: models.FileKindSource},
		{Path: "src/lib/rules/evaluate.test.ts", Kind: models.FileKindTest},
		{Path: "src/app/api/worker/tick/route.ts", Kind: models.FileKindSource},
		{Path: "src/lib/worker/local-worker-loop.ts", Kind: models.FileKindSource},
		{Path: "src/lib/worker/local-mock-worker.ts", Kind: models.FileKindSource},
		{Path: "src/lib/worker/live-monitor.test.ts", Kind: models.FileKindTest},
		{Path: "src/app/watchlist/page.tsx", Kind: models.FileKindSource},
		{Path: "src/app/api/watchlist/route.ts", Kind: models.FileKindSource},
		{Path: "src/app/api/watchlist/[symbol]/route.ts", Kind: models.FileKindSource},
		{Path: "src/app/api/market/bars/[symbol]/route.ts", Kind: models.FileKindSource},
		{Path: "src/lib/market/provider.ts", Kind: models.FileKindSource},
		{Path: "src/lib/market/chart-bars.ts", Kind: models.FileKindSource},
		{Path: "src/lib/market/chart-bars.test.ts", Kind: models.FileKindTest},
		{Path: "src/app/api/auth/me/route.ts", Kind: models.FileKindSource},
		{Path: "src/app/api/auth/login/route.ts", Kind: models.FileKindSource},
		{Path: "src/lib/auth/session.ts", Kind: models.FileKindSource},
		{Path: "src/lib/auth/password.ts", Kind: models.FileKindSource},
		{Path: "src/lib/auth/rate-limit.ts", Kind: models.FileKindSource},
		{Path: "src/lib/db/repositories.ts", Kind: models.FileKindSource},
		{Path: "src/lib/config/env.ts", Kind: models.FileKindConfig},
	}
	return &models.Analysis{
		RepoPath:    "/tmp/stkapp",
		RepoName:    "stkapp",
		GeneratedAt: time.Now(),
		Files:       files,
		Features: models.FeatureMap{
			Features: []models.FeatureCluster{
				{
					Name:         "Rules / alerts",
					StartHere:    []string{"src/lib/rules/schema.ts", "src/app/api/rules/route.ts", "src/lib/db/repositories.ts"},
					RelatedTests: []string{"src/lib/rules/evaluate.test.ts"},
					SearchTerms:  []string{"rule", "alert", "validation", "schema"},
					AvoidFirst:   []string{"src/app/globals.css"},
					Routes:       []string{"POST /api/rules", "POST /api/rules/validate"},
					Confidence:   "high",
				},
				{
					Name:         "Worker",
					StartHere:    []string{"src/app/api/worker/tick/route.ts", "src/lib/worker/local-worker-loop.ts", "src/lib/worker/local-mock-worker.ts"},
					RelatedTests: []string{"src/lib/worker/live-monitor.test.ts"},
					SearchTerms:  []string{"worker", "tick", "monitor"},
					Routes:       []string{"POST /api/worker/tick"},
					Confidence:   "high",
				},
				{
					Name:        "Watchlist",
					StartHere:   []string{"src/app/watchlist/page.tsx", "src/app/api/watchlist/route.ts", "src/app/api/watchlist/[symbol]/route.ts", "src/lib/db/repositories.ts"},
					SearchTerms: []string{"watchlist", "delete", "edit", "symbol"},
					Routes:      []string{"GET /api/watchlist", "POST /api/watchlist", "DELETE /api/watchlist/:symbol"},
					Confidence:  "high",
				},
				{
					Name:        "Market data",
					StartHere:   []string{"src/app/api/market/bars/[symbol]/route.ts", "src/lib/market/provider.ts", "src/lib/market/chart-bars.ts"},
					SearchTerms: []string{"market", "bars", "quote", "provider"},
					Routes:      []string{"GET /api/market/bars/:symbol"},
					Confidence:  "high",
				},
				{
					Name:        "Auth",
					StartHere:   []string{"src/lib/auth/session.ts", "src/app/api/auth/me/route.ts", "src/app/api/auth/login/route.ts"},
					SearchTerms: []string{"auth", "session", "cookie", "login"},
					Routes:      []string{"GET /api/auth/me", "POST /api/auth/login"},
					Confidence:  "high",
				},
			},
			RouteChains: []models.RouteChain{
				{Route: "POST /api/rules", Files: []string{"src/app/api/rules/route.ts", "src/lib/rules/schema.ts", "src/lib/db/repositories.ts"}, Tests: []string{"src/lib/rules/evaluate.test.ts"}},
				{Route: "POST /api/rules/validate", Files: []string{"src/app/api/rules/validate/route.ts", "src/lib/rules/schema.ts"}, Tests: []string{"src/lib/rules/evaluate.test.ts"}},
				{Route: "POST /api/worker/tick", Files: []string{"src/app/api/worker/tick/route.ts", "src/lib/worker/local-worker-loop.ts", "src/lib/worker/local-mock-worker.ts"}, Tests: []string{"src/lib/worker/live-monitor.test.ts"}},
				{Route: "DELETE /api/watchlist/:symbol", Files: []string{"src/app/api/watchlist/[symbol]/route.ts", "src/lib/db/repositories.ts", "src/lib/auth/session.ts"}},
				{Route: "GET /api/market/bars/:symbol", Files: []string{"src/app/api/market/bars/[symbol]/route.ts", "src/lib/market/provider.ts", "src/lib/market/chart-bars.ts"}, Tests: []string{"src/lib/market/chart-bars.test.ts"}},
				{Route: "GET /api/auth/me", Files: []string{"src/app/api/auth/me/route.ts", "src/lib/auth/session.ts"}},
			},
		},
		Symbols: models.SymbolIndex{Files: []models.FileSymbols{
			{Path: "src/lib/rules/schema.ts", Symbols: []models.ExportedSymbol{{Name: "RuleSchema", Kind: "const"}, {Name: "validateRuleInput", Kind: "function"}}},
			{Path: "src/lib/db/repositories.ts", Symbols: []models.ExportedSymbol{{Name: "RulesRepository", Kind: "class"}, {Name: "WatchlistRepository", Kind: "class"}}},
			{Path: "src/lib/worker/local-worker-loop.ts", Symbols: []models.ExportedSymbol{{Name: "runWorkerTick", Kind: "function"}}},
		}},
		Dependencies: models.DependencyGraph{TopConnectedFiles: []models.ConnectedFileSummary{
			{Path: "src/lib/db/repositories.ts"},
			{Path: "src/lib/auth/session.ts"},
		}},
	}
}

func containsPath(items []string, want string) bool {
	for _, item := range items {
		if item == want || strings.EqualFold(item, want) {
			return true
		}
	}
	return false
}
