package analyzers

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/wcw-wcw/stackindex/internal/models"
)

const (
	contextEvidenceLimit = 5
	signalLimit          = 12
	directoryRoleLimit   = 12
	keyFileRoleLimit     = 18
)

type readmeContext struct {
	Title    string
	Summary  string
	Headings []string
	Terms    []string
}

type purposeRule struct {
	purpose  string
	terms    []string
	fallback string
}

type scoredPurpose struct {
	rule         purposeRule
	readmeScore  int
	packageScore int
	supportScore int
	totalScore   int
}

var purposeRules = []purposeRule{
	{purpose: "Local-first developer dashboard", fallback: "Developer dashboard", terms: []string{"devflow", "developer dashboard", "developer-dashboard", "local-first developer", "local-first dashboard", "development dashboard", "project tracker", "task tracker", "local-first project tracker", "agent workspace", "developer workflow", "workflow dashboard", "coding session", "coding sessions"}},
	{purpose: "Tauri desktop app", fallback: "Desktop productivity app", terms: []string{"tauri", "desktop app", "desktop application", "desktop productivity", "local desktop", "system tray", "native desktop"}},
	{purpose: "Assessment/education application", fallback: "Assessment-taking web application", terms: []string{"assessment", "assessments", "quiz", "quizzes", "teacher", "student", "students", "submission", "submissions", "score", "scores", "grading", "grade", "multiple-choice", "multiple choice"}},
	{purpose: "Board-game/game arena application", fallback: "Board-game arena application", terms: []string{"board game", "board-game", "connect four", "connect-four", "tic-tac-toe", "reversi", "game session", "game sessions", "legal move", "legal moves", "ai move", "ai moves", "boardarena", "arena"}},
	{purpose: "Portfolio/personal site", fallback: "Personal portfolio site", terms: []string{"portfolio", "resume", "recruiter", "contact links", "personal site", "personal website", "showcase site"}},
	{purpose: "Twitter-style social application", fallback: "Social web application", terms: []string{"tweet", "tweets", "post", "posts", "repost", "reposts", "follow", "followers", "following", "timeline", "hashtag", "hashtags", "mention", "mentions", "profile", "profiles"}},
	{purpose: "Stock monitoring and alerting application", fallback: "Stock monitoring web application", terms: []string{"stock", "stocks", "market", "markets", "watchlist", "watchlists", "alert", "alerts", "alpaca", "spy", "trading", "candles", "ticker", "tickers"}},
	{purpose: "Anime recommendation/discovery application", fallback: "Anime discovery web application", terms: []string{"anime", "recommendation", "recommendations", "recommend", "myanimelist", "mal", "catalog", "embeddings", "embedding", "pgvector", "similar", "similarity"}},
	{purpose: "Go CLI/TUI repository analysis tool", fallback: "Developer tooling application", terms: []string{"stackindex", "cli", "tui", "analyze repo", "analyze repository", "repo orientation", "orientation file", "search plan", "coding agent", "coding agents", "agent-facing", "codebase", "stack detection", "report", "reports", "deployment readiness"}},
}

var readmeNoisePrefixes = []string{"[![", "![", "<img", "<picture", "<!--"}

func AnalyzeProjectContext(root string, files []models.FileInfo, pkg *models.PackageInfo, stack models.StackInfo, env models.EnvAnalysis, routes []models.RouteInfo) (models.ProjectContext, models.StructureMap) {
	readme := extractReadmeContext(root, files)
	structure := AnalyzeStructureMap(files, routes)
	context := models.ProjectContext{
		Purpose:       "Unknown project purpose",
		Confidence:    "low",
		ReadmeTitle:   readme.Title,
		ReadmeSummary: readme.Summary,
		DocSignals:    capStrings(uniqueStrings(append(readme.Headings, readme.Terms...)), signalLimit),
		EnvSignals:    envSignals(env),
	}
	if pkg != nil {
		context.PackageName = pkg.Name
		context.PackageDescription = pkg.Description
		context.ScriptSignals = scriptSignals(pkg.Scripts)
	}
	context.Purpose, context.Confidence, context.Evidence = inferPurpose(context, structure, pkg, stack, routes)
	return context, structure
}

