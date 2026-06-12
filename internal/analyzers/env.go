package analyzers

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/will/stackmap/internal/models"
)

var envPatterns = []*regexp.Regexp{
	regexp.MustCompile(`process\.env\.([A-Za-z_][A-Za-z0-9_]*)`),
	regexp.MustCompile(`import\.meta\.env\.([A-Za-z_][A-Za-z0-9_]*)`),
	regexp.MustCompile(`Deno\.env\.get\(["']([A-Za-z_][A-Za-z0-9_]*)["']\)`),
	regexp.MustCompile(`os\.Getenv\(["']([A-Za-z_][A-Za-z0-9_]*)["']\)`),
}

func AnalyzeEnv(root string, files []models.FileInfo) (models.EnvAnalysis, []models.Finding) {
	used := map[string]map[string]bool{}
	var exampleFile string
	exampleVars := map[string]bool{}
	envFilePresent := false

	for _, file := range files {
		base := filepath.Base(file.Path)
		if base == ".env.example" {
			exampleFile = file.Path
			for _, name := range parseEnvExample(filepath.Join(root, file.Path)) {
				exampleVars[name] = true
			}
			continue
		}
		if base == ".env" {
			envFilePresent = true
			continue
		}
		if file.Kind != models.FileKindSource && file.Kind != models.FileKindConfig {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, file.Path))
		if err != nil {
			continue
		}
		for _, name := range ExtractEnvVars(string(data)) {
			if used[name] == nil {
				used[name] = map[string]bool{}
			}
			used[name][file.Path] = true
		}
	}

	result := models.EnvAnalysis{UsesEnvVars: len(used) > 0, ExampleFile: exampleFile, EnvFilePresent: envFilePresent}
	for name := range exampleVars {
		result.ExampleVars = append(result.ExampleVars, name)
	}
	sort.Strings(result.ExampleVars)
	for name, fileSet := range used {
		var paths []string
		for path := range fileSet {
			paths = append(paths, path)
		}
		sort.Strings(paths)
		result.UsedVars = append(result.UsedVars, models.EnvVar{Name: name, Files: paths})
		if exampleFile == "" || !exampleVars[name] {
			result.MissingFromExample = append(result.MissingFromExample, name)
		}
	}
	sort.Slice(result.UsedVars, func(i, j int) bool { return result.UsedVars[i].Name < result.UsedVars[j].Name })
	sort.Strings(result.MissingFromExample)

	var findings []models.Finding
	if len(used) > 0 && exampleFile == "" {
		findings = append(findings, models.Finding{Severity: models.SeverityMedium, Category: "env", Message: "Environment variables are used, but no .env.example file was found.", Recommendation: "Add a .env.example with variable names and safe placeholder values."})
	} else if len(result.MissingFromExample) > 0 {
		findings = append(findings, models.Finding{Severity: models.SeverityMedium, Category: "env", Message: "Some environment variables used in code are missing from .env.example.", File: exampleFile, Recommendation: "Document the missing variables in .env.example."})
	}
	for _, name := range result.ExampleVars {
		if secretLike(name) && envExampleHasRealLookingValue(filepath.Join(root, exampleFile), name) {
			findings = append(findings, models.Finding{Severity: models.SeverityHigh, Category: "env", Message: "A secret-looking variable in .env.example appears to have a concrete value.", File: exampleFile, Recommendation: "Use safe placeholders in .env.example and keep real values only in local .env files."})
			break
		}
	}
	return result, findings
}

func ExtractEnvVars(content string) []string {
	seen := map[string]bool{}
	for _, re := range envPatterns {
		for _, match := range re.FindAllStringSubmatch(content, -1) {
			seen[match[1]] = true
		}
	}
	var names []string
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func parseEnvExample(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		name := strings.TrimSpace(strings.SplitN(line, "=", 2)[0])
		if name != "" {
			seen[name] = true
		}
	}
	var names []string
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func secretLike(name string) bool {
	upper := strings.ToUpper(name)
	return strings.Contains(upper, "SECRET") || strings.Contains(upper, "TOKEN") || strings.Contains(upper, "KEY") || strings.Contains(upper, "PASSWORD")
}

func envExampleHasRealLookingValue(path, name string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, name+"=") {
			value := strings.TrimSpace(strings.SplitN(line, "=", 2)[1])
			if value == "" {
				return false
			}
			lower := strings.ToLower(value)
			return lower != "changeme" && lower != "placeholder" && lower != "example" && !strings.Contains(lower, "your_") && !strings.Contains(lower, "<")
		}
	}
	return false
}
