package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/wcw-wcw/stackindex/internal/models"
)

const DefaultModel = "llama3.2:3b"
const FallbackModel = "qwen:7b"

const (
	defaultBaseURL = "http://127.0.0.1:11434"
	routeLimit     = 40
	findingLimit   = 20
	scriptLimit    = 12
	contextLimit   = 5
	structureLimit = 8
	fieldLimit     = 700
	itemLimit      = 220
)

const missingSectionFallback = "No AI summary was generated for this section."

var trailingCommaRE = regexp.MustCompile(`,\s*([}\]])`)
var envVarNameRE = regexp.MustCompile(`\b[A-Z][A-Z0-9]+(?:_[A-Z0-9]+)+\b`)

const (
	relevancePassed        = "passed"
	relevanceLowConfidence = "low_confidence"
)

const (
	statusGeneratedStructured      = "generated_structured"
	statusGeneratedText            = "generated_text"
	statusFallbackModelUnavailable = "fallback_model_unavailable"
	statusFallbackIrrelevant       = "fallback_irrelevant"
	statusFallbackEmpty            = "fallback_empty"
	statusFallbackParseFailed      = "fallback_parse_failed"
)

type OllamaClient struct {
	BaseURL string
	Model   string
	Client  *http.Client
}

type OllamaModelInfo struct {
	Name       string `json:"name"`
	ModifiedAt string `json:"modifiedAt,omitempty"`
	Size       int64  `json:"size,omitempty"`
}

type request struct {
	Model   string                 `json:"model"`
	Prompt  string                 `json:"prompt"`
	Stream  bool                   `json:"stream"`
	Format  string                 `json:"format,omitempty"`
	Options map[string]interface{} `json:"options,omitempty"`
}

type response struct {
	Response string `json:"response"`
	Error    string `json:"error,omitempty"`
}

type tagsResponse struct {
	Models []tagModel `json:"models"`
}

type tagModel struct {
	Name       string `json:"name"`
	ModifiedAt string `json:"modified_at"`
	Size       int64  `json:"size"`
}

type AIFactsheet struct {
	RepositoryName      string              `json:"repositoryName"`
	ScannedPath         string              `json:"scannedPath"`
	FilesScanned        int                 `json:"filesScanned"`
	FileCounts          map[string]int      `json:"fileCounts"`
	FindingCounts       map[string]int      `json:"findingCounts"`
	DetectedStack       aiDetectedStack     `json:"detectedStack"`
	ProjectContext      aiProjectContext    `json:"projectContext"`
	StructureSummary    aiStructureSummary  `json:"structureSummary"`
	DependencySummary   aiDependencySummary `json:"dependencySummary"`
	HealthSummary       aiHealthSummary     `json:"healthSummary"`
	PackageScripts      map[string]string   `json:"packageScripts,omitempty"`
	APIRoutes           []compactRoute      `json:"apiRoutes,omitempty"`
	APIRoutesTotal      int                 `json:"apiRoutesTotal"`
	Environment         compactEnv          `json:"environment"`
	DeploymentReadiness aiDeploymentSummary `json:"deploymentReadiness"`
	TopFindings         []compactFinding    `json:"topFindings,omitempty"`
	FindingsTotal       int                 `json:"findingsTotal"`
}

type aiProjectContext struct {
	Purpose            string   `json:"purpose,omitempty"`
	Confidence         string   `json:"confidence,omitempty"`
	Evidence           []string `json:"evidence,omitempty"`
	ReadmeTitle        string   `json:"readmeTitle,omitempty"`
	PackageName        string   `json:"packageName,omitempty"`
	PackageDescription string   `json:"packageDescription,omitempty"`
}

type aiStructureSummary struct {
	Directories []compactDirectoryRole `json:"directories,omitempty"`
	KeyFiles    []compactFileRole      `json:"keyFiles,omitempty"`
}

type aiDependencySummary struct {
	Entrypoints       []string               `json:"entrypoints,omitempty"`
	TopConnectedFiles []compactConnectedFile `json:"topConnectedFiles,omitempty"`
	ArchitectureHints []string               `json:"architectureHints,omitempty"`
	UnresolvedCount   int                    `json:"unresolvedCount"`
}

type compactDirectoryRole struct {
	Path string `json:"path"`
	Role string `json:"role"`
}

type compactFileRole struct {
	Path       string `json:"path"`
	Role       string `json:"role"`
	Importance string `json:"importance"`
}

type compactConnectedFile struct {
	Path            string `json:"path"`
	Role            string `json:"role,omitempty"`
	ImportsCount    int    `json:"importsCount"`
	ImportedByCount int    `json:"importedByCount"`
	WhyItMatters    string `json:"whyItMatters,omitempty"`
}

type aiDetectedStack struct {
	Languages         []string `json:"languages,omitempty"`
	Frameworks        []string `json:"frameworks,omitempty"`
	Databases         []string `json:"databases,omitempty"`
	TestingFrameworks []string `json:"testingFrameworks,omitempty"`
	DeploymentTargets []string `json:"deploymentTargets,omitempty"`
}

type aiHealthSummary struct {
	StackDetected         bool `json:"stackDetected"`
	TestsPresent          bool `json:"testsPresent"`
	HealthEndpointPresent bool `json:"healthEndpointPresent"`
	EnvExamplePresent     bool `json:"envExamplePresent"`
	MigrationFilesPresent bool `json:"migrationFilesPresent"`
	DeploymentDocsPresent bool `json:"deploymentDocsPresent"`
}

