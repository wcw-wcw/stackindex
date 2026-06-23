package audit

import (
	"fmt"
	"strings"

	"github.com/wcw-wcw/stackindex/internal/models"
)

type Options struct {
	AllowMedium       bool
	AllowMissingTests bool
	FailOnLow         bool
}

func Evaluate(analysis *models.Analysis, opts Options) *models.AuditResult {
	result := &models.AuditResult{
		Mode:              "deployment-readiness",
		AllowMedium:       opts.AllowMedium,
		AllowMissingTests: opts.AllowMissingTests,
		FailOnLow:         opts.FailOnLow,
	}

	high, medium, low := severityCounts(analysis)
	if high > 0 {
		result.Reasons = append(result.Reasons, pluralizeCount(high, "high finding")+" detected.")
	}
	if medium > 0 {
		message := pluralizeCount(medium, "medium finding") + " detected."
		if opts.AllowMedium {
			result.Warnings = append(result.Warnings, message)
		} else {
			result.Reasons = append(result.Reasons, message)
		}
	}
	if low > 0 {
		message := pluralizeCount(low, "low finding") + " detected."
		if opts.FailOnLow {
			result.Reasons = append(result.Reasons, message)
		} else {
			result.Warnings = append(result.Warnings, message)
		}
	}
	if !stackDetected(analysis.Stack) {
		result.Reasons = append(result.Reasons, "No stack was detected.")
	}
	if analysis.Env.UsesEnvVars && analysis.Env.ExampleFile == "" {
		result.Reasons = append(result.Reasons, "Environment variables were detected but no `.env.example` file was found.")
	}
	result.HasBackendSurface = hasBackendSurface(analysis)
	result.RequiresHealthEndpoint = deploymentDetected(analysis) && result.HasBackendSurface
	if deploymentDetected(analysis) && !analysis.Deployment.HasHealthEndpoint {
		if result.HasBackendSurface {
			result.Reasons = append(result.Reasons, "Backend/API deployment surface detected but no health endpoint was found.")
		} else {
			result.Warnings = append(result.Warnings, "Deployment target detected without a health endpoint; this may be acceptable for static frontend apps.")
		}
	}
	if !testsDetected(analysis.Tests) {
		message := "Tests were not detected."
		if opts.AllowMissingTests {
			result.Warnings = append(result.Warnings, message)
		} else {
			result.Reasons = append(result.Reasons, message)
		}
	}

	result.Passed = len(result.Reasons) == 0
	if result.Passed {
		result.ExitCode = 0
	} else {
		result.ExitCode = 1
	}
	return result
}

func severityCounts(analysis *models.Analysis) (int, int, int) {
	var high, medium, low int
	testsAlreadyAudited := !testsDetected(analysis.Tests)
	for _, finding := range analysis.Findings {
		switch finding.Severity {
		case models.SeverityHigh:
			high++
		case models.SeverityMedium:
			medium++
		case models.SeverityLow:
			if testsAlreadyAudited && finding.Category == "tests" {
				continue
			}
			low++
		}
	}
	return high, medium, low
}

func stackDetected(stack models.StackInfo) bool {
	return len(stack.Languages)+len(stack.Frameworks)+len(stack.Libraries)+len(stack.Databases)+len(stack.Testing)+len(stack.Deployment) > 0
}

func testsDetected(tests models.TestAnalysis) bool {
	return tests.HasTestFiles || tests.HasTestScript
}

func deploymentDetected(analysis *models.Analysis) bool {
	return len(analysis.Stack.Deployment) > 0
}

func hasBackendSurface(analysis *models.Analysis) bool {
	if len(analysis.Routes) > 0 || analysis.Deployment.HasHealthEndpoint {
		return true
	}
	if hasAnyLower(analysis.Stack.Frameworks, "express", "fastify", "koa", "hono") {
		return true
	}
	if packageHasBackendIndicator(analysis.PackageInfo) {
		return true
	}
	return false
}

func packageHasBackendIndicator(pkg *models.PackageInfo) bool {
	if pkg == nil {
		return false
	}
	backendDeps := []string{
		"express",
		"fastify",
		"koa",
		"hono",
		"@hono/node-server",
		"@fastify/http-proxy",
		"apollo-server",
		"graphql-yoga",
	}
	for dep := range allPackageDeps(pkg) {
		if hasAnyLower([]string{dep}, backendDeps...) {
			return true
		}
	}
	for name, command := range pkg.Scripts {
		if scriptLooksBackend(name, command) {
			return true
		}
	}
	return false
}

func allPackageDeps(pkg *models.PackageInfo) map[string]bool {
	deps := map[string]bool{}
	for name := range pkg.Dependencies {
		deps[name] = true
	}
	for name := range pkg.DevDependencies {
		deps[name] = true
	}
	return deps
}

func scriptLooksBackend(name, command string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	command = strings.ToLower(strings.TrimSpace(command))
	if name == "server" || strings.Contains(name, ":server") {
		return true
	}
	return strings.Contains(command, "server.") || strings.Contains(command, "/server") || strings.Contains(command, " api/")
}

func hasAnyLower(values []string, needles ...string) bool {
	needleSet := map[string]bool{}
	for _, needle := range needles {
		needleSet[strings.ToLower(needle)] = true
	}
	for _, value := range values {
		if needleSet[strings.ToLower(strings.TrimSpace(value))] {
			return true
		}
	}
	return false
}

func pluralizeCount(count int, label string) string {
	if count == 1 {
		return fmt.Sprintf("1 %s", label)
	}
	return fmt.Sprintf("%d %ss", count, label)
}
