package qa

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/wcw-wcw/stackindex/internal/ai"
	"github.com/wcw-wcw/stackindex/internal/models"
)

const (
	ModeDeterministic = "deterministic"
	ModeAI            = "ai"

	ConfidenceHigh   = "high"
	ConfidenceMedium = "medium"
	ConfidenceLow    = "low"
)

type Options struct {
	UseAI          bool
	Model          string
	DebugDir       string
	FallbackModels []string
	Generate       func(context.Context, string, string) (string, error)
}

type questionType string

const (
	questionPurpose     questionType = "purpose"
	questionStack       questionType = "stack"
	questionRoutes      questionType = "routes"
	questionStructure   questionType = "structure"
	questionGraph       questionType = "graph"
	questionAuth        questionType = "auth"
	questionDatabase    questionType = "database"
	questionLocalRun    questionType = "local_run"
	questionDeployment  questionType = "deployment"
	questionTests       questionType = "tests"
	questionEnvironment questionType = "environment"
	questionConnection  questionType = "connection"
	questionChanges     questionType = "changes"
	questionUnsupported questionType = "unsupported"
)

var envVarNameRE = regexp.MustCompile(`\b[A-Z][A-Z0-9]+(?:_[A-Z0-9]+)+\b`)

func Ask(ctx context.Context, analysis *models.Analysis, question string, opts Options) *models.QAResult {
	result := AnswerDeterministically(analysis, question)
	if !opts.UseAI {
		return result
	}
	return synthesizeWithAI(ctx, analysis, result, opts)
}

func AnswerDeterministically(analysis *models.Analysis, question string) *models.QAResult {
	if analysis == nil {
		return unsupported(question, "No StackIndex analysis was available.")
	}
	switch classify(question) {
	case questionPurpose:
		return answerPurpose(analysis, question)
	case questionStack:
		return answerStack(analysis, question)
	case questionRoutes:
		return answerRoutes(analysis, question)
	case questionStructure:
		return answerStructure(analysis, question)
	case questionGraph:
		return answerGraph(analysis, question)
	case questionAuth:
		return answerAuth(analysis, question)
	case questionDatabase:
		return answerDatabase(analysis, question)
	case questionLocalRun:
		return answerLocalRun(analysis, question)
	case questionDeployment:
		return answerDeployment(analysis, question)
	case questionTests:
		return answerTests(analysis, question)
	case questionEnvironment:
		return answerEnvironment(analysis, question)
	case questionConnection:
		return answerConnection(analysis, question)
	case questionChanges:
		return answerChanges(analysis, question)
	default:
		return unsupported(question, "")
	}
}

func classify(question string) questionType {
	q := normalize(question)
	switch {
	case containsAny(q, "what changed since last report", "what changed since the last report", "what changed since last analysis", "what changed since the last analysis", "changed since last", "changes since previous", "changes since last", "new routes were added", "what new routes were added", "env vars changed", "environment variables changed"):
		return questionChanges
	case containsAny(q, "frontend connected to the backend", "frontend connect", "connect to the backend", "front end connected", "client connected", "frontend connect to the backend", "frontend talk to the backend", "client talk to the api", "where is the api client", "api client", "frontend call api", "frontend calls api", "where does the frontend call api", "where are api calls made"):
		return questionConnection
	case containsAny(q, "what is this project", "what is this repo for", "what is this project for", "summarize this project", "what does this app do"):
		return questionPurpose
	case containsAny(q, "where is auth", "where is authentication", "where is login", "where is sign in", "where are protected routes", "where is middleware", "auth handled", "authentication handled", "login handled", "protected routes", "middleware"):
		return questionAuth
	case containsAny(q, "how do i run this locally", "run this locally", "local setup", "setup locally", "what scripts matter", "what scripts are important", "how do i build", "how do i test", "build and test", "dev command", "start command"):
		return questionLocalRun
	case containsAny(q, "how does this project use neon", "how does this project use postgres", "how does this project use postgresql", "what database", "what db", "where is the database configured", "where is the database initialized", "where is the db client", "where is db client", "where are schema files", "where are migrations", "are there migrations", "how does storage work", "database configured", "storage work", "use neon", "use postgres", "use postgresql", "migrations", "schema files"):
		return questionDatabase
	case containsAny(q, "what stack", "what technologies", "what frameworks", "using react", "using next", "using vite", "using fastapi"):
		return questionStack
	case containsAny(q, "where are the api routes", "what endpoints exist", "does this have a backend", "what routes does it expose", "api routes", "endpoints"):
		return questionRoutes
	case containsAny(q, "important files", "files should i read first", "what files should i read first", "where should i start", "how is it organized", "what folders matter", "what does src/lib do", "structure", "organized", "read first"):
		return questionStructure
	case containsAny(q, "how are files connected", "what imports what", "most connected files", "shared modules", "dependency graph", "dependencies"):
		return questionGraph
	case containsAny(q, "deployment ready", "before deployment", "what are the risks", "health checks", "health check", "env example", "review before deployment", "deployment-sensitive", "deployment sensitive", "before deploying"):
		return questionDeployment
	case containsAny(q, "does it have tests", "how do i run tests", "test framework", "tests", "testing"):
		return questionTests
	case containsAny(q, "what env vars", "env configured", "missing env example", "environment variables", "env vars", "environment", "where are env vars used", "config deployment sensitive", "deployment-sensitive config"):
		return questionEnvironment
	default:
		return questionUnsupported
	}
}

func answerPurpose(a *models.Analysis, question string) *models.QAResult {
	purpose := strings.TrimSpace(a.Context.Purpose)
	if purpose == "" {
		purpose = "StackIndex could not infer a specific project purpose from the deterministic analysis."
	}
	answer := purpose
	if a.Context.Confidence != "" {
		answer = fmt.Sprintf("%s Confidence is %s.", purpose, a.Context.Confidence)
	}
	if a.Context.ReadmeSummary != "" {
		answer += " README summary: " + a.Context.ReadmeSummary
	}
	evidence := []models.QAEvidence{}
	addEvidence(&evidence, "context", "Purpose", a.Context.Purpose, "")
	addEvidence(&evidence, "context", "Confidence", a.Context.Confidence, "")
	addEvidence(&evidence, "readme", "README title", a.Context.ReadmeTitle, "README.md")
	addEvidence(&evidence, "readme", "README summary", a.Context.ReadmeSummary, "README.md")
	for _, item := range capStrings(a.Context.Evidence, 5) {
		addEvidence(&evidence, "context", "Purpose evidence", item, "")
	}
	return result(question, answer, confidenceOr(a.Context.Confidence, ConfidenceMedium), evidence)
}

func answerStack(a *models.Analysis, question string) *models.QAResult {
	var parts []string
	appendStackPart := func(label string, values []string) {
		if len(values) > 0 {
			parts = append(parts, fmt.Sprintf("%s: %s", label, strings.Join(values, ", ")))
		}
	}
	appendStackPart("languages", a.Stack.Languages)
	appendStackPart("frameworks", a.Stack.Frameworks)
	appendStackPart("libraries", a.Stack.Libraries)
	appendStackPart("databases", a.Stack.Databases)
	appendStackPart("testing", a.Stack.Testing)
	appendStackPart("deployment", a.Stack.Deployment)
	answer := "No stack technologies were detected."
	if len(parts) > 0 {
		answer = "Detected stack includes " + strings.Join(parts, "; ") + "."
	}
	evidence := []models.QAEvidence{}
	addStackEvidence(&evidence, "Languages", a.Stack.Languages)
	addStackEvidence(&evidence, "Frameworks", a.Stack.Frameworks)
	addStackEvidence(&evidence, "Libraries", a.Stack.Libraries)
	addStackEvidence(&evidence, "Databases", a.Stack.Databases)
	addStackEvidence(&evidence, "Testing", a.Stack.Testing)
	addStackEvidence(&evidence, "Deployment", a.Stack.Deployment)
	if a.PackageInfo != nil {
		addEvidence(&evidence, "context", "Package", a.PackageInfo.Name, "package.json")
		addEvidence(&evidence, "context", "Package description", a.PackageInfo.Description, "package.json")
	}
	confidence := ConfidenceLow
	if len(parts) >= 2 {
		confidence = ConfidenceHigh
	} else if len(parts) == 1 {
		confidence = ConfidenceMedium
	}
	return result(question, answer, confidence, evidence)
}

