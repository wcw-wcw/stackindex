package analyzers

import (
	"path/filepath"
	"time"

	"github.com/wcw-wcw/stackindex/internal/models"
	"github.com/wcw-wcw/stackindex/internal/scanner"
)

func Analyze(root string) (*models.Analysis, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	files, err := scanner.Walk(absRoot)
	if err != nil {
		return nil, err
	}

	pkg, packageFindings, err := AnalyzePackage(absRoot, files)
	if err != nil {
		return nil, err
	}
	env, envFindings := AnalyzeEnv(absRoot, files)
	routes := AnalyzeRoutes(absRoot, files)
	tests, testFindings := AnalyzeTests(files, pkg)
	deployment, deploymentFindings := AnalyzeDeployment(absRoot, files, pkg, env, routes)
	stack := DetectStack(absRoot, files, pkg)
	context, structure := AnalyzeProjectContext(absRoot, files, pkg, stack, env, routes)
	dependencies := AnalyzeDependencyGraph(absRoot, files, pkg, structure, routes, deployment)

	findings := append([]models.Finding{}, packageFindings...)
	findings = append(findings, envFindings...)
	findings = append(findings, testFindings...)
	findings = append(findings, deploymentFindings...)

	return &models.Analysis{
		RepoPath:     absRoot,
		RepoName:     filepath.Base(absRoot),
		GeneratedAt:  time.Now(),
		Files:        files,
		Stack:        stack,
		PackageInfo:  pkg,
		Context:      context,
		Structure:    structure,
		Dependencies: dependencies,
		Env:          env,
		Routes:       routes,
		Tests:        tests,
		Deployment:   deployment,
		Findings:     findings,
	}, nil
}
