package report

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wcw-wcw/stackindex/internal/models"
)

func WriteMarkdown(root string, analysis *models.Analysis) error {
	outDir := filepath.Join(root, ".stackindex", "reports")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "repo-index.md"), []byte(Markdown(analysis)), 0644)
}

func ExportAll(root string, analysis *models.Analysis) error {
	if err := AttachChangeSummary(root, analysis); err != nil {
		return err
	}
	if err := WriteJSON(root, analysis); err != nil {
		return err
	}
	if err := WriteMarkdown(root, analysis); err != nil {
		return err
	}
	_, err := WriteSnapshot(root)
	return err
}

func Markdown(a *models.Analysis) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# StackIndex Repo Index\n\n")
	fmt.Fprintf(&b, "Generated: %s\n\n", a.GeneratedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&b, "This file is an agent-facing map of the repository. Read it before broad file searches so follow-up exploration can stay targeted.\n\n")

	fmt.Fprintf(&b, "## Repository Snapshot\n\n")
	fmt.Fprintf(&b, "- Repository: `%s`\n- Path: `%s`\n- Files indexed: %d\n- Finding summary: %s\n\n", a.RepoName, a.RepoPath, len(a.Files), findingSummary(a.Findings))

	writeIndexQuality(&b, a)
	writeProjectContext(&b, a)
	writeFeatureMap(&b, a)
	writeTaskSearchRecipes(&b, a)
	writeRouteChains(&b, a)
	writeAgentSearchGuide(&b, a)
	writeSearchBudgetHints(&b, a)

	writeAuditResult(&b, a)

	writeAIProjectSummary(&b, a)

	writeChangeSummary(&b, a)

	fmt.Fprintf(&b, "## Detected Stack\n\n")
	writeList(&b, "Languages", a.Stack.Languages)
	writeList(&b, "Frameworks", a.Stack.Frameworks)
	writeList(&b, "Libraries", a.Stack.Libraries)
	writeList(&b, "Databases", a.Stack.Databases)
	writeList(&b, "Testing", a.Stack.Testing)
	writeList(&b, "Deployment", a.Stack.Deployment)
	fmt.Fprintln(&b)

	writeProjectStructure(&b, a)
	writeKeyFiles(&b, a)
	writeFileConnections(&b, a)
	writeArchitectureHints(&b, a)

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
		writeNameList(&b, "Missing required from .env.example", a.Env.MissingRequiredFromExample)
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
		fmt.Fprintln(&b, "- Keep this index current by running `stackindex analyze . --no-tui` after meaningful repository changes.")
	}
	return b.String()
}

func writeIndexQuality(b *strings.Builder, a *models.Analysis) {
	q := a.Quality
	fmt.Fprintf(b, "## Index Quality\n\n")
	fmt.Fprintf(b, "- Generated/cache folders ignored: %s\n", present(q.GeneratedOrCacheDirsIgnored))
	fmt.Fprintf(b, "- Ignored generated/cache directories: %d\n", totalIgnoredDirs(q.IgnoredDirCounts))
	fmt.Fprintf(b, "- Large files skipped: %d\n", q.LargeFilesSkipped)
	fmt.Fprintf(b, "- Binary files skipped: %d\n", q.BinaryFilesSkipped)
	fmt.Fprintf(b, "- Unresolved internal imports: %d\n", q.UnresolvedInternalImports)
	if len(q.Warnings) > 0 {
		fmt.Fprintln(b, "- Confidence warnings:")
		for _, warning := range q.Warnings {
			fmt.Fprintf(b, "  - %s\n", warning)
		}
	}
	fmt.Fprintln(b)
}

func writeFeatureMap(b *strings.Builder, a *models.Analysis) {
	if len(a.Features.Features) == 0 {
		return
	}
	fmt.Fprintf(b, "## Feature Map\n\n")
	for _, feature := range a.Features.Features {
		fmt.Fprintf(b, "### %s\n\n", feature.Name)
		writePathList(b, "Start here", feature.StartHere)
		writePathList(b, "Related tests", feature.RelatedTests)
		writeNameList(b, "Search terms", feature.SearchTerms)
		writeNameList(b, "Routes", feature.Routes)
		writeNameList(b, "Avoid first", feature.AvoidFirst)
		fmt.Fprintf(b, "- Confidence: %s\n\n", feature.Confidence)
	}
}

