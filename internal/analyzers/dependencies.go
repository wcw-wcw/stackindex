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
	dependencyNodeLimit      = 200
	dependencyEdgeLimit      = 400
	unresolvedImportLimit    = 100
	topConnectedFileLimit    = 10
	architectureHintLimit    = 5
	reportConnectedFileLimit = 10
)

var (
	jsImportFromPattern    = regexp.MustCompile(`(?m)\bimport\s+(?:type\s+)?(?:[^"']+?\s+from\s+)?["']([^"']+)["']`)
	jsExportFromPattern    = regexp.MustCompile(`(?m)\bexport\s+(?:type\s+)?(?:\*|\{[^}]*\})\s+from\s+["']([^"']+)["']`)
	jsRequirePattern       = regexp.MustCompile(`(?m)\brequire\s*\(\s*["']([^"']+)["']\s*\)`)
	jsDynamicImportPattern = regexp.MustCompile(`(?m)\bimport\s*\(\s*["']([^"']+)["']\s*\)`)
	goImportBlockPattern   = regexp.MustCompile(`(?s)\bimport\s*\((.*?)\)`)
	goImportSinglePattern  = regexp.MustCompile(`(?m)^\s*import\s+(?:[._A-Za-z0-9]+\s+)?("[^"]+")`)
	goImportStringPattern  = regexp.MustCompile(`"([^"]+)"`)
)

var jsImportExtensions = []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"}

type importRef struct {
	path string
	kind string
}

type graphWork struct {
	nodes              map[string]*models.DependencyNode
	edges              []models.DependencyEdge
	unresolved         []models.UnresolvedImport
	importsByFile      map[string]int
	importedByFile     map[string]int
	roleByFile         map[string]string
	importanceByFile   map[string]string
	fileByPath         map[string]models.FileInfo
	goPackageFileByDir map[string]string
	entrypoints        []string
	entrypointSet      map[string]bool
}

func AnalyzeDependencyGraph(root string, files []models.FileInfo, pkg *models.PackageInfo, structure models.StructureMap, routes []models.RouteInfo, deployment models.DeploymentAnalysis) models.DependencyGraph {
	work := graphWork{
		nodes:              map[string]*models.DependencyNode{},
		importsByFile:      map[string]int{},
		importedByFile:     map[string]int{},
		roleByFile:         map[string]string{},
		importanceByFile:   map[string]string{},
		fileByPath:         map[string]models.FileInfo{},
		goPackageFileByDir: map[string]string{},
		entrypointSet:      map[string]bool{},
	}
	for _, file := range files {
		work.fileByPath[file.Path] = file
		if file.Language == "Go" && file.Kind != models.FileKindTest {
			dir := filepath.ToSlash(filepath.Dir(file.Path))
			if _, ok := work.goPackageFileByDir[dir]; !ok || filepath.Base(file.Path) == "main.go" {
				work.goPackageFileByDir[dir] = file.Path
			}
		}
	}
	for _, role := range structure.KeyFiles {
		work.roleByFile[role.Path] = role.Role
		work.importanceByFile[role.Path] = role.Importance
	}
	work.entrypoints = detectEntrypoints(files, structure, routes, pkg, deployment)
	for _, path := range work.entrypoints {
		work.entrypointSet[path] = true
		if work.roleByFile[path] == "" {
			work.roleByFile[path] = entrypointRole(path)
		}
		if work.importanceByFile[path] == "" {
			work.importanceByFile[path] = "high"
		}
	}

	for _, file := range files {
		if !isSupportedDependencySource(file) {
			continue
		}
		imports := extractImports(root, file)
		if len(imports) == 0 && !work.entrypointSet[file.Path] {
			continue
		}
		work.ensureNode(file.Path)
		for _, imp := range imports {
			work.addImportEdge(file.Path, imp, pkg)
		}
	}

	nodes := work.sortedNodes()
	edges := capDependencyEdges(work.edges, dependencyEdgeLimit)
	unresolved := capUnresolvedImports(work.unresolved, unresolvedImportLimit)
	top := work.topConnectedFiles()
	hints := architectureHints(files, routes, deployment, work.entrypoints, top, edges)
	return models.DependencyGraph{
		Nodes:             capDependencyNodes(nodes, dependencyNodeLimit),
		Edges:             edges,
		Entrypoints:       capStrings(work.entrypoints, reportConnectedFileLimit),
		UnresolvedImports: unresolved,
		TopConnectedFiles: top,
		ArchitectureHints: capStrings(hints, architectureHintLimit),
	}
}

