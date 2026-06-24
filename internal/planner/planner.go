package planner

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/wcw-wcw/stackindex/internal/models"
)

type Recommendation struct {
	Path   string   `json:"path"`
	Score  int      `json:"score"`
	Reason []string `json:"reason"`
}

type SearchPlan struct {
	Task                string           `json:"task"`
	MatchedFeature      string           `json:"matchedFeature,omitempty"`
	RecommendedFiles    []Recommendation `json:"recommendedFiles"`
	RelatedTests        []Recommendation `json:"relatedTests,omitempty"`
	SearchTerms         []string         `json:"searchTerms,omitempty"`
	Directories         []string         `json:"directories,omitempty"`
	AvoidFirst          []string         `json:"avoidFirst,omitempty"`
	Warnings            []string         `json:"warnings,omitempty"`
	AnalysisGeneratedAt string           `json:"analysisGeneratedAt,omitempty"`
}

type Fixture struct {
	Name                  string   `json:"name"`
	Task                  string   `json:"task"`
	ExpectedRelevantFiles []string `json:"expectedRelevantFiles"`
	ExpectedFeature       string   `json:"expectedFeature,omitempty"`
	ExpectedRoutes        []string `json:"expectedRoutes,omitempty"`
}

type Score struct {
	Task                  string   `json:"task"`
	PrecisionAt5          float64  `json:"precisionAt5"`
	RecallAt10            float64  `json:"recallAt10"`
	TopHit                bool     `json:"topHit"`
	TestsIncluded         bool     `json:"testsIncluded"`
	BroadSearchRisk       bool     `json:"broadSearchRisk"`
	UnderSearchRisk       bool     `json:"underSearchRisk"`
	Warnings              []string `json:"warnings,omitempty"`
	MatchedFeature        string   `json:"matchedFeature,omitempty"`
	ExpectedFeature       string   `json:"expectedFeature,omitempty"`
	TopRecommendation     string   `json:"topRecommendation,omitempty"`
	RelevantFoundInTop10  int      `json:"relevantFoundInTop10"`
	ExpectedRelevantCount int      `json:"expectedRelevantCount"`
}

func LoadAnalysis(root string) (*models.Analysis, string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, "", err
	}
	path := filepath.Join(absRoot, ".stackindex", "analysis.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, path, fmt.Errorf("no StackIndex analysis found at %s; run `stackindex analyze %s --no-tui` first", path, shellQuote(absRoot))
		}
		return nil, path, err
	}
	var analysis models.Analysis
	if err := json.Unmarshal(data, &analysis); err != nil {
		return nil, path, fmt.Errorf("could not read StackIndex analysis at %s: %w", path, err)
	}
	if stale, newest := analysisLooksStale(absRoot, &analysis); stale {
		return nil, path, fmt.Errorf("StackIndex analysis at %s may be stale; newest source file changed at %s after analysis was generated at %s. Run `stackindex analyze %s --no-tui`", path, newest.Format("2006-01-02 15:04:05"), analysis.GeneratedAt.Format("2006-01-02 15:04:05"), shellQuote(absRoot))
	}
	return &analysis, path, nil
}