func writeTaskSearchRecipes(b *strings.Builder, a *models.Analysis) {
	fmt.Fprintf(b, "## Task Search Recipes\n\n")
	if len(a.Routes) > 0 {
		fmt.Fprintln(b, "### Fix API Behavior")
		fmt.Fprintln(b)
		fmt.Fprintln(b, "1. Open the matching file in API Routes or Route Implementation Chains.")
		fmt.Fprintln(b, "2. Follow the listed imports into shared `src/lib/`, database, schema, or validation files.")
		fmt.Fprintln(b, "3. Open the nearest related test from Feature Map or Route Implementation Chains.")
		fmt.Fprintln(b, "4. Broaden to whole-repo search only after route, schema, storage, and tests are checked.")
		fmt.Fprintln(b)
	}
	if hasFrontendSurface(a) {
		fmt.Fprintln(b, "### Fix UI Behavior")
		fmt.Fprintln(b)
		fmt.Fprintln(b, "1. Start from the matching `src/app/<feature>/page.*` or sibling component in Feature Map.")
		fmt.Fprintln(b, "2. Search inside that feature folder and matching `src/lib/<feature>/` before shared modules.")
		fmt.Fprintln(b, "3. Check API calls and route handlers before editing shared database code.")
		fmt.Fprintln(b, "4. Avoid global styles/assets unless the task is visual or layout-specific.")
		fmt.Fprintln(b)
	}
	if a.Env.UsesEnvVars || a.Deployment.HasEnvExample {
		fmt.Fprintln(b, "### Fix Env Or Deployment Issue")
		fmt.Fprintln(b)
		fmt.Fprintln(b, "1. Open `.env.example` if present.")
		fmt.Fprintln(b, "2. Open env/config files from Key Files or Feature Map.")
		fmt.Fprintln(b, "3. Check deployment docs, migrations, and package scripts.")
		fmt.Fprintln(b, "4. Avoid unrelated feature files unless they reference the specific env var.")
		fmt.Fprintln(b)
	}
	if hasWorkerSurface(a) {
		fmt.Fprintln(b, "### Debug Worker Or Scheduled Job")
		fmt.Fprintln(b)
		fmt.Fprintln(b, "1. Start from worker entries in Feature Map or Key Files.")
		fmt.Fprintln(b, "2. Follow imports into shared config, repositories, and domain logic.")
		fmt.Fprintln(b, "3. Check scripts and tests that mention worker, tick, queue, cron, or sync.")
		fmt.Fprintln(b, "4. Broaden only after the worker entrypoint and its direct dependencies are understood.")
		fmt.Fprintln(b)
	}
}

func writeRouteChains(b *strings.Builder, a *models.Analysis) {
	if len(a.Features.RouteChains) == 0 {
		return
	}
	fmt.Fprintf(b, "## Route Implementation Chains\n\n")
	for _, chain := range a.Features.RouteChains {
		fmt.Fprintf(b, "- `%s`\n", chain.Route)
		if len(chain.Files) > 0 {
			fmt.Fprintln(b, "  - Follow:")
			for _, file := range chain.Files {
				fmt.Fprintf(b, "    - `%s`\n", file)
			}
		}
		if len(chain.Tests) > 0 {
			fmt.Fprintln(b, "  - Tests:")
			for _, file := range chain.Tests {
				fmt.Fprintf(b, "    - `%s`\n", file)
			}
		}
	}
	fmt.Fprintln(b)
}

