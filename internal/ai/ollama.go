package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/will/stackmap/internal/models"
)

const DefaultModel = "qwen2.5-coder:7b"

const (
	defaultBaseURL = "http://127.0.0.1:11434"
	routeLimit     = 40
	findingLimit   = 20
	scriptLimit    = 12
	fieldLimit     = 700
	itemLimit      = 220
)

const missingSectionFallback = "No AI summary was generated for this section."

var trailingCommaRE = regexp.MustCompile(`,\s*([}\]])`)

const (
	relevancePassed        = "passed"
	relevanceLowConfidence = "low_confidence"
)

type OllamaClient struct {
	BaseURL string
	Model   string
	Client  *http.Client
}

type request struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
	Format string `json:"format,omitempty"`
}

type response struct {
	Response string `json:"response"`
	Error    string `json:"error,omitempty"`
}

type AIFactsheet struct {
	RepositoryName      string              `json:"repositoryName"`
	ScannedPath         string              `json:"scannedPath"`
	FilesScanned        int                 `json:"filesScanned"`
	FileCounts          map[string]int      `json:"fileCounts"`
	FindingCounts       map[string]int      `json:"findingCounts"`
	DetectedStack       aiDetectedStack     `json:"detectedStack"`
	HealthSummary       aiHealthSummary     `json:"healthSummary"`
	PackageScripts      map[string]string   `json:"packageScripts,omitempty"`
	APIRoutes           []compactRoute      `json:"apiRoutes,omitempty"`
	APIRoutesTotal      int                 `json:"apiRoutesTotal"`
	Environment         compactEnv          `json:"environment"`
	DeploymentReadiness aiDeploymentSummary `json:"deploymentReadiness"`
	TopFindings         []compactFinding    `json:"topFindings,omitempty"`
	FindingsTotal       int                 `json:"findingsTotal"`
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

func Summarize(ctx context.Context, analysis *models.Analysis, model string) *models.AISummary {
	if model == "" {
		model = DefaultModel
	}
	client := OllamaClient{
		BaseURL: defaultBaseURL,
		Model:   model,
		Client:  &http.Client{Timeout: 45 * time.Second},
	}
	summary := &models.AISummary{Enabled: true, Model: model, GeneratedAt: time.Now()}

	checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := client.CheckAvailable(checkCtx); err != nil {
		summary.Warning = fmt.Sprintf("AI summary was requested but Ollama was unavailable: %v", err)
		return summary
	}

	generateCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	text, err := client.Generate(generateCtx, promptFor(analysis))
	if err != nil {
		summary.Warning = fmt.Sprintf("AI summary was requested but Ollama failed: %v", err)
		return summary
	}
	applyModelResponse(summary, text, analysis)
	if summary.ParseError != "" {
		refineCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		defer cancel()
		refineText, err := client.Generate(refineCtx, refinementPromptFor(analysis, text))
		if err == nil {
			refined := &models.AISummary{Enabled: true, Model: model, GeneratedAt: summary.GeneratedAt}
			applyModelResponse(refined, refineText, analysis)
			if refined.ParseError == "" && refined.Relevance != relevanceLowConfidence {
				*summary = *refined
			}
		}
	}
	return summary
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

func (c OllamaClient) Generate(ctx context.Context, prompt string) (string, error) {
	if c.Client == nil {
		c.Client = &http.Client{Timeout: 45 * time.Second}
	}
	body, err := json.Marshal(request{Model: c.Model, Prompt: prompt, Stream: false, Format: "json"})
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

func BuildCompactInput(a *models.Analysis) AIFactsheet {
	return BuildAIFactsheet(a)
}

func promptFor(a *models.Analysis) string {
	data, _ := json.MarshalIndent(BuildAIFactsheet(a), "", "  ")
	return `You are StackMap, a local-only software engineering documentation assistant.

Return only valid JSON. Do not wrap the response in Markdown. Do not use code fences.
You are summarizing the StackMap analysis factsheet below, not answering questions about individual file paths.
Do not explain Unix paths, source files, package names, or general programming concepts.
Do not list or define environment variables.
Use only the provided factsheet. Do not invent features, services, routes, dependencies, or deployment behavior. Do not claim to have read source files. Keep every value concise and practical.
Mention the detected project type and concrete stack when available, such as languages, frameworks, databases, testing tools, and deployment targets.
At least one output field must mention an exact detected stack term from the factsheet, for example a language, framework, database, testing tool, or deployment target.
Fill the fields with concrete content from the analysis. Do not return empty strings. Do not return placeholder values. When the analysis supports it, include 2 to 5 concise string items in each array.

Every field is required. Arrays must contain strings only. Return this exact JSON schema:
{
  "projectSummary": "string",
  "architectureOverview": "string",
  "keyStrengths": ["string"],
  "potentialRisks": ["string"],
  "recommendedNextSteps": ["string"]
}

Example of valid output:
{
  "projectSummary": "shop-demo is a Next.js and React web app with PostgreSQL integration and Vercel deployment signals.",
  "architectureOverview": "The app uses Next.js routes for HTTP APIs, package scripts for build/test workflows, and deployment readiness checks for env examples and health endpoints.",
  "keyStrengths": ["Next.js API routes are detected", "PostgreSQL and Vercel are explicitly identified"],
  "potentialRisks": ["Document any missing required environment variables before deployment"],
  "recommendedNextSteps": ["Run the detected Vitest test script before release", "Keep Vercel deployment notes current"]
}

StackMap analysis factsheet:
` + string(data)
}

func refinementPromptFor(a *models.Analysis, previous string) string {
	data, _ := json.MarshalIndent(BuildAIFactsheet(a), "", "  ")
	return `Your previous response could not be parsed as the required StackMap JSON summary.

Return only valid JSON. Do not wrap the response in Markdown. Do not use code fences. Do not include explanations.
You are summarizing the StackMap analysis factsheet below, not answering questions about individual file paths.
Do not explain Unix paths, source files, package names, or general programming concepts.
Do not list or define environment variables.
Every field is required. Arrays must contain strings only. Keep values concise. Use only the factsheet below and do not invent details.
Mention the detected project type and concrete stack when available.
At least one output field must mention an exact detected stack term from the factsheet, for example a language, framework, database, testing tool, or deployment target.
Fill the fields with concrete content from the analysis. Do not return empty strings. Do not return placeholder values. When the analysis supports it, include 2 to 5 concise string items in each array.

Required schema:
{
  "projectSummary": "string",
  "architectureOverview": "string",
  "keyStrengths": ["string"],
  "potentialRisks": ["string"],
  "recommendedNextSteps": ["string"]
}

Previous invalid response, for context only:
` + capText(previous, 1200) + `

StackMap analysis factsheet:
` + string(data)
}

func applyModelResponse(summary *models.AISummary, text string, analysis *models.Analysis) {
	parsed, err := ParseModelResponse(text)
	if err != nil {
		summary.RawText = cleanRawResponse(text)
		summary.ParseError = err.Error()
		markRelevance(summary, summary.RawText, analysis)
		return
	}
	summary.ProjectSummary = parsed.ProjectSummary
	summary.ArchitectureOverview = parsed.ArchitectureOverview
	summary.KeyStrengths = parsed.KeyStrengths
	summary.PotentialRisks = parsed.PotentialRisks
	summary.RecommendedNextSteps = parsed.RecommendedNextSteps
	markRelevance(summary, structuredText(summary), analysis)
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