func Plan(task string, analysis *models.Analysis) SearchPlan {
	task = strings.TrimSpace(task)
	tokens := taskTokens(task)
	candidates := map[string]*Recommendation{}
	tests := map[string]*Recommendation{}
	dirScores := map[string]int{}
	termScores := map[string]int{}
	avoid := map[string]bool{}

	bestFeature, featureScore := matchFeature(tokens, analysis.Features.Features)
	if bestFeature != nil {
		for _, path := range bestFeature.StartHere {
			addCandidate(candidates, path, 80+featureScore, "matches feature "+bestFeature.Name)
			dirScores[parentDir(path)] += 4
		}
		for _, path := range bestFeature.RelatedTests {
			addCandidate(tests, path, 55+featureScore, "related test for feature "+bestFeature.Name)
			addCandidate(candidates, path, 35+featureScore, "test evidence for feature "+bestFeature.Name)
		}
		for _, term := range bestFeature.SearchTerms {
			termScores[term] += 5
		}
		for _, item := range bestFeature.AvoidFirst {
			avoid[item] = true
		}
	}

	for _, chain := range analysis.Features.RouteChains {
		score := routeChainScore(tokens, chain)
		if score == 0 {
			continue
		}
		for i, path := range chain.Files {
			addCandidate(candidates, path, score+routeChainFileBonus(path)-i, "route chain for "+chain.Route)
			dirScores[parentDir(path)] += 3
		}
		for _, path := range chain.Tests {
			addCandidate(tests, path, score+25, "test near route chain "+chain.Route)
			addCandidate(candidates, path, score+10, "test near route chain "+chain.Route)
		}
		for _, term := range routeTerms(chain.Route) {
			termScores[term] += 3
		}
	}

	for _, file := range analysis.Symbols.Files {
		score := symbolsScore(tokens, file)
		if score == 0 {
			continue
		}
		addCandidate(candidates, file.Path, score+symbolFileBonus(file.Path), "exports matching task terms")
		dirScores[parentDir(file.Path)] += 2
	}

	for _, file := range analysis.Dependencies.TopConnectedFiles {
		score := pathTokenScore(tokens, file.Path)
		if score > 0 {
			addCandidate(candidates, file.Path, score+12, "dependency hub related to task terms")
		}
	}

	addSpecializedTaskCandidates(tokens, analysis, candidates, tests, dirScores, termScores, avoid)

	recommended := sortedRecommendations(candidates)
	relatedTests := sortedRecommendations(tests)
	searchTerms := sortedScoredKeys(termScores, 12)
	directories := sortedScoredKeys(dirScores, 8)
	avoidFirst := sortedBoolKeys(avoid, 8)
	warnings := planWarnings(tokens, recommended, relatedTests)
	matched := ""
	if bestFeature != nil {
		matched = bestFeature.Name
	}
	return SearchPlan{
		Task:                task,
		MatchedFeature:      matched,
		RecommendedFiles:    capRecommendations(recommended, 15),
		RelatedTests:        capRecommendations(relatedTests, 8),
		SearchTerms:         searchTerms,
		Directories:         directories,
		AvoidFirst:          avoidFirst,
		Warnings:            warnings,
		AnalysisGeneratedAt: analysis.GeneratedAt.Format("2006-01-02 15:04:05"),
	}
}

func ScorePlan(fixture Fixture, plan SearchPlan) Score {
	expected := normalizeSet(fixture.ExpectedRelevantFiles)
	top5 := recommendationPaths(plan.RecommendedFiles, 5)
	top10 := recommendationPaths(plan.RecommendedFiles, 10)
	precisionHits := countHits(top5, expected)
	recallHits := countHits(top10, expected)
	precision := 0.0
	if len(top5) > 0 {
		precision = float64(precisionHits) / float64(len(top5))
	}
	recall := 0.0
	if len(expected) > 0 {
		recall = float64(recallHits) / float64(len(expected))
	}
	topHit := len(top5) > 0 && expected[top5[0]]
	testsExpected := fixtureExpectsTests(fixture)
	testsIncluded := !testsExpected || planIncludesTest(plan)
	broad := broadSearchRisk(top5)
	under := underSearchRisk(fixture, top10, plan)
	var warnings []string
	if broad {
		warnings = append(warnings, "broad-search risk")
	}
	if under {
		warnings = append(warnings, "under-search risk")
	}
	if testsExpected && !testsIncluded {
		warnings = append(warnings, "expected tests were not recommended")
	}
	top := ""
	if len(top5) > 0 {
		top = top5[0]
	}
	return Score{
		Task:                  fixture.Task,
		PrecisionAt5:          round2(precision),
		RecallAt10:            round2(recall),
		TopHit:                topHit,
		TestsIncluded:         testsIncluded,
		BroadSearchRisk:       broad,
		UnderSearchRisk:       under,
		Warnings:              warnings,
		MatchedFeature:        plan.MatchedFeature,
		ExpectedFeature:       fixture.ExpectedFeature,
		TopRecommendation:     top,
		RelevantFoundInTop10:  recallHits,
		ExpectedRelevantCount: len(expected),
	}
}