type aiDeploymentSummary struct {
	HasReadme                bool `json:"hasReadme"`
	ReadmeMentionsSetup      bool `json:"readmeMentionsSetup"`
	ReadmeMentionsDeploy     bool `json:"readmeMentionsDeploy"`
	HasEnvExample            bool `json:"hasEnvExample"`
	HasDockerfile            bool `json:"hasDockerfile"`
	HasVercelConfig          bool `json:"hasVercelConfig"`
	HasHealthEndpoint        bool `json:"hasHealthEndpoint"`
	HasMigrationFiles        bool `json:"hasMigrationFiles"`
	ReadmeMentionsMigrations bool `json:"readmeMentionsMigrations"`
}

type compactFinding struct {
	Severity       models.Severity `json:"severity"`
	Category       string          `json:"category"`
	Message        string          `json:"message"`
	Recommendation string          `json:"recommendation,omitempty"`
}

type compactRoute struct {
	Method     string `json:"method"`
	Path       string `json:"path"`
	SourceFile string `json:"sourceFile"`
	Confidence string `json:"confidence"`
	Note       string `json:"note,omitempty"`
}

type compactTests struct {
	HasTestFiles       bool     `json:"hasTestFiles"`
	HasTestScript      bool     `json:"hasTestScript"`
	Frameworks         []string `json:"frameworks,omitempty"`
	TestFileCount      int      `json:"testFileCount"`
	TestScript         string   `json:"testScript,omitempty"`
	PlaywrightDetected bool     `json:"playwrightDetected"`
}

type compactEnv struct {
	UsesEnvVars             bool           `json:"usesEnvVars"`
	HasExampleFile          bool           `json:"hasExampleFile"`
	EnvFilePresent          bool           `json:"envFilePresent"`
	UsedVarCount            int            `json:"usedVarCount"`
	ExampleVarCount         int            `json:"exampleVarCount"`
	MissingFromExampleCount int            `json:"missingFromExampleCount"`
	MissingRequiredCount    int            `json:"missingRequiredCount"`
	Classifications         map[string]int `json:"classifications,omitempty"`
}

type aiJSONResponse struct {
	ProjectSummary       string   `json:"projectSummary"`
	ArchitectureOverview string   `json:"architectureOverview"`
	KeyStrengths         []string `json:"keyStrengths"`
	PotentialRisks       []string `json:"potentialRisks"`
	RecommendedNextSteps []string `json:"recommendedNextSteps"`
}

type SummaryOptions struct {
	DebugDir       string
	FallbackModels []string
}

type DebugArtifacts struct {
	Factsheet       AIFactsheet `json:"factsheet"`
	Prompt          string      `json:"prompt"`
	RawResponse     string      `json:"rawResponse,omitempty"`
	RetryResponse   string      `json:"retryResponse,omitempty"`
	ParseError      string      `json:"parseError,omitempty"`
	Relevance       string      `json:"relevance,omitempty"`
	RelevanceReason string      `json:"relevanceReason,omitempty"`
	Warning         string      `json:"warning,omitempty"`
}

func Summarize(ctx context.Context, analysis *models.Analysis, model string) *models.AISummary {
	return SummarizeWithOptions(ctx, analysis, model, SummaryOptions{})
}

func SummarizeWithOptions(ctx context.Context, analysis *models.Analysis, model string, opts SummaryOptions) *models.AISummary {
	modelsToTry := modelCandidates(model, opts.FallbackModels)
	var last *models.AISummary
	var lastDebug DebugArtifacts
	var attempted []string
	for _, candidate := range modelsToTry {
		attempted = append(attempted, candidate)
		summary, debug := summarizeOne(ctx, analysis, candidate)
		summary.AttemptedModels = append([]string{}, attempted...)
		if isUsableAISummary(summary) {
			writeDebugIfEnabled(opts.DebugDir, debug, summary)
			return summary
		}
		last = summary
		lastDebug = debug
	}
	if last == nil {
		last = &models.AISummary{Enabled: true, Model: DefaultModel, AttemptedModels: []string{DefaultModel}, GeneratedAt: time.Now(), Status: statusFallbackEmpty}
		lastDebug = DebugArtifacts{Factsheet: BuildAIFactsheet(analysis), Prompt: promptFor(analysis)}
	}
	last.AttemptedModels = append([]string{}, attempted...)
	writeDebugIfEnabled(opts.DebugDir, lastDebug, last)
	return last
}

func modelCandidates(model string, fallbacks []string) []string {
	var candidates []string
	if strings.TrimSpace(model) == "" {
		candidates = append(candidates, DefaultModel, FallbackModel)
	} else {
		candidates = append(candidates, strings.TrimSpace(model))
	}
	candidates = append(candidates, fallbacks...)
	seen := map[string]bool{}
	out := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" || seen[candidate] {
			continue
		}
		seen[candidate] = true
		out = append(out, candidate)
	}
	return out
}