func answerRoutes(a *models.Analysis, question string) *models.QAResult {
	if len(a.Routes) == 0 {
		return result(question, "No API routes were detected in StackIndex's static analysis.", ConfidenceMedium, nil)
	}
	groups := routeGroups(a.Routes)
	answer := fmt.Sprintf("This project exposes %d detected API route%s.", len(a.Routes), pluralS(len(a.Routes)))
	if len(groups) > 0 {
		answer += " Main route areas appear under " + strings.Join(groups, ", ") + "."
	}
	answer += " Review the source files listed in the evidence for exact handlers."
	evidence := []models.QAEvidence{}
	for _, route := range capRoutes(a.Routes, 8) {
		label := strings.TrimSpace(route.Method + " " + route.Path)
		addEvidence(&evidence, "route", label, route.Confidence, route.SourceFile)
	}
	return result(question, answer, ConfidenceHigh, evidence)
}

func answerStructure(a *models.Analysis, question string) *models.QAResult {
	var fragments []string
	if len(a.Structure.Directories) > 0 {
		fragments = append(fragments, "Important folders include "+joinDirectoryRoles(capDirectories(a.Structure.Directories, 5))+".")
	}
	startingFiles := rankedStartingFiles(a)
	if len(startingFiles) > 0 {
		fragments = append(fragments, "Read these first: "+joinFileRoles(capFiles(startingFiles, 6))+".")
	}
	answer := "StackIndex did not identify important folders or files."
	if len(fragments) > 0 {
		answer = strings.Join(fragments, " ")
	}
	evidence := []models.QAEvidence{}
	for _, dir := range capDirectories(a.Structure.Directories, 6) {
		addEvidence(&evidence, "structure", dir.Path, dir.Role, dir.Path)
	}
	for _, file := range capFiles(startingFiles, 8) {
		addEvidence(&evidence, "file", file.Path, file.Role, file.Path)
	}
	return result(question, answer, confidenceForCount(len(evidence)), evidence)
}

func answerGraph(a *models.Analysis, question string) *models.QAResult {
	g := a.Dependencies
	var fragments []string
	if len(g.Entrypoints) > 0 {
		fragments = append(fragments, "Entrypoints include "+strings.Join(capStrings(g.Entrypoints, 5), ", ")+".")
	}
	if len(g.TopConnectedFiles) > 0 {
		fragments = append(fragments, "The most connected files include "+joinConnectedFiles(capConnected(g.TopConnectedFiles, 5))+".")
	}
	if len(g.ArchitectureHints) > 0 {
		fragments = append(fragments, "Architecture hints: "+strings.Join(capStrings(g.ArchitectureHints, 3), " "))
	}
	if len(fragments) == 0 {
		fragments = append(fragments, fmt.Sprintf("StackIndex built a lightweight graph with %d nodes and %d edges, but did not identify standout connected files.", len(g.Nodes), len(g.Edges)))
	}
	evidence := []models.QAEvidence{}
	for _, path := range capStrings(g.Entrypoints, 5) {
		addEvidence(&evidence, "graph", "Entrypoint", path, path)
	}
	for _, file := range capConnected(g.TopConnectedFiles, 6) {
		addEvidence(&evidence, "graph", file.Path, fmt.Sprintf("%d imports, imported by %d. %s", file.ImportsCount, file.ImportedByCount, file.WhyItMatters), file.Path)
	}
	for _, hint := range capStrings(g.ArchitectureHints, 3) {
		addEvidence(&evidence, "graph", "Architecture hint", hint, "")
	}
	return result(question, strings.Join(fragments, " "), confidenceForCount(len(g.Nodes)+len(g.Edges)+len(evidence)), evidence)
}

func answerDatabase(a *models.Analysis, question string) *models.QAResult {
	evidence := databaseEvidence(a)
	var sentences []string
	if len(a.Stack.Databases) > 0 {
		sentences = append(sentences, "Detected database/storage: "+strings.Join(a.Stack.Databases, ", ")+".")
	} else {
		sentences = append(sentences, "StackIndex did not detect a named database in the stack.")
	}
	envNames := databaseEnvNames(a.Env)
	if len(envNames) > 0 {
		sentences = append(sentences, "Configuration appears to use "+strings.Join(capStrings(envNames, 4), ", ")+".")
	}
	if len(a.Deployment.MigrationFiles) > 0 {
		sentences = append(sentences, fmt.Sprintf("Migrations are present, including %s.", strings.Join(capStrings(a.Deployment.MigrationFiles, 3), ", ")))
	} else if a.Deployment.HasMigrationFiles {
		sentences = append(sentences, "Migration files were detected.")
	}
	scripts := databaseScripts(a.PackageInfo)
	if len(scripts) > 0 {
		sentences = append(sentences, "Relevant scripts include "+strings.Join(capStrings(scripts, 4), ", ")+".")
	}
	contextSignals := databaseContextSignals(a)
	if len(contextSignals) > 0 {
		sentences = append(sentences, databaseContextAnswer(contextSignals))
	}
	files := databaseFiles(a)
	if len(files) > 0 {
		sentences = append(sentences, "Database-related files include "+strings.Join(capStrings(files, 4), ", ")+".")
	}
	return result(question, strings.Join(sentences, " "), confidenceForCount(len(evidence)), evidence)
}

func answerAuth(a *models.Analysis, question string) *models.QAResult {
	evidence := authEvidence(a)
	if len(evidence) == 0 {
		return result(question, "StackIndex did not find strong auth/login/protected-route evidence. It is not claiming auth exists from weak signals alone.", ConfidenceMedium, nil)
	}
	var fragments []string
	routes := authRoutes(a.Routes)
	files := authFiles(a)
	envNames := authEnvNames(a.Env)
	if len(routes) > 0 {
		fragments = append(fragments, fmt.Sprintf("Auth is likely handled around %d auth-related route%s.", len(routes), pluralS(len(routes))))
	}
	if len(files) > 0 {
		fragments = append(fragments, "Likely auth/protection files include "+strings.Join(capStrings(files, 5), ", ")+".")
	}
	if len(envNames) > 0 {
		fragments = append(fragments, "Auth-related configuration appears to use "+strings.Join(capStrings(envNames, 4), ", ")+".")
	}
	if len(fragments) == 0 {
		fragments = append(fragments, "StackIndex found heuristic auth signals; review the evidence before assuming auth is complete.")
	}
	return result(question, strings.Join(fragments, " "), confidenceForAuthEvidence(evidence), evidence)
}

func answerLocalRun(a *models.Analysis, question string) *models.QAResult {
	var fragments []string
	scripts := localRunScripts(a.PackageInfo)
	if len(scripts) > 0 {
		fragments = append(fragments, "Package scripts that matter: "+strings.Join(capStrings(scripts, 6), ", ")+".")
	}
	if a.PackageInfo != nil && a.PackageInfo.PackageManagerHint != "" {
		fragments = append(fragments, "Package manager hint: "+a.PackageInfo.PackageManagerHint+".")
	}
	if module := goModuleName(a.PackageInfo); module != "" {
		fragments = append(fragments, "Go module detected: "+module+"; `go test ./...` is the safest generic Go test command.")
	}
	if hasPythonSource(a) {
		fragments = append(fragments, "Python files were detected, but StackIndex only suggests Python run commands when scripts or docs identify them.")
	}
	if a.Deployment.ReadmeMentionsSetup {
		fragments = append(fragments, "README appears to include setup instructions.")
	}
	if a.Tests.TestScript != "" {
		fragments = append(fragments, "Tests can use the detected test script: "+a.Tests.TestScript+".")
	}
	if len(fragments) == 0 {
		fragments = append(fragments, "StackIndex did not find explicit local run scripts or setup clues, so it will not invent commands.")
	}
	evidence := []models.QAEvidence{}
	for _, script := range scripts {
		addEvidence(&evidence, "script", "Package script", script, "package.json")
	}
	if a.PackageInfo != nil {
		addEvidence(&evidence, "package", "Package manager", a.PackageInfo.PackageManagerHint, "package.json")
		addEvidence(&evidence, "package", "Go module", a.PackageInfo.ModuleName, "go.mod")
	}
	addEvidence(&evidence, "readme", "README mentions setup", fmt.Sprintf("%t", a.Deployment.ReadmeMentionsSetup), "README.md")
	for _, file := range capStrings(a.Tests.TestFiles, 4) {
		addEvidence(&evidence, "file", "Test file", file, file)
	}
	return result(question, strings.Join(fragments, " "), confidenceForCount(len(evidence)), evidence)
}

