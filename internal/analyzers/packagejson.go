package analyzers

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"

	"github.com/will/stackmap/internal/models"
)

type packageJSON struct {
	Name            string            `json:"name"`
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

func AnalyzePackage(root string, files []models.FileInfo) (*models.PackageInfo, []models.Finding, error) {
	if !hasFile(files, "package.json") {
		return nil, nil, nil
	}
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return nil, nil, err
	}
	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, nil, err
	}
	info := &models.PackageInfo{
		Name:               pkg.Name,
		PackageManagerHint: packageManagerHint(files),
		Scripts:            pkg.Scripts,
		Dependencies:       pkg.Dependencies,
		DevDependencies:    pkg.DevDependencies,
	}
	return info, PackageFindings(info), nil
}

func PackageFindings(info *models.PackageInfo) []models.Finding {
	if info == nil {
		return nil
	}
	var findings []models.Finding
	if info.Scripts == nil {
		info.Scripts = map[string]string{}
	}
	if _, ok := info.Scripts["build"]; !ok && looksBuildableJS(info) {
		findings = append(findings, models.Finding{Severity: models.SeverityMedium, Category: "package", Message: "No build script found.", File: "package.json", Recommendation: "Add a build script if this project needs a production bundle."})
	}
	if _, ok := info.Scripts["test"]; !ok && hasAnyDep(info, "vitest", "jest", "@playwright/test") {
		findings = append(findings, models.Finding{Severity: models.SeverityLow, Category: "tests", Message: "Test tooling is installed, but no test script was found.", File: "package.json", Recommendation: "Add a test script that runs the configured test framework."})
	}
	if _, ok := info.Scripts["lint"]; !ok && hasAnyDep(info, "eslint", "typescript") {
		findings = append(findings, models.Finding{Severity: models.SeverityLow, Category: "package", Message: "No lint script found.", File: "package.json", Recommendation: "Add a lint script for local and CI readiness."})
	}
	if hasAnyDep(info, "express", "fastify", "koa") {
		if _, ok := info.Scripts["start"]; !ok {
			findings = append(findings, models.Finding{Severity: models.SeverityMedium, Category: "package", Message: "Backend dependencies detected, but no start script was found.", File: "package.json", Recommendation: "Add a production start script."})
		}
	}
	return findings
}

func ParsePackageJSON(data []byte) (*models.PackageInfo, error) {
	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, err
	}
	if pkg.Name == "" && len(pkg.Scripts) == 0 && len(pkg.Dependencies) == 0 && len(pkg.DevDependencies) == 0 {
		return nil, errors.New("package.json does not contain recognized fields")
	}
	return &models.PackageInfo{Name: pkg.Name, Scripts: pkg.Scripts, Dependencies: pkg.Dependencies, DevDependencies: pkg.DevDependencies}, nil
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