func extractReadmeContext(root string, files []models.FileInfo) readmeContext {
	var readmePath string
	for _, candidate := range []string{"README.md", "readme.md"} {
		if hasFile(files, candidate) {
			readmePath = candidate
			break
		}
	}
	if readmePath == "" {
		return readmeContext{}
	}
	data, err := os.ReadFile(filepath.Join(root, readmePath))
	if err != nil {
		return readmeContext{}
	}
	return parseReadmeContext(string(data))
}

func parseReadmeContext(content string) readmeContext {
	var ctx readmeContext
	var paragraph []string
	inCode := false
	summaryDone := false
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
			inCode = !inCode
			continue
		}
		if inCode || line == "" || isReadmeNoise(line) {
			if len(paragraph) > 0 {
				ctx.Summary = capText(strings.Join(paragraph, " "), 420)
				summaryDone = true
				paragraph = nil
			}
			continue
		}
		if strings.HasPrefix(line, "#") {
			heading := strings.TrimSpace(strings.TrimLeft(line, "#"))
			if heading != "" {
				if ctx.Title == "" && strings.HasPrefix(line, "# ") {
					ctx.Title = capText(heading, 90)
				}
				ctx.Headings = append(ctx.Headings, capText(heading, 80))
			}
			continue
		}
		if looksLikeInstallOrDependencyLine(line) {
			continue
		}
		if summaryDone {
			continue
		}
		paragraph = append(paragraph, line)
		if len(strings.Join(paragraph, " ")) >= 360 {
			ctx.Summary = capText(strings.Join(paragraph, " "), 420)
			summaryDone = true
			paragraph = nil
		}
	}
	if ctx.Summary == "" {
		ctx.Summary = capText(strings.Join(paragraph, " "), 420)
	}
	ctx.Terms = repeatedDomainTerms(content)
	ctx.Headings = capStrings(uniqueStrings(ctx.Headings), signalLimit)
	return ctx
}

func isReadmeNoise(line string) bool {
	lower := strings.ToLower(line)
	for _, prefix := range readmeNoisePrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return strings.HasPrefix(lower, "|") || strings.Contains(lower, "license")
}

func looksLikeInstallOrDependencyLine(line string) bool {
	lower := strings.ToLower(line)
	return strings.HasPrefix(lower, "$ ") ||
		strings.HasPrefix(lower, "npm ") ||
		strings.HasPrefix(lower, "pnpm ") ||
		strings.HasPrefix(lower, "yarn ") ||
		strings.HasPrefix(lower, "go run ") ||
		strings.HasPrefix(lower, "go test ") ||
		strings.HasPrefix(lower, "docker ")
}

var wordPattern = regexp.MustCompile(`[A-Za-z][A-Za-z0-9_-]{2,}`)

func repeatedDomainTerms(content string) []string {
	stop := map[string]bool{
		"the": true, "and": true, "for": true, "with": true, "this": true, "that": true, "from": true, "into": true, "your": true, "you": true,
		"run": true, "use": true, "using": true, "install": true, "setup": true, "local": true, "project": true, "application": true, "app": true,
	}
	counts := map[string]int{}
	for _, word := range wordPattern.FindAllString(strings.ToLower(stripCodeBlocks(content)), -1) {
		if stop[word] || len(word) > 30 {
			continue
		}
		counts[word]++
	}
	type termCount struct {
		term  string
		count int
	}
	var terms []termCount
	for term, count := range counts {
		if count >= 2 {
			terms = append(terms, termCount{term: term, count: count})
		}
	}
	sort.Slice(terms, func(i, j int) bool {
		if terms[i].count == terms[j].count {
			return terms[i].term < terms[j].term
		}
		return terms[i].count > terms[j].count
	})
	out := make([]string, 0, len(terms))
	for _, item := range terms {
		out = append(out, item.term)
		if len(out) == signalLimit {
			break
		}
	}
	return out
}