func summarizeOne(ctx context.Context, analysis *models.Analysis, model string) (*models.AISummary, DebugArtifacts) {
	client := OllamaClient{
		BaseURL: defaultBaseURL,
		Model:   model,
		Client:  &http.Client{Timeout: 45 * time.Second},
	}
	summary := &models.AISummary{Enabled: true, Model: model, GeneratedAt: time.Now()}
	debug := DebugArtifacts{
		Factsheet: BuildAIFactsheet(analysis),
		Prompt:    promptFor(analysis),
	}

	checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := client.CheckAvailable(checkCtx); err != nil {
		summary.Warning = fmt.Sprintf("AI summary was requested but Ollama was unavailable: %v", err)
		summary.Status = statusFallbackModelUnavailable
		debug.Warning = summary.Warning
		return summary, debug
	}

	generateCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	text, err := client.Generate(generateCtx, debug.Prompt)
	if err != nil {
		summary.Warning = fmt.Sprintf("AI summary was requested but Ollama failed: %v", err)
		if strings.Contains(strings.ToLower(err.Error()), "empty response") {
			summary.Status = statusFallbackEmpty
		} else {
			summary.Status = statusFallbackModelUnavailable
		}
		debug.Warning = summary.Warning
		return summary, debug
	}
	debug.RawResponse = text
	applyModelResponse(summary, text, analysis)
	if !isUsableAISummary(summary) {
		refineCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		defer cancel()
		refineText, err := client.Generate(refineCtx, refinementPromptFor(analysis, text, summary.RelevanceReason))
		if err == nil {
			debug.RetryResponse = refineText
			refined := &models.AISummary{Enabled: true, Model: model, GeneratedAt: summary.GeneratedAt}
			applyModelResponse(refined, refineText, analysis)
			if isUsableAISummary(refined) {
				*summary = *refined
			} else {
				summary.RetryRawText = cleanRawResponse(refineText)
			}
		}
	}
	debug.ParseError = summary.ParseError
	debug.Relevance = summary.Relevance
	debug.RelevanceReason = summary.RelevanceReason
	return summary, debug
}

func (c OllamaClient) CheckAvailable(ctx context.Context) error {
	if c.Client == nil {
		c.Client = &http.Client{Timeout: 3 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(c.BaseURL, "/")+"/api/tags", nil)
	if err != nil {
		return err
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ollama returned HTTP %d", resp.StatusCode)
	}
	return nil
}

func ListOllamaModels(ctx context.Context) ([]OllamaModelInfo, error) {
	listCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return listOllamaModels(listCtx, defaultBaseURL, &http.Client{Timeout: 3 * time.Second})
}

func listOllamaModels(ctx context.Context, baseURL string, client *http.Client) ([]OllamaModelInfo, error) {
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/api/tags", nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama is unavailable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ollama returned HTTP %d", resp.StatusCode)
	}
	var out tagsResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return nil, fmt.Errorf("ollama returned an invalid model list: %w", err)
	}
	models := make([]OllamaModelInfo, 0, len(out.Models))
	for _, model := range out.Models {
		name := strings.TrimSpace(model.Name)
		if name == "" {
			continue
		}
		models = append(models, OllamaModelInfo{
			Name:       name,
			ModifiedAt: model.ModifiedAt,
			Size:       model.Size,
		})
	}
	return models, nil
}

func (c OllamaClient) Generate(ctx context.Context, prompt string) (string, error) {
	if c.Client == nil {
		c.Client = &http.Client{Timeout: 45 * time.Second}
	}
	body, err := json.Marshal(request{
		Model:  c.Model,
		Prompt: prompt,
		Stream: false,
		Options: map[string]interface{}{
			"temperature": 0,
			"top_p":       0.2,
		},
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.BaseURL, "/")+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		if len(bytes.TrimSpace(msg)) > 0 {
			return "", fmt.Errorf("ollama returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
		}
		return "", fmt.Errorf("ollama returned HTTP %d", resp.StatusCode)
	}
	var out response
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.Error != "" {
		return "", errors.New(out.Error)
	}
	text := strings.TrimSpace(out.Response)
	if text == "" {
		return "", errors.New("ollama returned an empty response")
	}
	return text, nil
}

func writeDebugIfEnabled(debugDir string, artifacts DebugArtifacts, summary *models.AISummary) {
	if strings.TrimSpace(debugDir) == "" {
		return
	}
	if summary != nil {
		if artifacts.ParseError == "" {
			artifacts.ParseError = summary.ParseError
		}
		if artifacts.Relevance == "" {
			artifacts.Relevance = summary.Relevance
		}
		if artifacts.RelevanceReason == "" {
			artifacts.RelevanceReason = summary.RelevanceReason
		}
		if artifacts.Warning == "" {
			artifacts.Warning = summary.Warning
		}
	}
	_ = WriteDebugFiles(debugDir, artifacts)
}

func WriteDebugFiles(debugDir string, artifacts DebugArtifacts) error {
	if strings.TrimSpace(debugDir) == "" {
		return nil
	}
	if err := os.MkdirAll(debugDir, 0755); err != nil {
		return err
	}
	for _, name := range debugArtifactFileNames() {
		_ = os.Remove(filepath.Join(debugDir, name))
	}
	factsheet, err := json.MarshalIndent(artifacts.Factsheet, "", "  ")
	if err != nil {
		return err
	}
	files := map[string]string{
		"factsheet.json": string(factsheet) + "\n",
		"factsheet.txt":  factsheetText(artifacts.Factsheet),
		"prompt.txt":     artifacts.Prompt,
	}
	if artifacts.RawResponse != "" {
		files["raw-response.txt"] = artifacts.RawResponse
	}
	if artifacts.RetryResponse != "" {
		files["retry-response.txt"] = artifacts.RetryResponse
	}
	if artifacts.ParseError != "" {
		files["parse-error.txt"] = artifacts.ParseError + "\n"
	}
	relevance := artifacts.Relevance
	if relevance == "" {
		relevance = "not_evaluated"
	}
	data, err := json.MarshalIndent(map[string]string{
		"relevance":       relevance,
		"relevanceReason": artifacts.RelevanceReason,
	}, "", "  ")
	if err != nil {
		return err
	}
	files["relevance-result.json"] = string(data) + "\n"
	if artifacts.Warning != "" {
		files["warning.txt"] = artifacts.Warning + "\n"
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(debugDir, name), []byte(sanitizeDebugContent(content)), 0644); err != nil {
			return err
		}
	}
	return nil
}

