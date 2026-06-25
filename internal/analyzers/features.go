package analyzers

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wcw-wcw/stackindex/internal/models"
)

const (
	featureFileLimit    = 8
	featureTestLimit    = 5
	routeChainFileLimit = 5
)

var featureStopTerms = map[string]bool{
	"api": true, "app": true, "src": true, "lib": true, "page": true, "route": true, "layout": true, "components": true,
	"component": true, "config": true, "utils": true, "util": true, "test": true, "tests": true, "spec": true, "types": true,
	"type": true, "index": true, "new": true, "edit": true, "settings": true, "setting": true, "main": true, "service": true, "services": true,
	"client": true, "server": true,
}

var genericRouteTerms = map[string]bool{
	"id": true, "slug": true, "symbol": true, "name": true, "key": true, "type": true, "all": true, "count": true, "read": true,
	"follow": true, "following": true, "follower": true, "followers": true, "game": true, "game_id": true, "main": true, "service": true,
	"move": true, "moves": true, "human": true,
}

var lowValueFeatureTerms = map[string]bool{
	"mjs": true, "js": true, "jsx": true, "ts": true, "tsx": true, "json": true,
	"script": true, "scripts": true, "gen": true, "generated": true, "schema": true, "schemas": true, "config": true,
	"start": true, "stop": true, "status": true, "verify": true, "validate": true, "setting": true, "settings": true,
	"all": true, "count": true, "read": true, "follow": true, "following": true, "follower": true, "followers": true, "service": true, "services": true, "main": true,
	"move": true, "moves": true, "human": true, "client": true, "server": true,
}

type featureWork struct {
	term   string
	paths  map[string]models.FileInfo
	routes map[string]bool
	tests  map[string]bool
	score  int
}

func AnalyzeFeatureMap(files []models.FileInfo, routes []models.RouteInfo, graph models.DependencyGraph) models.FeatureMap {
	features, quality := buildFeatureClusters(files, routes)
	return models.FeatureMap{
		Features:    features,
		RouteChains: buildRouteChains(routes, graph, files),
		Quality:     quality,
	}
}

func buildFeatureClusters(files []models.FileInfo, routes []models.RouteInfo) ([]models.FeatureCluster, models.FeatureMapQuality) {
	workByTerm := map[string]*featureWork{}
	fileByPath := map[string]models.FileInfo{}
	quality := models.FeatureMapQuality{Confidence: "medium"}
	for _, file := range files {
		fileByPath[file.Path] = file
		generated, generic := featureSuppressionHintsForPath(file.Path)
		if generated {
			quality.CandidateCount++
			quality.SuppressedCount++
			quality.GeneratedCount++
		}
		if generic {
			quality.CandidateCount++
			quality.SuppressedCount++
			quality.GenericTermCount++
		}
		for _, term := range featureTermsForPath(file.Path) {
			work := ensureFeatureWork(workByTerm, term)
			work.paths[file.Path] = file
			work.score += scoreFeatureFile(file)
			if file.Kind == models.FileKindTest {
				work.tests[file.Path] = true
			}
		}
	}
	for _, route := range routes {
		for _, term := range featureTermsForRoute(route) {
			work := ensureFeatureWork(workByTerm, term)
			work.routes[route.Method+" "+route.Path] = true
			if file, ok := fileByPath[route.SourceFile]; ok {
				work.paths[route.SourceFile] = file
			}
			work.score += 4
		}
	}

	var works []*featureWork
	for _, work := range workByTerm {
		quality.CandidateCount++
		if work.term == "symbol" && hasSpecificSymbolFeature(workByTerm) {
			quality.SuppressedCount++
			quality.GenericTermCount++
			continue
		}
		if isLowValueFeatureWork(work) {
			quality.SuppressedCount++
			quality.GenericTermCount++
			continue
		}
		if featureWorkGeneratedOnly(work) {
			quality.SuppressedCount++
			quality.GeneratedCount++
			continue
		}
		boost := featureTermBoost(work.term)
		work.score += boost
		if (len(work.paths)+len(work.routes) < 2 && boost < 25) || work.score < 4 {
			continue
		}
		works = append(works, work)
	}
	sort.SliceStable(works, func(i, j int) bool {
		if works[i].score == works[j].score {
			return works[i].term < works[j].term
		}
		return works[i].score > works[j].score
	})

	var out []models.FeatureCluster
	for _, work := range works {
		cluster := models.FeatureCluster{
			Name:         featureDisplayName(work.term),
			StartHere:    featureStartHere(work),
			RelatedTests: sortedBoolKeys(work.tests, 0),
			SearchTerms:  featureSearchTerms(work.term),
			AvoidFirst:   featureAvoidFirst(),
			Routes:       sortedBoolKeys(work.routes, 0),
			Confidence:   featureConfidence(work),
		}
		if len(cluster.StartHere) == 0 {
			continue
		}
		out = append(out, cluster)
	}
	quality.UsefulCount = len(out)
	if len(out) < 2 {
		quality.Confidence = "low"
		quality.Reason = "Feature Map has fewer than 2 useful compact features; use Agent Search Guide and Key Files first."
	} else if len(out) < 4 && quality.CandidateCount > 0 && quality.SuppressedCount*2 >= quality.CandidateCount {
		quality.Confidence = "low"
		quality.Reason = "Most candidate features were generated or generic tooling terms; use Agent Search Guide and Key Files first."
	} else {
		quality.Confidence = "high"
	}
	return out, quality
}

