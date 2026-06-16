package report

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/will/stackmap/internal/models"
)

const noPreviousSnapshotMessage = "No previous snapshot yet. Run StackMap again after another analysis to see changes."

func AttachChangeSummary(root string, current *models.Analysis) error {
	summary, err := BuildChangeSummary(root, current)
	if err != nil {
		return err
	}
	current.Changes = summary
	return nil
}

func BuildChangeSummary(root string, current *models.Analysis) (*models.ChangeSummary, error) {
	if current == nil {
		return &models.ChangeSummary{Message: noPreviousSnapshotMessage}, nil
	}
	snapshots, err := ListSnapshots(root)
	if err != nil {
		return nil, err
	}
	for _, snapshot := range snapshots {
		previous, err := readSnapshotAnalysis(snapshot.JSONPath)
		if err != nil {
			continue
		}
		if !sameRepo(previous, current) {
			continue
		}
		return compareAnalyses(snapshot.Timestamp, previous, current), nil
	}
	return &models.ChangeSummary{
		HasPrevious:      false,
		Message:          noPreviousSnapshotMessage,
		CurrentSnapshot:  "current report",
		GeneratedAt:      current.GeneratedAt,
		AuditStatusAfter: auditStatus(current.Audit),
	}, nil
}

func compareAnalyses(previousSnapshot string, previous, current *models.Analysis) *models.ChangeSummary {
	summary := &models.ChangeSummary{
		HasPrevious:       true,
		PreviousSnapshot:  previousSnapshot,
		CurrentSnapshot:   "current report",
		GeneratedAt:       current.GeneratedAt,
		AuditStatusBefore: auditStatus(previous.Audit),
		AuditStatusAfter:  auditStatus(current.Audit),
	}
	summary.AddedRoutes, summary.RemovedRoutes = diffSets(routeKeys(previous.Routes), routeKeys(current.Routes))
	summary.AddedEnvVars, summary.RemovedEnvVars = diffSets(envVarNames(previous.Env), envVarNames(current.Env))
	summary.AddedFindings, summary.ResolvedFindings = diffSets(findingKeys(previous.Findings), findingKeys(current.Findings))
	summary.StackChanges = signalChanges("stack", stackSignals(previous.Stack), stackSignals(current.Stack))
	summary.FrameworkChanges = signalChanges("framework", previous.Stack.Frameworks, current.Stack.Frameworks)
	summary.DatabaseChanges = signalChanges("database", previous.Stack.Databases, current.Stack.Databases)
	summary.TestSignalChanges = signalChanges("test", testSignals(previous), testSignals(current))
	summary.DeploymentSignalChanges = signalChanges("deployment", deploymentSignalsForChange(previous), deploymentSignalsForChange(current))
	summary.KeyFileChanges = signalChanges("key file", keyFilePaths(previous.Structure.KeyFiles), keyFilePaths(current.Structure.KeyFiles))
	summary.SummaryBullets = changeBullets(summary)
	if len(summary.SummaryBullets) == 0 {
		summary.SummaryBullets = []string{"No deterministic changes were detected since the previous snapshot."}
	}
	return summary
}

func readSnapshotAnalysis(path string) (*models.Analysis, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var analysis models.Analysis
	if err := json.Unmarshal(data, &analysis); err != nil {
		return nil, err
	}
	return &analysis, nil
}

func sameRepo(previous, current *models.Analysis) bool {
	if previous == nil || current == nil {
		return false
	}
	if strings.TrimSpace(previous.RepoPath) != "" && strings.TrimSpace(current.RepoPath) != "" {
		return previous.RepoPath == current.RepoPath
	}
	return strings.TrimSpace(previous.RepoName) != "" && previous.RepoName == current.RepoName
}

func routeKeys(routes []models.RouteInfo) []string {
	values := make([]string, 0, len(routes))
	for _, route := range routes {
		key := strings.TrimSpace(strings.ToUpper(route.Method) + " " + route.Path)
		if key != "" {
			values = append(values, key)
		}
	}
	return uniqueSorted(values)
}

func envVarNames(env models.EnvAnalysis) []string {
	var values []string
	for _, item := range env.UsedVars {
		values = append(values, item.Name)
	}
	values = append(values, env.ExampleVars...)
	values = append(values, env.MissingFromExample...)
	values = append(values, env.MissingRequiredFromExample...)
	return uniqueSorted(values)
}