func BuiltInFixtures() []Fixture {
	return []Fixture{
		{
			Name:            "rule validation bug",
			Task:            "fix rule validation bug",
			ExpectedFeature: "Rules / alerts",
			ExpectedRoutes:  []string{"POST /api/rules", "POST /api/rules/validate"},
			ExpectedRelevantFiles: []string{
				"src/app/api/rules/route.ts",
				"src/app/api/rules/validate/route.ts",
				"src/lib/rules/schema.ts",
				"src/lib/db/repositories.ts",
				"src/lib/rules/evaluate.test.ts",
			},
		},
		{
			Name:            "worker tick/debug issue",
			Task:            "debug worker tick issue",
			ExpectedFeature: "Worker",
			ExpectedRoutes:  []string{"POST /api/worker/tick"},
			ExpectedRelevantFiles: []string{
				"src/app/api/worker/tick/route.ts",
				"src/lib/worker/local-mock-worker.ts",
				"src/lib/worker/local-worker-loop.ts",
				"src/lib/worker/live-monitor.test.ts",
				"src/lib/worker/status.test.ts",
			},
		},
		{
			Name:            "watchlist delete/edit issue",
			Task:            "fix watchlist delete edit issue",
			ExpectedFeature: "Watchlist",
			ExpectedRoutes:  []string{"GET /api/watchlist", "POST /api/watchlist"},
			ExpectedRelevantFiles: []string{
				"src/app/watchlist/page.tsx",
				"src/app/api/watchlist/route.ts",
				"src/app/api/watchlist/[symbol]/route.ts",
				"src/lib/db/repositories.ts",
				"src/lib/auth/session.ts",
			},
		},
		{
			Name:            "market bars loading issue",
			Task:            "fix market bars loading issue",
			ExpectedFeature: "Market data",
			ExpectedRoutes:  []string{"GET /api/market/bars/:symbol"},
			ExpectedRelevantFiles: []string{
				"src/app/api/market/bars/[symbol]/route.ts",
				"src/lib/market/provider.ts",
				"src/lib/market/chart-bars.ts",
				"src/lib/market/chart-bars.test.ts",
				"src/lib/config/env.ts",
			},
		},
		{
			Name:            "auth/session issue",
			Task:            "fix auth session cookie issue",
			ExpectedFeature: "Auth",
			ExpectedRoutes:  []string{"GET /api/auth/me", "POST /api/auth/login"},
			ExpectedRelevantFiles: []string{
				"src/app/api/auth/me/route.ts",
				"src/app/api/auth/login/route.ts",
				"src/lib/auth/session.ts",
				"src/lib/auth/password.ts",
				"src/lib/auth/rate-limit.ts",
			},
		},
		{
			Name:            "env/deployment issue",
			Task:            "fix env deployment config issue",
			ExpectedFeature: "",
			ExpectedRelevantFiles: []string{
				".env.example",
				"src/lib/config/env.ts",
				"package.json",
				"README.md",
			},
		},
	}
}

func LoadFixtures(root string) []Fixture {
	absRoot, err := filepath.Abs(root)
	if err == nil {
		root = absRoot
	}
	path := filepath.Join(root, ".stackindex", "eval-fixtures.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return BuiltInFixtures()
	}
	var fixtures []Fixture
	if err := json.Unmarshal(data, &fixtures); err != nil || len(fixtures) == 0 {
		return BuiltInFixtures()
	}
	return fixtures
}

