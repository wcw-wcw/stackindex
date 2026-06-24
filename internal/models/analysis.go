package models

import "time"

type StackInfo struct {
	Languages  []string `json:"languages"`
	Frameworks []string `json:"frameworks"`
	Libraries  []string `json:"libraries"`
	Databases  []string `json:"databases"`
	Testing    []string `json:"testing"`
	Deployment []string `json:"deployment"`
}

type PackageInfo struct {
	Name               string            `json:"name,omitempty"`
	Description        string            `json:"description,omitempty"`
	ModuleName         string            `json:"moduleName,omitempty"`
	PackageManagerHint string            `json:"packageManagerHint,omitempty"`
	Scripts            map[string]string `json:"scripts,omitempty"`
	Dependencies       map[string]string `json:"dependencies,omitempty"`
	DevDependencies    map[string]string `json:"devDependencies,omitempty"`
}

type EnvVar struct {
	Name           string   `json:"name"`
	Files          []string `json:"files,omitempty"`
	Classification string   `json:"classification,omitempty"`
	ScriptOnly     bool     `json:"scriptOnly,omitempty"`
	MissingExample bool     `json:"missingExample,omitempty"`
}

type EnvAnalysis struct {
	UsesEnvVars                bool     `json:"usesEnvVars"`
	ExampleFile                string   `json:"exampleFile,omitempty"`
	ExampleVars                []string `json:"exampleVars,omitempty"`
	UsedVars                   []EnvVar `json:"usedVars,omitempty"`
	MissingFromExample         []string `json:"missingFromExample,omitempty"`
	MissingRequiredFromExample []string `json:"missingRequiredFromExample,omitempty"`
	EnvFilePresent             bool     `json:"envFilePresent"`
}

type TestAnalysis struct {
	HasTestFiles       bool     `json:"hasTestFiles"`
	HasTestScript      bool     `json:"hasTestScript"`
	Frameworks         []string `json:"frameworks,omitempty"`
	TestFiles          []string `json:"testFiles,omitempty"`
	TestScript         string   `json:"testScript,omitempty"`
	PlaywrightDetected bool     `json:"playwrightDetected"`
}

type DeploymentAnalysis struct {
	HasReadme                bool     `json:"hasReadme"`
	ReadmeMentionsSetup      bool     `json:"readmeMentionsSetup"`
	ReadmeMentionsDeploy     bool     `json:"readmeMentionsDeploy"`
	HasEnvExample            bool     `json:"hasEnvExample"`
	HasDockerfile            bool     `json:"hasDockerfile"`
	HasVercelConfig          bool     `json:"hasVercelConfig"`
	HasHealthEndpoint        bool     `json:"hasHealthEndpoint"`
	HasMigrationFiles        bool     `json:"hasMigrationFiles"`
	ReadmeMentionsMigrations bool     `json:"readmeMentionsMigrations"`
	DeploymentFiles          []string `json:"deploymentFiles,omitempty"`
	MigrationFiles           []string `json:"migrationFiles,omitempty"`
}

type AISummary struct {
	Enabled              bool      `json:"enabled"`
	Model                string    `json:"model,omitempty"`
	AttemptedModels      []string  `json:"attemptedModels,omitempty"`
	GeneratedAt          time.Time `json:"generatedAt,omitempty"`
	ProjectSummary       string    `json:"projectSummary,omitempty"`
	ArchitectureOverview string    `json:"architectureOverview,omitempty"`
	KeyStrengths         []string  `json:"keyStrengths,omitempty"`
	PotentialRisks       []string  `json:"potentialRisks,omitempty"`
	RecommendedNextSteps []string  `json:"recommendedNextSteps,omitempty"`
	LocalNotes           string    `json:"localNotes,omitempty"`
	Status               string    `json:"status,omitempty"`
	RawText              string    `json:"rawText,omitempty"`
	RetryRawText         string    `json:"retryRawText,omitempty"`
	ParseError           string    `json:"parseError,omitempty"`
	Relevance            string    `json:"relevance,omitempty"`
	RelevanceReason      string    `json:"relevanceReason,omitempty"`
	Warning              string    `json:"warning,omitempty"`
}

