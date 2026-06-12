package report

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/will/stackmap/internal/models"
)

func WriteMarkdown(root string, analysis *models.Analysis) error {
	outDir := filepath.Join(root, ".stackmap", "reports")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "repo-report.md"), []byte(Markdown(analysis)), 0644)
}

func ExportAll(root string, analysis *models.Analysis) error {
	if err := WriteJSON(root, analysis); err != nil {
		return err
	}
	return WriteMarkdown(root, analysis)
}

func Markdown(a *models.Analysis) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# StackMap Report\n\n")
	fmt.Fprintf(&b, "Generated: %s\n\n", a.GeneratedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&b, "## Project Summary\n\n")
	fmt.Fprintf(&b, "- Repository: `%s`\n- Path: `%s`\n- Files scanned: %d\n- Findings: %s\n\n", a.RepoName, a.RepoPath, len(a.Files), findingSummary(a.Findings))

	fmt.Fprintf(&b, "## Detected Stack\n\n")
	writeList(&b, "Languages", a.Stack.Languages)
	writeList(&b, "Frameworks", a.Stack.Frameworks)
	writeList(&b, "Databases", a.Stack.Databases)
	writeList(&b, "Testing", a.Stack.Testing)
	writeList(&b, "Deployment", a.Stack.Deployment)
	fmt.Fprintln(&b)

	fmt.Fprintf(&b, "## File Overview\n\n")
	for _, line := range fileOverview(a.Files) {
		fmt.Fprintf(&b, "- %s\n", line)
	}
	fmt.Fprintln(&b)

	fmt.Fprintf(&b, "## Package Scripts\n\n")
	if a.PackageInfo == nil || len(a.PackageInfo.Scripts) == 0 {
		fmt.Fprintln(&b, "No package scripts detected.")
		fmt.Fprintln(&b)
	} else {
		for _, name := range sortedKeys(a.PackageInfo.Scripts) {
			fmt.Fprintf(&b, "- `%s`: `%s`\n", name, a.PackageInfo.Scripts[name])
		}
		fmt.Fprintln(&b)
	}

	fmt.Fprintf(&b, "## Environment Variables\n\n")
	if !a.Env.UsesEnvVars {
		fmt.Fprintln(&b, "No environment variable usage detected.")
		fmt.Fprintln(&b)
	} else {
		fmt.Fprintf(&b, "- `.env.example`: %s\n", present(a.Env.ExampleFile != ""))
		if len(a.Env.MissingFromExample) > 0 {
			fmt.Fprintf(&b, "- Missing from example: `%s`\n", strings.Join(a.Env.MissingFromExample, "`, `"))
		}
		fmt.Fprintln(&b)
	}

	fmt.Fprintf(&b, "## API Routes\n\n")
	if len(a.Routes) == 0 {
		fmt.Fprintln(&b, "No API routes detected.")
		fmt.Fprintln(&b)
	} else {
		for _, route := range a.Routes {
			fmt.Fprintf(&b, "- `%s %s` in `%s` (%s confidence)\n", route.Method, route.Path, route.SourceFile, route.Confidence)
		}
		fmt.Fprintln(&b)
	}

	fmt.Fprintf(&b, "## Tests\n\n")
	fmt.Fprintf(&b, "- Test files: %s\n- Test script: %s\n", present(a.Tests.HasTestFiles), present(a.Tests.HasTestScript))
	writeList(&b, "Frameworks", a.Tests.Frameworks)
	fmt.Fprintln(&b)

	fmt.Fprintf(&b, "## Deployment Readiness\n\n")
	fmt.Fprintf(&b, "- README: %s\n- `.env.example`: %s\n- Dockerfile: %s\n- Vercel config: %s\n- Health endpoint: %s\n- Migration files: %s\n\n", present(a.Deployment.HasReadme), present(a.Deployment.HasEnvExample), present(a.Deployment.HasDockerfile), present(a.Deployment.HasVercelConfig), present(a.Deployment.HasHealthEndpoint), present(a.Deployment.HasMigrationFiles))

	fmt.Fprintf(&b, "## Findings\n\n")
	if len(a.Findings) == 0 {
		fmt.Fprintln(&b, "No findings. Nice and quiet.")
		fmt.Fprintln(&b)
	} else {
		for _, f := range a.Findings {
			file := ""
			if f.File != "" {
				file = fmt.Sprintf(" (`%s`)", f.File)
			}
			fmt.Fprintf(&b, "- **%s / %s**: %s%s\n", f.Severity, f.Category, f.Message, file)
			if f.Recommendation != "" {
				fmt.Fprintf(&b, "  Recommendation: %s\n", f.Recommendation)
			}
		}
		fmt.Fprintln(&b)
	}

	fmt.Fprintf(&b, "## Recommended Next Steps\n\n")
	if a.AI != nil && a.AI.NextSteps != "" {
		fmt.Fprintf(&b, "%s\n", a.AI.NextSteps)
	} else if len(a.Findings) > 0 {
		for _, f := range a.Findings {
			if f.Recommendation != "" {
				fmt.Fprintf(&b, "- %s\n", f.Recommendation)
			}
		}
	} else {
		fmt.Fprintln(&b, "- Keep reports current by running `stackmap analyze . --no-tui` before deployment reviews.")
	}
	return b.String()
}

func writeList(b *strings.Builder, label string, items []string) {
	if len(items) == 0 {
		fmt.Fprintf(b, "- %s: none detected\n", label)
		return
	}
	fmt.Fprintf(b, "- %s: %s\n", label, strings.Join(items, ", "))
}

func fileOverview(files []models.FileInfo) []string {
	counts := map[models.FileKind]int{}
	for _, file := range files {
		counts[file.Kind]++
	}
	return []string{
		fmt.Sprintf("Source files: %d", counts[models.FileKindSource]),
		fmt.Sprintf("Config files: %d", counts[models.FileKindConfig]),
		fmt.Sprintf("Test files: %d", counts[models.FileKindTest]),
		fmt.Sprintf("Docs: %d", counts[models.FileKindDoc]),
	}
}

func findingSummary(findings []models.Finding) string {
	counts := map[models.Severity]int{}
	for _, f := range findings {
		counts[f.Severity]++
	}
	return fmt.Sprintf("%d high, %d medium, %d low, %d info", counts[models.SeverityHigh], counts[models.SeverityMedium], counts[models.SeverityLow], counts[models.SeverityInfo])
}

func present(ok bool) string {
	if ok {
		return "yes"
	}
	return "no"
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