func FormatPlan(plan SearchPlan) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Task: %s\n", plan.Task)
	if plan.MatchedFeature != "" {
		fmt.Fprintf(&b, "Matched feature: %s\n", plan.MatchedFeature)
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Recommended first files:")
	for _, rec := range plan.RecommendedFiles {
		fmt.Fprintf(&b, "- `%s` (score %d): %s\n", rec.Path, rec.Score, strings.Join(rec.Reason, "; "))
	}
	fmt.Fprintln(&b)
	writeStringList(&b, "Related tests", recPaths(plan.RelatedTests))
	writeStringList(&b, "Search terms", plan.SearchTerms)
	writeStringList(&b, "Directories to inspect", plan.Directories)
	writeStringList(&b, "Avoid first", plan.AvoidFirst)
	if len(plan.Warnings) > 0 {
		writeStringList(&b, "Warnings", plan.Warnings)
	}
	return b.String()
}

func FormatEval(scores []Score) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%-30s  %-9s  %-8s  %-7s  %s\n", "Task", "P@5", "R@10", "TopHit", "Warnings")
	fmt.Fprintf(&b, "%-30s  %-9s  %-8s  %-7s  %s\n", strings.Repeat("-", 30), strings.Repeat("-", 9), strings.Repeat("-", 8), strings.Repeat("-", 7), strings.Repeat("-", 20))
	for _, score := range scores {
		topHit := "no"
		if score.TopHit {
			topHit = "yes"
		}
		warnings := "none"
		if len(score.Warnings) > 0 {
			warnings = strings.Join(score.Warnings, ", ")
		}
		fmt.Fprintf(&b, "%-30s  %-9.2f  %-8.2f  %-7s  %s\n", capText(score.Task, 30), score.PrecisionAt5, score.RecallAt10, topHit, warnings)
	}
	return b.String()
}

func matchFeature(tokens map[string]bool, features []models.FeatureCluster) (*models.FeatureCluster, int) {
	var best *models.FeatureCluster
	bestScore := 0
	for i := range features {
		feature := &features[i]
		score := 0
		for _, token := range taskTokenList(feature.Name + " " + strings.Join(feature.SearchTerms, " ") + " " + strings.Join(feature.Routes, " ") + " " + strings.Join(feature.StartHere, " ")) {
			if tokens[token] {
				score += 3
			}
		}
		for token := range tokens {
			if featureNameMatchesToken(feature.Name, token) {
				score += 8
			}
		}
		if score > bestScore {
			best = feature
			bestScore = score
		}
	}
	if bestScore < 3 {
		return nil, 0
	}
	return best, bestScore
}

func featureNameMatchesToken(name, token string) bool {
	name = strings.ToLower(name)
	switch token {
	case "rule", "rules", "alert", "alerts":
		return strings.Contains(name, "rule") || strings.Contains(name, "alert")
	case "watchlist":
		return strings.Contains(name, "watchlist")
	case "worker", "tick":
		return strings.Contains(name, "worker")
	case "market", "bars", "quote", "quotes":
		return strings.Contains(name, "market")
	case "auth", "session", "cookie", "login":
		return strings.Contains(name, "auth")
	default:
		return strings.Contains(name, token)
	}
}

func routeChainScore(tokens map[string]bool, chain models.RouteChain) int {
	routeScore := 0
	routeHits := 0
	for _, token := range taskTokenList(chain.Route) {
		if tokens[token] {
			routeScore += 14
			routeHits++
		}
	}
	fileScore := 0
	fileHits := 0
	for _, token := range taskTokenList(strings.Join(chain.Files, " ") + " " + strings.Join(chain.Tests, " ")) {
		if tokens[token] {
			fileScore += 3
			fileHits++
		}
	}
	if routeHits < 2 && fileHits < 2 {
		return 0
	}
	if routeScore == 0 && fileScore < 12 {
		return 0
	}
	return routeScore + fileScore
}

func symbolsScore(tokens map[string]bool, file models.FileSymbols) int {
	score := pathTokenScore(tokens, file.Path)
	for _, symbol := range file.Symbols {
		for _, token := range taskTokenList(symbol.Name) {
			if tokens[token] {
				score += 8
			}
		}
	}
	return score
}

