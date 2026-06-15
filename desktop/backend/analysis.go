package backend

import (
	"context"
	"errors"
	"path/filepath"
	"sort"
	"strings"

	stackmapapp "github.com/will/stackmap/internal/app"
	"github.com/will/stackmap/internal/models"
	stackmapreport "github.com/will/stackmap/internal/report"
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
	GeneratedAt    string         `json:"generatedAt"`
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
	Context        ContextView    `json:"context"`
	Audit          AuditView      `json:"audit"`
	APIRoutes      []RouteView    `json:"apiRoutes"`
	TestSummary    TestsView      `json:"testSummary"`
	DeploymentInfo DeploymentView `json:"deploymentInfo"`
	AI             AIView         `json:"ai"`
	Reports        ReportsView    `json:"reports"`
}

type ContextView struct {
	Purpose            string   `json:"purpose"`
	Confidence         string   `json:"confidence"`
	Evidence           []string `json:"evidence"`
	ReadmeTitle        string   `json:"readmeTitle,omitempty"`
	ReadmeSummary      string   `json:"readmeSummary,omitempty"`
	PackageName        string   `json:"packageName,omitempty"`
	PackageDescription string   `json:"packageDescription,omitempty"`
}

type AuditView struct {
	Status                 string   `json:"status"`
	ExitCode               int      `json:"exitCode,omitempty"`
	Blockers               []string `json:"blockers"`
	Warnings               []string `json:"warnings"`
	Mode                   string   `json:"mode,omitempty"`
	HasBackendSurface      bool     `json:"hasBackendSurface"`
	RequiresHealthEndpoint bool     `json:"requiresHealthEndpoint"`
}

type RouteView struct {
	Method     string `json:"method"`
	Path       string `json:"path"`
	SourceFile string `json:"sourceFile"`
	Confidence string `json:"confidence"`
	Note       string `json:"note,omitempty"`
}

type TestsView struct {
	HasTestFiles       bool     `json:"hasTestFiles"`
	HasTestScript      bool     `json:"hasTestScript"`
	Frameworks         []string `json:"frameworks"`
	TestFiles          []string `json:"testFiles"`
	TestScript         string   `json:"testScript,omitempty"`
	PlaywrightDetected bool     `json:"playwrightDetected"`
}

type DeploymentView struct {
	HasReadme                bool     `json:"hasReadme"`
	HasEnvExample            bool     `json:"hasEnvExample"`
	HasDockerfile            bool     `json:"hasDockerfile"`
	HasVercelConfig          bool     `json:"hasVercelConfig"`
	HasHealthEndpoint        bool     `json:"hasHealthEndpoint"`
	HasMigrationFiles        bool     `json:"hasMigrationFiles"`
	ReadmeMentionsDeploy     bool     `json:"readmeMentionsDeploy"`
	ReadmeMentionsMigrations bool     `json:"readmeMentionsMigrations"`
	DeploymentFiles          []string `json:"deploymentFiles"`
	MigrationFiles           []string `json:"migrationFiles"`
}

type AIView struct {
	Status               string   `json:"status"`
	Model                string   `json:"model,omitempty"`
	AttemptedModels      []string `json:"attemptedModels"`
	ProjectSummary       string   `json:"projectSummary,omitempty"`
	ArchitectureOverview string   `json:"architectureOverview,omitempty"`
	KeyStrengths         []string `json:"keyStrengths"`
	PotentialRisks       []string `json:"potentialRisks"`
	RecommendedNextSteps []string `json:"recommendedNextSteps"`
	LocalNotes           string   `json:"localNotes,omitempty"`
	DeterministicSummary string   `json:"deterministicSummary"`
	Warning              string   `json:"warning,omitempty"`
}

