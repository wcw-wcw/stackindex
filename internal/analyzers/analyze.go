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
	walk, err := scanner.WalkDetailed(absRoot)
	if err != nil {
		return nil, err
	}
	files := walk.Files

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
	features := AnalyzeFeatureMap(files, routes, dependencies)
	quality := walk.Quality
	quality.UnresolvedInternalImports = countUnresolvedInternalImports(dependencies.UnresolvedImports)
	quality.Warnings = indexQualityWarnings(quality)

	findings := append([]models.Finding{}, packageFindings...)
	findings = append(findings, envFindings...)
	findings = append(findings, testFindings...)
	findings = append(findings, deploymentFindings...)

	return &models.Analysis{
		RepoPath:     absRoot,
		RepoName:     filepath.Base(absRoot),
		GeneratedAt:  time.Now(),
		Files:        files,
		Quality:      quality,
		Stack:        stack,
		PackageInfo:  pkg,
		Context:      context,
		Structure:    structure,
		Features:     features,
		Dependencies: dependencies,
		Env:          env,
		Routes:       routes,
		Tests:        tests,
		Deployment:   deployment,
		Findings:     findings,
	}, nil
}

func countUnresolvedInternalImports(imports []models.UnresolvedImport) int {
	count := 0
	for _, item := range imports {
		if item.ImportPath == "" {
			continue
		}
		if item.ImportPath[0] == '.' {
			count++
		}
	}
	return count
}

func indexQualityWarnings(quality models.IndexQuality) []string {
	var warnings []string
	if !quality.GeneratedOrCacheDirsIgnored {
		warnings = append(warnings, "Generated/cache directory ignores were not applied.")
	}
	if quality.UnresolvedInternalImports > 0 {
		warnings = append(warnings, "Some internal imports could not be resolved; route chains may be incomplete.")
	}
	if quality.LargeFilesSkipped > 0 {
		warnings = append(warnings, "Large files were skipped to keep the index compact.")
	}
	if quality.BinaryFilesSkipped > 0 {
		warnings = append(warnings, "Binary files were skipped.")
	}
	return warnings
}