func answerDeployment(a *models.Analysis, question string) *models.QAResult {
	audit := localAudit(a)
	var fragments []string
	if audit.Passed {
		fragments = append(fragments, "Audit-style deployment checks would pass; no deterministic blockers were found.")
	} else {
		fragments = append(fragments, fmt.Sprintf("Audit-style deployment checks would fail; review %d blocker%s: %s", len(audit.Reasons), pluralS(len(audit.Reasons)), strings.Join(audit.Reasons, " ")))
	}
	if len(audit.Warnings) > 0 {
		fragments = append(fragments, fmt.Sprintf("Warnings to consider: %s", strings.Join(audit.Warnings, " ")))
	}
	fragments = append(fragments, deploymentSignals(a.Deployment))
	evidence := []models.QAEvidence{}
	for _, reason := range audit.Reasons {
		addEvidence(&evidence, "audit", "Audit blocker", reason, "")
	}
	for _, warning := range audit.Warnings {
		addEvidence(&evidence, "audit", "Audit warning", warning, "")
	}
	addDeploymentEvidence(&evidence, a.Deployment)
	for _, finding := range capFindings(a.Findings, 5) {
		addEvidence(&evidence, "finding", string(finding.Severity)+" "+finding.Category, finding.Message, finding.File)
	}
	confidence := ConfidenceHigh
	if len(evidence) == 0 {
		confidence = ConfidenceMedium
	}
	return result(question, strings.Join(fragments, " "), confidence, evidence)
}

func answerTests(a *models.Analysis, question string) *models.QAResult {
	tests := a.Tests
	var fragments []string
	if tests.HasTestFiles || tests.HasTestScript {
		fragments = append(fragments, "Tests were detected.")
	} else {
		fragments = append(fragments, "StackIndex did not detect test files or a package test script.")
	}
	if len(tests.Frameworks) > 0 {
		fragments = append(fragments, "Frameworks: "+strings.Join(tests.Frameworks, ", ")+".")
	}
	if tests.TestScript != "" {
		fragments = append(fragments, "Run tests with the package `test` script: "+tests.TestScript+".")
	}
	evidence := []models.QAEvidence{}
	addEvidence(&evidence, "script", "test", tests.TestScript, "package.json")
	for _, framework := range tests.Frameworks {
		addEvidence(&evidence, "context", "Test framework", framework, "")
	}
	for _, file := range capStrings(tests.TestFiles, 6) {
		addEvidence(&evidence, "file", "Test file", file, file)
	}
	return result(question, strings.Join(fragments, " "), confidenceForBool(tests.HasTestFiles || tests.HasTestScript), evidence)
}

func answerEnvironment(a *models.Analysis, question string) *models.QAResult {
	env := a.Env
	if !env.UsesEnvVars {
		return result(question, "StackIndex did not detect environment variable usage.", ConfidenceMedium, nil)
	}
	answer := fmt.Sprintf("Environment variables are used in this project. StackIndex found %d used variable reference%s and %d variable%s in the example file.", len(env.UsedVars), pluralS(len(env.UsedVars)), len(env.ExampleVars), pluralS(len(env.ExampleVars)))
	if env.ExampleFile != "" {
		answer += " Example file: `" + env.ExampleFile + "`."
	} else {
		answer += " No `.env.example` file was detected."
	}
	if len(env.MissingRequiredFromExample) > 0 {
		answer += fmt.Sprintf(" %d required variable%s appear missing from the example.", len(env.MissingRequiredFromExample), pluralS(len(env.MissingRequiredFromExample)))
	}
	evidence := []models.QAEvidence{}
	addEvidence(&evidence, "env", "Example file", env.ExampleFile, env.ExampleFile)
	addEvidence(&evidence, "env", "Env file present", fmt.Sprintf("%t", env.EnvFilePresent), "")
	addEvidence(&evidence, "env", "Used var count", fmt.Sprintf("%d", len(env.UsedVars)), "")
	addEvidence(&evidence, "env", "Missing required count", fmt.Sprintf("%d", len(env.MissingRequiredFromExample)), "")
	for _, item := range capStrings(env.MissingRequiredFromExample, 5) {
		addEvidence(&evidence, "env", "Missing required from example", item, "")
	}
	for _, item := range capEnvVars(env.UsedVars, 6) {
		addEvidence(&evidence, "env", item.Name, item.Classification, strings.Join(item.Files, ", "))
	}
	return result(question, answer, ConfidenceHigh, evidence)
}

func answerConnection(a *models.Analysis, question string) *models.QAResult {
	var fragments []string
	frontendDirs := matchingDirectoryPrefixes(a, "frontend", "src")
	backendDirs := matchingDirectoryPrefixes(a, "backend", "api", "server")
	clientFiles := frontendAPIClientFiles(a)
	if len(frontendDirs) > 0 || len(backendDirs) > 0 {
		var sides []string
		if len(frontendDirs) > 0 {
			sides = append(sides, "frontend under "+strings.Join(capStrings(frontendDirs, 2), ", "))
		}
		if len(backendDirs) > 0 {
			sides = append(sides, "backend/API surface under "+strings.Join(capStrings(backendDirs, 2), ", "))
		}
		fragments = append(fragments, "StackIndex sees "+strings.Join(sides, " and ")+".")
	}
	if len(clientFiles) > 0 {
		fragments = append(fragments, "Frontend API client files include "+strings.Join(capStrings(clientFiles, 4), ", ")+".")
	}
	if len(a.Routes) > 0 {
		fragments = append(fragments, fmt.Sprintf("The frontend/backend boundary is visible through %d detected API route%s.", len(a.Routes), pluralS(len(a.Routes))))
	} else {
		fragments = append(fragments, "StackIndex did not detect explicit API routes, so it cannot prove a frontend/backend connection.")
	}
	backendFrameworks := backendFrameworkSignals(a.Stack.Frameworks)
	if len(backendFrameworks) > 0 {
		fragments = append(fragments, "Backend framework signals include "+strings.Join(backendFrameworks, ", ")+".")
	}
	apiEnv := apiBaseEnvNames(a.Env)
	if len(apiEnv) > 0 {
		fragments = append(fragments, "API base configuration appears to use "+strings.Join(capStrings(apiEnv, 3), ", ")+".")
	}
	if len(a.Stack.Deployment) > 0 {
		fragments = append(fragments, "Deployment boundary signals include "+strings.Join(a.Stack.Deployment, ", ")+".")
	}
	if len(a.Dependencies.Entrypoints) > 0 {
		fragments = append(fragments, "Detected entrypoints include "+strings.Join(capStrings(a.Dependencies.Entrypoints, 4), ", ")+".")
	}
	if len(a.Dependencies.ArchitectureHints) > 0 {
		fragments = append(fragments, strings.Join(capStrings(a.Dependencies.ArchitectureHints, 2), " "))
	}
	evidence := []models.QAEvidence{}
	for _, file := range capStrings(clientFiles, 6) {
		addEvidence(&evidence, "file", "Frontend API client", file, file)
	}
	for _, route := range capRoutes(a.Routes, 6) {
		addEvidence(&evidence, "route", strings.TrimSpace(route.Method+" "+route.Path), route.Confidence, route.SourceFile)
	}
	for _, name := range capStrings(apiEnv, 4) {
		addEvidence(&evidence, "env", "API base env", name, "")
	}
	for _, path := range capStrings(a.Dependencies.Entrypoints, 4) {
		addEvidence(&evidence, "graph", "Entrypoint", path, path)
	}
	for _, file := range capConnected(a.Dependencies.TopConnectedFiles, 4) {
		addEvidence(&evidence, "graph", file.Path, file.WhyItMatters, file.Path)
	}
	return result(question, strings.Join(fragments, " "), confidenceForCount(len(evidence)), evidence)
}

