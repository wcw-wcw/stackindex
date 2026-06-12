package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/will/stackmap/internal/models"
)

const DefaultModel = "qwen2.5-coder:7b"

const (
	defaultBaseURL = "http://127.0.0.1:11434"
	routeLimit     = 40
	findingLimit   = 20
	fileLimit      = 30
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
}

type response struct {
	Response string `json:"response"`
	Error    string `json:"error,omitempty"`
}

type CompactInput struct {
	RepoName          string                    `json:"repoName"`
	Stack             models.StackInfo          `json:"stack"`
	FileCounts        map[models.FileKind]int   `json:"fileCounts"`
	PackageScripts    map[string]string         `json:"packageScripts,omitempty"`
	Routes            []compactRoute            `json:"routes,omitempty"`
	RoutesTotal       int                       `json:"routesTotal"`
	Tests             compactTests              `json:"tests"`
	Deployment        models.DeploymentAnalysis `json:"deployment"`
	Env               compactEnv                `json:"env"`
	Findings          []models.Finding          `json:"findings,omitempty"`
	FindingsTotal     int                       `json:"findingsTotal"`
	TopImportantFiles []string                  `json:"topImportantFiles,omitempty"`
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
	UsesEnvVars                bool           `json:"usesEnvVars"`
	HasExampleFile             bool           `json:"hasExampleFile"`
	EnvFilePresent             bool           `json:"envFilePresent"`
	UsedVarCount               int            `json:"usedVarCount"`
	ExampleVarCount            int            `json:"exampleVarCount"`
	MissingFromExample         []string       `json:"missingFromExample,omitempty"`
	MissingRequiredFromExample []string       `json:"missingRequiredFromExample,omitempty"`
	Classifications            map[string]int `json:"classifications,omitempty"`
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
	applyModelResponse(summary, text)
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
	body, err := json.Marshal(request{Model: c.Model, Prompt: prompt, Stream: false})
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

func BuildCompactInput(a *models.Analysis) CompactInput {
	input := CompactInput{
		RepoName:      a.RepoName,
		Stack:         a.Stack,
		FileCounts:    fileCounts(a.Files),
		RoutesTotal:   len(a.Routes),
		Tests:         compactTestsFrom(a.Tests),
		Deployment:    a.Deployment,
		Env:           compactEnvFrom(a.Env),
		Findings:      cappedFindings(a.Findings, findingLimit),
		FindingsTotal: len(a.Findings),
	}
	if a.PackageInfo != nil {
		input.PackageScripts = copyStringMap(a.PackageInfo.Scripts)
	}
	for _, route := range capRoutes(a.Routes, routeLimit) {
		input.Routes = append(input.Routes, compactRoute{
			Method:     route.Method,
			Path:       route.Path,
			SourceFile: route.SourceFile,
			Confidence: route.Confidence,
			Note:       route.Note,
		})
	}
	input.TopImportantFiles = importantFiles(a.Files, fileLimit)
	return input
}

func promptFor(a *models.Analysis) string {
	data, _ := json.MarshalIndent(BuildCompactInput(a), "", "  ")
	return `You are StackMap, a local-only software engineering documentation assistant.

Use only the provided static analysis data. Do not invent features, services, routes, dependencies, or deployment behavior. Do not claim to have read source files. Be concise, practical, and useful to engineers.

Return only valid JSON with this exact shape:
{
  "projectSummary": "...",
  "architectureOverview": "...",
  "keyStrengths": ["..."],
  "potentialRisks": ["..."],
  "recommendedNextSteps": ["..."]
}

Analysis data:
` + string(data)
}

func applyModelResponse(summary *models.AISummary, text string) {
	parsed, err := ParseModelResponse(text)
	if err != nil {
		summary.RawText = strings.TrimSpace(text)
		return
	}
	summary.ProjectSummary = parsed.ProjectSummary
	summary.ArchitectureOverview = parsed.ArchitectureOverview
	summary.KeyStrengths = parsed.KeyStrengths
	summary.PotentialRisks = parsed.PotentialRisks
	summary.RecommendedNextSteps = parsed.RecommendedNextSteps
}

func ParseModelResponse(text string) (models.AISummary, error) {
	var parsed aiJSONResponse
	candidate := extractJSONObject(strings.TrimSpace(text))
	if candidate == "" {
		return models.AISummary{}, errors.New("response did not contain a JSON object")
	}
	if err := json.Unmarshal([]byte(candidate), &parsed); err != nil {
		return models.AISummary{}, err
	}
	return models.AISummary{
		ProjectSummary:       strings.TrimSpace(parsed.ProjectSummary),
		ArchitectureOverview: strings.TrimSpace(parsed.ArchitectureOverview),
		KeyStrengths:         trimList(parsed.KeyStrengths),
		PotentialRisks:       trimList(parsed.PotentialRisks),
		RecommendedNextSteps: trimList(parsed.RecommendedNextSteps),
	}, nil
}

func extractJSONObject(text string) string {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start == -1 || end == -1 || end < start {
		return ""
	}
	return text[start : end+1]
}

func fileCounts(files []models.FileInfo) map[models.FileKind]int {
	counts := map[models.FileKind]int{
		models.FileKindSource: 0,
		models.FileKindConfig: 0,
		models.FileKindTest:   0,
		models.FileKindDoc:    0,
		models.FileKindOther:  0,
	}
	for _, file := range files {
		counts[file.Kind]++
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
		UsesEnvVars:                env.UsesEnvVars,
		HasExampleFile:             env.ExampleFile != "",
		EnvFilePresent:             env.EnvFilePresent,
		UsedVarCount:               len(env.UsedVars),
		ExampleVarCount:            len(env.ExampleVars),
		MissingFromExample:         append([]string{}, env.MissingFromExample...),
		MissingRequiredFromExample: append([]string{}, env.MissingRequiredFromExample...),
		Classifications:            map[string]int{},
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

func importantFiles(files []models.FileInfo, limit int) []string {
	weights := map[models.FileKind]int{
		models.FileKindConfig: 0,
		models.FileKindDoc:    1,
		models.FileKindSource: 2,
		models.FileKindTest:   3,
		models.FileKindOther:  4,
	}
	candidates := append([]models.FileInfo{}, files...)
	sort.SliceStable(candidates, func(i, j int) bool {
		wi := fileWeight(candidates[i], weights)
		wj := fileWeight(candidates[j], weights)
		if wi != wj {
			return wi < wj
		}
		if candidates[i].SizeBytes != candidates[j].SizeBytes {
			return candidates[i].SizeBytes > candidates[j].SizeBytes
		}
		return candidates[i].Path < candidates[j].Path
	})
	var out []string
	for _, file := range candidates {
		if strings.HasPrefix(file.Path, ".env") || strings.Contains(file.Path, "/.env") {
			continue
		}
		out = append(out, file.Path)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func fileWeight(file models.FileInfo, weights map[models.FileKind]int) int {
	base := strings.ToLower(file.Path)
	switch {
	case base == "package.json", base == "go.mod", base == "cargo.toml", base == "pyproject.toml":
		return -5
	case base == "readme.md":
		return -4
	case strings.Contains(base, "dockerfile"), strings.Contains(base, "vercel.json"):
		return -3
	}
	if weight, ok := weights[file.Kind]; ok {
		return weight
	}
	return 9
}

func copyStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func trimList(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}
