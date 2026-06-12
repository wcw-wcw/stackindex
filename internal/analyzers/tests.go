package analyzers

import (
	"path/filepath"
	"strings"

	"github.com/will/stackmap/internal/models"
)

func AnalyzeTests(files []models.FileInfo, pkg *models.PackageInfo) (models.TestAnalysis, []models.Finding) {
	var result models.TestAnalysis
	for _, file := range files {
		base := strings.ToLower(filepath.Base(file.Path))
		if strings.Contains(base, ".test.") || strings.Contains(base, ".spec.") || strings.HasSuffix(base, "_test.go") {
			result.HasTestFiles = true
			result.TestFiles = append(result.TestFiles, file.Path)
		}
		switch {
		case strings.HasPrefix(base, "vitest.config"):
			result.Frameworks = appendUnique(result.Frameworks, "Vitest")
		case strings.HasPrefix(base, "jest.config"):
			result.Frameworks = appendUnique(result.Frameworks, "Jest")
		case strings.HasPrefix(base, "playwright.config"):
			result.Frameworks = appendUnique(result.Frameworks, "Playwright")
			result.PlaywrightDetected = true
		}
	}
	if pkg != nil {
		if script, ok := pkg.Scripts["test"]; ok {
			result.HasTestScript = true
			result.TestScript = script
		}
		if hasAnyDep(pkg, "vitest") {
			result.Frameworks = appendUnique(result.Frameworks, "Vitest")
		}
		if hasAnyDep(pkg, "jest") {
			result.Frameworks = appendUnique(result.Frameworks, "Jest")
		}
		if hasAnyDep(pkg, "@playwright/test") {
			result.Frameworks = appendUnique(result.Frameworks, "Playwright")
			result.PlaywrightDetected = true
		}
	}

	var findings []models.Finding
	if !result.HasTestFiles && (pkg != nil || hasSourceFiles(files)) {
		findings = append(findings, models.Finding{Severity: models.SeverityLow, Category: "tests", Message: "No obvious test files were found.", Recommendation: "Add a small smoke test or unit test suite for the critical path."})
	}
	if pkg != nil && !result.HasTestScript && result.HasTestFiles {
		findings = append(findings, models.Finding{Severity: models.SeverityLow, Category: "tests", Message: "Test files exist, but no package test script was found.", File: "package.json", Recommendation: "Add a test script so tests are easy to run locally and in CI."})
	}
	return result, findings
}