func answerChanges(a *models.Analysis, question string) *models.QAResult {
	changes := a.Changes
	if changes == nil || !changes.HasPrevious {
		message := "StackIndex needs at least two snapshots before it can answer change questions."
		if changes != nil && changes.Message != "" {
			message = changes.Message
		}
		return result(question, message, ConfidenceMedium, nil)
	}
	var fragments []string
	if len(changes.SummaryBullets) > 0 {
		fragments = append(fragments, strings.Join(changes.SummaryBullets, " "))
	}
	if len(changes.AddedRoutes) > 0 {
		fragments = append(fragments, "New routes: "+strings.Join(capStrings(changes.AddedRoutes, 6), ", ")+".")
	}
	if len(changes.RemovedRoutes) > 0 {
		fragments = append(fragments, "Removed routes: "+strings.Join(capStrings(changes.RemovedRoutes, 6), ", ")+".")
	}
	if len(changes.AddedEnvVars) > 0 || len(changes.RemovedEnvVars) > 0 {
		fragments = append(fragments, fmt.Sprintf("Environment variables changed: added %s; removed %s.", joinOrNone(changes.AddedEnvVars), joinOrNone(changes.RemovedEnvVars)))
	}
	if changes.AuditStatusBefore != "" && changes.AuditStatusAfter != "" && changes.AuditStatusBefore != changes.AuditStatusAfter {
		fragments = append(fragments, fmt.Sprintf("Audit status changed from %s to %s.", changes.AuditStatusBefore, changes.AuditStatusAfter))
	}
	if len(changes.AddedFindings) > 0 || len(changes.ResolvedFindings) > 0 {
		fragments = append(fragments, fmt.Sprintf("Findings changed: %d added, %d resolved.", len(changes.AddedFindings), len(changes.ResolvedFindings)))
	}
	if len(fragments) == 0 {
		fragments = append(fragments, "No deterministic changes were detected since the previous snapshot.")
	}
	evidence := []models.QAEvidence{}
	addEvidence(&evidence, "change", "Previous snapshot", changes.PreviousSnapshot, "")
	addEvidence(&evidence, "audit", "Audit status before", changes.AuditStatusBefore, "")
	addEvidence(&evidence, "audit", "Audit status after", changes.AuditStatusAfter, "")
	for _, route := range capStrings(changes.AddedRoutes, 6) {
		addEvidence(&evidence, "route", "Added route", route, "")
	}
	for _, route := range capStrings(changes.RemovedRoutes, 6) {
		addEvidence(&evidence, "route", "Removed route", route, "")
	}
	for _, name := range capStrings(changes.AddedEnvVars, 6) {
		addEvidence(&evidence, "env", "Added env var", name, "")
	}
	for _, name := range capStrings(changes.RemovedEnvVars, 6) {
		addEvidence(&evidence, "env", "Removed env var", name, "")
	}
	for _, finding := range capStrings(changes.AddedFindings, 4) {
		addEvidence(&evidence, "finding", "Added finding", finding, "")
	}
	for _, finding := range capStrings(changes.ResolvedFindings, 4) {
		addEvidence(&evidence, "finding", "Resolved finding", finding, "")
	}
	return result(question, strings.Join(fragments, " "), ConfidenceHigh, evidence)
}

func unsupported(question, detail string) *models.QAResult {
	answer := "StackIndex ask can answer local evidence questions about project purpose, detected stack, auth/login clues, database/storage, local setup scripts, API routes, important files, frontend/backend connections, deployment readiness, tests, environment configuration, and changes since the last snapshot."
	if detail != "" {
		answer = detail + " " + answer
	}
	answer += ` Try "Where is auth handled?", "Where is the database initialized?", "How do I run this locally?", "What files should I read first?", or "What changed since last analysis?".`
	return &models.QAResult{
		Question:   question,
		Answer:     answer,
		Confidence: ConfidenceLow,
		Mode:       ModeDeterministic,
		Warnings:   []string{"unsupported question type"},
	}
}

func result(question, answer, confidence string, evidence []models.QAEvidence) *models.QAResult {
	return &models.QAResult{
		Question:   question,
		Answer:     strings.TrimSpace(answer),
		Confidence: confidenceOr(confidence, ConfidenceMedium),
		Evidence:   evidence,
		Mode:       ModeDeterministic,
	}
}

func synthesizeWithAI(ctx context.Context, analysis *models.Analysis, deterministic *models.QAResult, opts Options) *models.QAResult {
	out := cloneResult(deterministic)
	out.Mode = ModeDeterministic
	modelsToTry := modelCandidates(opts.Model, opts.FallbackModels)
	factsheet := BuildFactsheet(deterministic)
	var raw, relevance string
	var attempted []string
	var lastWarning string
	for _, model := range modelsToTry {
		attempted = append(attempted, model)
		prompt := PromptFor(deterministic, factsheet)
		text, err := generate(ctx, prompt, model, opts)
		raw = text
		if err != nil {
			lastWarning = fmt.Sprintf("AI Q&A was requested but local Ollama failed for %s: %v", model, err)
			continue
		}
		if reason := validateAIAnswer(text, deterministic); reason != "" {
			lastWarning = "AI answer was rejected: " + reason
			relevance = reason
			continue
		}
		out.Answer = strings.TrimSpace(text)
		out.Mode = ModeAI
		out.Model = model
		out.AttemptedModels = append([]string{}, attempted...)
		writeDebug(opts.DebugDir, deterministic.Question, deterministic.Answer, factsheet, prompt, raw, "passed")
		return out
	}
	out.AttemptedModels = append([]string{}, attempted...)
	if lastWarning == "" {
		lastWarning = "AI Q&A was requested but no local model returned a usable answer."
	}
	out.Warnings = append(out.Warnings, lastWarning)
	writeDebug(opts.DebugDir, deterministic.Question, deterministic.Answer, factsheet, PromptFor(deterministic, factsheet), raw, relevanceOr(relevance, lastWarning))
	_ = analysis
	return out
}

func BuildFactsheet(result *models.QAResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Question: %s\n", result.Question)
	fmt.Fprintf(&b, "Deterministic answer: %s\n", result.Answer)
	fmt.Fprintf(&b, "Confidence: %s\n", result.Confidence)
	fmt.Fprintln(&b, "Evidence:")
	if len(result.Evidence) == 0 {
		fmt.Fprintln(&b, "- none")
	}
	for _, ev := range result.Evidence {
		path := ""
		if ev.Path != "" {
			path = " path=" + ev.Path
		}
		fmt.Fprintf(&b, "- kind=%s label=%s value=%s%s\n", ev.Kind, ev.Label, ev.Value, path)
	}
	if len(result.Warnings) > 0 {
		fmt.Fprintf(&b, "Warnings: %s\n", strings.Join(result.Warnings, "; "))
	}
	return b.String()
}

func PromptFor(result *models.QAResult, factsheet string) string {
	return `You are StackIndex, a local-only repository Q&A assistant.

Answer the user's question using only the deterministic StackIndex factsheet below.
Do not add facts, files, endpoints, environment variables, risks, tests, databases, or architecture claims that are not present in the factsheet.
Do not say you inspected source code directly.
If evidence is sparse, say what StackIndex detected and what it could not prove.
Return a concise plain-language answer in one short paragraph plus up to 3 bullets only when useful.

StackIndex Q&A factsheet:
` + factsheet
}

func generate(ctx context.Context, prompt, model string, opts Options) (string, error) {
	if opts.Generate != nil {
		return opts.Generate(ctx, prompt, model)
	}
	client := ai.OllamaClient{
		BaseURL: "http://127.0.0.1:11434",
		Model:   model,
		Client:  &http.Client{Timeout: 30 * time.Second},
	}
	checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := client.CheckAvailable(checkCtx); err != nil {
		return "", err
	}
	generateCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return client.Generate(generateCtx, prompt)
}

func validateAIAnswer(text string, deterministic *models.QAResult) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return "empty response"
	}
	lower := strings.ToLower(text)
	if strings.Contains(lower, "as an ai") || strings.Contains(lower, "i don't have access") || strings.Contains(lower, "not provided") {
		return "response did not answer from provided evidence"
	}
	if envVarNameRE.MatchString(text) && !evidenceContainsEnvName(deterministic.Evidence, text) {
		return "response listed environment variable names not present in evidence"
	}
	if len(deterministic.Evidence) == 0 {
		return ""
	}
	if !mentionsEvidence(text, deterministic.Evidence) && !sharesAnswerTerm(text, deterministic.Answer) {
		return "response did not mention provided evidence or deterministic answer terms"
	}
	return ""
}

func writeDebug(debugDir, question, deterministic, factsheet, prompt, raw, relevance string) {
	if strings.TrimSpace(debugDir) == "" {
		return
	}
	_ = os.MkdirAll(debugDir, 0755)
	files := map[string]string{
		"question.txt":             question + "\n",
		"deterministic-answer.txt": deterministic + "\n",
		"qa-factsheet.txt":         factsheet,
		"prompt.txt":               prompt,
	}
	if raw != "" {
		files["raw-response.txt"] = raw
	}
	data, _ := json.MarshalIndent(map[string]string{"relevance": relevanceOr(relevance, "not_evaluated")}, "", "  ")
	files["relevance-result.json"] = string(data) + "\n"
	for name, content := range files {
		_ = os.WriteFile(filepath.Join(debugDir, name), []byte(sanitizeDebugContent(content)), 0644)
	}
}