func debugArtifactFileNames() []string {
	return []string{
		"factsheet.json",
		"factsheet.txt",
		"prompt.txt",
		"raw-response.txt",
		"retry-response.txt",
		"parse-error.txt",
		"relevance-result.json",
		"warning.txt",
	}
}

func factsheetText(f AIFactsheet) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Repository: %s\n", f.RepositoryName)
	fmt.Fprintf(&b, "Scanned path: %s\n", f.ScannedPath)
	fmt.Fprintf(&b, "Files scanned: %d\n", f.FilesScanned)
	fmt.Fprintf(&b, "Languages: %s\n", strings.Join(f.DetectedStack.Languages, ", "))
	fmt.Fprintf(&b, "Frameworks: %s\n", strings.Join(f.DetectedStack.Frameworks, ", "))
	fmt.Fprintf(&b, "Databases: %s\n", strings.Join(f.DetectedStack.Databases, ", "))
	fmt.Fprintf(&b, "Testing: %s\n", strings.Join(f.DetectedStack.TestingFrameworks, ", "))
	fmt.Fprintf(&b, "Deployment: %s\n", strings.Join(f.DetectedStack.DeploymentTargets, ", "))
	fmt.Fprintf(&b, "Tests present: %t\n", f.HealthSummary.TestsPresent)
	fmt.Fprintf(&b, "Health endpoint present: %t\n", f.HealthSummary.HealthEndpointPresent)
	fmt.Fprintf(&b, "Env example present: %t\n", f.HealthSummary.EnvExamplePresent)
	fmt.Fprintf(&b, "Migration files present: %t\n", f.HealthSummary.MigrationFilesPresent)
	fmt.Fprintf(&b, "Deployment docs present: %t\n", f.HealthSummary.DeploymentDocsPresent)
	fmt.Fprintf(&b, "Entrypoints: %s\n", strings.Join(f.DependencySummary.Entrypoints, ", "))
	fmt.Fprintf(&b, "Architecture hints: %s\n", strings.Join(f.DependencySummary.ArchitectureHints, "; "))
	fmt.Fprintf(&b, "Findings total: %d\n", f.FindingsTotal)
	return b.String()
}

func sanitizeDebugContent(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = sanitizeDebugLine(line)
	}
	return strings.Join(lines, "\n")
}

func sanitizeDebugLine(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "}") || strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "]") {
		return line
	}
	if eq := strings.Index(line, "="); eq > 0 && !strings.Contains(line[:eq], " ") {
		return line[:eq+1] + "[redacted]"
	}
	return line
}

func BuildAIFactsheet(a *models.Analysis) AIFactsheet {
	input := AIFactsheet{
		RepositoryName: a.RepoName,
		ScannedPath:    a.RepoPath,
		FilesScanned:   len(a.Files),
		FileCounts:     fileCounts(a.Files),
		FindingCounts:  findingCounts(a.Findings),
		DetectedStack: aiDetectedStack{
			Languages:         append([]string{}, a.Stack.Languages...),
			Frameworks:        append([]string{}, a.Stack.Frameworks...),
			Databases:         append([]string{}, a.Stack.Databases...),
			TestingFrameworks: append([]string{}, a.Stack.Testing...),
			DeploymentTargets: append([]string{}, a.Stack.Deployment...),
		},
		ProjectContext: aiProjectContext{
			Purpose:            capText(a.Context.Purpose, itemLimit),
			Confidence:         a.Context.Confidence,
			Evidence:           capStringFields(a.Context.Evidence, contextLimit),
			ReadmeTitle:        capText(a.Context.ReadmeTitle, itemLimit),
			PackageName:        capText(a.Context.PackageName, itemLimit),
			PackageDescription: capText(a.Context.PackageDescription, itemLimit),
		},
		StructureSummary:  compactStructureSummary(a.Structure),
		DependencySummary: compactDependencySummary(a.Dependencies),
		HealthSummary: aiHealthSummary{
			StackDetected:         stackDetected(a.Stack),
			TestsPresent:          a.Tests.HasTestFiles || a.Tests.HasTestScript,
			HealthEndpointPresent: a.Deployment.HasHealthEndpoint,
			EnvExamplePresent:     a.Deployment.HasEnvExample,
			MigrationFilesPresent: a.Deployment.HasMigrationFiles,
			DeploymentDocsPresent: a.Deployment.ReadmeMentionsDeploy,
		},
		APIRoutesTotal: len(a.Routes),
		Environment:    compactEnvFrom(a.Env),
		DeploymentReadiness: aiDeploymentSummary{
			HasReadme:                a.Deployment.HasReadme,
			ReadmeMentionsSetup:      a.Deployment.ReadmeMentionsSetup,
			ReadmeMentionsDeploy:     a.Deployment.ReadmeMentionsDeploy,
			HasEnvExample:            a.Deployment.HasEnvExample,
			HasDockerfile:            a.Deployment.HasDockerfile,
			HasVercelConfig:          a.Deployment.HasVercelConfig,
			HasHealthEndpoint:        a.Deployment.HasHealthEndpoint,
			HasMigrationFiles:        a.Deployment.HasMigrationFiles,
			ReadmeMentionsMigrations: a.Deployment.ReadmeMentionsMigrations,
		},
		TopFindings:   compactFindings(a.Findings, findingLimit),
		FindingsTotal: len(a.Findings),
	}
	if a.PackageInfo != nil {
		input.PackageScripts = cappedStringMap(a.PackageInfo.Scripts, scriptLimit)
	}
	for _, route := range capRoutes(a.Routes, routeLimit) {
		input.APIRoutes = append(input.APIRoutes, compactRoute{
			Method:     route.Method,
			Path:       route.Path,
			SourceFile: route.SourceFile,
			Confidence: route.Confidence,
			Note:       route.Note,
		})
	}
	return input
}