func isSupportedDependencySource(file models.FileInfo) bool {
	if file.Kind != models.FileKindSource && file.Kind != models.FileKindTest {
		return false
	}
	switch strings.ToLower(filepath.Ext(file.Path)) {
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".go":
		return true
	default:
		return false
	}
}

func extractImports(root string, file models.FileInfo) []importRef {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file.Path)))
	if err != nil {
		return nil
	}
	content := string(data)
	switch strings.ToLower(filepath.Ext(file.Path)) {
	case ".go":
		return extractGoImports(content)
	default:
		return extractJSImports(content)
	}
}

func extractJSImports(content string) []importRef {
	var imports []importRef
	for _, re := range []*regexp.Regexp{jsImportFromPattern, jsExportFromPattern, jsRequirePattern, jsDynamicImportPattern} {
		for _, match := range re.FindAllStringSubmatch(content, -1) {
			imports = append(imports, importRef{path: strings.TrimSpace(match[1])})
		}
	}
	return dedupeImportRefs(imports)
}

func extractGoImports(content string) []importRef {
	var imports []importRef
	withoutBlocks := goImportBlockPattern.ReplaceAllStringFunc(content, func(block string) string {
		for _, match := range goImportStringPattern.FindAllStringSubmatch(block, -1) {
			imports = append(imports, importRef{path: match[1]})
		}
		return ""
	})
	for _, match := range goImportSinglePattern.FindAllStringSubmatch(withoutBlocks, -1) {
		path := strings.Trim(match[1], `"`)
		if path != "" {
			imports = append(imports, importRef{path: path})
		}
	}
	return dedupeImportRefs(imports)
}

func dedupeImportRefs(in []importRef) []importRef {
	seen := map[string]bool{}
	var out []importRef
	for _, item := range in {
		if item.path == "" || seen[item.path] {
			continue
		}
		seen[item.path] = true
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].path < out[j].path })
	return out
}

func (w *graphWork) addImportEdge(from string, imp importRef, pkg *models.PackageInfo) {
	path := strings.TrimSpace(imp.path)
	if path == "" {
		return
	}
	edge := models.DependencyEdge{From: from, ImportPath: path, Confidence: "medium"}
	fromExt := strings.ToLower(filepath.Ext(from))
	switch {
	case strings.HasPrefix(path, "."):
		edge.Kind = "relative"
		edge.Confidence = "high"
		if target, ok := w.resolveRelativeImport(from, path); ok {
			edge.To = target
			w.importsByFile[from]++
			w.importedByFile[target]++
			w.ensureNode(from)
			w.ensureNode(target)
		} else {
			edge.Kind = "unresolved"
			w.unresolved = append(w.unresolved, models.UnresolvedImport{From: from, ImportPath: path, Reason: "relative import did not match a file or index file"})
		}
	case fromExt == ".go" && pkg != nil && pkg.ModuleName != "" && (path == pkg.ModuleName || strings.HasPrefix(path, pkg.ModuleName+"/")):
		edge.Kind = "internal"
		edge.Confidence = "medium"
		if target, ok := w.resolveGoModuleImport(path, pkg.ModuleName); ok {
			edge.To = target
			w.importsByFile[from]++
			w.importedByFile[target]++
			w.ensureNode(from)
			w.ensureNode(target)
		} else {
			edge.Kind = "unresolved"
			w.unresolved = append(w.unresolved, models.UnresolvedImport{From: from, ImportPath: path, Reason: "Go module import did not match a scanned package directory"})
		}
	case fromExt == ".go" && !isGoStdlibImport(path):
		edge.Kind = "external"
		edge.Confidence = "high"
	default:
		edge.Kind = "package"
		edge.Confidence = "high"
	}
	w.edges = append(w.edges, edge)
}

func (w *graphWork) resolveRelativeImport(from, importPath string) (string, bool) {
	baseDir := filepath.ToSlash(filepath.Dir(from))
	if baseDir == "." {
		baseDir = ""
	}
	cleaned := filepath.ToSlash(filepath.Clean(filepath.Join(baseDir, importPath)))
	if cleaned == "." {
		return "", false
	}
	if file, ok := w.fileByPath[cleaned]; ok && isSupportedDependencySource(file) {
		return cleaned, true
	}
	if filepath.Ext(cleaned) == "" {
		for _, ext := range jsImportExtensions {
			candidate := cleaned + ext
			if _, ok := w.fileByPath[candidate]; ok {
				return candidate, true
			}
		}
		for _, ext := range jsImportExtensions {
			candidate := filepath.ToSlash(filepath.Join(cleaned, "index"+ext))
			if _, ok := w.fileByPath[candidate]; ok {
				return candidate, true
			}
		}
	}
	return "", false
}