func writeAgentSearchGuide(b *strings.Builder, a *models.Analysis) {
	fmt.Fprintf(b, "## Agent Search Guide\n\n")
	readFirst := agentReadFirstFiles(a)
	if len(readFirst) == 0 {
		fmt.Fprintln(b, "- Start with repository documentation and top-level configuration files.")
	} else {
		fmt.Fprintln(b, "- Read first:")
		for _, file := range readFirst {
			fmt.Fprintf(b, "  - `%s`", file.Path)
			if role := reportFileRole(file); role != "" {
				fmt.Fprintf(b, " - %s", role)
			}
			fmt.Fprintln(b)
		}
	}
	if len(a.Structure.Directories) > 0 {
		fmt.Fprintln(b, "- Search by directory role:")
		for _, dir := range capReportDirectoryRoles(a.Structure.Directories, 6) {
			fmt.Fprintf(b, "  - `%s` - %s\n", dir.Path, dir.Role)
		}
	}
	if len(a.Dependencies.TopConnectedFiles) > 0 {
		fmt.Fprintln(b, "- Inspect dependency hubs before leaf files:")
		for _, file := range capReportConnectedFiles(a.Dependencies.TopConnectedFiles, 5) {
			fmt.Fprintf(b, "  - `%s` - %s\n", file.Path, connectionCounts(file))
		}
	}
	fmt.Fprintln(b)
}

func writePathList(b *strings.Builder, label string, paths []string) {
	if len(paths) == 0 {
		fmt.Fprintf(b, "- %s: none detected\n", label)
		return
	}
	fmt.Fprintf(b, "- %s:\n", label)
	for _, path := range paths {
		fmt.Fprintf(b, "  - `%s`\n", path)
	}
}

func writeSearchBudgetHints(b *strings.Builder, a *models.Analysis) {
	fmt.Fprintf(b, "## Search Budget Hints\n\n")
	fmt.Fprintln(b, "- Prefer symbol/path searches inside the key directories above before scanning the whole repo.")
	if len(a.Routes) > 0 {
		fmt.Fprintln(b, "- For API behavior, start from the API Routes section and follow imports from each route file.")
	}
	if a.Env.UsesEnvVars {
		fmt.Fprintln(b, "- For configuration questions, inspect the Environment Variables section before opening env-related files.")
	}
	if a.Tests.HasTestFiles || a.Tests.HasTestScript {
		fmt.Fprintln(b, "- For expected behavior, use the Tests section to find representative test files and scripts.")
	}
	if len(a.Findings) > 0 {
		fmt.Fprintln(b, "- For risk review, start with Findings; each item names the category and usually the relevant file.")
	}
	fmt.Fprintln(b)
}

func agentReadFirstFiles(a *models.Analysis) []models.FileRole {
	if len(a.Structure.KeyFiles) == 0 {
		return nil
	}
	priorities := []string{"entrypoint", "package manifest", "readme", "configuration", "route", "deployment", "test"}
	var out []models.FileRole
	seen := map[string]bool{}
	for _, priority := range priorities {
		for _, file := range a.Structure.KeyFiles {
			if seen[file.Path] {
				continue
			}
			role := strings.ToLower(file.Role + " " + file.Path)
			if strings.Contains(role, priority) {
				out = append(out, file)
				seen[file.Path] = true
				if len(out) == 8 {
					return out
				}
			}
		}
	}
	for _, file := range a.Structure.KeyFiles {
		if seen[file.Path] {
			continue
		}
		out = append(out, file)
		if len(out) == 8 {
			return out
		}
	}
	return out
}

func writeProjectContext(b *strings.Builder, a *models.Analysis) {
	if strings.TrimSpace(a.Context.Purpose) == "" {
		return
	}
	fmt.Fprintf(b, "## Project Context\n\n")
	fmt.Fprintf(b, "- Likely purpose: %s\n", a.Context.Purpose)
	fmt.Fprintf(b, "- Confidence: %s\n", a.Context.Confidence)
	if a.Context.ReadmeTitle != "" {
		fmt.Fprintf(b, "- README title: %s\n", a.Context.ReadmeTitle)
	}
	if a.Context.ReadmeSummary != "" {
		fmt.Fprintf(b, "- README summary: %s\n", a.Context.ReadmeSummary)
	}
	if len(a.Context.Evidence) > 0 {
		fmt.Fprintln(b, "- Evidence:")
		fmt.Fprintln(b)
		for _, item := range capReportStrings(a.Context.Evidence, 5) {
			fmt.Fprintf(b, "  - %s\n", item)
		}
	}
	fmt.Fprintln(b)
}

