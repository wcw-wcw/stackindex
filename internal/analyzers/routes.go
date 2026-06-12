package analyzers

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/will/stackmap/internal/models"
)

var expressRoutePattern = regexp.MustCompile(`\b(?:app|router)\.(get|post|put|patch|delete|head|options|all)\(\s*["']([^"']+)["']`)

func AnalyzeRoutes(root string, files []models.FileInfo) []models.RouteInfo {
	var routes []models.RouteInfo
	for _, file := range files {
		if file.Kind != models.FileKindSource {
			continue
		}
		lower := strings.ToLower(file.Path)
		if strings.Contains(lower, "app/api/") && (strings.HasSuffix(lower, "/route.ts") || strings.HasSuffix(lower, "/route.js")) {
			routes = append(routes, nextAppRoute(file.Path)...)
		}
		if strings.Contains(lower, "pages/api/") {
			routes = append(routes, models.RouteInfo{Method: "ANY", Path: pagesAPIRoute(file.Path), SourceFile: file.Path, Confidence: "medium"})
		}
		data, err := os.ReadFile(filepath.Join(root, file.Path))
		if err != nil {
			continue
		}
		routes = append(routes, ExtractExpressRoutes(string(data), file.Path)...)
	}
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Path == routes[j].Path {
			return routes[i].Method < routes[j].Method
		}
		return routes[i].Path < routes[j].Path
	})
	return dedupeRoutes(routes)
}

func ExtractExpressRoutes(content, source string) []models.RouteInfo {
	var routes []models.RouteInfo
	for _, match := range expressRoutePattern.FindAllStringSubmatch(content, -1) {
		routes = append(routes, models.RouteInfo{Method: strings.ToUpper(match[1]), Path: match[2], SourceFile: source, Confidence: "medium"})
	}
	return routes
}

func nextAppRoute(path string) []models.RouteInfo {
	dir := filepath.ToSlash(filepath.Dir(path))
	idx := strings.Index(dir, "app/api/")
	if idx == -1 {
		return nil
	}
	route := "/" + strings.TrimPrefix(dir[idx:], "app/")
	route = strings.ReplaceAll(route, "[", ":")
	route = strings.ReplaceAll(route, "]", "")
	return []models.RouteInfo{{Method: "ANY", Path: route, SourceFile: path, Confidence: "high"}}
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