func (w *graphWork) resolveGoModuleImport(importPath, moduleName string) (string, bool) {
	dir := strings.TrimPrefix(importPath, moduleName)
	dir = strings.TrimPrefix(dir, "/")
	if dir == "" {
		dir = "."
	}
	dir = filepath.ToSlash(dir)
	target, ok := w.goPackageFileByDir[dir]
	return target, ok
}

func isGoStdlibImport(importPath string) bool {
	first := strings.Split(importPath, "/")[0]
	return !strings.Contains(first, ".")
}

func (w *graphWork) ensureNode(path string) {
	if _, ok := w.nodes[path]; ok {
		return
	}
	file, ok := w.fileByPath[path]
	if !ok {
		return
	}
	role := w.roleByFile[path]
	if role == "" {
		role = inferredFileRole(path)
	}
	importance := w.importanceByFile[path]
	if importance == "" {
		importance = "low"
	}
	if w.entrypointSet[path] {
		importance = "high"
	}
	w.nodes[path] = &models.DependencyNode{Path: path, Role: role, Language: file.Language, Importance: importance}
}

func (w *graphWork) sortedNodes() []models.DependencyNode {
	for path, node := range w.nodes {
		node.ImportsCount = w.importsByFile[path]
		node.ImportedByCount = w.importedByFile[path]
		if node.Importance == "low" {
			score := node.ImportsCount + node.ImportedByCount
			if score >= 4 || node.ImportedByCount >= 2 {
				node.Importance = "high"
			} else if score >= 2 || w.entrypointSet[path] {
				node.Importance = "medium"
			}
		}
	}
	out := make([]models.DependencyNode, 0, len(w.nodes))
	for _, node := range w.nodes {
		out = append(out, *node)
	}
	sort.SliceStable(out, func(i, j int) bool {
		si := out[i].ImportsCount + out[i].ImportedByCount
		sj := out[j].ImportsCount + out[j].ImportedByCount
		if si == sj {
			if importanceRank(out[i].Importance) == importanceRank(out[j].Importance) {
				return out[i].Path < out[j].Path
			}
			return importanceRank(out[i].Importance) > importanceRank(out[j].Importance)
		}
		return si > sj
	})
	return out
}

func (w *graphWork) topConnectedFiles() []models.ConnectedFileSummary {
	nodes := w.sortedNodes()
	var out []models.ConnectedFileSummary
	for _, node := range nodes {
		if node.ImportsCount+node.ImportedByCount == 0 && !w.entrypointSet[node.Path] {
			continue
		}
		out = append(out, models.ConnectedFileSummary{
			Path:            node.Path,
			Role:            node.Role,
			ImportsCount:    node.ImportsCount,
			ImportedByCount: node.ImportedByCount,
			WhyItMatters:    whyFileMatters(node, w.entrypointSet[node.Path]),
		})
		if len(out) == topConnectedFileLimit {
			break
		}
	}
	return out
}

func whyFileMatters(node models.DependencyNode, entrypoint bool) string {
	role := strings.ToLower(node.Role)
	switch {
	case entrypoint && node.ImportsCount > 0:
		return "Entrypoint that connects to other project modules."
	case strings.Contains(role, "api route"):
		return "API route handler connected to shared application code."
	case strings.Contains(role, "worker"):
		return "Operational or worker file connected to supporting modules."
	case node.ImportedByCount >= 2:
		return "Shared module imported by multiple files."
	case node.ImportsCount >= 2:
		return "Coordinates several internal modules."
	default:
		return "Connected to the internal dependency graph."
	}
}