func writeProjectStructure(b *strings.Builder, a *models.Analysis) {
	if len(a.Structure.Directories) == 0 {
		return
	}
	fmt.Fprintf(b, "## Project Structure\n\n")
	for _, dir := range capReportDirectoryRoles(a.Structure.Directories, 8) {
		fmt.Fprintf(b, "- `%s` — %s.", dir.Path, dir.Role)
		if dir.FileCount > 0 {
			fmt.Fprintf(b, " %d files scanned.", dir.FileCount)
		}
		fmt.Fprintln(b)
	}
	fmt.Fprintln(b)
}

func writeKeyFiles(b *strings.Builder, a *models.Analysis) {
	if len(a.Structure.KeyFiles) == 0 {
		return
	}
	fmt.Fprintf(b, "## Key Files\n\n")
	for _, file := range capReportFileRoles(a.Structure.KeyFiles, 10) {
		role := reportFileRole(file)
		if role == "" {
			continue
		}
		fmt.Fprintf(b, "- `%s` — %s.\n", file.Path, role)
	}
	fmt.Fprintln(b)
}

func reportFileRole(file models.FileRole) string {
	role := strings.TrimSpace(file.Role)
	if role != "" {
		return role
	}
	lower := strings.ToLower(file.Path)
	switch {
	case strings.Contains(lower, "deploy"):
		return "Deployment documentation"
	case strings.HasPrefix(lower, "docs/") || strings.HasSuffix(lower, ".md"):
		return "Documentation file"
	case strings.HasPrefix(lower, "api/"):
		return "Serverless/API function"
	case strings.HasPrefix(lower, "scripts/"):
		return "Operational script"
	default:
		return ""
	}
}

func writeFileConnections(b *strings.Builder, a *models.Analysis) {
	if len(a.Dependencies.TopConnectedFiles) == 0 {
		return
	}
	fmt.Fprintf(b, "## File Connections\n\n")
	for _, file := range capReportConnectedFiles(a.Dependencies.TopConnectedFiles, 10) {
		role := strings.TrimSpace(file.Role)
		if role == "" {
			role = "connected source file"
		}
		detail := file.WhyItMatters
		if detail == "" {
			detail = connectionCounts(file)
		} else {
			detail = strings.TrimSuffix(detail, ".") + "; " + connectionCounts(file)
		}
		fmt.Fprintf(b, "- `%s` — %s; %s.\n", file.Path, sentenceLower(role), detail)
	}
	fmt.Fprintln(b)
}

func writeArchitectureHints(b *strings.Builder, a *models.Analysis) {
	if len(a.Dependencies.ArchitectureHints) == 0 {
		return
	}
	fmt.Fprintf(b, "## Architecture Hints\n\n")
	for _, hint := range capReportStrings(a.Dependencies.ArchitectureHints, 5) {
		fmt.Fprintf(b, "- %s\n", hint)
	}
	fmt.Fprintln(b)
}

func connectionCounts(file models.ConnectedFileSummary) string {
	return fmt.Sprintf("imports %d internal file(s), imported by %d internal file(s)", file.ImportsCount, file.ImportedByCount)
}

func sentenceLower(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}
	return strings.ToLower(value[:1]) + value[1:]
}