func pathTokenScore(tokens map[string]bool, path string) int {
	score := 0
	for _, token := range taskTokenList(path) {
		if tokens[token] {
			score += 5
		}
	}
	return score
}

func addSpecializedTaskCandidates(tokens map[string]bool, analysis *models.Analysis, candidates, tests map[string]*Recommendation, dirScores, termScores map[string]int, avoid map[string]bool) {
	if tokens["rule"] || tokens["rules"] || tokens["alert"] || tokens["alerts"] || tokens["validation"] || tokens["schema"] {
		for _, file := range analysis.Files {
			lower := strings.ToLower(file.Path)
			if strings.Contains(lower, "repositor") || strings.Contains(lower, "/db/") {
				addCandidate(candidates, file.Path, 50, "storage often participates in rule validation behavior")
				dirScores[parentDir(file.Path)] += 3
			}
		}
	}
	if tokens["env"] || tokens["environment"] || tokens["deploy"] || tokens["deployment"] || tokens["config"] {
		for _, file := range analysis.Files {
			lower := strings.ToLower(file.Path)
			switch {
			case file.Path == ".env.example":
				addCandidate(candidates, file.Path, 90, "environment template")
			case strings.Contains(lower, "/config/") || strings.Contains(lower, "env."):
				addCandidate(candidates, file.Path, 80, "configuration/env file")
			case file.Path == "package.json":
				addCandidate(candidates, file.Path, 65, "scripts and deployment commands")
			case strings.Contains(lower, "deploy") || strings.Contains(lower, "readme"):
				addCandidate(candidates, file.Path, 50, "deployment/setup documentation")
			}
		}
		termScores["env"] += 5
		termScores["deployment"] += 4
		avoid["unrelated feature files"] = true
	}
	for _, rec := range candidates {
		dirScores[parentDir(rec.Path)]++
	}
}

func addCandidate(items map[string]*Recommendation, path string, score int, reason string) {
	path = strings.TrimSpace(path)
	if path == "" {
		return
	}
	if existing, ok := items[path]; ok {
		existing.Score += score
		if reason != "" && !containsString(existing.Reason, reason) {
			existing.Reason = append(existing.Reason, reason)
		}
		return
	}
	items[path] = &Recommendation{Path: path, Score: score, Reason: []string{reason}}
}

func sortedRecommendations(items map[string]*Recommendation) []Recommendation {
	out := make([]Recommendation, 0, len(items))
	for _, item := range items {
		sort.Strings(item.Reason)
		out = append(out, *item)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			if plannerPathRank(out[i].Path) == plannerPathRank(out[j].Path) {
				return out[i].Path < out[j].Path
			}
			return plannerPathRank(out[i].Path) > plannerPathRank(out[j].Path)
		}
		return out[i].Score > out[j].Score
	})
	return out
}

func plannerPathRank(path string) int {
	lower := strings.ToLower(path)
	switch {
	case strings.Contains(lower, "schema") || strings.Contains(lower, "validat"):
		return 9
	case strings.Contains(lower, "repositor") || strings.Contains(lower, "/db/"):
		return 8
	case strings.Contains(lower, "auth") || strings.Contains(lower, "session"):
		return 7
	case strings.Contains(lower, "service") || strings.Contains(lower, "provider"):
		return 6
	case strings.Contains(lower, "/api/"):
		return 5
	case strings.Contains(lower, ".test.") || strings.HasSuffix(lower, "_test.go"):
		return 4
	default:
		return 1
	}
}

func routeChainFileBonus(path string) int {
	lower := strings.ToLower(path)
	if strings.Contains(lower, "schema") || strings.Contains(lower, "validat") {
		return 34
	}
	if strings.Contains(lower, "repositor") || strings.Contains(lower, "/db/") {
		return 32
	}
	return plannerPathRank(path) * 2
}

func symbolFileBonus(path string) int {
	return plannerPathRank(path)
}

