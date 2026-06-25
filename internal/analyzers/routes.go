package analyzers

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/wcw-wcw/stackindex/internal/models"
)

var expressRoutePattern = regexp.MustCompile(`\b(?:app|router)\.(get|post|put|patch|delete|head|options|all)\(\s*["']([^"']+)["']`)
var expressRequirePattern = regexp.MustCompile(`(?m)\b(?:const|let|var)\s+([A-Za-z_$][A-Za-z0-9_$]*)\s*=\s*require\(\s*["']([^"']+)["']\s*\)`)
var expressImportPattern = regexp.MustCompile(`(?m)\bimport\s+([A-Za-z_$][A-Za-z0-9_$]*)\s+from\s+["']([^"']+)["']`)
var expressUseVarPattern = regexp.MustCompile(`\bapp\.use\(\s*["']([^"']+)["']\s*,\s*([A-Za-z_$][A-Za-z0-9_$]*)\s*\)`)
var expressUseRequirePattern = regexp.MustCompile(`\bapp\.use\(\s*["']([^"']+)["']\s*,\s*require\(\s*["']([^"']+)["']\s*\)\s*\)`)
var fastAPIRouterPrefixPattern = regexp.MustCompile(`\bAPIRouter\s*\([^)]*prefix\s*=\s*["']([^"']+)["']`)
var fastAPIDecoratorPattern = regexp.MustCompile(`(?m)@\s*(?:router|app)\.(get|post|put|patch|delete|head|options)\(\s*["']([^"']*)["']`)
var fastAPIIncludeRouterPattern = regexp.MustCompile(`\bapp\.include_router\(\s*([A-Za-z_$][A-Za-z0-9_$\.]*)\s*(?:,\s*prefix\s*=\s*["']([^"']+)["'])?`)
var pythonImportRouterPattern = regexp.MustCompile(`(?m)\bfrom\s+([A-Za-z0-9_\.]+)\s+import\s+([A-Za-z_$][A-Za-z0-9_$]*)`)
var nextRouteMethodPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?m)\bexport\s+(?:async\s+)?function\s+(GET|POST|PUT|PATCH|DELETE|OPTIONS|HEAD)\s*\(`),
	regexp.MustCompile(`(?m)\bexport\s+const\s+(GET|POST|PUT|PATCH|DELETE|OPTIONS|HEAD)\s*=`),
	regexp.MustCompile(`(?m)\bexport\s*\{[^}]*\bas\s+(GET|POST|PUT|PATCH|DELETE|OPTIONS|HEAD)\b[^}]*\}`),
}

func AnalyzeRoutes(root string, files []models.FileInfo) []models.RouteInfo {
	var routes []models.RouteInfo
	expressMounts := detectExpressMounts(root, files)
	fastAPIMounts := detectFastAPIMounts(root, files)
	for _, file := range files {
		if file.Kind != models.FileKindSource {
			continue
		}
		lower := strings.ToLower(file.Path)
		data, err := os.ReadFile(filepath.Join(root, file.Path))
		if err != nil {
			continue
		}
		if strings.Contains(lower, "app/api/") && (strings.HasSuffix(lower, "/route.ts") || strings.HasSuffix(lower, "/route.js")) {
			routes = append(routes, nextAppRoute(file.Path, string(data))...)
		}
		if strings.Contains(lower, "pages/api/") {
			routes = append(routes, models.RouteInfo{Method: "ANY", Path: pagesAPIRoute(file.Path), SourceFile: file.Path, Confidence: "medium"})
		}
		if isVercelAPIFunction(file.Path) {
			routes = append(routes, vercelAPIFunctionRoute(file.Path))
		}
		if isLocalAPIScript(file.Path) {
			routes = append(routes, localAPIScriptRoute(file.Path))
		}
		if isJavaScriptRouteSource(file.Path) {
			expressRoutes := ExtractExpressRoutes(string(data), file.Path)
			if prefix := expressMounts[file.Path]; prefix != "" {
				expressRoutes = withRoutePrefix(expressRoutes, prefix)
			}
			routes = append(routes, expressRoutes...)
		}
		routes = append(routes, ExtractFastAPIRoutes(string(data), file.Path, fastAPIMounts[file.Path])...)
	}
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Path == routes[j].Path {
			return routes[i].Method < routes[j].Method
		}
		return routes[i].Path < routes[j].Path
	})
	return dedupeRoutes(routes)
}

func isJavaScriptRouteSource(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".js", ".mjs", ".cjs", ".ts":
		return true
	default:
		return false
	}
}

func detectExpressMounts(root string, files []models.FileInfo) map[string]string {
	mounts := map[string]string{}
	for _, file := range files {
		if file.Kind != models.FileKindSource {
			continue
		}
		lower := strings.ToLower(filepath.ToSlash(file.Path))
		base := filepath.Base(lower)
		if base != "server.js" && base != "server.mjs" && base != "app.js" && base != "app.mjs" && base != "index.js" && base != "index.mjs" && !strings.HasSuffix(lower, "/server.js") && !strings.HasSuffix(lower, "/app.js") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file.Path)))
		if err != nil {
			continue
		}
		content := string(data)
		imports := map[string]string{}
		for _, match := range expressRequirePattern.FindAllStringSubmatch(content, -1) {
			imports[match[1]] = match[2]
		}
		for _, match := range expressImportPattern.FindAllStringSubmatch(content, -1) {
			imports[match[1]] = match[2]
		}
		for _, match := range expressUseRequirePattern.FindAllStringSubmatch(content, -1) {
			if target, ok := resolveMountedRoutePath(file.Path, match[2], files); ok {
				mounts[target] = cleanRoutePath(match[1])
			}
		}
		for _, match := range expressUseVarPattern.FindAllStringSubmatch(content, -1) {
			if importPath := imports[match[2]]; importPath != "" {
				if target, ok := resolveMountedRoutePath(file.Path, importPath, files); ok {
					mounts[target] = cleanRoutePath(match[1])
				}
			}
		}
	}
	return mounts
}

func resolveMountedRoutePath(from, importPath string, files []models.FileInfo) (string, bool) {
	if !strings.HasPrefix(importPath, ".") {
		return "", false
	}
	baseDir := filepath.ToSlash(filepath.Dir(from))
	cleaned := filepath.ToSlash(filepath.Clean(filepath.Join(baseDir, importPath)))
	exts := []string{"", ".js", ".mjs", ".ts", ".cjs", "/index.js", "/index.mjs", "/index.ts"}
	fileSet := map[string]bool{}
	for _, file := range files {
		fileSet[file.Path] = true
	}
	for _, ext := range exts {
		candidate := cleaned + ext
		if fileSet[candidate] {
			return candidate, true
		}
	}
	return "", false
}

func withRoutePrefix(routes []models.RouteInfo, prefix string) []models.RouteInfo {
	if prefix == "" {
		return routes
	}
	out := make([]models.RouteInfo, 0, len(routes))
	for _, route := range routes {
		route.Path = joinRoutePaths(prefix, route.Path)
		if route.Note == "" {
			route.Note = "Mounted Express router."
		}
		out = append(out, route)
	}
	return out
}

func ExtractExpressRoutes(content, source string) []models.RouteInfo {
	var routes []models.RouteInfo
	for _, match := range expressRoutePattern.FindAllStringSubmatch(content, -1) {
		routes = append(routes, models.RouteInfo{Method: strings.ToUpper(match[1]), Path: match[2], SourceFile: source, Confidence: "medium"})
	}
	return routes
}

func ExtractFastAPIRoutes(content, source, mountPrefix string) []models.RouteInfo {
	prefix := mountPrefix
	if match := fastAPIRouterPrefixPattern.FindStringSubmatch(content); match != nil {
		prefix = joinRoutePaths(prefix, match[1])
	}
	var routes []models.RouteInfo
	for _, match := range fastAPIDecoratorPattern.FindAllStringSubmatch(content, -1) {
		routes = append(routes, models.RouteInfo{
			Method:     strings.ToUpper(match[1]),
			Path:       joinRoutePaths(prefix, match[2]),
			SourceFile: source,
			Confidence: "medium",
			Note:       "FastAPI route decorator.",
		})
	}
	return routes
}

func detectFastAPIMounts(root string, files []models.FileInfo) map[string]string {
	mounts := map[string]string{}
	for _, file := range files {
		if file.Kind != models.FileKindSource || strings.ToLower(filepath.Ext(file.Path)) != ".py" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file.Path)))
		if err != nil {
			continue
		}
		content := string(data)
		if !strings.Contains(content, "include_router") {
			continue
		}
		imports := map[string]string{}
		for _, match := range pythonImportRouterPattern.FindAllStringSubmatch(content, -1) {
			imports[match[2]] = strings.ReplaceAll(match[1], ".", "/")
		}
		for _, match := range fastAPIIncludeRouterPattern.FindAllStringSubmatch(content, -1) {
			name := strings.TrimSuffix(match[1], ".router")
			prefix := cleanRoutePath(match[2])
			if module := imports[name]; module != "" {
				if target, ok := resolvePythonModulePath(file.Path, module, files); ok {
					mounts[target] = prefix
				}
			}
		}
	}
	return mounts
}

func resolvePythonModulePath(from, module string, files []models.FileInfo) (string, bool) {
	baseDir := filepath.ToSlash(filepath.Dir(from))
	candidates := []string{
		filepath.ToSlash(filepath.Join(baseDir, module+".py")),
		filepath.ToSlash(filepath.Join(module + ".py")),
	}
	fileSet := map[string]bool{}
	for _, file := range files {
		fileSet[file.Path] = true
	}
	for _, candidate := range candidates {
		if fileSet[candidate] {
			return candidate, true
		}
	}
	return "", false
}

func nextAppRoute(path, content string) []models.RouteInfo {
	dir := filepath.ToSlash(filepath.Dir(path))
	idx := strings.Index(dir, "app/api/")
	if idx == -1 {
		return nil
	}
	route := "/" + strings.TrimPrefix(dir[idx:], "app/")
	route = strings.ReplaceAll(route, "[", ":")
	route = strings.ReplaceAll(route, "]", "")
	methods := ExtractNextRouteMethods(content)
	if len(methods) == 0 {
		return []models.RouteInfo{{Method: "ANY", Path: route, SourceFile: path, Confidence: "low", Note: "No exported HTTP handler was detected."}}
	}
	routes := make([]models.RouteInfo, 0, len(methods))
	for _, method := range methods {
		routes = append(routes, models.RouteInfo{Method: method, Path: route, SourceFile: path, Confidence: "high"})
	}
	return routes
}

func ExtractNextRouteMethods(content string) []string {
	seen := map[string]bool{}
	for _, re := range nextRouteMethodPatterns {
		for _, match := range re.FindAllStringSubmatch(content, -1) {
			seen[strings.ToUpper(match[1])] = true
		}
	}
	order := []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD"}
	var methods []string
	for _, method := range order {
		if seen[method] {
			methods = append(methods, method)
		}
	}
	return methods
}

func pagesAPIRoute(path string) string {
	noExt := strings.TrimSuffix(path, filepath.Ext(path))
	idx := strings.Index(noExt, "pages/api/")
	if idx == -1 {
		return "/" + noExt
	}
	route := "/" + strings.TrimPrefix(noExt[idx:], "pages/")
	route = strings.TrimSuffix(route, "/index")
	route = strings.ReplaceAll(route, "[", ":")
	route = strings.ReplaceAll(route, "]", "")
	return route
}

func isVercelAPIFunction(path string) bool {
	lower := strings.ToLower(filepath.ToSlash(path))
	if !strings.HasPrefix(lower, "api/") {
		return false
	}
	switch strings.ToLower(filepath.Ext(lower)) {
	case ".js", ".ts", ".mjs", ".cjs":
		return true
	default:
		return false
	}
}

func vercelAPIFunctionRoute(path string) models.RouteInfo {
	noExt := strings.TrimSuffix(filepath.ToSlash(path), filepath.Ext(path))
	routePath := "/" + strings.TrimSuffix(noExt, "/index")
	return models.RouteInfo{
		Method:     "ANY",
		Path:       routePath,
		SourceFile: path,
		Confidence: "medium",
		Note:       "Vercel-style serverless function file.",
	}
}

func isLocalAPIScript(path string) bool {
	lower := strings.ToLower(filepath.ToSlash(path))
	if !strings.HasPrefix(lower, "scripts/") || strings.Contains(lower, ".test.") {
		return false
	}
	base := strings.ToLower(filepath.Base(lower))
	if !strings.Contains(base, "api") {
		return false
	}
	switch strings.ToLower(filepath.Ext(base)) {
	case ".js", ".ts", ".mjs", ".cjs":
		return true
	default:
		return false
	}
}

func localAPIScriptRoute(path string) models.RouteInfo {
	noExt := strings.TrimSuffix(filepath.ToSlash(path), filepath.Ext(path))
	return models.RouteInfo{
		Method:     "LOCAL",
		Path:       "/" + noExt,
		SourceFile: path,
		Confidence: "low",
		Note:       "Local API/server script detected by filename.",
	}
}

func joinRoutePaths(parts ...string) string {
	var cleaned []string
	for _, part := range parts {
		part = cleanRoutePath(part)
		if part == "" || part == "/" {
			continue
		}
		cleaned = append(cleaned, strings.Trim(part, "/"))
	}
	if len(cleaned) == 0 {
		return "/"
	}
	return "/" + strings.Join(cleaned, "/")
}

func cleanRoutePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if len(path) > 1 {
		path = strings.TrimRight(path, "/")
	}
	return path
}

func dedupeRoutes(in []models.RouteInfo) []models.RouteInfo {
	seen := map[string]bool{}
	var out []models.RouteInfo
	for _, route := range in {
		key := route.Method + " " + route.Path + " " + route.SourceFile
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, route)
	}
	return out
}