func WriteLatest(root string, result *models.QAResult) error {
	if result == nil {
		return errors.New("qa result is nil")
	}
	if err := os.MkdirAll(qaDir(root), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(LatestPath(root), data, 0644)
}

func AppendHistory(root string, result *models.QAResult) error {
	if result == nil {
		return errors.New("qa result is nil")
	}
	if err := os.MkdirAll(qaDir(root), 0755); err != nil {
		return err
	}
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	file, err := os.OpenFile(HistoryPath(root), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(data)
	return err
}

func WriteLatestAndAppendHistory(root string, result *models.QAResult) (latestErr, historyErr error) {
	latestErr = WriteLatest(root, result)
	if latestErr != nil {
		return latestErr, nil
	}
	historyErr = AppendHistory(root, result)
	return nil, historyErr
}

func ReadLatest(root string) (*models.QAResult, error) {
	data, err := os.ReadFile(LatestPath(root))
	if err != nil {
		return nil, err
	}
	var result models.QAResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	if strings.TrimSpace(result.Question) == "" && strings.TrimSpace(result.Answer) == "" {
		return nil, errors.New("latest qa result is empty")
	}
	return &result, nil
}

func ReadRecentHistory(root string, limit int) ([]models.QAResult, error) {
	if limit <= 0 {
		return nil, nil
	}
	file, err := os.Open(HistoryPath(root))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	var results []models.QAResult
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var result models.QAResult
		if err := json.Unmarshal([]byte(line), &result); err != nil {
			continue
		}
		if strings.TrimSpace(result.Question) == "" && strings.TrimSpace(result.Answer) == "" {
			continue
		}
		results = append(results, result)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(results) <= limit {
		reverseQAResults(results)
		return results, nil
	}
	recent := append([]models.QAResult(nil), results[len(results)-limit:]...)
	reverseQAResults(recent)
	return recent, nil
}

func LatestPath(root string) string {
	return filepath.Join(qaDir(root), "latest-question.json")
}

func HistoryPath(root string) string {
	return filepath.Join(qaDir(root), "history.jsonl")
}

func qaDir(root string) string {
	return filepath.Join(root, ".stackindex", "qa")
}

func reverseQAResults(results []models.QAResult) {
	for i, j := 0, len(results)-1; i < j; i, j = i+1, j-1 {
		results[i], results[j] = results[j], results[i]
	}
}

func MarshalJSON(result *models.QAResult) ([]byte, error) {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func FormatText(result *models.QAResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Answer:\n%s\n", readableAnswer(result.Answer))
	fmt.Fprintf(&b, "\nConfidence: %s\nMode: %s\n", result.Confidence, result.Mode)
	if result.Model != "" {
		fmt.Fprintf(&b, "Model: %s\n", result.Model)
	}
	if len(result.Warnings) > 0 {
		fmt.Fprintf(&b, "Warnings: %s\n", strings.Join(result.Warnings, "; "))
	}
	if len(result.Evidence) > 0 {
		fmt.Fprintln(&b, "\nEvidence:")
		for _, ev := range capEvidence(result.Evidence, 10) {
			path := ""
			if ev.Path != "" {
				path = " (`" + ev.Path + "`)"
			}
			fmt.Fprintf(&b, "- %s: %s%s\n", ev.Label, ev.Value, path)
		}
	}
	return b.String()
}

func readableAnswer(answer string) string {
	answer = strings.TrimSpace(answer)
	if answer == "" || strings.Contains(answer, "\n") {
		return answer
	}
	sentences := splitSentences(answer)
	if len(sentences) <= 1 {
		return answer
	}
	var lines []string
	var current string
	for _, sentence := range sentences {
		if current == "" {
			current = sentence
			continue
		}
		if len(current)+1+len(sentence) > 110 {
			lines = append(lines, current)
			current = sentence
		} else {
			current += " " + sentence
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return strings.Join(lines, "\n")
}

func splitSentences(answer string) []string {
	var sentences []string
	start := 0
	for i := 0; i < len(answer); i++ {
		if answer[i] != '.' && answer[i] != '!' && answer[i] != '?' {
			continue
		}
		end := i + 1
		if end < len(answer) && answer[end] != ' ' {
			continue
		}
		sentence := strings.TrimSpace(answer[start:end])
		if sentence != "" {
			sentences = append(sentences, sentence)
		}
		start = end
	}
	if tail := strings.TrimSpace(answer[start:]); tail != "" {
		sentences = append(sentences, tail)
	}
	return sentences
}

func modelCandidates(model string, fallbacks []string) []string {
	var candidates []string
	if strings.TrimSpace(model) == "" {
		candidates = append(candidates, ai.DefaultModel, ai.FallbackModel)
	} else {
		candidates = append(candidates, strings.TrimSpace(model))
	}
	candidates = append(candidates, fallbacks...)
	seen := map[string]bool{}
	var out []string
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

func localAudit(a *models.Analysis) models.AuditResult {
	result := models.AuditResult{Mode: "deployment-readiness"}
	if !stackDetected(a.Stack) {
		result.Reasons = append(result.Reasons, "No stack was detected.")
	}
	if a.Env.UsesEnvVars && a.Env.ExampleFile == "" {
		result.Reasons = append(result.Reasons, "Environment variables were detected but no `.env.example` file was found.")
	}
	if deploymentDetected(a) && !a.Deployment.HasHealthEndpoint {
		if backendSurface(a) {
			result.Reasons = append(result.Reasons, "Backend/API deployment surface detected but no health endpoint was found.")
		} else {
			result.Warnings = append(result.Warnings, "Deployment target detected without a health endpoint; this may be acceptable for static frontend apps.")
		}
	}
	if !a.Tests.HasTestFiles && !a.Tests.HasTestScript {
		result.Reasons = append(result.Reasons, "Tests were not detected.")
	}
	for _, finding := range a.Findings {
		switch finding.Severity {
		case models.SeverityHigh:
			result.Reasons = append(result.Reasons, "High finding: "+finding.Message)
		case models.SeverityMedium:
			result.Reasons = append(result.Reasons, "Medium finding: "+finding.Message)
		case models.SeverityLow:
			result.Warnings = append(result.Warnings, "Low finding: "+finding.Message)
		}
	}
	result.Passed = len(result.Reasons) == 0
	if !result.Passed {
		result.ExitCode = 1
	}
	result.HasBackendSurface = backendSurface(a)
	result.RequiresHealthEndpoint = deploymentDetected(a) && result.HasBackendSurface
	return result
}

func stackDetected(stack models.StackInfo) bool {
	return len(stack.Languages)+len(stack.Frameworks)+len(stack.Libraries)+len(stack.Databases)+len(stack.Testing)+len(stack.Deployment) > 0
}

func deploymentDetected(a *models.Analysis) bool {
	return len(a.Stack.Deployment) > 0 || a.Deployment.HasDockerfile || a.Deployment.HasVercelConfig
}

func backendSurface(a *models.Analysis) bool {
	if len(a.Routes) > 0 || a.Deployment.HasHealthEndpoint {
		return true
	}
	for _, framework := range a.Stack.Frameworks {
		if containsAny(strings.ToLower(framework), "express", "fastify", "koa", "hono", "fastapi") {
			return true
		}
	}
	return false
}

func routeGroups(routes []models.RouteInfo) []string {
	seen := map[string]bool{}
	var groups []string
	for _, route := range routes {
		group := routeGroup(route)
		if group != "" && !seen[group] {
			seen[group] = true
			groups = append(groups, group)
		}
		if len(groups) == 8 {
			break
		}
	}
	sort.Strings(groups)
	return groups
}

func routeGroup(route models.RouteInfo) string {
	source := filepath.ToSlash(route.SourceFile)
	if strings.Contains(source, "/api/") {
		before, after, _ := strings.Cut(source, "/api/")
		parts := strings.Split(after, "/")
		if len(parts) > 1 {
			return before + "/api/" + parts[0]
		}
		return before + "/api"
	}
	if strings.HasPrefix(source, "api/") {
		parts := strings.Split(source, "/")
		if len(parts) > 1 {
			return "api/" + parts[1]
		}
		return "api"
	}
	return filepath.ToSlash(filepath.Dir(source))
}

func deploymentSignals(d models.DeploymentAnalysis) string {
	var signals []string
	signals = append(signals, "README "+present(d.HasReadme))
	signals = append(signals, ".env.example "+present(d.HasEnvExample))
	signals = append(signals, "health endpoint "+present(d.HasHealthEndpoint))
	signals = append(signals, "Dockerfile "+present(d.HasDockerfile))
	signals = append(signals, "Vercel config "+present(d.HasVercelConfig))
	signals = append(signals, "migration files "+present(d.HasMigrationFiles))
	return "Readiness signals: " + strings.Join(signals, ", ") + "."
}

func addDeploymentEvidence(evidence *[]models.QAEvidence, d models.DeploymentAnalysis) {
	addEvidence(evidence, "audit", "README", fmt.Sprintf("%t", d.HasReadme), "README.md")
	addEvidence(evidence, "audit", ".env.example", fmt.Sprintf("%t", d.HasEnvExample), ".env.example")
	addEvidence(evidence, "audit", "Health endpoint", fmt.Sprintf("%t", d.HasHealthEndpoint), "")
	addEvidence(evidence, "audit", "Dockerfile", fmt.Sprintf("%t", d.HasDockerfile), "Dockerfile")
	for _, file := range capStrings(d.DeploymentFiles, 5) {
		addEvidence(evidence, "file", "Deployment file", file, file)
	}
	for _, file := range capStrings(d.MigrationFiles, 5) {
		addEvidence(evidence, "file", "Migration file", file, file)
	}
}

func databaseEvidence(a *models.Analysis) []models.QAEvidence {
	evidence := []models.QAEvidence{}
	for _, database := range a.Stack.Databases {
		addEvidence(&evidence, "database", "Detected database", database, "")
	}
	for _, name := range databaseEnvNames(a.Env) {
		addEvidence(&evidence, "env", "Database env", name, envVarFiles(a.Env, name))
	}
	for _, file := range a.Deployment.MigrationFiles {
		addEvidence(&evidence, "migration", "Migration file", file, file)
	}
	for _, script := range databaseScripts(a.PackageInfo) {
		addEvidence(&evidence, "script", "Database script", script, "package.json")
	}
	for _, dep := range databaseDependencies(a.PackageInfo) {
		addEvidence(&evidence, "package", "Database package", dep, "package.json")
	}
	for _, item := range databaseContextSignals(a) {
		addEvidence(&evidence, "context", "Database context", item, "")
	}
	for _, file := range databaseFiles(a) {
		addEvidence(&evidence, "file", "Database file", file, file)
	}
	return dedupeEvidence(evidence)
}

func databaseEnvNames(env models.EnvAnalysis) []string {
	seen := map[string]bool{}
	var out []string
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] || !looksDatabaseRelated(name) {
			return
		}
		seen[name] = true
		out = append(out, name)
	}
	for _, item := range env.UsedVars {
		add(item.Name)
	}
	for _, name := range env.ExampleVars {
		add(name)
	}
	for _, name := range env.MissingFromExample {
		add(name)
	}
	for _, name := range env.MissingRequiredFromExample {
		add(name)
	}
	sort.Strings(out)
	return out
}

func apiBaseEnvNames(env models.EnvAnalysis) []string {
	seen := map[string]bool{}
	var out []string
	add := func(name string) {
		name = strings.TrimSpace(name)
		upper := strings.ToUpper(name)
		if name == "" || seen[name] {
			return
		}
		if strings.Contains(upper, "API_BASE") || strings.Contains(upper, "BASE_URL") || strings.Contains(upper, "PUBLIC_API") || strings.Contains(upper, "VITE_API") || strings.Contains(upper, "NEXT_PUBLIC_API") {
			seen[name] = true
			out = append(out, name)
		}
	}
	for _, item := range env.UsedVars {
		add(item.Name)
	}
	for _, name := range env.ExampleVars {
		add(name)
	}
	sort.Strings(out)
	return out
}

func databaseScripts(pkg *models.PackageInfo) []string {
	if pkg == nil {
		return nil
	}
	var out []string
	for name, command := range pkg.Scripts {
		combined := strings.ToLower(name + " " + command)
		if containsAny(combined, "db:", "database", "migrate", "migration", "seed", "import", "embedding", "embeddings", "prisma", "drizzle", "pgvector", "neon", "postgres") {
			out = append(out, name+": "+command)
		}
	}
	sort.Strings(out)
	return out
}

func databaseDependencies(pkg *models.PackageInfo) []string {
	if pkg == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	addDeps := func(deps map[string]string) {
		for name, version := range deps {
			if !looksDatabaseRelated(name) {
				continue
			}
			value := name
			if version != "" {
				value += "@" + version
			}
			if !seen[value] {
				seen[value] = true
				out = append(out, value)
			}
		}
	}
	addDeps(pkg.Dependencies)
	addDeps(pkg.DevDependencies)
	sort.Strings(out)
	return out
}

func databaseFiles(a *models.Analysis) []string {
	seen := map[string]bool{}
	var out []string
	add := func(path, role string) {
		if path == "" || seen[path] {
			return
		}
		text := strings.ToLower(path + " " + role)
		if !containsAny(text, "database", "/db", "db/", "db.", "neon", "postgres", "pgvector", "prisma", "drizzle", "migration", "storage") {
			return
		}
		seen[path] = true
		out = append(out, path)
	}
	for _, file := range a.Structure.KeyFiles {
		add(file.Path, file.Role)
	}
	for _, file := range a.Dependencies.TopConnectedFiles {
		add(file.Path, file.Role+" "+file.WhyItMatters)
	}
	for _, node := range a.Dependencies.Nodes {
		add(node.Path, node.Role)
	}
	for _, file := range a.Files {
		add(file.Path, "")
	}
	sort.Strings(out)
	return out
}

func authEvidence(a *models.Analysis) []models.QAEvidence {
	evidence := []models.QAEvidence{}
	for _, route := range authRoutes(a.Routes) {
		addEvidence(&evidence, "route", strings.TrimSpace(route.Method+" "+route.Path), route.Confidence, route.SourceFile)
	}
	for _, file := range authFiles(a) {
		addEvidence(&evidence, "file", "Likely auth file", file, file)
	}
	for _, name := range authEnvNames(a.Env) {
		addEvidence(&evidence, "env", "Auth env", name, "")
	}
	for _, dep := range authDependencies(a.PackageInfo) {
		addEvidence(&evidence, "package", "Auth package", dep, "package.json")
	}
	return dedupeEvidence(evidence)
}

func authRoutes(routes []models.RouteInfo) []models.RouteInfo {
	var out []models.RouteInfo
	for _, route := range routes {
		combined := strings.ToLower(route.Path + " " + route.SourceFile)
		if looksAuthRelated(combined) {
			out = append(out, route)
		}
	}
	return capRoutes(out, 8)
}

func authFiles(a *models.Analysis) []string {
	seen := map[string]bool{}
	var out []string
	add := func(path, role string) {
		path = filepath.ToSlash(strings.TrimSpace(path))
		if path == "" || seen[path] {
			return
		}
		combined := strings.ToLower(path + " " + role)
		if !looksAuthRelated(combined) {
			return
		}
		seen[path] = true
		out = append(out, path)
	}
	for _, file := range a.Structure.KeyFiles {
		add(file.Path, file.Role)
	}
	for _, file := range a.Dependencies.TopConnectedFiles {
		add(file.Path, file.Role+" "+file.WhyItMatters)
	}
	for _, node := range a.Dependencies.Nodes {
		add(node.Path, node.Role)
	}
	for _, file := range a.Files {
		add(file.Path, "")
	}
	for _, route := range a.Routes {
		add(route.SourceFile, route.Path)
	}
	sort.Strings(out)
	return out
}

func authEnvNames(env models.EnvAnalysis) []string {
	seen := map[string]bool{}
	var out []string
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] || !looksAuthRelated(name) {
			return
		}
		seen[name] = true
		out = append(out, name)
	}
	for _, item := range env.UsedVars {
		add(item.Name)
	}
	for _, name := range env.ExampleVars {
		add(name)
	}
	for _, name := range env.MissingFromExample {
		add(name)
	}
	for _, name := range env.MissingRequiredFromExample {
		add(name)
	}
	sort.Strings(out)
	return out
}

func authDependencies(pkg *models.PackageInfo) []string {
	if pkg == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	addDeps := func(deps map[string]string) {
		for name, version := range deps {
			if !looksAuthRelated(name) {
				continue
			}
			value := name
			if version != "" {
				value += "@" + version
			}
			if !seen[value] {
				seen[value] = true
				out = append(out, value)
			}
		}
	}
	addDeps(pkg.Dependencies)
	addDeps(pkg.DevDependencies)
	sort.Strings(out)
	return out
}

func looksAuthRelated(value string) bool {
	lower := strings.ToLower(value)
	if containsAny(lower, "login", "signin", "sign-in", "sign_in", "session", "middleware", "protected", "jwt", "oauth", "nextauth", "next-auth", "clerk", "supabase/auth", "passport", "bcrypt", "password", "authentication") {
		return true
	}
	for _, token := range strings.FieldsFunc(lower, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	}) {
		if token == "auth" {
			return true
		}
	}
	return false
}

func confidenceForAuthEvidence(evidence []models.QAEvidence) string {
	strong := 0
	for _, item := range evidence {
		if item.Kind == "route" || item.Kind == "package" {
			strong++
		}
		if item.Kind == "file" && containsAny(item.Path, "/auth/", "auth.", "middleware.", "login") {
			strong++
		}
	}
	if strong >= 2 {
		return ConfidenceHigh
	}
	if len(evidence) > 0 {
		return ConfidenceMedium
	}
	return ConfidenceLow
}

func localRunScripts(pkg *models.PackageInfo) []string {
	if pkg == nil {
		return nil
	}
	priority := map[string]int{
		"dev":       10,
		"start":     20,
		"build":     30,
		"test":      40,
		"lint":      50,
		"typecheck": 60,
		"preview":   70,
	}
	var scripts []string
	for name, command := range pkg.Scripts {
		lowerName := strings.ToLower(name)
		lowerCommand := strings.ToLower(command)
		if _, ok := priority[lowerName]; ok || containsAny(lowerName+" "+lowerCommand, "dev", "start", "build", "test", "lint", "typecheck", "preview", "serve") {
			scripts = append(scripts, name+": "+command)
		}
	}
	sort.Slice(scripts, func(i, j int) bool {
		leftName, _, _ := strings.Cut(scripts[i], ":")
		rightName, _, _ := strings.Cut(scripts[j], ":")
		left, leftOK := priority[strings.ToLower(leftName)]
		right, rightOK := priority[strings.ToLower(rightName)]
		if leftOK && rightOK {
			return left < right
		}
		if leftOK {
			return true
		}
		if rightOK {
			return false
		}
		return scripts[i] < scripts[j]
	})
	return scripts
}

func goModuleName(pkg *models.PackageInfo) string {
	if pkg == nil {
		return ""
	}
	return strings.TrimSpace(pkg.ModuleName)
}

func hasPythonSource(a *models.Analysis) bool {
	for _, file := range a.Files {
		if strings.EqualFold(file.Language, "Python") || strings.EqualFold(filepath.Ext(file.Path), ".py") {
			return true
		}
	}
	return false
}

func rankedStartingFiles(a *models.Analysis) []models.FileRole {
	type scoredFile struct {
		file  models.FileRole
		score int
	}
	seen := map[string]int{}
	files := map[string]models.FileRole{}
	add := func(path, role, importance string, baseScore int) {
		path = filepath.ToSlash(strings.TrimSpace(path))
		if path == "" {
			return
		}
		score := baseScore + startingFileScore(path, role, importance)
		if current, ok := seen[path]; ok && current >= score {
			return
		}
		seen[path] = score
		files[path] = models.FileRole{Path: path, Role: fallback(role, inferredFileRole(path)), Importance: importance}
	}
	for _, file := range a.Structure.KeyFiles {
		add(file.Path, file.Role, file.Importance, 20)
	}
	for _, path := range a.Dependencies.Entrypoints {
		add(path, "Entrypoint", "high", 18)
	}
	for _, file := range a.Dependencies.TopConnectedFiles {
		add(file.Path, fallback(file.Role, file.WhyItMatters), "high", 12)
	}
	for _, route := range a.Routes {
		add(route.SourceFile, "API route handler", route.Confidence, 10)
	}
	for _, file := range a.Deployment.MigrationFiles {
		add(file, "Migration/schema file", "medium", 8)
	}
	for _, file := range a.Deployment.DeploymentFiles {
		add(file, "Deployment config", "medium", 7)
	}
	for _, file := range a.Files {
		add(file.Path, "", "", 0)
	}
	if a.Deployment.HasReadme {
		add("README.md", "Project overview and setup notes", "high", 30)
	}
	if a.PackageInfo != nil {
		add("package.json", "Package manifest and scripts", "high", 24)
		if a.PackageInfo.ModuleName != "" {
			add("go.mod", "Go module manifest", "high", 22)
		}
	}
	var scored []scoredFile
	for path, file := range files {
		scored = append(scored, scoredFile{file: file, score: seen[path]})
	}
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].file.Path < scored[j].file.Path
		}
		return scored[i].score > scored[j].score
	})
	out := make([]models.FileRole, 0, len(scored))
	for _, item := range scored {
		out = append(out, item.file)
	}
	return out
}