func featureSuppressionHintsForPath(path string) (generated bool, generic bool) {
	lower := strings.ToLower(filepath.ToSlash(path))
	generated = isGeneratedFeaturePath(lower)
	parts := strings.FieldsFunc(lower, func(r rune) bool {
		return r == '/' || r == '-' || r == '_' || r == '.' || r == '[' || r == ']'
	})
	for _, part := range parts {
		part = strings.TrimSpace(strings.ToLower(part))
		if lowValueFeatureTerms[part] {
			generic = true
			break
		}
	}
	return generated, generic
}

func featureTermBoost(term string) int {
	switch term {
	case "watchlist":
		return 30
	case "post", "user", "profile", "search", "hashtag", "game", "ai-move", "backend-service", "frontend-game-ui":
		return 25
	case "combat", "ability", "match-flow", "movement", "hud", "viewmodel", "effect", "inventory", "lobby", "arena", "training-dummy", "remote", "weapon-definition":
		return 25
	case "rule", "alert", "market", "notification", "worker", "auth":
		return 20
	case "symbol-level":
		return 15
	default:
		return 0
	}
}

func hasSpecificSymbolFeature(workByTerm map[string]*featureWork) bool {
	for _, term := range []string{"symbol-level", "watchlist", "market"} {
		if work, ok := workByTerm[term]; ok && len(work.paths)+len(work.routes) > 0 {
			return true
		}
	}
	return false
}

func ensureFeatureWork(items map[string]*featureWork, term string) *featureWork {
	if work, ok := items[term]; ok {
		return work
	}
	work := &featureWork{term: term, paths: map[string]models.FileInfo{}, routes: map[string]bool{}, tests: map[string]bool{}}
	items[term] = work
	return work
}

func featureTermsForPath(path string) []string {
	lower := strings.ToLower(filepath.ToSlash(path))
	if isFeatureNoisePath(lower) {
		return nil
	}
	var terms []string
	terms = append(terms, domainFeatureTermsForPath(lower)...)
	if strings.Contains(lower, "symbol-level") {
		terms = append(terms, "symbol-level")
	}
	parts := strings.FieldsFunc(lower, func(r rune) bool {
		return r == '/' || r == '-' || r == '_' || r == '.' || r == '[' || r == ']' || r == '{' || r == '}'
	})
	for i, part := range parts {
		part = normalizeFeatureTerm(part)
		if part == "" || featureStopTerms[part] {
			continue
		}
		if lowValueFeatureTerms[part] && !hasStrongDomainInPath(lower) {
			continue
		}
		if genericRouteTerms[part] && strings.Contains(lower, "symbol-level") {
			continue
		}
		if i > 0 && (parts[i-1] == "app" || parts[i-1] == "api" || parts[i-1] == "lib" || parts[i-1] == "scripts") {
			terms = append(terms, part)
			continue
		}
		if strings.Contains(lower, "/"+part+"/") || strings.Contains(lower, part+".test.") || strings.Contains(lower, part+".spec.") {
			terms = append(terms, part)
		}
	}
	return uniqueFeatureTerms(terms)
}

func isFeatureNoisePath(path string) bool {
	base := filepath.Base(path)
	if strings.HasSuffix(base, ".md5") || strings.Contains(base, "lock") {
		return true
	}
	if isGeneratedFeaturePath(path) {
		return true
	}
	switch strings.ToLower(filepath.Ext(base)) {
	case ".css", ".scss", ".sass", ".less", ".svg", ".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico":
		return true
	default:
		return false
	}
}

