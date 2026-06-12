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
var nextRouteMethodPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?m)\bexport\s+(?:async\s+)?function\s+(GET|POST|PUT|PATCH|DELETE|OPTIONS|HEAD)\s*\(`),
	regexp.MustCompile(`(?m)\bexport\s+const\s+(GET|POST|PUT|PATCH|DELETE|OPTIONS|HEAD)\s*=`),
	regexp.MustCompile(`(?m)\bexport\s*\{[^}]*\bas\s+(GET|POST|PUT|PATCH|DELETE|OPTIONS|HEAD)\b[^}]*\}`),
}

func AnalyzeRoutes(root string, files []models.FileInfo) []models.RouteInfo {
	var routes []models.RouteInfo
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