type AuditResult struct {
	Passed                 bool     `json:"passed"`
	ExitCode               int      `json:"exitCode"`
	Reasons                []string `json:"reasons,omitempty"`
	Warnings               []string `json:"warnings,omitempty"`
	Mode                   string   `json:"mode"`
	AllowMedium            bool     `json:"allowMedium"`
	AllowMissingTests      bool     `json:"allowMissingTests"`
	FailOnLow              bool     `json:"failOnLow"`
	HasBackendSurface      bool     `json:"hasBackendSurface"`
	RequiresHealthEndpoint bool     `json:"requiresHealthEndpoint"`
}

type QAResult struct {
	Question        string       `json:"question"`
	Answer          string       `json:"answer"`
	Confidence      string       `json:"confidence"`
	Evidence        []QAEvidence `json:"evidence,omitempty"`
	Mode            string       `json:"mode"`
	Model           string       `json:"model,omitempty"`
	AttemptedModels []string     `json:"attemptedModels,omitempty"`
	Warnings        []string     `json:"warnings,omitempty"`
}

type QAEvidence struct {
	Kind  string `json:"kind"`
	Label string `json:"label"`
	Value string `json:"value"`
	Path  string `json:"path,omitempty"`
}

type ProjectContext struct {
	Purpose            string   `json:"purpose"`
	Confidence         string   `json:"confidence"`
	Evidence           []string `json:"evidence,omitempty"`
	ReadmeTitle        string   `json:"readmeTitle,omitempty"`
	ReadmeSummary      string   `json:"readmeSummary,omitempty"`
	PackageName        string   `json:"packageName,omitempty"`
	PackageDescription string   `json:"packageDescription,omitempty"`
	DocSignals         []string `json:"docSignals,omitempty"`
	ScriptSignals      []string `json:"scriptSignals,omitempty"`
	EnvSignals         []string `json:"envSignals,omitempty"`
}

type StructureMap struct {
	Directories []DirectoryRole `json:"directories,omitempty"`
	KeyFiles    []FileRole      `json:"keyFiles,omitempty"`
}

type FeatureMap struct {
	Features    []FeatureCluster `json:"features,omitempty"`
	RouteChains []RouteChain     `json:"routeChains,omitempty"`
}

type FeatureCluster struct {
	Name         string   `json:"name"`
	StartHere    []string `json:"startHere,omitempty"`
	RelatedTests []string `json:"relatedTests,omitempty"`
	SearchTerms  []string `json:"searchTerms,omitempty"`
	AvoidFirst   []string `json:"avoidFirst,omitempty"`
	Routes       []string `json:"routes,omitempty"`
	Confidence   string   `json:"confidence"`
}

type RouteChain struct {
	Route   string   `json:"route"`
	Files   []string `json:"files,omitempty"`
	Tests   []string `json:"tests,omitempty"`
	Summary string   `json:"summary,omitempty"`
}

type SymbolIndex struct {
	Files []FileSymbols `json:"files,omitempty"`
}

type FileSymbols struct {
	Path    string           `json:"path"`
	Symbols []ExportedSymbol `json:"symbols,omitempty"`
}

type ExportedSymbol struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type DependencyGraph struct {
	Nodes                  []DependencyNode       `json:"nodes,omitempty"`
	Edges                  []DependencyEdge       `json:"edges,omitempty"`
	Entrypoints            []string               `json:"entrypoints,omitempty"`
	UnresolvedImports      []UnresolvedImport     `json:"unresolvedImports,omitempty"`
	TopConnectedFiles      []ConnectedFileSummary `json:"topConnectedFiles,omitempty"`
	ArchitectureHints      []string               `json:"architectureHints,omitempty"`
	AliasConfig            *AliasConfigInfo       `json:"aliasConfig,omitempty"`
	AliasImportsResolved   int                    `json:"aliasImportsResolved"`
	AliasImportsUnresolved int                    `json:"aliasImportsUnresolved"`
}

type AliasConfigInfo struct {
	Source  string              `json:"source,omitempty"`
	BaseURL string              `json:"baseUrl,omitempty"`
	Paths   map[string][]string `json:"paths,omitempty"`
}

