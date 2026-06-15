package app

import (
	"context"
	"path/filepath"

	"github.com/will/stackmap/internal/ai"
	"github.com/will/stackmap/internal/analyzers"
	"github.com/will/stackmap/internal/audit"
	"github.com/will/stackmap/internal/models"
	"github.com/will/stackmap/internal/qa"
	"github.com/will/stackmap/internal/report"
)

type AnalyzeOptions struct {
	Path         string
	RunAudit     bool
	AuditOptions audit.Options
	UseAI        bool
	Model        string
	AIDebug      bool
}

type AnalyzeResult struct {
	Root     string
	Analysis *models.Analysis
}

func Analyze(ctx context.Context, opts AnalyzeOptions) (*AnalyzeResult, error) {
	target := opts.Path
	if target == "" {
		target = "."
	}
	root, err := filepath.Abs(target)
	if err != nil {
		return nil, err
	}
	analysis, err := analyzers.Analyze(root)
	if err != nil {
		return nil, err
	}
	if opts.UseAI {
		aiOpts := ai.SummaryOptions{}
		if opts.AIDebug {
			aiOpts.DebugDir = filepath.Join(root, ".stackmap", "ai-debug")
		}
		analysis.AI = ai.SummarizeWithOptions(ctx, analysis, opts.Model, aiOpts)
	}
	if opts.RunAudit {
		analysis.Audit = audit.Evaluate(analysis, opts.AuditOptions)
	}
	return &AnalyzeResult{Root: root, Analysis: analysis}, nil
}

func ExportReports(root string, analysis *models.Analysis) error {
	return report.ExportAll(root, analysis)
}

type AskOptions struct {
	Root     string
	Question string
	UseAI    bool
	Model    string
	AIDebug  bool
}

func Ask(ctx context.Context, analysis *models.Analysis, opts AskOptions) (*models.QAResult, error) {
	root := opts.Root
	if root == "" && analysis != nil {
		root = analysis.RepoPath
	}
	if root != "" {
		absRoot, err := filepath.Abs(root)
		if err != nil {
			return nil, err
		}
		root = absRoot
	}
	qaOpts := qa.Options{UseAI: opts.UseAI, Model: opts.Model}
	if opts.AIDebug && root != "" {
		qaOpts.DebugDir = filepath.Join(root, ".stackmap", "ai-debug", "ask")
	}
	result := qa.Ask(ctx, analysis, opts.Question, qaOpts)
	latestErr, historyErr := qa.WriteLatestAndAppendHistory(root, result)
	if latestErr != nil {
		result.Warnings = append(result.Warnings, "could not write .stackmap/qa/latest-question.json: "+latestErr.Error())
	} else if historyErr != nil {
		result.Warnings = append(result.Warnings, "could not append .stackmap/qa/history.jsonl: "+historyErr.Error())
	}
	return result, nil
}