func startingFileScore(path, role, importance string) int {
	text := strings.ToLower(path + " " + role + " " + importance)
	score := 0
	switch {
	case strings.EqualFold(filepath.Base(path), "README.md"):
		score += 60
	case strings.EqualFold(filepath.Base(path), "package.json"), strings.EqualFold(filepath.Base(path), "go.mod"), strings.EqualFold(filepath.Base(path), "pyproject.toml"):
		score += 45
	}
	if containsAny(text, "entry", "main.", "app.", "page.", "route.", "server", "handler") {
		score += 20
	}
	if containsAny(text, "database", "/db", "db.", "schema", "migration") {
		score += 12
	}
	if containsAny(text, "config", "env", "docker", "vercel") {
		score += 8
	}
	if strings.Contains(text, "high") {
		score += 8
	}
	return score
}

func inferredFileRole(path string) string {
	lower := strings.ToLower(path)
	switch {
	case strings.EqualFold(filepath.Base(path), "README.md"):
		return "Project overview"
	case strings.EqualFold(filepath.Base(path), "package.json"):
		return "Package manifest and scripts"
	case strings.EqualFold(filepath.Base(path), "go.mod"):
		return "Go module manifest"
	case containsAny(lower, "route.", "/api/"):
		return "API route handler"
	case containsAny(lower, "db", "database", "schema", "migration"):
		return "Database/data layer"
	default:
		return "Relevant file"
	}
}