func taskTokens(task string) map[string]bool {
	out := map[string]bool{}
	for _, token := range taskTokenList(task) {
		out[token] = true
		for _, synonym := range synonyms(token) {
			out[synonym] = true
		}
	}
	return out
}

func taskTokenList(text string) []string {
	text = strings.ToLower(text)
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	})
	seen := map[string]bool{}
	var out []string
	for _, field := range fields {
		field = normalizeToken(field)
		if field == "" || seen[field] {
			continue
		}
		seen[field] = true
		out = append(out, field)
	}
	return out
}

func normalizeToken(token string) string {
	token = strings.TrimSpace(strings.ToLower(token))
	if len(token) < 2 || plannerStop[token] {
		return ""
	}
	if strings.HasSuffix(token, "ies") && len(token) > 4 {
		token = strings.TrimSuffix(token, "ies") + "y"
	} else if strings.HasSuffix(token, "s") && len(token) > 4 {
		token = strings.TrimSuffix(token, "s")
	}
	return token
}

var plannerStop = map[string]bool{
	"fix": true, "debug": true, "issue": true, "bug": true, "the": true, "and": true, "for": true, "with": true, "from": true, "into": true,
	"api": true, "route": true, "get": true, "post": true, "put": true, "patch": true, "delete": true, "id": true,
}

func synonyms(token string) []string {
	switch token {
	case "rule":
		return []string{"rules", "alert", "alerts", "validation", "schema"}
	case "validation", "validate":
		return []string{"schema", "rule", "rules"}
	case "worker", "tick":
		return []string{"worker", "tick", "job"}
	case "watchlist":
		return []string{"watchlist", "symbol"}
	case "market", "bars":
		return []string{"market", "bars", "quote", "provider"}
	case "auth", "session", "cookie":
		return []string{"auth", "session", "cookie", "login"}
	case "env", "environment", "deployment", "deploy":
		return []string{"env", "config", "deployment"}
	default:
		return nil
	}
}

func routeTerms(route string) []string {
	return taskTokenList(route)
}

func parentDir(path string) string {
	dir := filepath.ToSlash(filepath.Dir(path))
	if dir == "." {
		return ""
	}
	return dir + "/"
}

func sortedScoredKeys(scores map[string]int, limit int) []string {
	type item struct {
		key   string
		score int
	}
	var items []item
	for key, score := range scores {
		if key != "" && score > 0 {
			items = append(items, item{key: key, score: score})
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].score == items[j].score {
			return items[i].key < items[j].key
		}
		return items[i].score > items[j].score
	})
	var out []string
	for _, item := range items {
		out = append(out, item.key)
		if limit > 0 && len(out) == limit {
			break
		}
	}
	return out
}

func sortedBoolKeys(items map[string]bool, limit int) []string {
	var out []string
	for item := range items {
		out = append(out, item)
	}
	sort.Strings(out)
	if limit > 0 && len(out) > limit {
		return out[:limit]
	}
	return out
}

func capRecommendations(items []Recommendation, limit int) []Recommendation {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	return items[:limit]
}

func planWarnings(tokens map[string]bool, recs []Recommendation, tests []Recommendation) []string {
	var warnings []string
	if len(recs) == 0 {
		warnings = append(warnings, "no specific files matched; run or refresh StackIndex analysis before broad searching")
	}
	if len(recs) > 0 && broadSearchRisk(recommendationPaths(recs, 5)) {
		warnings = append(warnings, "top recommendations are broad; inspect matched feature and route chains before whole-repo search")
	}
	if (tokens["validation"] || tokens["schema"] || tokens["rule"]) && !pathsContain(recommendationPaths(recs, 10), "schema") {
		warnings = append(warnings, "schema/validation file not found in top recommendations")
	}
	if (tokens["worker"] || tokens["tick"]) && len(tests) == 0 {
		warnings = append(warnings, "worker task has no related tests in recommendations")
	}
	return warnings
}

func recommendationPaths(recs []Recommendation, limit int) []string {
	if limit <= 0 || len(recs) < limit {
		limit = len(recs)
	}
	out := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		out = append(out, recs[i].Path)
	}
	return out
}