func writeAuditResult(b *strings.Builder, a *models.Analysis) {
	if a.Audit == nil {
		return
	}
	status := "failed"
	if a.Audit.Passed {
		status = "passed"
	}
	fmt.Fprintf(b, "## Audit Result\n\n")
	fmt.Fprintf(b, "- Status: %s\n", status)
	fmt.Fprintf(b, "- Exit code: %d\n", a.Audit.ExitCode)
	if len(a.Audit.Reasons) == 0 {
		fmt.Fprintln(b, "- Blocking issues: none")
	} else {
		fmt.Fprintln(b, "- Blocking issues:")
		fmt.Fprintln(b)
		for _, reason := range a.Audit.Reasons {
			fmt.Fprintf(b, "  - %s\n", reason)
		}
	}
	if len(a.Audit.Warnings) > 0 {
		fmt.Fprintln(b, "- Warnings:")
		fmt.Fprintln(b)
		for _, warning := range a.Audit.Warnings {
			fmt.Fprintf(b, "  - %s\n", warning)
		}
	}
	fmt.Fprintln(b)
}

func writeAIProjectSummary(b *strings.Builder, a *models.Analysis) {
	ai := a.AI
	if ai == nil {
		return
	}
	fmt.Fprintf(b, "## AI Notes\n\n")
	fmt.Fprintln(b, DeterministicAISummary(a))
	fmt.Fprintln(b)
	if hasUsableLocalNotes(ai) {
		fmt.Fprintln(b, "### Local AI Notes")
		fmt.Fprintln(b)
		fmt.Fprintln(b, strings.TrimSpace(ai.LocalNotes))
		fmt.Fprintln(b)
		return
	}
	if hasStructuredAISummary(ai) && ai.Relevance != "low_confidence" && ai.Warning == "" {
		fmt.Fprintln(b, "### Local AI Notes")
		fmt.Fprintln(b)
		writeTextSection(b, "Summary", ai.ProjectSummary)
		writeTextSection(b, "Architecture Overview", ai.ArchitectureOverview)
		writeBulletSection(b, "Key Strengths", ai.KeyStrengths)
		writeBulletSection(b, "Potential Risks", ai.PotentialRisks)
		writeBulletSection(b, "Recommended Next Steps", ai.RecommendedNextSteps)
		return
	}
	fmt.Fprintf(b, "Local AI summary unavailable: %s did not return usable project-summary text.\n\n", aiModelText(ai))
}

func hasStructuredAISummary(ai *models.AISummary) bool {
	return ai.ProjectSummary != "" || ai.ArchitectureOverview != "" || len(ai.KeyStrengths)+len(ai.PotentialRisks)+len(ai.RecommendedNextSteps) > 0
}

func hasUsableLocalNotes(ai *models.AISummary) bool {
	return ai != nil && strings.TrimSpace(ai.LocalNotes) != "" && ai.Relevance != "low_confidence" && ai.Warning == ""
}

func writeTextSection(b *strings.Builder, label, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	fmt.Fprintf(b, "### %s\n\n%s\n\n", label, value)
}

func writeBulletSection(b *strings.Builder, label string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(b, "### %s\n\n", label)
	for _, item := range items {
		fmt.Fprintf(b, "- %s\n", item)
	}
	fmt.Fprintln(b)
}

func writeDeterministicAIFallback(b *strings.Builder, a *models.Analysis, ai *models.AISummary) {
	fmt.Fprintln(b, DeterministicAISummary(a))
	fmt.Fprintln(b)
	fmt.Fprintf(b, "Local AI summary unavailable: %s did not return usable project-summary text.\n\n", aiModelText(ai))
}