func compactDependencySummary(graph models.DependencyGraph) aiDependencySummary {
	out := aiDependencySummary{
		Entrypoints:       capStringFields(graph.Entrypoints, structureLimit),
		ArchitectureHints: capStringFields(graph.ArchitectureHints, contextLimit),
		UnresolvedCount:   len(graph.UnresolvedImports),
	}
	for _, file := range graph.TopConnectedFiles {
		out.TopConnectedFiles = append(out.TopConnectedFiles, compactConnectedFile{
			Path:            capText(file.Path, itemLimit),
			Role:            capText(file.Role, itemLimit),
			ImportsCount:    file.ImportsCount,
			ImportedByCount: file.ImportedByCount,
			WhyItMatters:    capText(file.WhyItMatters, itemLimit),
		})
		if len(out.TopConnectedFiles) == structureLimit {
			break
		}
	}
	return out
}

func compactStructureSummary(structure models.StructureMap) aiStructureSummary {
	var out aiStructureSummary
	for _, dir := range structure.Directories {
		out.Directories = append(out.Directories, compactDirectoryRole{Path: dir.Path, Role: capText(dir.Role, itemLimit)})
		if len(out.Directories) == structureLimit {
			break
		}
	}
	for _, file := range structure.KeyFiles {
		out.KeyFiles = append(out.KeyFiles, compactFileRole{Path: file.Path, Role: capText(file.Role, itemLimit), Importance: file.Importance})
		if len(out.KeyFiles) == structureLimit {
			break
		}
	}
	return out
}

func BuildCompactInput(a *models.Analysis) AIFactsheet {
	return BuildAIFactsheet(a)
}

func promptFor(a *models.Analysis) string {
	data, _ := json.MarshalIndent(BuildAIFactsheet(a), "", "  ")
	return `You are StackIndex, a local-only software engineering documentation assistant.

Write a concise plain-language project summary for a StackIndex report. Do not return JSON.
Use this exact shape:
One short paragraph, then 2 to 4 Markdown bullets.

You are summarizing only the StackIndex analysis factsheet below, not answering questions about individual file paths.
Do not explain Unix paths, source files, package names, or general programming concepts.
Do not list or define environment variables.
Use only the provided factsheet. Do not invent architecture, services, security issues, routes, dependencies, migrations, databases, monorepo structure, or deployment behavior. Do not claim to have read source files.
Mention the detected project type and concrete stack when available, such as languages, frameworks, databases, testing tools, and deployment targets.
Use dependencySummary facts to explain how main pieces fit together, which entrypoints exist, and what areas deserve review.
Mention at least one exact detected stack term from the factsheet, for example a language, framework, database, testing tool, or deployment target.
Keep the summary practical and bounded. Avoid generic advice unless it is supported by findings in the factsheet.
Do not mention a connection, entrypoint, database, migration, worker, route, or shared module unless it appears in the factsheet.

StackIndex analysis factsheet:
` + string(data)
}

func refinementPromptFor(a *models.Analysis, previous, reason string) string {
	data, _ := json.MarshalIndent(BuildAIFactsheet(a), "", "  ")
	if strings.TrimSpace(reason) == "" {
		reason = "The previous response was missing, irrelevant, or included unsupported details."
	}
	return `Your previous response was not usable as StackIndex local AI notes.

Rewrite it as concise plain text or Markdown. Do not return JSON.
Use this exact shape: one short paragraph, then 2 to 4 Markdown bullets.
You are summarizing only the StackIndex analysis factsheet below, not answering questions about individual file paths.
Do not explain Unix paths, source files, package names, or general programming concepts.
Do not list or define environment variables.
Use only the factsheet below and do not invent architecture, services, security issues, routes, dependencies, migrations, databases, monorepo structure, or deployment behavior.
Use dependencySummary facts to explain main pieces and entrypoints only when present.
Mention at least one exact detected stack term from the factsheet.

Why the previous response was rejected:
` + capText(reason, 500) + `

Previous invalid response, for context only:
` + capText(previous, 1200) + `

StackIndex analysis factsheet:
` + string(data)
}

func applyModelResponse(summary *models.AISummary, text string, analysis *models.Analysis) {
	cleaned := cleanRawResponse(text)
	summary.RawText = cleaned
	parsed, err := ParseModelResponse(text)
	if err != nil {
		summary.ParseError = err.Error()
		applyPlainTextResponse(summary, cleaned, analysis)
		return
	}
	summary.ProjectSummary = parsed.ProjectSummary
	summary.ArchitectureOverview = parsed.ArchitectureOverview
	summary.KeyStrengths = parsed.KeyStrengths
	summary.PotentialRisks = parsed.PotentialRisks
	summary.RecommendedNextSteps = parsed.RecommendedNextSteps
	markRelevance(summary, structuredText(summary), analysis)
	if summary.Relevance != relevanceLowConfidence {
		markUnsupportedStructuredClaims(summary, analysis)
	}
	if summary.Relevance == relevanceLowConfidence {
		summary.Status = statusFallbackIrrelevant
		return
	}
	summary.Status = statusGeneratedStructured
}

func applyPlainTextResponse(summary *models.AISummary, text string, analysis *models.Analysis) {
	text = normalizePlainSummary(text)
	if strings.TrimSpace(text) == "" {
		summary.Status = statusFallbackEmpty
		return
	}
	markRelevance(summary, text, analysis)
	if summary.Relevance != relevanceLowConfidence {
		markUnsupportedPlainClaims(summary, text, analysis)
	}
	if summary.Relevance == relevanceLowConfidence {
		summary.Status = statusFallbackIrrelevant
		return
	}
	summary.LocalNotes = text
	summary.Status = statusGeneratedText
}