func databaseContextSignals(a *models.Analysis) []string {
	candidates := []string{a.Context.ReadmeSummary, a.Context.PackageDescription}
	candidates = append(candidates, a.Context.Evidence...)
	candidates = append(candidates, a.Context.DocSignals...)
	candidates = append(candidates, a.Context.EnvSignals...)
	var out []string
	for _, item := range candidates {
		item = strings.TrimSpace(item)
		if item != "" && looksDatabaseRelated(item) {
			out = append(out, item)
		}
		if len(out) == 5 {
			break
		}
	}
	return out
}

func databaseContextAnswer(signals []string) string {
	terms := []string{}
	joined := strings.ToLower(strings.Join(signals, " "))
	for _, term := range []string{"Neon Postgres", "Postgres", "pgvector", "migrations", "DATABASE_URL"} {
		if strings.Contains(joined, strings.ToLower(term)) {
			terms = append(terms, term)
		}
	}
	if len(terms) > 0 {
		return "Project context also mentions " + strings.Join(terms, ", ") + "."
	}
	return "Project context includes database/storage signals."
}

func looksDatabaseRelated(value string) bool {
	return containsAny(value, "database", "database_url", "postgres", "postgresql", "neon", "pgvector", "vector", "sqlite", "prisma", "drizzle", "migration", "migrations", "db_", "_db", "pg")
}

func frontendAPIClientFiles(a *models.Analysis) []string {
	seen := map[string]bool{}
	var out []string
	add := func(path, role string) {
		path = filepath.ToSlash(strings.TrimSpace(path))
		if path == "" || seen[path] {
			return
		}
		lower := strings.ToLower(path + " " + role)
		if isFrontendAPIClientPath(lower) {
			seen[path] = true
			out = append(out, path)
		}
	}
	for _, file := range a.Structure.KeyFiles {
		add(file.Path, file.Role)
	}
	for _, file := range a.Dependencies.TopConnectedFiles {
		add(file.Path, file.Role+" "+file.WhyItMatters)
	}
	for _, node := range a.Dependencies.Nodes {
		add(node.Path, node.Role)
	}
	for _, file := range a.Files {
		add(file.Path, "")
	}
	sort.Strings(out)
	return out
}