type DependencyNode struct {
	Path            string `json:"path"`
	Role            string `json:"role,omitempty"`
	Language        string `json:"language,omitempty"`
	ImportsCount    int    `json:"importsCount"`
	ImportedByCount int    `json:"importedByCount"`
	Importance      string `json:"importance"`
}

type DependencyEdge struct {
	From       string `json:"from"`
	To         string `json:"to,omitempty"`
	ImportPath string `json:"importPath"`
	Kind       string `json:"kind"`
	Confidence string `json:"confidence"`
}

type UnresolvedImport struct {
	From       string `json:"from"`
	ImportPath string `json:"importPath"`
	Reason     string `json:"reason"`
}

type ConnectedFileSummary struct {
	Path            string `json:"path"`
	Role            string `json:"role,omitempty"`
	ImportsCount    int    `json:"importsCount"`
	ImportedByCount int    `json:"importedByCount"`
	WhyItMatters    string `json:"whyItMatters"`
}

type DirectoryRole struct {
	Path      string   `json:"path"`
	Role      string   `json:"role"`
	Evidence  []string `json:"evidence,omitempty"`
	FileCount int      `json:"fileCount"`
}

type FileRole struct {
	Path       string   `json:"path"`
	Role       string   `json:"role"`
	Evidence   []string `json:"evidence,omitempty"`
	Importance string   `json:"importance"`
}

type Analysis struct {
	RepoPath     string             `json:"repoPath"`
	RepoName     string             `json:"repoName"`
	GeneratedAt  time.Time          `json:"generatedAt"`
	Files        []FileInfo         `json:"files"`
	Quality      IndexQuality       `json:"indexQuality"`
	Stack        StackInfo          `json:"stack"`
	PackageInfo  *PackageInfo       `json:"packageInfo,omitempty"`
	Context      ProjectContext     `json:"projectContext"`
	Structure    StructureMap       `json:"structureMap"`
	Features     FeatureMap         `json:"featureMap"`
	Symbols      SymbolIndex        `json:"symbolIndex"`
	Dependencies DependencyGraph    `json:"dependencyGraph"`
	Env          EnvAnalysis        `json:"env"`
	Routes       []RouteInfo        `json:"routes"`
	Tests        TestAnalysis       `json:"tests"`
	Deployment   DeploymentAnalysis `json:"deployment"`
	Findings     []Finding          `json:"findings"`
	AI           *AISummary         `json:"ai,omitempty"`
	Audit        *AuditResult       `json:"audit,omitempty"`
	Changes      *ChangeSummary     `json:"changes,omitempty"`
}

type ChangeSummary struct {
	HasPrevious             bool      `json:"hasPrevious"`
	Message                 string    `json:"message,omitempty"`
	PreviousSnapshot        string    `json:"previousSnapshot,omitempty"`
	CurrentSnapshot         string    `json:"currentSnapshot,omitempty"`
	GeneratedAt             time.Time `json:"generatedAt,omitempty"`
	SummaryBullets          []string  `json:"summaryBullets,omitempty"`
	AddedRoutes             []string  `json:"addedRoutes,omitempty"`
	RemovedRoutes           []string  `json:"removedRoutes,omitempty"`
	AddedEnvVars            []string  `json:"addedEnvVars,omitempty"`
	RemovedEnvVars          []string  `json:"removedEnvVars,omitempty"`
	AddedFindings           []string  `json:"addedFindings,omitempty"`
	ResolvedFindings        []string  `json:"resolvedFindings,omitempty"`
	AuditStatusBefore       string    `json:"auditStatusBefore,omitempty"`
	AuditStatusAfter        string    `json:"auditStatusAfter,omitempty"`
	StackChanges            []string  `json:"stackChanges,omitempty"`
	FrameworkChanges        []string  `json:"frameworkChanges,omitempty"`
	DatabaseChanges         []string  `json:"databaseChanges,omitempty"`
	TestSignalChanges       []string  `json:"testSignalChanges,omitempty"`
	DeploymentSignalChanges []string  `json:"deploymentSignalChanges,omitempty"`
	KeyFileChanges          []string  `json:"keyFileChanges,omitempty"`
}