func isUsableAISummary(summary *models.AISummary) bool {
	if summary == nil || summary.Warning != "" || summary.Relevance == relevanceLowConfidence {
		return false
	}
	return summary.Status == statusGeneratedStructured || summary.Status == statusGeneratedText
}

func ParseModelResponse(text string) (models.AISummary, error) {
	var parsed aiJSONResponse
	candidates := extractJSONObjects(text)
	if len(candidates) == 0 {
		return models.AISummary{}, errors.New("response did not contain a JSON object")
	}
	var lastErr error
	for _, candidate := range candidates {
		candidate = repairCommonJSON(candidate)
		if err := json.Unmarshal([]byte(candidate), &parsed); err != nil {
			lastErr = err
			continue
		}
		if !hasUsableParsedContent(parsed) {
			lastErr = errors.New("JSON summary object contained no usable text")
			continue
		}
		return cleanParsedSummary(parsed), nil
	}
	if lastErr == nil {
		lastErr = errors.New("response did not contain a valid JSON summary object")
	}
	return models.AISummary{}, lastErr
}

func extractJSONObject(text string) string {
	candidates := extractJSONObjects(text)
	if len(candidates) == 0 {
		return ""
	}
	return candidates[0]
}

func extractJSONObjects(text string) []string {
	text = strings.TrimSpace(text)
	depth := 0
	start := -1
	inString := false
	escaped := false
	var out []string
	for i, r := range text {
		if inString {
			if escaped {
				escaped = false
				continue
			}
			switch r {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch r {
		case '"':
			inString = true
		case '{':
			if depth == 0 {
				start = i
			}
			depth++
		case '}':
			if depth == 0 {
				continue
			}
			depth--
			if depth == 0 && start >= 0 {
				out = append(out, text[start:i+len(string(r))])
				start = -1
			}
		}
	}
	return out
}

func repairCommonJSON(text string) string {
	text = strings.TrimSpace(text)
	text = strings.ReplaceAll(text, "\uFF0C", ",")
	text = trailingCommaRE.ReplaceAllString(text, "$1")
	return text
}

func cleanParsedSummary(parsed aiJSONResponse) models.AISummary {
	return models.AISummary{
		ProjectSummary:       requiredString(parsed.ProjectSummary),
		ArchitectureOverview: requiredString(parsed.ArchitectureOverview),
		KeyStrengths:         cleanList(parsed.KeyStrengths),
		PotentialRisks:       cleanList(parsed.PotentialRisks),
		RecommendedNextSteps: cleanList(parsed.RecommendedNextSteps),
	}
}

func hasUsableParsedContent(parsed aiJSONResponse) bool {
	if strings.TrimSpace(parsed.ProjectSummary) != "" || strings.TrimSpace(parsed.ArchitectureOverview) != "" {
		return true
	}
	return len(cleanList(parsed.KeyStrengths))+len(cleanList(parsed.PotentialRisks))+len(cleanList(parsed.RecommendedNextSteps)) > 0
}

func requiredString(value string) string {
	value = capText(strings.TrimSpace(value), fieldLimit)
	if value == "" {
		return missingSectionFallback
	}
	return value
}

func cleanList(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = capText(strings.TrimSpace(item), itemLimit)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func capStringFields(items []string, limit int) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = capText(strings.TrimSpace(item), itemLimit)
		if item != "" {
			out = append(out, item)
			if limit > 0 && len(out) == limit {
				break
			}
		}
	}
	return out
}

func capText(text string, limit int) string {
	runes := []rune(text)
	if limit <= 0 || len(runes) <= limit {
		return text
	}
	if limit <= 3 {
		return string(runes[:limit])
	}
	return string(runes[:limit-3]) + "..."
}

func cleanRawResponse(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	lower := strings.ToLower(text)
	if strings.HasPrefix(lower, "```json") || strings.HasPrefix(text, "```") {
		lines := strings.Split(text, "\n")
		if len(lines) >= 2 && strings.HasPrefix(strings.TrimSpace(lines[0]), "```") && strings.TrimSpace(lines[len(lines)-1]) == "```" {
			text = strings.TrimSpace(strings.Join(lines[1:len(lines)-1], "\n"))
		}
	}
	if text == "" || text == "{}" || text == "[]" {
		return ""
	}
	candidates := extractJSONObjects(text)
	if len(candidates) == 1 {
		var parsed aiJSONResponse
		if err := json.Unmarshal([]byte(repairCommonJSON(candidates[0])), &parsed); err == nil && !hasUsableParsedContent(parsed) {
			return ""
		}
	}
	return capText(text, 3000)
}

func normalizePlainSummary(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	candidates := extractJSONObjects(text)
	if len(candidates) == 1 && strings.TrimSpace(candidates[0]) == text {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(text), "```json") {
		return ""
	}
	if strings.HasPrefix(text, "```") {
		lines := strings.Split(text, "\n")
		if len(lines) >= 2 && strings.HasPrefix(strings.TrimSpace(lines[0]), "```") && strings.TrimSpace(lines[len(lines)-1]) == "```" {
			text = strings.TrimSpace(strings.Join(lines[1:len(lines)-1], "\n"))
		}
	}
	return capText(text, 2000)
}

func markRelevance(summary *models.AISummary, text string, analysis *models.Analysis) {
	if strings.TrimSpace(text) == "" {
		return
	}
	terms := relevanceTerms(analysis)
	if len(terms) == 0 {
		summary.Relevance = relevancePassed
		summary.RelevanceReason = "No detected stack terms were available for relevance validation."
		return
	}
	matched := matchingTerm(text, terms)
	if matched == "" {
		summary.Relevance = relevanceLowConfidence
		summary.RelevanceReason = "Model output did not mention any detected language, framework, database, testing tool, or deployment target."
		return
	}
	summary.Relevance = relevancePassed
	summary.RelevanceReason = fmt.Sprintf("Model output mentioned detected stack term %q.", matched)
}

func markUnsupportedStructuredClaims(summary *models.AISummary, analysis *models.Analysis) {
	reason := unsupportedStructuredClaimReason(structuredText(summary), analysis)
	if reason == "" {
		return
	}
	summary.Relevance = relevanceLowConfidence
	summary.RelevanceReason = reason
}

func markUnsupportedPlainClaims(summary *models.AISummary, text string, analysis *models.Analysis) {
	reason := unsupportedStructuredClaimReason(text, analysis)
	if reason == "" {
		return
	}
	summary.Relevance = relevanceLowConfidence
	summary.RelevanceReason = reason
}

func unsupportedStructuredClaimReason(text string, analysis *models.Analysis) string {
	lower := strings.ToLower(text)
	if strings.Contains(lower, "microservice") || strings.Contains(lower, "monolithic") || strings.Contains(lower, "server-side rendering") || strings.Contains(lower, "server side rendering") {
		return "Model output described service topology, but StackIndex does not detect service topology."
	}
	if strings.Contains(lower, "monorepo") {
		return "Model output described a monorepo, but StackIndex does not detect repository topology."
	}
	if envVarNameRE.MatchString(text) {
		return "Model output listed environment variable names, but StackIndex AI notes only summarize environment readiness counts."
	}
	if mentionsFactsheetMeta(lower) {
		return "Model output summarized StackIndex factsheet field names instead of the project."
	}
	if mentionsDatabase(lower) && (analysis == nil || len(analysis.Stack.Databases) == 0) {
		return "Model output mentioned database/storage details, but StackIndex did not detect a database or storage layer."
	}
	if strings.Contains(lower, "migration") && (analysis == nil || !analysis.Deployment.HasMigrationFiles) {
		return "Model output mentioned migrations, but StackIndex did not detect migration files."
	}
	if strings.Contains(lower, "missing migration") && analysis != nil && analysis.Deployment.HasMigrationFiles {
		return "Model output claimed migration files were missing, but StackIndex detected migration files."
	}
	if mentionsMissingRequiredEnv(lower) && (analysis == nil || len(analysis.Env.MissingRequiredFromExample) == 0) {
		return "Model output claimed required environment variables were missing, but StackIndex did not detect missing required environment variables."
	}
	if mentionsSecurityFinding(lower) && !hasFindingCategory(analysis, "security") {
		return "Model output mentioned security findings, but StackIndex did not detect security findings."
	}
	if mentionsTestCoverageClaim(lower) {
		return "Model output claimed test coverage, but StackIndex only detects test files, test scripts, and test tooling."
	}
	if mentionsMissingTests(lower) && analysis != nil && (analysis.Tests.HasTestFiles || analysis.Tests.HasTestScript) {
		return "Model output claimed tests were missing or insufficient, but StackIndex detected tests."
	}
	if strings.Contains(lower, "strong testing") && (analysis == nil || (!analysis.Tests.HasTestFiles && !analysis.Tests.HasTestScript)) {
		return "Model output claimed a strong testing posture, but StackIndex did not detect tests."
	}
	if strings.Contains(lower, "deployment-ready") || strings.Contains(lower, "production-ready") {
		return "Model output claimed deployment or production readiness, but StackIndex only reports readiness signals."
	}
	if strings.Contains(lower, "currently deployed") || strings.Contains(lower, "deployed on ") || strings.Contains(lower, "deployed to ") {
		return "Model output claimed current deployment state, but StackIndex only detects deployment targets and configuration signals."
	}
	if strings.Contains(lower, "requires ") {
		return "Model output claimed project requirements, but StackIndex only reports detected project facts."
	}
	if strings.Contains(lower, "reachable") {
		return "Model output claimed runtime reachability, but StackIndex does not execute or probe the application."
	}
	if strings.Contains(lower, "critical nature") || strings.Contains(lower, "mission-critical") || strings.Contains(lower, "business-critical") {
		return "Model output characterized project criticality, but StackIndex does not detect business or operational criticality."
	}
	return ""
}

func mentionsDatabase(lower string) bool {
	for _, term := range []string{"database", "postgres", "postgresql", "neon", "sqlite", "prisma", "drizzle"} {
		if strings.Contains(lower, term) {
			return true
		}
	}
	return false
}

func mentionsFactsheetMeta(lower string) bool {
	for _, term := range []string{"provided information", "provided data", "factsheet", "stackdetected", "healthsummary", "topfindings", "findingstotal", "dependencysummary", "architecturesummary"} {
		if strings.Contains(lower, term) {
			return true
		}
	}
	return false
}

func mentionsSecurityFinding(lower string) bool {
	for _, term := range []string{"security header", "security vulnerability", "security vulnerabilities", "security measure", "security measures", "security risk", "security risks", "security issue", "security issues", "security finding", "security findings"} {
		if strings.Contains(lower, term) {
			return true
		}
	}
	return false
}

func mentionsMissingTests(lower string) bool {
	for _, term := range []string{"no tests", "no test files", "lack of testing", "insufficient testing"} {
		if strings.Contains(lower, term) {
			return true
		}
	}
	return false
}

func mentionsMissingRequiredEnv(lower string) bool {
	for _, term := range []string{"missing required variable", "missing required environment", "required variable missing", "required environment variable missing"} {
		if strings.Contains(lower, term) {
			return true
		}
	}
	return false
}

func mentionsTestCoverageClaim(lower string) bool {
	for _, term := range []string{"tested by", "covered by tests", "test coverage", "well-tested", "well tested"} {
		if strings.Contains(lower, term) {
			return true
		}
	}
	return false
}

func hasFindingCategory(analysis *models.Analysis, category string) bool {
	if analysis == nil {
		return false
	}
	for _, finding := range analysis.Findings {
		if strings.EqualFold(finding.Category, category) {
			return true
		}
	}
	return false
}

func structuredText(summary *models.AISummary) string {
	var parts []string
	parts = append(parts, summary.ProjectSummary, summary.ArchitectureOverview)
	parts = append(parts, summary.KeyStrengths...)
	parts = append(parts, summary.PotentialRisks...)
	parts = append(parts, summary.RecommendedNextSteps...)
	return strings.Join(parts, "\n")
}

func relevanceTerms(a *models.Analysis) []string {
	if a == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, group := range [][]string{a.Stack.Languages, a.Stack.Frameworks, a.Stack.Databases, a.Stack.Testing, a.Stack.Deployment} {
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
		}
	}
	return out
}

