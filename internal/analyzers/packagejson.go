package analyzers

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wcw-wcw/stackindex/internal/models"
)

type packageJSON struct {
	Name            string            `json:"name"`
	Description     string            `json:"description"`
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

func AnalyzePackage(root string, files []models.FileInfo) (*models.PackageInfo, []models.Finding, error) {
	info := &models.PackageInfo{PackageManagerHint: packageManagerHint(files)}
	hasPackage := false
	for _, file := range files {
		if filepath.Base(file.Path) != "package.json" {
			continue
		}
		if isGeneratedPackageJSONPath(file.Path) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file.Path)))
		if err != nil {
			return nil, nil, err
		}
		var pkg packageJSON
		if err := json.Unmarshal(data, &pkg); err != nil {
			return nil, nil, err
		}
		mergePackageJSON(info, pkg, file.Path)
		hasPackage = true
	}
	if hasFile(files, "go.mod") {
		data, err := os.ReadFile(filepath.Join(root, "go.mod"))
		if err != nil {
			return nil, nil, err
		}
		info.ModuleName = parseGoModuleName(string(data))
		if info.Name == "" && info.ModuleName != "" {
			info.Name = filepath.Base(info.ModuleName)
		}
		hasPackage = true
	}
	if !hasPackage {
		return nil, nil, nil
	}
	return info, PackageFindings(info), nil
}

func mergePackageJSON(info *models.PackageInfo, pkg packageJSON, path string) {
	if info.Scripts == nil {
		info.Scripts = map[string]string{}
	}
	if info.Dependencies == nil {
		info.Dependencies = map[string]string{}
	}
	if info.DevDependencies == nil {
		info.DevDependencies = map[string]string{}
	}
	if path == "package.json" || info.Name == "" {
		if pkg.Name != "" {
			info.Name = pkg.Name
		}
		if pkg.Description != "" {
			info.Description = pkg.Description
		}
	}
	prefix := strings.TrimSuffix(filepath.ToSlash(filepath.Dir(path)), ".")
	for name, command := range pkg.Scripts {
		key := name
		if path != "package.json" && prefix != "" {
			key = prefix + ":" + name
		}
		info.Scripts[key] = command
	}
	for dep, version := range pkg.Dependencies {
		info.Dependencies[dep] = version
	}
	for dep, version := range pkg.DevDependencies {
		info.DevDependencies[dep] = version
	}
}

func PackageFindings(info *models.PackageInfo) []models.Finding {
	if info == nil {
		return nil
	}
	var findings []models.Finding
	if info.Scripts == nil {
		info.Scripts = map[string]string{}
	}
	if !hasScriptIntent(info, "build") && looksBuildableJS(info) {
		findings = append(findings, models.Finding{Severity: models.SeverityMedium, Category: "package", Message: "No build script found.", File: "package.json", Recommendation: "Add a build script if this project needs a production bundle."})
	}
	if !hasScriptIntent(info, "test") && hasAnyDep(info, "vitest", "jest", "@playwright/test") {
		findings = append(findings, models.Finding{Severity: models.SeverityLow, Category: "tests", Message: "Test tooling is installed, but no test script was found.", File: "package.json", Recommendation: "Add a test script that runs the configured test framework."})
	}
	if !hasScriptIntent(info, "lint") && hasAnyDep(info, "eslint", "typescript") {
		findings = append(findings, models.Finding{Severity: models.SeverityLow, Category: "package", Message: "No lint script found.", File: "package.json", Recommendation: "Add a lint script for local and CI readiness."})
	}
	if hasAnyDep(info, "express", "fastify", "koa") {
		if !hasScriptIntent(info, "start") && !hasBackendDevScript(info) {
			findings = append(findings, models.Finding{Severity: models.SeverityMedium, Category: "package", Message: "Backend dependencies detected, but no start script was found.", File: "package.json", Recommendation: "Add a production start script."})
		}
	}
	return findings
}

func isGeneratedPackageJSONPath(path string) bool {
	lower := strings.ToLower(filepath.ToSlash(path))
	return strings.Contains(lower, "wailsjs/runtime/package.json") ||
		strings.Contains(lower, "/generated/") ||
		strings.Contains(lower, "/gen/")
}

func hasScriptIntent(info *models.PackageInfo, intent string) bool {
	if info == nil {
		return false
	}
	intent = strings.ToLower(intent)
	for name := range info.Scripts {
		lower := strings.ToLower(name)
		if lower == intent || strings.HasSuffix(lower, ":"+intent) || strings.Contains(lower, "/"+intent) {
			return true
		}
	}
	return false
}

func hasBackendDevScript(info *models.PackageInfo) bool {
	if info == nil {
		return false
	}
	for name, command := range info.Scripts {
		lower := strings.ToLower(name + " " + command)
		if (strings.Contains(lower, "dev:api") || strings.Contains(lower, "backend") || strings.Contains(lower, "server")) &&
			(strings.Contains(lower, "server.js") || strings.Contains(lower, "server.mjs") || strings.Contains(lower, "app.js") || strings.Contains(lower, "app.mjs")) {
			return true
		}
	}
	return false
}

func ParsePackageJSON(data []byte) (*models.PackageInfo, error) {
	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, err
	}
	if pkg.Name == "" && pkg.Description == "" && len(pkg.Scripts) == 0 && len(pkg.Dependencies) == 0 && len(pkg.DevDependencies) == 0 {
		return nil, errors.New("package.json does not contain recognized fields")
	}
	return &models.PackageInfo{Name: pkg.Name, Description: pkg.Description, Scripts: pkg.Scripts, Dependencies: pkg.Dependencies, DevDependencies: pkg.DevDependencies}, nil
}

func parseGoModuleName(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}

func allDeps(info *models.PackageInfo) map[string]string {
	out := map[string]string{}
	for k, v := range info.Dependencies {
		out[k] = v
	}
	for k, v := range info.DevDependencies {
		out[k] = v
	}
	return out
}

func hasAnyDep(info *models.PackageInfo, names ...string) bool {
	deps := allDeps(info)
	for _, name := range names {
		if _, ok := deps[name]; ok {
			return true
		}
	}
	return false
}

func looksBuildableJS(info *models.PackageInfo) bool {
	return hasAnyDep(info, "vite", "next", "react", "typescript", "@sveltejs/kit")
}

func packageManagerHint(files []models.FileInfo) string {
	switch {
	case hasFile(files, "pnpm-lock.yaml"):
		return "pnpm"
	case hasFile(files, "yarn.lock"):
		return "yarn"
	case hasFile(files, "package-lock.json"):
		return "npm"
	case hasFile(files, "bun.lockb"), hasFile(files, "bun.lock"):
		return "bun"
	default:
		return ""
	}
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