func DeterministicAISummary(a *models.Analysis) string {
	stackTerms := compactStackTerms(a.Stack)
	projectType := deterministicProjectType(a.Stack)
	if projectType != "" {
		stackTerms = removeProjectTypeTerms(stackTerms, projectType)
	}
	stackPhrase := "a local codebase"
	if projectType != "" && len(stackTerms) > 0 {
		stackPhrase = fmt.Sprintf("%s using %s", projectType, humanJoin(stackTerms))
	} else if projectType != "" {
		stackPhrase = projectType
	} else if len(stackTerms) > 0 {
		stackPhrase = "a project using " + humanJoin(stackTerms)
	}
	parts := []string{fmt.Sprintf("StackIndex detected this as %s.", withArticle(stackPhrase))}
	var readiness []string
	if a.Tests.HasTestFiles || a.Tests.HasTestScript {
		readiness = append(readiness, "tests")
	}
	if a.Deployment.HasHealthEndpoint {
		readiness = append(readiness, "health endpoints")
	}
	if a.Deployment.HasMigrationFiles {
		readiness = append(readiness, "migration files")
	}
	if a.Deployment.ReadmeMentionsDeploy {
		readiness = append(readiness, "deployment docs")
	}
	if a.Deployment.HasEnvExample {
		readiness = append(readiness, "an env example")
	}
	if len(readiness) > 0 {
		verb := "are"
		if len(readiness) == 1 {
			verb = "is"
		}
		parts = append(parts, "The project appears deployment-aware: "+humanJoin(readiness)+" "+verb+" present.")
	}
	if len(a.Findings) == 0 {
		parts = append(parts, "No actionable findings were detected.")
	} else {
		parts = append(parts, fmt.Sprintf("StackIndex found %s worth reviewing.", findingSummary(a.Findings)))
	}
	return strings.Join(parts, " ")
}

func removeProjectTypeTerms(terms []string, projectType string) []string {
	projectType = strings.ToLower(projectType)
	var out []string
	for _, term := range terms {
		key := strings.ToLower(strings.TrimSpace(term))
		if key == "" || strings.Contains(projectType, key) {
			continue
		}
		out = append(out, term)
	}
	return out
}

func writeIndentedBlock(b *strings.Builder, text string) {
	for _, line := range strings.Split(text, "\n") {
		line = strings.ReplaceAll(line, "```", "~~~")
		fmt.Fprintf(b, "    %s\n", line)
	}
	fmt.Fprintln(b)
}

