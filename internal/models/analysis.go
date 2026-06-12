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
	GeneratedAt          time.Time `json:"generatedAt,omitempty"`
	ProjectSummary       string    `json:"projectSummary,omitempty"`
	ArchitectureOverview string    `json:"architectureOverview,omitempty"`
	KeyStrengths         []string  `json:"keyStrengths,omitempty"`
	PotentialRisks       []string  `json:"potentialRisks,omitempty"`
	RecommendedNextSteps []string  `json:"recommendedNextSteps,omitempty"`
	RawText              string    `json:"rawText,omitempty"`
	Warning              string    `json:"warning,omitempty"`
}

type Analysis struct {
	RepoPath    string             `json:"repoPath"`
	RepoName    string             `json:"repoName"`
	GeneratedAt time.Time          `json:"generatedAt"`
	Files       []FileInfo         `json:"files"`
	Stack       StackInfo          `json:"stack"`
	PackageInfo *PackageInfo       `json:"packageInfo,omitempty"`
	Env         EnvAnalysis        `json:"env"`
	Routes      []RouteInfo        `json:"routes"`
	Tests       TestAnalysis       `json:"tests"`
	Deployment  DeploymentAnalysis `json:"deployment"`
	Findings    []Finding          `json:"findings"`
	AI          *AISummary         `json:"ai,omitempty"`
}