func featureTermsForRoute(route models.RouteInfo) []string {
	var terms []string
	pathTerms := featureTermsForPath(route.SourceFile)
	terms = append(terms, routeFeatureTerms(route.Path)...)
	for _, part := range strings.FieldsFunc(strings.ToLower(route.Path), func(r rune) bool {
		return r == '/' || r == '-' || r == '_' || r == ':' || r == '[' || r == ']' || r == '{' || r == '}'
	}) {
		part = normalizeFeatureTerm(part)
		if part == "" || featureStopTerms[part] {
			continue
		}
		if genericRouteTerms[part] && !hasRouteTermDomainEvidence(part, route.SourceFile, pathTerms) {
			continue
		}
		terms = append(terms, part)
	}
	terms = append(terms, pathTerms...)
	return uniqueFeatureTerms(terms)
}

func hasRouteTermDomainEvidence(term, sourcePath string, pathTerms []string) bool {
	for _, pathTerm := range pathTerms {
		if pathTerm == term {
			return true
		}
	}
	lower := strings.ToLower(filepath.ToSlash(sourcePath))
	return strings.Contains(lower, term+"-") || strings.Contains(lower, "-"+term) || strings.Contains(lower, term+"_") || strings.Contains(lower, "_"+term)
}

func normalizeFeatureTerm(term string) string {
	term = strings.TrimSpace(strings.ToLower(term))
	term = strings.Trim(term, "{}:")
	term = strings.TrimSuffix(term, "ies") + map[bool]string{true: "y", false: ""}[strings.HasSuffix(term, "ies")]
	if strings.HasSuffix(term, "s") && len(term) > 4 {
		term = strings.TrimSuffix(term, "s")
	}
	if len(term) < 3 || lowValueFeatureTerms[term] {
		return ""
	}
	return term
}

func isGeneratedFeaturePath(path string) bool {
	lower := strings.ToLower(filepath.ToSlash(path))
	return strings.HasPrefix(lower, "src-tauri/gen/") ||
		strings.HasPrefix(lower, "generated/") ||
		strings.HasPrefix(lower, "gen/") ||
		strings.Contains(lower, "/generated/") ||
		strings.Contains(lower, "/gen/") ||
		strings.Contains(lower, "/schemas/generated/") ||
		strings.Contains(lower, "/schema/generated/") ||
		strings.Contains(lower, "/json-schema/") ||
		strings.Contains(lower, "/json-schemas/") ||
		strings.Contains(lower, "/schema/") && strings.Contains(lower, "generated")
}

func domainFeatureTermsForPath(path string) []string {
	var terms []string
	rules := []struct {
		term  string
		parts []string
		luau  bool
	}{
		{"auth", []string{"auth", "login", "logout", "session", "sessions"}, false},
		{"post", []string{"post", "posts", "repost", "reposts", "timeline"}, false},
		{"user", []string{"user", "users", "account", "accounts"}, false},
		{"profile", []string{"profile", "profiles"}, false},
		{"notification", []string{"notification", "notifications"}, false},
		{"search", []string{"search"}, false},
		{"hashtag", []string{"hashtag", "hashtags"}, false},
		{"game", []string{"game", "games", "match", "matches"}, false},
		{"ai-move", []string{"ai-move", "ai-moves"}, false},
		{"backend-service", []string{"service", "services"}, false},
		{"frontend-game-ui", []string{"board", "canvas", "game-ui", "gameui"}, false},
		{"combat", []string{"combat", "attack", "damage", "hitbox", "parry", "block"}, true},
		{"ability", []string{"ability", "abilities", "skill", "skills"}, true},
		{"match-flow", []string{"matchflow", "match-flow", "round", "rounds", "match", "matches"}, true},
		{"movement", []string{"movement", "dash", "sprint", "jump", "motor"}, true},
		{"hud", []string{"hud", "healthbar", "crosshair", "ui"}, true},
		{"viewmodel", []string{"viewmodel", "view-model", "camera", "arms"}, true},
		{"effect", []string{"effect", "effects", "vfx", "particle", "particles"}, true},
		{"inventory", []string{"inventory", "loadout", "backpack"}, true},
		{"lobby", []string{"lobby", "queue", "queues"}, true},
		{"arena", []string{"arena", "arenas", "map", "maps"}, true},
		{"training-dummy", []string{"training", "dummy", "dummies"}, true},
		{"remote", []string{"remote", "remotes", "remoteevent", "remotefunction"}, true},
		{"weapon-definition", []string{"weapon", "weapons", "sword", "blade", "definitions", "definition"}, true},
	}
	for _, rule := range rules {
		if rule.luau && !isLuauPath(path) {
			continue
		}
		for _, part := range rule.parts {
			if domainPathPartMatches(path, part, rule.luau) {
				terms = append(terms, rule.term)
				break
			}
		}
	}
	if strings.Contains(path, "src/server/services/") || strings.Contains(path, "/services/") {
		terms = append(terms, "backend-service")
	}
	if strings.Contains(path, "src/client/controllers/") {
		terms = append(terms, "frontend-game-ui")
	}
	return terms
}

