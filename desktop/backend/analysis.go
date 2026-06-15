package backend

import (
	"context"
	"errors"
	"path/filepath"
	"sort"
	"strings"

	stackmapapp "github.com/will/stackmap/internal/app"
	"github.com/will/stackmap/internal/models"
)

type AnalyzeRequest struct {
	Path     string `json:"path"`
	RunAudit bool   `json:"runAudit"`
	UseAI    bool   `json:"useAI"`
	Model    string `json:"model"`
}

type AnalyzeResponse struct {
	RepoName       string         `json:"repoName"`
	RepoPath       string         `json:"repoPath"`
	Files          int            `json:"files"`
	Routes         int            `json:"routes"`
	Tests          int            `json:"tests"`
	Findings       map[string]int `json:"findings"`
	Stack          []string       `json:"stack"`
	Languages      []string       `json:"languages"`
	Frameworks     []string       `json:"frameworks"`
	Databases      []string       `json:"databases"`
	Deployment     []string       `json:"deployment"`
	AuditStatus    string         `json:"auditStatus,omitempty"`
	AuditExitCode  int            `json:"auditExitCode,omitempty"`
	AIStatus       string         `json:"aiStatus,omitempty"`
	AIModel        string         `json:"aiModel,omitempty"`
	JSONReportPath string         `json:"jsonReportPath"`
	MDReportPath   string         `json:"mdReportPath"`
}

func AnalyzeProject(ctx context.Context, request AnalyzeRequest) (*AnalyzeResponse, error) {
	target := strings.TrimSpace(request.Path)
	if target == "" {
		return nil, errors.New("project path is required")
	}
	absPath, err := filepath.Abs(target)
	if err != nil {
		return nil, err
	}
	result, err := stackmapapp.Analyze(ctx, stackmapapp.AnalyzeOptions{
		Path:     absPath,
		RunAudit: request.RunAudit,
		UseAI:    request.UseAI,
		Model:    strings.TrimSpace(request.Model),
	})
	if err != nil {
		return nil, err
	}
	if err := stackmapapp.ExportReports(result.Root, result.Analysis); err != nil {
		return nil, err
	}
	return BuildAnalyzeResponse(result.Root, result.Analysis, request), nil
}

func BuildAnalyzeResponse(root string, analysis *models.Analysis, request AnalyzeRequest) *AnalyzeResponse {
	response := &AnalyzeResponse{
		RepoName:       analysis.RepoName,
		RepoPath:       analysis.RepoPath,
		Files:          len(analysis.Files),
		Routes:         len(analysis.Routes),
		Tests:          len(analysis.Tests.TestFiles),
		Findings:       findingCounts(analysis.Findings),
		Stack:          compactStack(analysis.Stack),
		Languages:      append([]string{}, analysis.Stack.Languages...),
		Frameworks:     append([]string{}, analysis.Stack.Frameworks...),
		Databases:      append([]string{}, analysis.Stack.Databases...),
		Deployment:     append([]string{}, analysis.Stack.Deployment...),
		AuditStatus:    "not run",
		AIStatus:       "not requested",
		JSONReportPath: filepath.Join(root, ".stackmap", "analysis.json"),
		MDReportPath:   filepath.Join(root, ".stackmap", "reports", "repo-report.md"),
	}
	if request.RunAudit {
		response.AuditStatus = "not run"
		if analysis.Audit != nil {
			response.AuditExitCode = analysis.Audit.ExitCode
			if analysis.Audit.Passed {
				response.AuditStatus = "passed"
			} else {
				response.AuditStatus = "failed"
			}
		}
	}
	if request.UseAI {
		response.AIStatus = "unavailable"
		if analysis.AI != nil {
			response.AIModel = analysis.AI.Model
			response.AIStatus = aiStatus(analysis.AI)
		}
	}
	return response
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

func compactStack(stack models.StackInfo) []string {
	seen := map[string]bool{}
	var values []string
	for _, group := range [][]string{stack.Languages, stack.Frameworks, stack.Databases, stack.Deployment} {
		for _, value := range group {
			value = strings.TrimSpace(value)
			key := strings.ToLower(value)
			if value == "" || seen[key] {
				continue
			}
			seen[key] = true
			values = append(values, value)
		}
	}
	sort.Strings(values)
	return values
}

func aiStatus(summary *models.AISummary) string {
	switch {
	case summary == nil:
		return "unavailable"
	case strings.HasPrefix(summary.Status, "generated"):
		return "generated"
	case summary.ProjectSummary != "":
		return "generated"
	case summary.Status != "":
		return summary.Status
	default:
		return "unavailable"
	}
}
