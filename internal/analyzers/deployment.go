package analyzers

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/will/stackmap/internal/models"
)

func AnalyzeDeployment(root string, files []models.FileInfo, pkg *models.PackageInfo, env models.EnvAnalysis, routes []models.RouteInfo) (models.DeploymentAnalysis, []models.Finding) {
	var result models.DeploymentAnalysis
	for _, file := range files {
		lower := strings.ToLower(file.Path)
		base := strings.ToLower(filepath.Base(file.Path))
		switch base {
		case "readme.md":
			result.HasReadme = true
			data, _ := os.ReadFile(filepath.Join(root, file.Path))
			readme := strings.ToLower(string(data))
			result.ReadmeMentionsSetup = strings.Contains(readme, "setup") || strings.Contains(readme, "install") || strings.Contains(readme, "run")
			result.ReadmeMentionsDeploy = strings.Contains(readme, "deploy") || strings.Contains(readme, "vercel") || strings.Contains(readme, "docker")
			result.ReadmeMentionsMigrations = strings.Contains(readme, "migration") || strings.Contains(readme, "migrate")
		case ".env.example":
			result.HasEnvExample = true
		case "dockerfile":
			result.HasDockerfile = true
			result.DeploymentFiles = append(result.DeploymentFiles, file.Path)
		case "vercel.json":
			result.HasVercelConfig = true
			result.DeploymentFiles = append(result.DeploymentFiles, file.Path)
		}
		if strings.Contains(lower, "migration") || strings.Contains(lower, "migrations") || strings.Contains(lower, "schema.prisma") {
			result.HasMigrationFiles = true
			result.MigrationFiles = append(result.MigrationFiles, file.Path)
		}
	}
	for _, route := range routes {
		if strings.Contains(strings.ToLower(route.Path), "health") {
			result.HasHealthEndpoint = true
			break
		}
	}

	var findings []models.Finding
	if !result.HasReadme {
		findings = append(findings, models.Finding{Severity: models.SeverityLow, Category: "docs", Message: "No README.md found.", Recommendation: "Add a README with setup, run, and deployment notes."})
	} else if !result.ReadmeMentionsSetup {
		findings = append(findings, models.Finding{Severity: models.SeverityLow, Category: "docs", Message: "README does not appear to mention setup, install, or run instructions.", File: "README.md", Recommendation: "Document the basic local development flow."})
	}
	if env.UsesEnvVars && !result.HasEnvExample {
		findings = append(findings, models.Finding{Severity: models.SeverityMedium, Category: "deployment", Message: "Environment variables are used but not documented in .env.example.", Recommendation: "Add .env.example before deployment handoff."})
	}
	if backendLikely(pkg, routes) && !result.HasHealthEndpoint {
		findings = append(findings, models.Finding{Severity: models.SeverityLow, Category: "deployment", Message: "Backend/API app detected, but no health endpoint was found.", Recommendation: "Add a simple /health or /api/health route for deployment monitoring."})
	}
	if result.HasMigrationFiles && result.HasReadme && !result.ReadmeMentionsMigrations {
		findings = append(findings, models.Finding{Severity: models.SeverityLow, Category: "deployment", Message: "Migration files exist, but README does not mention migration instructions.", File: "README.md", Recommendation: "Document how to run migrations in setup/deploy flows."})
	}
	if likelyVercel(pkg, files, result) && result.HasReadme && !result.ReadmeMentionsDeploy {
		findings = append(findings, models.Finding{Severity: models.SeverityLow, Category: "deployment", Message: "Vercel deployment is likely, but deployment notes were not found in README.", File: "README.md", Recommendation: "Add brief deployment notes for Vercel configuration and required env vars."})
	}
	return result, findings
}

func backendLikely(pkg *models.PackageInfo, routes []models.RouteInfo) bool {
	return len(routes) > 0 || (pkg != nil && hasAnyDep(pkg, "express", "fastify", "koa", "hono"))
}

func likelyVercel(pkg *models.PackageInfo, files []models.FileInfo, deployment models.DeploymentAnalysis) bool {
	return deployment.HasVercelConfig || hasFile(files, "next.config.js") || hasFile(files, "next.config.ts") || (pkg != nil && hasAnyDep(pkg, "next", "vercel"))
}