func matchingTerm(text string, terms []string) string {
	lower := strings.ToLower(text)
	for _, term := range terms {
		if containsTerm(lower, strings.ToLower(term)) {
			return term
		}
	}
	return ""
}

func containsTerm(lowerText, lowerTerm string) bool {
	if lowerTerm == "" {
		return false
	}
	if strings.ContainsAny(lowerTerm, ".#+-/ ") {
		return strings.Contains(lowerText, lowerTerm)
	}
	for _, field := range strings.FieldsFunc(lowerText, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	}) {
		if field == lowerTerm {
			return true
		}
	}
	return false
}

func fileCounts(files []models.FileInfo) map[string]int {
	counts := map[string]int{
		string(models.FileKindSource): 0,
		string(models.FileKindConfig): 0,
		string(models.FileKindTest):   0,
		string(models.FileKindDoc):    0,
		string(models.FileKindOther):  0,
	}
	for _, file := range files {
		counts[string(file.Kind)]++
	}
	return counts
}

func findingCounts(findings []models.Finding) map[string]int {
	counts := map[string]int{
		string(models.SeverityHigh):   0,
		string(models.SeverityMedium): 0,
		string(models.SeverityLow):    0,
		string(models.SeverityInfo):   0,
	}
	for _, finding := range findings {
		counts[string(finding.Severity)]++
	}
	return counts
}