func writeTopFixes(b *strings.Builder, a *models.Analysis) {
	if len(a.Findings) == 0 {
		fmt.Fprintln(b, "- No actionable findings. Keep the repo index current before handoffs or deeper agent work.")
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

func compactStackTerms(stack models.StackInfo) []string {
	seen := map[string]bool{}
	var out []string
	for _, group := range [][]string{stack.Frameworks, stack.Languages, stack.Databases, stack.Testing, stack.Deployment} {
		for _, term := range group {
			term = strings.TrimSpace(term)
			if term == "" {
				continue
			}
			key := strings.ToLower(term)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, term)
			if len(out) == 8 {
				return out
			}
		}
	}
	return out
}

func deterministicProjectType(stack models.StackInfo) string {
	frameworks := strings.ToLower(strings.Join(stack.Frameworks, " "))
	languages := strings.ToLower(strings.Join(stack.Languages, " "))
	switch {
	case strings.Contains(frameworks, "next.js") && strings.Contains(frameworks, "react"):
		return "a Next.js/React application"
	case strings.Contains(frameworks, "next.js"):
		return "a Next.js application"
	case strings.Contains(frameworks, "vite") && strings.Contains(frameworks, "react"):
		return "a Vite/React application"
	case strings.Contains(frameworks, "vite"):
		return "a Vite application"
	case strings.Contains(frameworks, "react"):
		return "a React application"
	case strings.Contains(frameworks, "express"):
		return "an Express application"
	case strings.Contains(languages, "go"):
		return "a Go application"
	case strings.Contains(languages, "typescript") || strings.Contains(languages, "javascript"):
		return "a TypeScript/JavaScript application"
	default:
		return ""
	}
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

func writeChangeSummary(b *strings.Builder, a *models.Analysis) {
	fmt.Fprintf(b, "## Changes Since Previous Snapshot\n\n")
	if a.Changes == nil || !a.Changes.HasPrevious {
		message := noPreviousSnapshotMessage
		if a.Changes != nil && a.Changes.Message != "" {
			message = a.Changes.Message
		}
		fmt.Fprintf(b, "%s\n\n", message)
		return
	}
	fmt.Fprintf(b, "- Previous snapshot: `%s`\n", a.Changes.PreviousSnapshot)
	fmt.Fprintf(b, "- Current generated: `%s`\n", a.Changes.GeneratedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(b, "- Audit status: `%s` -> `%s`\n\n", a.Changes.AuditStatusBefore, a.Changes.AuditStatusAfter)
	for _, bullet := range a.Changes.SummaryBullets {
		fmt.Fprintf(b, "- %s\n", bullet)
	}
	fmt.Fprintln(b)
	writeNameList(b, "Added routes", a.Changes.AddedRoutes)
	writeNameList(b, "Removed routes", a.Changes.RemovedRoutes)
	writeNameList(b, "Added env vars", a.Changes.AddedEnvVars)
	writeNameList(b, "Removed env vars", a.Changes.RemovedEnvVars)
	writeNameList(b, "Added findings", a.Changes.AddedFindings)
	writeNameList(b, "Resolved findings", a.Changes.ResolvedFindings)
	writeNameList(b, "Stack changes", a.Changes.StackChanges)
	writeNameList(b, "Framework changes", a.Changes.FrameworkChanges)
	writeNameList(b, "Database changes", a.Changes.DatabaseChanges)
	writeNameList(b, "Test signal changes", a.Changes.TestSignalChanges)
	writeNameList(b, "Deployment signal changes", a.Changes.DeploymentSignalChanges)
	writeNameList(b, "Key file changes", a.Changes.KeyFileChanges)
	fmt.Fprintln(b)
}

func present(ok bool) string {
	if ok {
		return "yes"
	}
	return "no"
}

func totalIgnoredDirs(counts map[string]int) int {
	total := 0
	for _, count := range counts {
		total += count
	}
	return total
}

func hasFrontendSurface(a *models.Analysis) bool {
	for _, file := range a.Files {
		lower := strings.ToLower(file.Path)
		if strings.HasPrefix(lower, "src/app/") || strings.Contains(lower, "/components/") || strings.HasSuffix(lower, ".tsx") || strings.HasSuffix(lower, ".jsx") {
			return true
		}
	}
	return false
}

func hasWorkerSurface(a *models.Analysis) bool {
	for _, file := range a.Files {
		if strings.Contains(strings.ToLower(file.Path), "worker") {
			return true
		}
	}
	for _, feature := range a.Features.Features {
		if strings.Contains(strings.ToLower(feature.Name), "worker") {
			return true
		}
	}
	return false
}

func withArticle(phrase string) string {
	if strings.HasPrefix(phrase, "a ") || strings.HasPrefix(phrase, "an ") || strings.HasPrefix(phrase, "the ") {
		return phrase
	}
	if phrase == "" {
		return "a local codebase"
	}
	switch strings.ToLower(phrase[:1]) {
	case "a", "e", "i", "o", "u":
		return "an " + phrase
	default:
		return "a " + phrase
	}
}

func humanJoin(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2:
		return items[0] + " and " + items[1]
	default:
		return strings.Join(items[:len(items)-1], ", ") + ", and " + items[len(items)-1]
	}
}

func aiModelText(ai *models.AISummary) string {
	if ai == nil {
		return "the selected model"
	}
	models := ai.AttemptedModels
	if len(models) == 0 && strings.TrimSpace(ai.Model) != "" {
		models = []string{ai.Model}
	}
	if len(models) == 0 {
		return "the selected model"
	}
	quoted := make([]string, 0, len(models))
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model != "" {
			quoted = append(quoted, "`"+model+"`")
		}
	}
	return humanJoin(quoted)
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func capReportStrings(in []string, limit int) []string {
	if len(in) <= limit {
		return in
	}
	return in[:limit]
}

func capReportDirectoryRoles(in []models.DirectoryRole, limit int) []models.DirectoryRole {
	if len(in) <= limit {
		return in
	}
	return in[:limit]
}

func capReportFileRoles(in []models.FileRole, limit int) []models.FileRole {
	if len(in) <= limit {
		return in
	}
	return in[:limit]
}

func capReportConnectedFiles(in []models.ConnectedFileSummary, limit int) []models.ConnectedFileSummary {
	if len(in) <= limit {
		return in
	}
	return in[:limit]
}