func stripCodeBlocks(content string) string {
	var out []string
	inCode := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inCode = !inCode
			continue
		}
		if !inCode {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

func inferPurpose(context models.ProjectContext, structure models.StructureMap, pkg *models.PackageInfo, stack models.StackInfo, routes []models.RouteInfo) (string, string, []string) {
	readmeSignals := []string{context.ReadmeTitle, context.ReadmeSummary}
	packageSignals := []string{context.PackageName, context.PackageDescription}
	supportSignals := purposeSupportSignalText(context, structure, pkg, stack, routes)
	var best scoredPurpose
	bestPurpose := "Unknown project purpose"
	for _, rule := range purposeRules {
		if rule.purpose == "Portfolio/personal site" && !explicitPortfolioText(strings.ToLower(strings.Join(append(readmeSignals, packageSignals...), " "))) {
			continue
		}
		readmeScore := weightedPurposeScore(readmeSignals, rule.terms, 4)
		packageScore := weightedPurposeScore(packageSignals, rule.terms, 3)
		supportScore := weightedPurposeScore(supportSignals, rule.terms, 1)
		total := readmeScore + packageScore + supportScore
		candidate := scoredPurpose{rule: rule, readmeScore: readmeScore, packageScore: packageScore, supportScore: supportScore, totalScore: total}
		if candidateBeats(candidate, best) {
			best = candidate
			bestPurpose = rule.purpose
		}
	}

	generic := genericPurposeFromReadme(context, stack)
	if best.totalScore == 0 {
		if generic != "" {
			return generic, "low", []string{"README/package metadata suggests a broad project type, but no specific domain category had strong evidence."}
		}
		return "Unknown project purpose", "low", purposeEvidence("Unknown project purpose", context, structure, routes, 0)
	}
	hasPrimaryEvidence := best.readmeScore >= 4 || best.packageScore >= 3
	if !hasPrimaryEvidence {
		if generic != "" {
			return generic, "low", []string{"Specific domain signals were weak or only came from supporting files, so StackIndex used a broad purpose instead."}
		}
		return "Unknown project purpose", "low", []string{"Specific domain signals were weak or only came from supporting files."}
	}

	switch {
	case best.totalScore >= 12 && (best.readmeScore >= 8 || best.packageScore >= 6):
		return bestPurpose, "high", purposeEvidence(bestPurpose, context, structure, routes, best.totalScore)
	case best.totalScore >= 7:
		return bestPurpose, "medium", purposeEvidence(bestPurpose, context, structure, routes, best.totalScore)
	default:
		if best.rule.fallback != "" {
			return best.rule.fallback, "low", []string{"README/package metadata suggests this broad purpose, but evidence was not strong enough for a high-confidence domain label."}
		}
		if generic != "" {
			return generic, "low", []string{"README/package metadata suggests a broad project type, but specific domain evidence was weak."}
		}
		return "Unknown project purpose", "low", []string{"Specific domain evidence was weak or conflicting."}
	}
}

func candidateBeats(candidate, best scoredPurpose) bool {
	if candidate.totalScore == best.totalScore {
		if candidate.readmeScore+candidate.packageScore == best.readmeScore+best.packageScore {
			return candidate.rule.purpose < best.rule.purpose
		}
		return candidate.readmeScore+candidate.packageScore > best.readmeScore+best.packageScore
	}
	return candidate.totalScore > best.totalScore
}

func weightedPurposeScore(signals []string, terms []string, weight int) int {
	joined := strings.ToLower(strings.Join(signals, " "))
	score := 0
	for _, term := range terms {
		if containsPurposeTerm(joined, strings.ToLower(term)) {
			score += weight
		}
	}
	return score
}

func containsPurposeTerm(signals, term string) bool {
	term = strings.TrimSpace(strings.ToLower(term))
	if term == "" {
		return false
	}
	if strings.Contains(term, " ") || strings.Contains(term, "-") {
		return strings.Contains(signals, term)
	}
	if len(term) <= 3 {
		for _, token := range wordPattern.FindAllString(signals, -1) {
			if strings.ToLower(token) == term {
				return true
			}
		}
		return false
	}
	return strings.Contains(signals, term)
}

func purposeSupportSignalText(context models.ProjectContext, structure models.StructureMap, pkg *models.PackageInfo, stack models.StackInfo, routes []models.RouteInfo) []string {
	var signals []string
	signals = append(signals, context.DocSignals...)
	signals = append(signals, context.ScriptSignals...)
	signals = append(signals, context.EnvSignals...)
	for _, dir := range structure.Directories {
		signals = append(signals, dir.Path, dir.Role)
	}
	for _, file := range structure.KeyFiles {
		signals = append(signals, file.Path, file.Role)
	}
	if pkg != nil {
		for dep := range pkg.Dependencies {
			signals = append(signals, dep)
		}
		for dep := range pkg.DevDependencies {
			signals = append(signals, dep)
		}
		signals = append(signals, pkg.ModuleName)
	}
	for _, group := range [][]string{stack.Languages, stack.Frameworks, stack.Libraries, stack.Databases, stack.Testing, stack.Deployment} {
		signals = append(signals, group...)
	}
	for _, route := range routes {
		signals = append(signals, route.Path, route.SourceFile)
	}
	return signals
}

func genericPurposeFromReadme(context models.ProjectContext, stack models.StackInfo) string {
	text := strings.ToLower(context.ReadmeTitle + " " + context.ReadmeSummary + " " + context.PackageDescription)
	switch {
	case strings.Contains(text, "developer dashboard") || strings.Contains(text, "devflow") || strings.Contains(text, "project tracker"):
		if strings.Contains(text, "local-first") {
			return "Local-first developer dashboard"
		}
		return "Developer dashboard"
	case strings.Contains(text, "tauri") || strings.Contains(text, "desktop app") || strings.Contains(text, "desktop application"):
		return "Tauri desktop app"
	case strings.Contains(text, "assessment") || strings.Contains(text, "quiz"):
		return "Assessment-taking web application"
	case explicitPortfolioText(text):
		return "Personal portfolio site"
	case strings.Contains(text, "board game") || strings.Contains(text, "board-game") || strings.Contains(text, "connect four") || strings.Contains(text, "game arena"):
		return "Board-game arena application"
	case strings.Contains(text, "frontend") || strings.Contains(strings.ToLower(strings.Join(stack.Frameworks, " ")), "react") || strings.Contains(strings.ToLower(strings.Join(stack.Frameworks, " ")), "vite"):
		return "Frontend web application"
	case strings.Contains(text, "web app") || strings.Contains(text, "application") || strings.Contains(text, "app"):
		return "General web application"
	default:
		return ""
	}
}

func explicitPortfolioText(text string) bool {
	if strings.Contains(text, "personal portfolio") || strings.Contains(text, "personal site") || strings.Contains(text, "personal website") {
		return true
	}
	return strings.Contains(text, "portfolio") && (strings.Contains(text, "resume") || strings.Contains(text, "recruiter") || strings.Contains(text, "case stud") || strings.Contains(text, "showcase"))
}

func purposeEvidence(purpose string, context models.ProjectContext, structure models.StructureMap, routes []models.RouteInfo, score int) []string {
	if purpose == "Unknown project purpose" || score == 0 {
		return []string{"No strong README, package, route, environment, or directory domain signals were detected."}
	}
	var evidence []string
	if context.ReadmeTitle != "" || context.ReadmeSummary != "" || context.PackageDescription != "" {
		evidence = append(evidence, "README/package metadata points to "+strings.TrimSuffix(strings.ToLower(purpose), ".")+".")
	}
	if len(context.ScriptSignals) > 0 {
		evidence = append(evidence, "Package scripts include "+strings.Join(capStrings(context.ScriptSignals, 4), ", ")+".")
	}
	if len(context.EnvSignals) > 0 {
		evidence = append(evidence, "Environment variable names include "+strings.Join(capStrings(context.EnvSignals, 4), ", ")+".")
	}
	routeTerms := routeDomainTerms(routes)
	if len(routeTerms) > 0 {
		evidence = append(evidence, "API routes include "+strings.Join(capStrings(routeTerms, 6), ", ")+" endpoints.")
	}
	if len(structure.Directories) > 0 {
		evidence = append(evidence, "Repository structure includes "+structure.Directories[0].Path+" for "+strings.ToLower(structure.Directories[0].Role)+".")
	}
	return capStrings(evidence, contextEvidenceLimit)
}

func routeDomainTerms(routes []models.RouteInfo) []string {
	seen := map[string]bool{}
	var terms []string
	for _, route := range routes {
		for _, part := range strings.FieldsFunc(route.Path, func(r rune) bool {
			return r == '/' || r == '-' || r == '_' || r == ':' || r == '[' || r == ']'
		}) {
			part = strings.TrimSpace(strings.ToLower(part))
			if part == "" || part == "api" || seen[part] {
				continue
			}
			seen[part] = true
			terms = append(terms, part)
		}
	}
	sort.Strings(terms)
	return terms
}

func scriptSignals(scripts map[string]string) []string {
	var signals []string
	for _, name := range sortedKeys(scripts) {
		lower := strings.ToLower(name + " " + scripts[name])
		for _, term := range []string{"tauri", "server", "server.js", "worker", "health", "smoke", "migrate", "migration", "seed", "sync", "demo", "audit", "analyze", "dev", "build", "test"} {
			if strings.Contains(lower, term) {
				signals = append(signals, name)
				break
			}
		}
	}
	return capStrings(uniqueStrings(signals), signalLimit)
}

func envSignals(env models.EnvAnalysis) []string {
	var signals []string
	for _, envVar := range env.UsedVars {
		signals = append(signals, envVar.Name)
	}
	signals = append(signals, env.ExampleVars...)
	sort.Strings(signals)
	return capStrings(uniqueStrings(signals), signalLimit)
}

func AnalyzeStructureMap(files []models.FileInfo, routes []models.RouteInfo) models.StructureMap {
	return models.StructureMap{
		Directories: detectDirectoryRoles(files),
		KeyFiles:    detectKeyFiles(files, routes),
	}
}

func detectDirectoryRoles(files []models.FileInfo) []models.DirectoryRole {
	counts := map[string]int{}
	for _, file := range files {
		if isRootLocalServer(file.Path) {
			counts["./"]++
		}
		for _, dir := range directoryAncestors(file.Path) {
			counts[dir]++
		}
	}
	var roles []models.DirectoryRole
	for _, rule := range directoryRoleRules() {
		if count := counts[rule.path]; count > 0 {
			roles = append(roles, models.DirectoryRole{Path: rule.path, Role: rule.role, Evidence: []string{rule.evidence}, FileCount: count})
		}
	}
	sort.SliceStable(roles, func(i, j int) bool {
		if roleImportance(roles[i].Path) == roleImportance(roles[j].Path) {
			return roles[i].Path < roles[j].Path
		}
		return roleImportance(roles[i].Path) > roleImportance(roles[j].Path)
	})
	return capDirectoryRoles(roles, directoryRoleLimit)
}

type directoryRule struct {
	path     string
	role     string
	evidence string
}

func directoryRoleRules() []directoryRule {
	return []directoryRule{
		{"cmd/", "CLI entrypoints", "Conventional Go command directory."},
		{"internal/", "Internal application packages", "Conventional Go internal package directory."},
		{"pkg/", "Reusable Go packages", "Conventional reusable Go package directory."},
		{"src/", "Frontend/source application code", "Common application source directory."},
		{"api/", "Serverless/API functions", "Top-level API/serverless functions directory."},
		{"src/app/api/", "Next.js API route handlers", "Next.js App Router API directory."},
		{"src/app/", "Next.js app routes/pages", "Next.js App Router directory."},
		{"src/components/", "UI components", "Common React component directory."},
		{"src/lib/", "Shared library/application code", "Common shared application code directory."},
		{"src/hooks/", "React hooks", "Common React hooks directory."},
		{"src/types/", "Shared types", "Common TypeScript types directory."},
		{"types/", "Shared types", "Common TypeScript types directory."},
		{"scripts/", "Operational scripts/tooling", "Common operational scripts directory."},
		{"database/", "Database schema/migrations", "Database directory detected."},
		{"database/migrations/", "Database schema migration files", "Migration directory detected."},
		{"db/", "Database schema/migrations", "Database directory detected."},
		{"db/migrations/", "Database schema migration files", "Migration directory detected."},
		{"migrations/", "Database schema migration files", "Migration directory detected."},
		{"docs/", "Documentation", "Documentation directory detected."},
		{"test/", "Tests", "Test directory detected."},
		{"tests/", "Tests", "Test directory detected."},
		{"__tests__/", "Tests", "Test directory detected."},
		{"public/", "Static assets", "Public/static asset directory detected."},
		{"backend/", "Backend service", "Backend directory detected."},
		{"backend/app/", "Backend application code", "Backend application package directory detected."},
		{"backend/app/api/", "Backend API routes", "Backend API route directory detected."},
		{"backend/routes/", "Backend API routes", "Backend routes directory detected."},
		{"frontend/", "Frontend app", "Frontend directory detected."},
		{"frontend/src/", "Frontend source code", "Frontend source directory detected."},
		{"./", "Root local Node backend/server", "Root server entrypoint detected."},
		{"src-tauri/", "Tauri desktop application shell", "Tauri source/config directory detected."},
		{"src-tauri/src/", "Tauri/Rust backend code", "Tauri Rust source directory detected."},
		{"src-tauri/capabilities/", "Tauri permissions/config", "Tauri capabilities directory detected."},
	}
}

func directoryAncestors(path string) []string {
	dir := filepath.ToSlash(filepath.Dir(path))
	if dir == "." || dir == "" {
		return nil
	}
	parts := strings.Split(dir, "/")
	var dirs []string
	for i := range parts {
		dirs = append(dirs, strings.Join(parts[:i+1], "/")+"/")
	}
	return dirs
}

func roleImportance(path string) int {
	switch path {
	case "./", "src-tauri/src/", "src-tauri/", "src/app/api/", "cmd/", "internal/", "src/lib/", "backend/", "backend/app/", "backend/app/api/", "backend/routes/", "frontend/", "frontend/src/", "api/", "database/migrations/", "db/migrations/", "migrations/":
		return 3
	case "src-tauri/capabilities/", "src/", "src/app/", "scripts/", "database/", "db/", "src/components/":
		return 2
	default:
		return 1
	}
}

func detectKeyFiles(files []models.FileInfo, routes []models.RouteInfo) []models.FileRole {
	routeFiles := map[string]models.RouteInfo{}
	for _, route := range routes {
		routeFiles[route.SourceFile] = route
	}
	var roles []models.FileRole
	for _, file := range files {
		if role, ok := keyFileRole(file, routeFiles); ok {
			roles = append(roles, role)
		}
	}
	sort.SliceStable(roles, func(i, j int) bool {
		if importanceRank(roles[i].Importance) == importanceRank(roles[j].Importance) {
			return roles[i].Path < roles[j].Path
		}
		return importanceRank(roles[i].Importance) > importanceRank(roles[j].Importance)
	})
	return capFileRoles(roles, keyFileRoleLimit)
}

func keyFileRole(file models.FileInfo, routeFiles map[string]models.RouteInfo) (models.FileRole, bool) {
	lower := strings.ToLower(file.Path)
	base := strings.ToLower(filepath.Base(file.Path))
	route, isRouteFile := routeFiles[file.Path]
	role := models.FileRole{Path: file.Path, Importance: "medium"}
	switch {
	case file.Path == "package.json":
		role.Role = "Node package manifest and scripts"
		role.Evidence = []string{"Standard Node package metadata file."}
		role.Importance = "high"
	case file.Path == "go.mod":
		role.Role = "Go module definition"
		role.Evidence = []string{"Standard Go module metadata file."}
		role.Importance = "high"
	case base == "readme.md":
		role.Role = "Project documentation"
		role.Evidence = []string{"README documentation file."}
		role.Importance = "high"
	case file.Path == ".env.example":
		role.Role = "Environment variable template"
		role.Evidence = []string{"Documents required or optional environment variables."}
		role.Importance = "high"
	case base == "dockerfile":
		role.Role = "Container build config"
		role.Evidence = []string{"Docker build file."}
		role.Importance = "high"
	case file.Path == "vercel.json":
		role.Role = "Vercel deployment config"
		role.Evidence = []string{"Vercel configuration file."}
		role.Importance = "high"
	case strings.HasPrefix(base, "next.config."):
		role.Role = "Next.js config"
		role.Evidence = []string{"Next.js configuration file."}
		role.Importance = "high"
	case strings.HasPrefix(base, "vite.config."):
		role.Role = "Vite config"
		role.Evidence = []string{"Vite configuration file."}
		role.Importance = "high"
	case file.Path == "src-tauri/tauri.conf.json" || file.Path == "tauri.conf.json":
		role.Role = "Tauri application config"
		role.Evidence = []string{"Tauri configuration file."}
		role.Importance = "high"
	case file.Path == "src-tauri/Cargo.toml" || file.Path == "src-tauri/cargo.toml":
		role.Role = "Tauri/Rust package manifest"
		role.Evidence = []string{"Cargo manifest under src-tauri."}
		role.Importance = "high"
	case file.Path == "tsconfig.json":
		role.Role = "TypeScript config"
		role.Evidence = []string{"TypeScript compiler configuration."}
	case isRootLocalServer(file.Path):
		role.Role = "Local Node backend/server entrypoint"
		role.Evidence = []string{"Root server/app file detected."}
		role.Importance = "high"
	case isFrontendEntrypointPath(file.Path):
		role.Role = "Frontend application entrypoint"
		role.Evidence = []string{"Vite/React-style frontend entrypoint detected."}
		role.Importance = "high"
	case file.Path == "src-tauri/src/main.rs" || file.Path == "src-tauri/src/lib.rs":
		role.Role = "Tauri/Rust backend entrypoint"
		role.Evidence = []string{"Tauri Rust entrypoint file."}
		role.Importance = "high"
	case strings.Contains(lower, "migration") || strings.Contains(lower, "migrations"):
		role.Role = "Database migration"
		role.Evidence = []string{"Path includes migration naming."}
		role.Importance = "medium"
	case routeLooksHealth(file.Path, routeFiles):
		role.Role = "Health endpoint implementation"
		role.Evidence = []string{"Detected API route path includes health."}
		role.Importance = "high"
	case isRouteFile:
		if strings.HasPrefix(lower, "api/") {
			role.Role = "Serverless/API function"
		} else if strings.HasPrefix(lower, "scripts/") {
			role.Role = "Local API/server script"
		} else {
			role.Role = "API route handler"
		}
		role.Evidence = []string{"Detected route " + route.Method + " " + route.Path + "."}
		role.Importance = "medium"
	case strings.HasPrefix(lower, "docs/") && strings.Contains(lower, "deploy"):
		role.Role = "Deployment documentation"
		role.Evidence = []string{"Documentation path mentions deployment."}
		role.Importance = "medium"
	case strings.Contains(lower, "worker") && file.Kind == models.FileKindSource:
		role.Role = "Background/worker process"
		role.Evidence = []string{"Filename or path includes worker."}
		role.Importance = "medium"
	case strings.Contains(lower, "smoke") || strings.Contains(lower, "health") || strings.Contains(lower, "check"):
		if strings.HasPrefix(lower, "scripts/") {
			role.Role = "Operational validation script"
			role.Evidence = []string{"Script path includes smoke, health, or check."}
			role.Importance = "medium"
		}
	case strings.HasPrefix(lower, "cmd/") && base == "main.go":
		role.Role = "Main CLI entrypoint"
		role.Evidence = []string{"Go command main file under cmd/."}
		role.Importance = "high"
	default:
		return models.FileRole{}, false
	}
	return role, true
}

func isRootLocalServer(path string) bool {
	switch strings.ToLower(filepath.ToSlash(path)) {
	case "server.js", "server.mjs", "app.js", "app.mjs":
		return true
	default:
		return false
	}
}

func isFrontendEntrypointPath(path string) bool {
	switch strings.ToLower(filepath.ToSlash(path)) {
	case "src/app.tsx", "src/app.ts", "src/app.jsx", "src/app.js", "src/main.tsx", "src/main.ts", "src/main.jsx", "src/main.js":
		return true
	default:
		return false
	}
}

func routeLooksHealth(path string, routeFiles map[string]models.RouteInfo) bool {
	route, ok := routeFiles[path]
	if !ok {
		return false
	}
	return strings.Contains(strings.ToLower(route.Path+" "+route.SourceFile), "health")
}

func importanceRank(value string) int {
	switch value {
	case "high":
		return 3
	case "medium":
		return 2
	default:
		return 1
	}
}

func capDirectoryRoles(in []models.DirectoryRole, limit int) []models.DirectoryRole {
	if len(in) <= limit {
		return in
	}
	return in[:limit]
}

func capFileRoles(in []models.FileRole, limit int) []models.FileRole {
	if len(in) <= limit {
		return in
	}
	return in[:limit]
}

func capStrings(in []string, limit int) []string {
	if len(in) <= limit {
		return in
	}
	return in[:limit]
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, item := range in {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		key := strings.ToLower(item)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	return out
}

func capText(text string, limit int) string {
	text = strings.Join(strings.Fields(text), " ")
	if len(text) <= limit {
		return text
	}
	return strings.TrimSpace(text[:limit-1]) + "..."
}