func inferredFileRole(path string) string {
	lower := strings.ToLower(path)
	base := strings.ToLower(filepath.Base(path))
	switch {
	case strings.Contains(lower, "app/api/") && strings.HasPrefix(base, "route."):
		return "API route handler"
	case strings.HasPrefix(lower, "api/"):
		return "Serverless/API function"
	case strings.HasPrefix(lower, "src/app/") && (strings.HasPrefix(base, "page.") || strings.HasPrefix(base, "layout.")):
		return "Next.js app page/layout"
	case strings.HasPrefix(lower, "cmd/") && base == "main.go":
		return "Main CLI entrypoint"
	case base == "main.tsx" || base == "main.ts" || base == "app.tsx" || base == "app.ts":
		return "Frontend entrypoint/component"
	case strings.Contains(lower, "worker"):
		return "Background/worker process"
	case strings.HasPrefix(lower, "scripts/"):
		return "Operational script"
	default:
		return "Source file"
	}
}

func detectEntrypoints(files []models.FileInfo, structure models.StructureMap, routes []models.RouteInfo, pkg *models.PackageInfo, deployment models.DeploymentAnalysis) []string {
	seen := map[string]bool{}
	var out []string
	add := func(path string) {
		if path == "" || seen[path] {
			return
		}
		seen[path] = true
		out = append(out, path)
	}
	routeFiles := map[string]bool{}
	for _, route := range routes {
		routeFiles[route.SourceFile] = true
	}
	for _, role := range structure.KeyFiles {
		lowerRole := strings.ToLower(role.Role)
		if strings.Contains(lowerRole, "entrypoint") || strings.Contains(lowerRole, "api route") || strings.Contains(lowerRole, "worker") || strings.Contains(lowerRole, "operational validation") {
			add(role.Path)
		}
	}
	for _, file := range files {
		lower := strings.ToLower(file.Path)
		base := strings.ToLower(filepath.Base(file.Path))
		switch {
		case routeFiles[file.Path]:
			add(file.Path)
		case strings.HasPrefix(lower, "cmd/") && base == "main.go":
			add(file.Path)
		case strings.HasPrefix(lower, "src/app/") && (strings.HasPrefix(base, "page.") || strings.HasPrefix(base, "layout.")):
			add(file.Path)
		case lower == "src/main.tsx" || lower == "src/main.ts" || lower == "src/main.jsx" || lower == "src/main.js" || lower == "src/app.tsx" || lower == "src/app.ts":
			add(file.Path)
		case strings.HasPrefix(lower, "scripts/") && scriptEntrypointName(lower):
			add(file.Path)
		case strings.Contains(lower, "worker") && isSupportedDependencySource(file):
			add(file.Path)
		case isExpressEntrypoint(lower, pkg):
			add(file.Path)
		}
	}
	for _, file := range deployment.MigrationFiles {
		if strings.HasPrefix(strings.ToLower(file), "scripts/") {
			add(file)
		}
	}
	sort.Strings(out)
	return out
}

func scriptEntrypointName(path string) bool {
	for _, term := range []string{"worker", "health", "smoke", "migrate", "migration", "seed", "sync"} {
		if strings.Contains(path, term) {
			return true
		}
	}
	return false
}

func isExpressEntrypoint(path string, pkg *models.PackageInfo) bool {
	if pkg == nil || !hasAnyDep(pkg, "express", "fastify", "koa") {
		return false
	}
	base := filepath.Base(path)
	return base == "server.ts" || base == "server.js" || base == "app.ts" || base == "app.js" || base == "index.ts" || base == "index.js"
}

func entrypointRole(path string) string {
	return inferredFileRole(path)
}

func architectureHints(files []models.FileInfo, routes []models.RouteInfo, deployment models.DeploymentAnalysis, entrypoints []string, top []models.ConnectedFileSummary, edges []models.DependencyEdge) []string {
	var hints []string
	add := func(hint string) {
		if hint == "" || containsString(hints, hint) {
			return
		}
		hints = append(hints, hint)
	}
	if apiRoutesImportShared(routes, edges) {
		add("API route files import shared library or database-related code.")
	}
	if workerImportsShared(entrypoints, edges) {
		add("Worker or operational scripts connect to shared application modules.")
	}
	if frontendEntrypointsImportUI(entrypoints, edges) {
		add("Frontend entrypoints connect to top-level UI or component modules.")
	}
	if deployment.HasMigrationFiles && hasDatabaseScript(entrypoints) {
		add("Database migration files are present alongside database-related scripts.")
	} else if deployment.HasMigrationFiles {
		add("Database migration files are present in the repository.")
	}
	if hasViteEntrypoint(entrypoints) && hasAPISurface(routes) && hasDatabaseOrDataTooling(files, entrypoints, deployment) {
		add("This appears to be a Vite/React frontend with supporting API or data tooling.")
	}
	if hasAPISurface(routes) && !deployment.HasHealthEndpoint {
		add("Serverless/API files or local API scripts are present, but no health endpoint was detected.")
	}
	if hasViteEntrypoint(entrypoints) && !hasAPISurface(routes) {
		add("This project appears mostly frontend/static based on Vite-style entrypoints and no detected API routes.")
	}
	if len(top) > 0 && top[0].ImportedByCount >= 2 {
		add("Shared modules are imported by multiple important files.")
	}
	return capStrings(hints, architectureHintLimit)
}

