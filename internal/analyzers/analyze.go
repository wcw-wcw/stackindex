package analyzers

import (
	"path/filepath"
	"time"

	"github.com/will/stackmap/internal/models"
	"github.com/will/stackmap/internal/scanner"
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
	stack := DetectStack(files, pkg)

	findings := append([]models.Finding{}, packageFindings...)
	findings = append(findings, envFindings...)
	findings = append(findings, testFindings...)
	findings = append(findings, deploymentFindings...)

	return &models.Analysis{
		RepoPath:    absRoot,
		RepoName:    filepath.Base(absRoot),
		GeneratedAt: time.Now(),
		Files:       files,
		Stack:       stack,
		PackageInfo: pkg,
		Env:         env,
		Routes:      routes,
		Tests:       tests,
		Deployment:  deployment,
		Findings:    findings,
	}, nil
}