func domainPathPartMatches(path, part string, allowShortSubstring bool) bool {
	if pathContainsFeatureToken(path, part) {
		return true
	}
	return (len(part) >= 5 || allowShortSubstring && len(part) >= 3) && strings.Contains(path, part)
}

func isLuauPath(path string) bool {
	return strings.HasSuffix(strings.ToLower(filepath.ToSlash(path)), ".luau")
}

func routeFeatureTerms(path string) []string {
	lower := strings.ToLower(path)
	var terms []string
	for _, rule := range []struct {
		term  string
		parts []string
	}{
		{"auth", []string{"auth", "login", "logout", "session"}},
		{"post", []string{"posts", "post", "reposts", "timeline"}},
		{"user", []string{"users", "user"}},
		{"profile", []string{"profiles", "profile"}},
		{"notification", []string{"notifications", "notification"}},
		{"search", []string{"search"}},
		{"hashtag", []string{"hashtags", "hashtag"}},
		{"game", []string{"games", "game"}},
		{"ai-move", []string{"ai", "moves/human", "moves/ai", "move"}},
	} {
		for _, part := range rule.parts {
			if pathContainsFeatureToken(lower, part) || strings.Contains(part, "/") && strings.Contains(lower, part) {
				terms = append(terms, rule.term)
				break
			}
		}
	}
	return terms
}

func pathContainsFeatureToken(path, token string) bool {
	path = strings.ToLower(filepath.ToSlash(path))
	token = strings.ToLower(token)
	if strings.Contains(token, "/") {
		return strings.Contains(path, token)
	}
	parts := strings.FieldsFunc(path, func(r rune) bool {
		return r == '/' || r == '-' || r == '_' || r == '.' || r == '[' || r == ']' || r == '{' || r == '}' || r == ':'
	})
	for _, part := range parts {
		if part == token {
			return true
		}
	}
	return false
}

func hasStrongDomainInPath(path string) bool {
	for _, term := range []string{
		"worker", "auth", "rule", "rules", "alert", "notification", "deployment", "deploy", "market", "watchlist", "tauri",
		"post", "posts", "user", "users", "profile", "profiles", "search", "hashtag", "hashtags", "game", "games",
		"combat", "ability", "abilities", "movement", "hud", "viewmodel", "effect", "inventory", "lobby", "arena", "remote", "weapon",
	} {
		if strings.Contains(path, term) {
			return true
		}
	}
	return false
}

func isLowValueFeatureWork(work *featureWork) bool {
	if work == nil {
		return true
	}
	if !lowValueFeatureTerms[work.term] {
		return false
	}
	for path := range work.paths {
		if hasStrongDomainInPath(strings.ToLower(path)) && !featureWorkGeneratedOnly(work) {
			return false
		}
	}
	for route := range work.routes {
		if hasStrongDomainInPath(strings.ToLower(route)) {
			return false
		}
	}
	return true
}

func featureWorkGeneratedOnly(work *featureWork) bool {
	if work == nil || len(work.paths) == 0 {
		return false
	}
	for path := range work.paths {
		if !isGeneratedFeaturePath(path) {
			return false
		}
	}
	return true
}

func uniqueFeatureTerms(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, term := range in {
		if term == "" || seen[term] {
			continue
		}
		seen[term] = true
		out = append(out, term)
	}
	sort.Strings(out)
	return out
}

func scoreFeatureFile(file models.FileInfo) int {
	lower := strings.ToLower(file.Path)
	switch {
	case strings.Contains(lower, "/api/") || strings.HasPrefix(lower, "api/"):
		return 4
	case strings.Contains(lower, "/lib/"):
		return 3
	case strings.Contains(lower, "/app/") || strings.HasPrefix(lower, "src/app/"):
		return 3
	case strings.HasPrefix(lower, "scripts/"):
		return 2
	case file.Kind == models.FileKindTest:
		return 2
	default:
		return 1
	}
}