func hasAPISurface(routes []models.RouteInfo) bool {
	return len(routes) > 0
}

func hasDatabaseOrDataTooling(files []models.FileInfo, entrypoints []string, deployment models.DeploymentAnalysis) bool {
	if deployment.HasMigrationFiles || hasDatabaseScript(entrypoints) {
		return true
	}
	for _, file := range files {
		lower := strings.ToLower(file.Path)
		if strings.Contains(lower, "database") || strings.Contains(lower, "/db") || strings.Contains(lower, "neon") || strings.Contains(lower, "catalog") {
			return true
		}
	}
	return false
}

func apiRoutesImportShared(routes []models.RouteInfo, edges []models.DependencyEdge) bool {
	routeFiles := map[string]bool{}
	for _, route := range routes {
		routeFiles[route.SourceFile] = true
	}
	for _, edge := range edges {
		if !routeFiles[edge.From] || edge.To == "" {
			continue
		}
		lower := strings.ToLower(edge.To)
		if strings.Contains(lower, "/lib/") || strings.Contains(lower, "/db") || strings.Contains(lower, "database") || strings.Contains(lower, "rule") {
			return true
		}
	}
	return false
}

func workerImportsShared(entrypoints []string, edges []models.DependencyEdge) bool {
	workers := map[string]bool{}
	for _, entry := range entrypoints {
		if strings.Contains(strings.ToLower(entry), "worker") || strings.HasPrefix(strings.ToLower(entry), "scripts/") {
			workers[entry] = true
		}
	}
	for _, edge := range edges {
		if workers[edge.From] && edge.To != "" {
			return true
		}
	}
	return false
}

func frontendEntrypointsImportUI(entrypoints []string, edges []models.DependencyEdge) bool {
	frontends := map[string]bool{}
	for _, entry := range entrypoints {
		lower := strings.ToLower(entry)
		if strings.HasPrefix(lower, "src/main.") || strings.HasPrefix(lower, "src/app.") || strings.Contains(lower, "/page.") {
			frontends[entry] = true
		}
	}
	for _, edge := range edges {
		if !frontends[edge.From] || edge.To == "" {
			continue
		}
		lower := strings.ToLower(edge.To)
		if strings.Contains(lower, "component") || strings.Contains(lower, "app.") || strings.Contains(lower, "/ui/") {
			return true
		}
	}
	return false
}

func hasDatabaseScript(paths []string) bool {
	for _, path := range paths {
		lower := strings.ToLower(path)
		if strings.Contains(lower, "migrate") || strings.Contains(lower, "migration") || strings.Contains(lower, "seed") || strings.Contains(lower, "db") {
			return true
		}
	}
	return false
}

func hasViteEntrypoint(paths []string) bool {
	for _, path := range paths {
		lower := strings.ToLower(path)
		if lower == "src/main.tsx" || lower == "src/main.ts" || lower == "src/main.jsx" || lower == "src/main.js" {
			return true
		}
	}
	return false
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func capDependencyNodes(in []models.DependencyNode, limit int) []models.DependencyNode {
	if len(in) <= limit {
		return in
	}
	return in[:limit]
}

func capDependencyEdges(in []models.DependencyEdge, limit int) []models.DependencyEdge {
	sort.SliceStable(in, func(i, j int) bool {
		if in[i].From == in[j].From {
			if in[i].To == in[j].To {
				return in[i].ImportPath < in[j].ImportPath
			}
			return in[i].To < in[j].To
		}
		return in[i].From < in[j].From
	})
	if len(in) <= limit {
		return in
	}
	return in[:limit]
}

func capUnresolvedImports(in []models.UnresolvedImport, limit int) []models.UnresolvedImport {
	sort.SliceStable(in, func(i, j int) bool {
		if in[i].From == in[j].From {
			return in[i].ImportPath < in[j].ImportPath
		}
		return in[i].From < in[j].From
	})
	if len(in) <= limit {
		return in
	}
	return in[:limit]
}
