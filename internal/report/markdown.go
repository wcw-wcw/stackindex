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

	writeAIProjectSummary(&b, a.AI)

	fmt.Fprintf(&b, "## Top Recommended Fixes\n\n")
	writeTopFixes(&b, a)

	fmt.Fprintf(&b, "## Health Summary\n\n")
	fmt.Fprintf(&b, "- Stack detected: %s\n", present(stackDetected(a.Stack)))
	fmt.Fprintf(&b, "- Tests present: %s\n", present(a.Tests.HasTestFiles || a.Tests.HasTestScript))
	fmt.Fprintf(&b, "- Health endpoint present: %s\n", present(a.Deployment.HasHealthEndpoint))
	fmt.Fprintf(&b, "- Env example present: %s\n", present(a.Deployment.HasEnvExample))
	fmt.Fprintf(&b, "- Migration files present: %s\n", present(a.Deployment.HasMigrationFiles))
	fmt.Fprintf(&b, "- Deployment docs present: %s\n\n", present(a.Deployment.ReadmeMentionsDeploy))

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
		fmt.Fprintln(&b)
		writeEnvGroup(&b, "Required app config", a.Env.UsedVars, "required_app_config")
		writeEnvGroup(&b, "Optional app config", a.Env.UsedVars, "optional_app_config")
		writeEnvGroup(&b, "Platform/build metadata", a.Env.UsedVars, "platform_provided", "build_metadata")
		writeEnvGroup(&b, "Script-only vars", a.Env.UsedVars, "test_or_script_only")
		writeNameList(&b, "Missing from .env.example", a.Env.MissingFromExample)
		fmt.Fprintln(&b)
	}

	fmt.Fprintf(&b, "## API Routes\n\n")
	if len(a.Routes) == 0 {
		fmt.Fprintln(&b, "No API routes detected.")
		fmt.Fprintln(&b)
	} else {
		for _, route := range a.Routes {
			note := ""
			if route.Note != "" {
				note = "; " + route.Note
			}
			fmt.Fprintf(&b, "- `%s %s` in `%s` (%s confidence%s)\n", route.Method, route.Path, route.SourceFile, route.Confidence, note)
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
	if a.AI != nil && len(a.AI.RecommendedNextSteps) > 0 {
		for _, step := range a.AI.RecommendedNextSteps {
			fmt.Fprintf(&b, "- %s\n", step)
		}
	} else if len(a.Findings) > 0 {
		writeFindingRecommendations(&b, a.Findings, 0)
	} else {
		fmt.Fprintln(&b, "- Keep reports current by running `stackmap analyze . --no-tui` before deployment reviews.")
	}
	return b.String()
}

func writeAIProjectSummary(b *strings.Builder, ai *models.AISummary) {
	if ai == nil {
		return
	}
	fmt.Fprintf(b, "## AI Project Summary\n\n")
	if ai.Warning != "" {
		fmt.Fprintln(b, "AI summary was requested but Ollama was unavailable or did not return a usable summary.")
		fmt.Fprintln(b)
		return
	}
	if ai.ProjectSummary == "" && ai.ArchitectureOverview == "" && len(ai.KeyStrengths)+len(ai.PotentialRisks)+len(ai.RecommendedNextSteps) == 0 {
		if ai.RawText != "" {
			fmt.Fprintln(b, "The model returned text that could not be parsed as structured JSON.")
			fmt.Fprintln(b)
			writeIndentedBlock(b, ai.RawText)
		}
		return
	}
	fmt.Fprintf(b, "Generated locally with `%s`.\n\n", ai.Model)
	fmt.Fprintf(b, "### Summary\n\n%s\n\n", fallbackText(ai.ProjectSummary, "No summary returned."))
	fmt.Fprintf(b, "### Architecture Overview\n\n%s\n\n", fallbackText(ai.ArchitectureOverview, "No architecture overview returned."))
	writeBulletSection(b, "Key Strengths", ai.KeyStrengths)
	writeBulletSection(b, "Potential Risks", ai.PotentialRisks)
	writeBulletSection(b, "Recommended Next Steps", ai.RecommendedNextSteps)
}

func writeBulletSection(b *strings.Builder, label string, items []string) {
	fmt.Fprintf(b, "### %s\n\n", label)
	if len(items) == 0 {
		fmt.Fprintln(b, "- None returned.")
		fmt.Fprintln(b)
		return
	}
	for _, item := range items {
		fmt.Fprintf(b, "- %s\n", item)
	}
	fmt.Fprintln(b)
}

func writeIndentedBlock(b *strings.Builder, text string) {
	for _, line := range strings.Split(text, "\n") {
		fmt.Fprintf(b, "    %s\n", line)
	}
	fmt.Fprintln(b)
}

func writeTopFixes(b *strings.Builder, a *models.Analysis) {
	if len(a.Findings) == 0 {
		fmt.Fprintln(b, "- No actionable findings. Keep the report current before deployment reviews.")
		fmt.Fprintln(b)
		return
	}
	written := writeFindingRecommendations(b, a.Findings, 5)
	if written == 0 {
		fmt.Fprintln(b, "- Review the findings below and decide whether each one matters for this repository.")
	}
	fmt.Fprintln(b)
}

func writeFindingRecommendations(b *strings.Builder, findings []models.Finding, limit int) int {
	written := 0
	seen := map[string]bool{}
	for _, severity := range []models.Severity{models.SeverityHigh, models.SeverityMedium, models.SeverityLow, models.SeverityInfo} {
		for _, f := range findings {
			if f.Severity != severity || f.Recommendation == "" || seen[f.Recommendation] {
				continue
			}
			fmt.Fprintf(b, "- **%s / %s**: %s\n", f.Severity, f.Category, f.Recommendation)
			seen[f.Recommendation] = true
			written++
			if limit > 0 && written >= limit {
				return written
			}
		}
	}
	return written
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

func stackDetected(stack models.StackInfo) bool {
	return len(stack.Languages)+len(stack.Frameworks)+len(stack.Libraries)+len(stack.Databases)+len(stack.Testing)+len(stack.Deployment) > 0
}

func writeEnvGroup(b *strings.Builder, label string, vars []models.EnvVar, classes ...string) {
	classSet := map[string]bool{}
	for _, class := range classes {
		classSet[class] = true
	}
	var lines []string
	for _, envVar := range vars {
		if !classSet[envVar.Classification] {
			continue
		}
		suffix := ""
		if envVar.MissingExample {
			suffix = " (missing from `.env.example`)"
		}
		if envVar.Classification == "platform_provided" || envVar.Classification == "build_metadata" {
			suffix = " (detected but likely platform/build provided)"
		}
		lines = append(lines, fmt.Sprintf("`%s`%s", envVar.Name, suffix))
	}
	if len(lines) == 0 {
		fmt.Fprintf(b, "- %s: none detected\n", label)
		return
	}
	fmt.Fprintf(b, "- %s: %s\n", label, strings.Join(lines, ", "))
}

func writeNameList(b *strings.Builder, label string, names []string) {
	if len(names) == 0 {
		fmt.Fprintf(b, "- %s: none\n", label)
		return
	}
	fmt.Fprintf(b, "- %s: `%s`\n", label, strings.Join(names, "`, `"))
}

func present(ok bool) string {
	if ok {
		return "yes"
	}
	return "no"
}

func fallbackText(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