func compactTestsFrom(t models.TestAnalysis) compactTests {
	return compactTests{
		HasTestFiles:       t.HasTestFiles,
		HasTestScript:      t.HasTestScript,
		Frameworks:         append([]string{}, t.Frameworks...),
		TestFileCount:      len(t.TestFiles),
		TestScript:         t.TestScript,
		PlaywrightDetected: t.PlaywrightDetected,
	}
}

func compactEnvFrom(env models.EnvAnalysis) compactEnv {
	out := compactEnv{
		UsesEnvVars:             env.UsesEnvVars,
		HasExampleFile:          env.ExampleFile != "",
		EnvFilePresent:          env.EnvFilePresent,
		UsedVarCount:            len(env.UsedVars),
		ExampleVarCount:         len(env.ExampleVars),
		MissingFromExampleCount: len(env.MissingFromExample),
		MissingRequiredCount:    len(env.MissingRequiredFromExample),
		Classifications:         map[string]int{},
	}
	for _, envVar := range env.UsedVars {
		class := envVar.Classification
		if class == "" {
			class = "unclassified"
		}
		out.Classifications[class]++
	}
	if len(out.Classifications) == 0 {
		out.Classifications = nil
	}
	return out
}

func compactFindings(findings []models.Finding, limit int) []compactFinding {
	capped := cappedFindings(findings, limit)
	out := make([]compactFinding, 0, len(capped))
	for _, finding := range capped {
		out = append(out, compactFinding{
			Severity:       finding.Severity,
			Category:       finding.Category,
			Message:        finding.Message,
			Recommendation: finding.Recommendation,
		})
	}
	return out
}

func capRoutes(routes []models.RouteInfo, limit int) []models.RouteInfo {
	if len(routes) <= limit {
		return append([]models.RouteInfo{}, routes...)
	}
	return append([]models.RouteInfo{}, routes[:limit]...)
}

func cappedFindings(findings []models.Finding, limit int) []models.Finding {
	if len(findings) <= limit {
		return append([]models.Finding{}, findings...)
	}
	return append([]models.Finding{}, findings[:limit]...)
}

func cappedStringMap(in map[string]string, limit int) map[string]string {
	if len(in) == 0 {
		return nil
	}
	keys := make([]string, 0, len(in))
	for key := range in {
		keys = append(keys, key)
	}
	sortStrings(keys)
	if limit > 0 && len(keys) > limit {
		keys = keys[:limit]
	}
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		out[key] = capText(in[key], itemLimit)
	}
	return out
}

func stackDetected(stack models.StackInfo) bool {
	return len(stack.Languages)+len(stack.Frameworks)+len(stack.Libraries)+len(stack.Databases)+len(stack.Testing)+len(stack.Deployment) > 0
}

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		value := values[i]
		j := i - 1
		for j >= 0 && values[j] > value {
			values[j+1] = values[j]
			j--
		}
		values[j+1] = value
	}
}
