package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/will/stackmap/internal/models"
)

const DefaultModel = "qwen2.5-coder:7b"

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

func Summarize(ctx context.Context, analysis *models.Analysis, model string) *models.AISummary {
	if model == "" {
		model = DefaultModel
	}
	client := OllamaClient{
		BaseURL: "http://127.0.0.1:11434",
		Model:   model,
		Client:  &http.Client{Timeout: 30 * time.Second},
	}
	text, err := client.Generate(ctx, promptFor(analysis))
	summary := &models.AISummary{Enabled: true, Model: model}
	if err != nil {
		summary.Warning = fmt.Sprintf("Ollama unavailable or failed: %v", err)
		return summary
	}
	summary.ProjectSummary, summary.NextSteps = splitAIResponse(text)
	return summary
}

func (c OllamaClient) Generate(ctx context.Context, prompt string) (string, error) {
	if c.Client == nil {
		c.Client = &http.Client{Timeout: 30 * time.Second}
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
		return "", fmt.Errorf("ollama returned HTTP %d", resp.StatusCode)
	}
	var out response
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.Error != "" {
		return "", errors.New(out.Error)
	}
	return strings.TrimSpace(out.Response), nil
}

func promptFor(a *models.Analysis) string {
	type compact struct {
		RepoName string              `json:"repoName"`
		Stack    models.StackInfo    `json:"stack"`
		Files    int                 `json:"files"`
		Routes   []models.RouteInfo  `json:"routes"`
		Tests    models.TestAnalysis `json:"tests"`
		Findings []models.Finding    `json:"findings"`
	}
	data, _ := json.MarshalIndent(compact{
		RepoName: a.RepoName,
		Stack:    a.Stack,
		Files:    len(a.Files),
		Routes:   a.Routes,
		Tests:    a.Tests,
		Findings: a.Findings,
	}, "", "  ")
	return "You are StackMap, a local-only codebase readiness assistant. Based only on this structured static analysis summary, write two concise sections: AI Project Summary and Recommended Next Steps. Do not claim to have read source files. Do not ask for cloud services.\n\n" + string(data)
}

func splitAIResponse(text string) (string, string) {
	lower := strings.ToLower(text)
	idx := strings.Index(lower, "recommended next steps")
	if idx == -1 {
		return text, ""
	}
	return strings.TrimSpace(text[:idx]), strings.TrimSpace(text[idx:])
}
