package report

import (
	"strings"
	"testing"
	"time"

	"github.com/will/stackmap/internal/models"
)

func TestMarkdownRendersStructuredAISummary(t *testing.T) {
	analysis := baseAnalysis()
	analysis.AI = &models.AISummary{
		Enabled:              true,
		Model:                "qwen:7b",
		ProjectSummary:       "A local-first analyzer.",
		ArchitectureOverview: "A Go CLI runs analyzers and writes reports.",
		KeyStrengths:         []string{"Deterministic static analysis"},
		PotentialRisks:       []string{"Limited framework coverage"},
		RecommendedNextSteps: []string{"Add a smoke test"},
	}

	out := Markdown(analysis)
	for _, want := range []string{
		"## AI Project Summary",
		"Generated locally with `qwen:7b`.",
		"### Summary",
		"A local-first analyzer.",
		"### Architecture Overview",
		"### Key Strengths",
		"- Deterministic static analysis",
		"### Potential Risks",
		"### Recommended Next Steps",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("Markdown did not contain %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "could not parse") {
		t.Fatalf("structured summary rendered parse fallback:\n%s", out)
	}
}

func TestMarkdownRendersGracefulAIFallback(t *testing.T) {
	analysis := baseAnalysis()
	analysis.AI = &models.AISummary{
		Enabled:    true,
		Model:      "qwen:7b",
		RawText:    "Helpful prose, but not JSON.\n```json\n{}\n```",
		ParseError: "invalid character",
	}

	out := Markdown(analysis)
	for _, want := range []string{
		"AI summary was requested with `qwen:7b`, but StackMap could not parse the model response as structured JSON.",
		"### Raw Model Summary",
		"    Helpful prose, but not JSON.",
		"    ~~~json",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("Markdown did not contain %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "```") {
		t.Fatalf("fallback emitted a code fence:\n%s", out)
	}
}

func TestMarkdownDoesNotEmitEmptyCodeFenceForEmptyAIRawText(t *testing.T) {
	analysis := baseAnalysis()
	analysis.AI = &models.AISummary{
		Enabled:    true,
		Model:      "qwen:7b",
		ParseError: "response did not contain a JSON object",
	}

	out := Markdown(analysis)
	if !strings.Contains(out, "No usable AI summary text was returned by the model.") {
		t.Fatalf("Markdown did not render no-usable-text fallback:\n%s", out)
	}
	if strings.Contains(out, "```") || strings.Contains(out, "### Raw Model Summary") {
		t.Fatalf("empty raw fallback rendered a code fence or raw section:\n%s", out)
	}
}

func baseAnalysis() *models.Analysis {
	return &models.Analysis{
		RepoPath:    "/tmp/demo",
		RepoName:    "demo",
		GeneratedAt: time.Date(2026, 6, 12, 9, 0, 0, 0, time.UTC),
		Files: []models.FileInfo{
			{Path: "main.go", Kind: models.FileKindSource},
		},
		Stack: models.StackInfo{Languages: []string{"Go"}},
	}
}