func backendFrameworkSignals(frameworks []string) []string {
	var out []string
	for _, framework := range frameworks {
		if containsAny(framework, "fastapi", "express", "fastify", "koa", "hono", "node.js") {
			out = append(out, framework)
		}
	}
	return out
}

func isFrontendAPIClientPath(lower string) bool {
	return strings.Contains(lower, "frontend/src/api/") ||
		strings.Contains(lower, "src/api/") ||
		strings.Contains(lower, "src/lib/api.") ||
		strings.Contains(lower, "src/lib/api/") ||
		strings.Contains(lower, "src/data/api.") ||
		strings.Contains(lower, "src/data/api/") ||
		strings.Contains(lower, "api-client") ||
		strings.Contains(lower, "apiclient") ||
		strings.Contains(lower, "client api")
}

func matchingDirectoryPrefixes(a *models.Analysis, prefixes ...string) []string {
	seen := map[string]bool{}
	var out []string
	for _, dir := range a.Structure.Directories {
		path := strings.Trim(filepath.ToSlash(dir.Path), "/")
		lower := strings.ToLower(path)
		for _, prefix := range prefixes {
			prefix = strings.Trim(strings.ToLower(prefix), "/")
			if lower == prefix || strings.HasPrefix(lower, prefix+"/") {
				root := strings.Split(path, "/")[0] + "/"
				if prefix == "src" {
					root = path
				}
				if !seen[root] {
					seen[root] = true
					out = append(out, root)
				}
			}
		}
	}
	sort.Strings(out)
	return out
}

func dedupeEvidence(items []models.QAEvidence) []models.QAEvidence {
	seen := map[string]bool{}
	var out []models.QAEvidence
	for _, item := range items {
		key := item.Kind + "\x00" + item.Label + "\x00" + item.Value + "\x00" + item.Path
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	return out
}

func addStackEvidence(evidence *[]models.QAEvidence, label string, values []string) {
	if len(values) > 0 {
		addEvidence(evidence, "context", label, strings.Join(values, ", "), "")
	}
}

func addEvidence(evidence *[]models.QAEvidence, kind, label, value, path string) {
	value = strings.TrimSpace(value)
	label = strings.TrimSpace(label)
	if value == "" && label == "" {
		return
	}
	*evidence = append(*evidence, models.QAEvidence{Kind: kind, Label: label, Value: value, Path: path})
}

func normalize(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.NewReplacer("?", "", ".", "", ",", "", "'", "").Replace(value)
	return strings.Join(strings.Fields(value), " ")
}

func containsAny(value string, needles ...string) bool {
	value = strings.ToLower(value)
	for _, needle := range needles {
		if strings.Contains(value, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func joinDirectoryRoles(dirs []models.DirectoryRole) string {
	var out []string
	for _, dir := range dirs {
		out = append(out, fmt.Sprintf("`%s` (%s)", dir.Path, dir.Role))
	}
	return strings.Join(out, ", ")
}

func joinFileRoles(files []models.FileRole) string {
	var out []string
	for _, file := range files {
		out = append(out, fmt.Sprintf("`%s` (%s)", file.Path, file.Role))
	}
	return strings.Join(out, ", ")
}

func joinConnectedFiles(files []models.ConnectedFileSummary) string {
	var out []string
	for _, file := range files {
		out = append(out, fmt.Sprintf("`%s` (%d imports, imported by %d)", file.Path, file.ImportsCount, file.ImportedByCount))
	}
	return strings.Join(out, ", ")
}

func capStrings(items []string, limit int) []string {
	if len(items) <= limit {
		return append([]string{}, items...)
	}
	return append([]string{}, items[:limit]...)
}

func capRoutes(items []models.RouteInfo, limit int) []models.RouteInfo {
	if len(items) <= limit {
		return append([]models.RouteInfo{}, items...)
	}
	return append([]models.RouteInfo{}, items[:limit]...)
}

func capDirectories(items []models.DirectoryRole, limit int) []models.DirectoryRole {
	if len(items) <= limit {
		return append([]models.DirectoryRole{}, items...)
	}
	return append([]models.DirectoryRole{}, items[:limit]...)
}

func capFiles(items []models.FileRole, limit int) []models.FileRole {
	if len(items) <= limit {
		return append([]models.FileRole{}, items...)
	}
	return append([]models.FileRole{}, items[:limit]...)
}

func capConnected(items []models.ConnectedFileSummary, limit int) []models.ConnectedFileSummary {
	if len(items) <= limit {
		return append([]models.ConnectedFileSummary{}, items...)
	}
	return append([]models.ConnectedFileSummary{}, items[:limit]...)
}

func capFindings(items []models.Finding, limit int) []models.Finding {
	if len(items) <= limit {
		return append([]models.Finding{}, items...)
	}
	return append([]models.Finding{}, items[:limit]...)
}

func capEnvVars(items []models.EnvVar, limit int) []models.EnvVar {
	if len(items) <= limit {
		return append([]models.EnvVar{}, items...)
	}
	return append([]models.EnvVar{}, items[:limit]...)
}

func capEvidence(items []models.QAEvidence, limit int) []models.QAEvidence {
	if len(items) <= limit {
		return append([]models.QAEvidence{}, items...)
	}
	return append([]models.QAEvidence{}, items[:limit]...)
}

func fallback(value, fallbackValue string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallbackValue
}

func envVarFiles(env models.EnvAnalysis, name string) string {
	for _, item := range env.UsedVars {
		if item.Name == name {
			return strings.Join(item.Files, ", ")
		}
	}
	return ""
}

func joinOrNone(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	return strings.Join(capStrings(values, 6), ", ")
}

func confidenceOr(value, fallback string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case ConfidenceHigh, ConfidenceMedium, ConfidenceLow:
		return value
	default:
		return fallback
	}
}

func confidenceForCount(count int) string {
	switch {
	case count >= 3:
		return ConfidenceHigh
	case count > 0:
		return ConfidenceMedium
	default:
		return ConfidenceLow
	}
}

func confidenceForBool(ok bool) string {
	if ok {
		return ConfidenceHigh
	}
	return ConfidenceMedium
}

func pluralS(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

func present(ok bool) string {
	if ok {
		return "present"
	}
	return "missing"
}

func cloneResult(in *models.QAResult) *models.QAResult {
	out := *in
	out.Evidence = append([]models.QAEvidence{}, in.Evidence...)
	out.AttemptedModels = append([]string{}, in.AttemptedModels...)
	out.Warnings = append([]string{}, in.Warnings...)
	return &out
}

func evidenceContainsEnvName(evidence []models.QAEvidence, text string) bool {
	for _, match := range envVarNameRE.FindAllString(text, -1) {
		found := false
		for _, ev := range evidence {
			if strings.Contains(ev.Value, match) || strings.Contains(ev.Label, match) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func mentionsEvidence(text string, evidence []models.QAEvidence) bool {
	lower := strings.ToLower(text)
	for _, ev := range evidence {
		for _, value := range []string{ev.Path, ev.Label, ev.Value} {
			value = strings.ToLower(strings.TrimSpace(value))
			if len(value) >= 4 && strings.Contains(lower, value) {
				return true
			}
			base := strings.ToLower(filepath.Base(value))
			if len(base) >= 4 && strings.Contains(lower, base) {
				return true
			}
		}
	}
	return false
}

func sharesAnswerTerm(text, answer string) bool {
	textTerms := keywordSet(text)
	for term := range keywordSet(answer) {
		if textTerms[term] {
			return true
		}
	}
	return false
}

func keywordSet(text string) map[string]bool {
	stop := map[string]bool{"this": true, "that": true, "with": true, "from": true, "have": true, "does": true, "were": true, "detected": true, "stackindex": true, "project": true}
	out := map[string]bool{}
	for _, raw := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-' || r == '/')
	}) {
		raw = strings.Trim(raw, "`")
		if len(raw) >= 5 && !stop[raw] {
			out[raw] = true
		}
	}
	return out
}

func sanitizeDebugContent(content string) string {
	var b bytes.Buffer
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if eq := strings.Index(line, "="); eq > 0 && !strings.Contains(line[:eq], " ") && envVarNameRE.MatchString(line[:eq]) {
			line = line[:eq+1] + "[redacted]"
		}
		if strings.Contains(strings.ToLower(trimmed), "secret") || strings.Contains(strings.ToLower(trimmed), "token") {
			line = envVarNameRE.ReplaceAllString(line, "[redacted-env]")
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

func relevanceOr(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