func findingKeys(findings []models.Finding) []string {
	values := make([]string, 0, len(findings))
	for _, finding := range findings {
		parts := []string{string(finding.Severity), finding.Category, finding.Message, finding.File}
		key := strings.TrimSpace(strings.Join(parts, " | "))
		if key != "" {
			values = append(values, key)
		}
	}
	return uniqueSorted(values)
}

func stackSignals(stack models.StackInfo) []string {
	var values []string
	values = append(values, stack.Languages...)
	values = append(values, stack.Libraries...)
	return uniqueSorted(values)
}

func testSignals(analysis *models.Analysis) []string {
	var values []string
	values = append(values, analysis.Stack.Testing...)
	values = append(values, analysis.Tests.Frameworks...)
	if analysis.Tests.HasTestFiles {
		values = append(values, "test files")
	}
	if analysis.Tests.HasTestScript {
		values = append(values, "test script")
	}
	if analysis.Tests.PlaywrightDetected {
		values = append(values, "Playwright")
	}
	if analysis.Tests.TestScript != "" {
		values = append(values, "script: "+analysis.Tests.TestScript)
	}
	return uniqueSorted(values)
}

func deploymentSignalsForChange(analysis *models.Analysis) []string {
	var values []string
	values = append(values, analysis.Stack.Deployment...)
	if analysis.Deployment.HasReadme {
		values = append(values, "README")
	}
	if analysis.Deployment.HasEnvExample {
		values = append(values, ".env.example")
	}
	if analysis.Deployment.HasDockerfile {
		values = append(values, "Dockerfile")
	}
	if analysis.Deployment.HasVercelConfig {
		values = append(values, "Vercel config")
	}
	if analysis.Deployment.HasHealthEndpoint {
		values = append(values, "health endpoint")
	}
	if analysis.Deployment.HasMigrationFiles {
		values = append(values, "migration files")
	}
	return uniqueSorted(values)
}

func keyFilePaths(files []models.FileRole) []string {
	values := make([]string, 0, len(files))
	for _, file := range files {
		values = append(values, file.Path)
	}
	return uniqueSorted(values)
}

func diffSets(before, after []string) ([]string, []string) {
	beforeSet := stringSet(before)
	afterSet := stringSet(after)
	var added, removed []string
	for value := range afterSet {
		if !beforeSet[value] {
			added = append(added, value)
		}
	}
	for value := range beforeSet {
		if !afterSet[value] {
			removed = append(removed, value)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	return added, removed
}

func signalChanges(label string, before, after []string) []string {
	added, removed := diffSets(before, after)
	var out []string
	for _, value := range added {
		out = append(out, fmt.Sprintf("added %s: %s", label, value))
	}
	for _, value := range removed {
		out = append(out, fmt.Sprintf("removed %s: %s", label, value))
	}
	return out
}

func changeBullets(summary *models.ChangeSummary) []string {
	var bullets []string
	appendCountBullet := func(added, removed []string, label string) {
		if len(added) > 0 || len(removed) > 0 {
			bullets = append(bullets, fmt.Sprintf("%s changed: %d added, %d removed.", label, len(added), len(removed)))
		}
	}
	appendCountBullet(summary.AddedRoutes, summary.RemovedRoutes, "Routes")
	appendCountBullet(summary.AddedEnvVars, summary.RemovedEnvVars, "Environment variables")
	appendCountBullet(summary.AddedFindings, summary.ResolvedFindings, "Audit findings")
	if summary.AuditStatusBefore != summary.AuditStatusAfter {
		bullets = append(bullets, fmt.Sprintf("Audit status changed from %s to %s.", summary.AuditStatusBefore, summary.AuditStatusAfter))
	}
	if len(summary.FrameworkChanges) > 0 || len(summary.DatabaseChanges) > 0 || len(summary.StackChanges) > 0 {
		bullets = append(bullets, "Detected stack signals changed.")
	}
	if len(summary.TestSignalChanges) > 0 {
		bullets = append(bullets, "Test signals changed.")
	}
	if len(summary.DeploymentSignalChanges) > 0 {
		bullets = append(bullets, "Deployment signals changed.")
	}
	if len(summary.KeyFileChanges) > 0 {
		bullets = append(bullets, "Key files changed.")
	}
	return bullets
}

func stringSet(values []string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out[value] = true
		}
	}
	return out
}

func uniqueSorted(values []string) []string {
	set := stringSet(values)
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
