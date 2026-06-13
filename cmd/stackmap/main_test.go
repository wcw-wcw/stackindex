package main

import (
	"errors"
	"testing"

	"github.com/will/stackmap/internal/models"
)

func TestAuditErrorIgnoresAIStatusAndModels(t *testing.T) {
	analysis := &models.Analysis{
		Findings: []models.Finding{
			{Severity: models.SeverityLow, Category: "tests", Message: "No test script found."},
		},
		AI: &models.AISummary{
			Enabled:         true,
			Model:           "qwen:7b",
			AttemptedModels: []string{"llama3.2:3b", "qwen:7b"},
			Status:          "fallback_model_unavailable",
			Warning:         "local model unavailable",
		},
	}

	if err := auditError(analysis); err != nil {
		t.Fatalf("auditError() = %v, want nil", err)
	}
}

func TestAuditErrorFailsOnStaticHighOrMediumFindings(t *testing.T) {
	analysis := &models.Analysis{
		Findings: []models.Finding{
			{Severity: models.SeverityMedium, Category: "env", Message: "Missing .env.example."},
			{Severity: models.SeverityHigh, Category: "env", Message: "Secret-looking value in .env.example."},
		},
		AI: &models.AISummary{
			Enabled: true,
			Model:   "llama3.2:3b",
			Status:  "generated_text",
		},
	}

	err := auditError(analysis)
	var failure auditFailure
	if !errors.As(err, &failure) {
		t.Fatalf("auditError() = %v, want auditFailure", err)
	}
	if failure.high != 1 || failure.medium != 1 {
		t.Fatalf("auditFailure counts = %d high, %d medium; want 1 high, 1 medium", failure.high, failure.medium)
	}
}

func TestNormalizeAnalyzeArgsKeepsAuditFlagBeforePositionals(t *testing.T) {
	got := normalizeAnalyzeArgs([]string{".", "--audit", "--ai"})
	want := []string{"--audit", "--ai", "."}
	if len(got) != len(want) {
		t.Fatalf("normalizeAnalyzeArgs length = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("normalizeAnalyzeArgs[%d] = %q, want %q (%v)", i, got[i], want[i], got)
		}
	}
}