func countHits(paths []string, expected map[string]bool) int {
	count := 0
	for _, path := range paths {
		if expected[path] {
			count++
		}
	}
	return count
}

func normalizeSet(paths []string) map[string]bool {
	out := map[string]bool{}
	for _, path := range paths {
		out[filepath.ToSlash(strings.TrimSpace(path))] = true
	}
	return out
}

func fixtureExpectsTests(fixture Fixture) bool {
	for _, path := range fixture.ExpectedRelevantFiles {
		if isTestPath(path) {
			return true
		}
	}
	return false
}

func planIncludesTest(plan SearchPlan) bool {
	for _, rec := range plan.RecommendedFiles {
		if isTestPath(rec.Path) {
			return true
		}
	}
	for _, rec := range plan.RelatedTests {
		if isTestPath(rec.Path) {
			return true
		}
	}
	return false
}

func isTestPath(path string) bool {
	lower := strings.ToLower(path)
	return strings.Contains(lower, ".test.") || strings.Contains(lower, ".spec.") || strings.HasSuffix(lower, "_test.go")
}

func broadSearchRisk(paths []string) bool {
	if len(paths) == 0 {
		return true
	}
	generic := 0
	for _, path := range paths {
		base := filepath.Base(path)
		lower := strings.ToLower(path)
		if base == "README.md" || base == "package.json" || base == "tsconfig.json" || strings.Contains(lower, "globals.css") || strings.Contains(lower, "layout.") {
			generic++
		}
	}
	return generic >= 2
}

func underSearchRisk(fixture Fixture, top10 []string, plan SearchPlan) bool {
	expected := normalizeSet(fixture.ExpectedRelevantFiles)
	if len(expected) == 0 {
		return false
	}
	if countHits(top10, expected) == 0 {
		return true
	}
	needsSchema := expectedPathContains(expected, "schema")
	needsStorage := expectedPathContains(expected, "repositor") || expectedPathContains(expected, "/db/")
	needsTests := expectedHasTest(expected)
	if needsSchema && !pathsContain(top10, "schema") {
		return true
	}
	if needsStorage && !pathsContain(top10, "repositor") && !pathsContain(top10, "/db/") {
		return true
	}
	if needsTests && !planIncludesTest(plan) {
		return true
	}
	return false
}

func expectedPathContains(paths map[string]bool, needle string) bool {
	for path := range paths {
		if strings.Contains(strings.ToLower(path), needle) {
			return true
		}
	}
	return false
}

func expectedHasTest(paths map[string]bool) bool {
	for path := range paths {
		if isTestPath(path) {
			return true
		}
	}
	return false
}

func pathsContain(paths []string, needle string) bool {
	for _, path := range paths {
		if strings.Contains(strings.ToLower(path), strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func recPaths(recs []Recommendation) []string {
	out := make([]string, 0, len(recs))
	for _, rec := range recs {
		out = append(out, rec.Path)
	}
	return out
}

func writeStringList(b *strings.Builder, label string, values []string) {
	fmt.Fprintf(b, "%s:\n", label)
	if len(values) == 0 {
		fmt.Fprintln(b, "- none")
		return
	}
	for _, value := range values {
		fmt.Fprintf(b, "- `%s`\n", value)
	}
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func capText(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	if limit <= 1 {
		return value[:limit]
	}
	return value[:limit-1] + "."
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}

func analysisLooksStale(root string, analysis *models.Analysis) (bool, time.Time) {
	if analysis == nil || analysis.GeneratedAt.IsZero() {
		return false, time.Time{}
	}
	newest := time.Time{}
	for _, file := range analysis.Files {
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(file.Path)))
		if err != nil {
			continue
		}
		if info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	if newest.IsZero() {
		return false, newest
	}
	return newest.After(analysis.GeneratedAt.Add(2 * time.Second)), newest
}

func shellQuote(path string) string {
	if path == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(path, "'", "'\\''") + "'"
}