func featureStartHere(work *featureWork) []string {
	var files []models.FileInfo
	for _, file := range work.paths {
		if file.Kind == models.FileKindTest {
			continue
		}
		if isFeatureNoisePath(strings.ToLower(file.Path)) {
			continue
		}
		files = append(files, file)
	}
	sort.SliceStable(files, func(i, j int) bool {
		if featurePathRank(files[i].Path) == featurePathRank(files[j].Path) {
			return files[i].Path < files[j].Path
		}
		return featurePathRank(files[i].Path) > featurePathRank(files[j].Path)
	})
	var out []string
	for _, file := range files {
		out = append(out, file.Path)
	}
	return out
}

func featurePathRank(path string) int {
	lower := strings.ToLower(path)
	switch {
	case strings.Contains(lower, "/api/") || strings.HasPrefix(lower, "api/"):
		return 5
	case strings.Contains(lower, "/app/") && (strings.Contains(lower, "page.") || strings.Contains(lower, "route.")):
		return 4
	case strings.Contains(lower, "/lib/"):
		return 3
	case strings.HasPrefix(lower, "scripts/"):
		return 2
	default:
		return 1
	}
}

func featureDisplayName(term string) string {
	switch term {
	case "rule", "alert":
		return "Rules / alerts"
	case "watchlist":
		return "Watchlist"
	case "notification":
		return "Notifications"
	case "market":
		return "Market data"
	case "worker":
		return "Worker"
	case "auth":
		return "Auth"
	case "post":
		return "Posts"
	case "user":
		return "Users"
	case "profile":
		return "Users / profiles"
	case "search":
		return "Search"
	case "hashtag":
		return "Hashtags"
	case "game":
		return "Games"
	case "ai-move":
		return "AI moves"
	case "backend-service":
		return "Backend services"
	case "frontend-game-ui":
		return "Frontend game UI"
	case "combat":
		return "Combat"
	case "ability":
		return "Abilities"
	case "match-flow":
		return "Match flow"
	case "movement":
		return "Movement"
	case "hud":
		return "HUD"
	case "viewmodel":
		return "Viewmodel"
	case "effect":
		return "Effects"
	case "inventory":
		return "Inventory"
	case "lobby":
		return "Lobby"
	case "arena":
		return "Arena"
	case "training-dummy":
		return "Training dummy"
	case "remote":
		return "Remotes"
	case "weapon-definition":
		return "Weapon definitions"
	case "symbol-level":
		return "Symbol levels"
	default:
		return strings.ToUpper(term[:1]) + term[1:]
	}
}

func featureSearchTerms(term string) []string {
	terms := []string{term}
	switch term {
	case "rule":
		terms = append(terms, "rules", "alert", "alerts", "condition", "operator")
	case "alert":
		terms = append(terms, "alerts", "rule", "rules", "notification")
	case "watchlist":
		terms = append(terms, "watchlists", "symbol", "ticker")
	case "notification":
		terms = append(terms, "notifications", "email", "webhook")
	case "market":
		terms = append(terms, "quote", "quotes", "ticker", "symbol", "candle")
	case "worker":
		terms = append(terms, "tick", "job", "cron", "queue")
	case "auth":
		terms = append(terms, "session", "cookie", "login", "logout")
	case "post":
		terms = append(terms, "posts", "post", "timeline", "repost", "reply")
	case "user":
		terms = append(terms, "users", "user", "account", "profile")
	case "profile":
		terms = append(terms, "profiles", "profile", "users", "bio")
	case "search":
		terms = append(terms, "search", "query", "results")
	case "hashtag":
		terms = append(terms, "hashtags", "hashtag", "tag")
	case "game":
		terms = append(terms, "games", "game", "match", "moves")
	case "ai-move":
		terms = append(terms, "ai", "move", "moves", "human")
	case "backend-service":
		terms = append(terms, "service", "services", "backend")
	case "frontend-game-ui":
		terms = append(terms, "frontend", "board", "ui", "game")
	case "combat":
		terms = append(terms, "attack", "damage", "hitbox", "parry")
	case "ability":
		terms = append(terms, "abilities", "skill", "cooldown")
	case "match-flow":
		terms = append(terms, "match", "round", "spawn", "victory")
	case "movement":
		terms = append(terms, "dash", "sprint", "jump", "controller")
	case "hud":
		terms = append(terms, "health", "crosshair", "bar", "ui")
	case "viewmodel":
		terms = append(terms, "camera", "arms", "viewmodel")
	case "effect":
		terms = append(terms, "effects", "vfx", "particles")
	case "inventory":
		terms = append(terms, "loadout", "backpack", "items")
	case "lobby":
		terms = append(terms, "queue", "spawn", "menu")
	case "arena":
		terms = append(terms, "map", "arena", "bounds")
	case "training-dummy":
		terms = append(terms, "training", "dummy", "target")
	case "remote":
		terms = append(terms, "remotes", "remoteevent", "remote", "network")
	case "weapon-definition":
		terms = append(terms, "weapon", "weapons", "definition", "sword")
	case "symbol-level":
		terms = append(terms, "symbol", "symbols", "levels", "targets")
	}
	return capStrings(uniqueStrings(terms), 8)
}