type ReportsView struct {
	JSONPath     string `json:"jsonPath"`
	MarkdownPath string `json:"markdownPath"`
	Directory    string `json:"directory"`
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
		GeneratedAt:    analysis.GeneratedAt.Format("2006-01-02 15:04:05"),
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
		Context:        buildContextView(analysis.Context),
		Audit:          AuditView{Status: "not run"},
		APIRoutes:      buildRouteViews(analysis.Routes),
		TestSummary:    buildTestsView(analysis.Tests),
		DeploymentInfo: buildDeploymentView(analysis.Deployment),
		AI:             AIView{Status: "not requested", DeterministicSummary: stackmapreport.DeterministicAISummary(analysis)},
	}
	response.Reports = ReportsView{
		JSONPath:     response.JSONReportPath,
		MarkdownPath: response.MDReportPath,
		Directory:    filepath.Join(root, ".stackmap"),
	}
	if request.RunAudit {
		response.AuditStatus = "not run"
		response.Audit.Status = "not run"
		if analysis.Audit != nil {
			response.AuditExitCode = analysis.Audit.ExitCode
			if analysis.Audit.Passed {
				response.AuditStatus = "passed"
			} else {
				response.AuditStatus = "failed"
			}
			response.Audit = buildAuditView(analysis.Audit, response.AuditStatus)
		}
	}
	if request.UseAI {
		response.AIStatus = "unavailable"
		response.AI.Status = "unavailable"
		if analysis.AI != nil {
			response.AIModel = analysis.AI.Model
			response.AIStatus = aiStatus(analysis.AI)
			response.AI = buildAIView(analysis, response.AIStatus)
		}
	}
	return response
}

func buildContextView(context models.ProjectContext) ContextView {
	return ContextView{
		Purpose:            context.Purpose,
		Confidence:         context.Confidence,
		Evidence:           copyStrings(context.Evidence),
		ReadmeTitle:        context.ReadmeTitle,
		ReadmeSummary:      context.ReadmeSummary,
		PackageName:        context.PackageName,
		PackageDescription: context.PackageDescription,
	}
}

func buildAuditView(audit *models.AuditResult, status string) AuditView {
	if audit == nil {
		return AuditView{Status: status}
	}
	return AuditView{
		Status:                 status,
		ExitCode:               audit.ExitCode,
		Blockers:               copyStrings(audit.Reasons),
		Warnings:               copyStrings(audit.Warnings),
		Mode:                   audit.Mode,
		HasBackendSurface:      audit.HasBackendSurface,
		RequiresHealthEndpoint: audit.RequiresHealthEndpoint,
	}
}

func buildRouteViews(routes []models.RouteInfo) []RouteView {
	views := make([]RouteView, 0, len(routes))
	for _, route := range routes {
		views = append(views, RouteView{
			Method:     route.Method,
			Path:       route.Path,
			SourceFile: route.SourceFile,
			Confidence: route.Confidence,
			Note:       route.Note,
		})
	}
	return views
}

func buildTestsView(tests models.TestAnalysis) TestsView {
	return TestsView{
		HasTestFiles:       tests.HasTestFiles,
		HasTestScript:      tests.HasTestScript,
		Frameworks:         copyStrings(tests.Frameworks),
		TestFiles:          copyStrings(tests.TestFiles),
		TestScript:         tests.TestScript,
		PlaywrightDetected: tests.PlaywrightDetected,
	}
}

func buildDeploymentView(deployment models.DeploymentAnalysis) DeploymentView {
	return DeploymentView{
		HasReadme:                deployment.HasReadme,
		HasEnvExample:            deployment.HasEnvExample,
		HasDockerfile:            deployment.HasDockerfile,
		HasVercelConfig:          deployment.HasVercelConfig,
		HasHealthEndpoint:        deployment.HasHealthEndpoint,
		HasMigrationFiles:        deployment.HasMigrationFiles,
		ReadmeMentionsDeploy:     deployment.ReadmeMentionsDeploy,
		ReadmeMentionsMigrations: deployment.ReadmeMentionsMigrations,
		DeploymentFiles:          copyStrings(deployment.DeploymentFiles),
		MigrationFiles:           copyStrings(deployment.MigrationFiles),
	}
}

func buildAIView(analysis *models.Analysis, status string) AIView {
	view := AIView{
		Status:               status,
		DeterministicSummary: stackmapreport.DeterministicAISummary(analysis),
	}
	if analysis.AI == nil {
		return view
	}
	view.Model = analysis.AI.Model
	view.AttemptedModels = copyStrings(analysis.AI.AttemptedModels)
	view.ProjectSummary = analysis.AI.ProjectSummary
	view.ArchitectureOverview = analysis.AI.ArchitectureOverview
	view.KeyStrengths = copyStrings(analysis.AI.KeyStrengths)
	view.PotentialRisks = copyStrings(analysis.AI.PotentialRisks)
	view.RecommendedNextSteps = copyStrings(analysis.AI.RecommendedNextSteps)
	view.LocalNotes = analysis.AI.LocalNotes
	view.Warning = analysis.AI.Warning
	return view
}

func copyStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	return append([]string{}, values...)
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