func featureAvoidFirst() []string {
	return []string{"generated reports", "global styles/assets", "lockfiles", "build/cache folders"}
}

func featureConfidence(work *featureWork) string {
	if len(work.routes) > 0 && len(work.paths) >= 3 {
		return "high"
	}
	if len(work.paths) >= 3 || len(work.routes) > 0 {
		return "medium"
	}
	return "low"
}

func buildRouteChains(routes []models.RouteInfo, graph models.DependencyGraph, files []models.FileInfo) []models.RouteChain {
	if len(routes) == 0 {
		return nil
	}
	edgesByFrom := map[string][]models.DependencyEdge{}
	for _, edge := range graph.Edges {
		if edge.To == "" || edge.Kind == "package" || edge.Kind == "external" {
			continue
		}
		edgesByFrom[edge.From] = append(edgesByFrom[edge.From], edge)
	}
	testsByTerm := testsByFeatureTerm(files)
	var chains []models.RouteChain
	seenRoute := map[string]bool{}
	for _, route := range routes {
		label := route.Method + " " + route.Path
		key := label + " " + route.SourceFile
		if seenRoute[key] {
			continue
		}
		seenRoute[key] = true
		chainFiles := followRouteImports(route.SourceFile, edgesByFrom)
		terms := featureTermsForRoute(route)
		chain := models.RouteChain{
			Route:   label,
			Files:   chainFiles,
			Tests:   routeChainTests(terms, testsByTerm),
			Summary: fmt.Sprintf("Start at `%s`, then follow the listed imports before broad searching.", route.SourceFile),
		}
		chains = append(chains, chain)
	}
	return chains
}

func followRouteImports(start string, edgesByFrom map[string][]models.DependencyEdge) []string {
	var out []string
	seen := map[string]bool{}
	queue := []string{start}
	for len(queue) > 0 && len(out) < routeChainFileLimit {
		current := queue[0]
		queue = queue[1:]
		if current == "" || seen[current] {
			continue
		}
		seen[current] = true
		out = append(out, current)
		edges := edgesByFrom[current]
		sort.SliceStable(edges, func(i, j int) bool {
			if routeChainRank(edges[i].To) == routeChainRank(edges[j].To) {
				return edges[i].To < edges[j].To
			}
			return routeChainRank(edges[i].To) > routeChainRank(edges[j].To)
		})
		for _, edge := range edges {
			if edge.To != "" && !seen[edge.To] {
				queue = append(queue, edge.To)
			}
		}
	}
	return out
}

func routeChainRank(path string) int {
	lower := strings.ToLower(path)
	switch {
	case strings.Contains(lower, "schema") || strings.Contains(lower, "validat"):
		return 5
	case strings.Contains(lower, "/db/") || strings.Contains(lower, "repositor"):
		return 4
	case strings.Contains(lower, "auth") || strings.Contains(lower, "session"):
		return 3
	case strings.Contains(lower, "service") || strings.Contains(lower, "provider"):
		return 2
	case strings.Contains(lower, "/lib/"):
		return 1
	default:
		return 0
	}
}

func testsByFeatureTerm(files []models.FileInfo) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	for _, file := range files {
		if file.Kind != models.FileKindTest {
			continue
		}
		for _, term := range featureTermsForPath(file.Path) {
			if out[term] == nil {
				out[term] = map[string]bool{}
			}
			out[term][file.Path] = true
		}
	}
	return out
}

func routeChainTests(terms []string, testsByTerm map[string]map[string]bool) []string {
	seen := map[string]bool{}
	for _, term := range terms {
		for path := range testsByTerm[term] {
			seen[path] = true
		}
	}
	return sortedBoolKeys(seen, featureTestLimit)
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
