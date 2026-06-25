export type AnalyzeRequest = {
  path: string;
  runAudit: boolean;
  useAI: boolean;
  model: string;
};

export type GitHubAnalyzeRequest = {
  url: string;
  runAudit: boolean;
  useAI: boolean;
  model: string;
  refresh: boolean;
};

export type AnalyzeResponse = {
  repoName: string;
  repoPath: string;
  sourceType?: string;
  githubUrl?: string;
  localCachePath?: string;
  generatedAt: string;
  files: number;
  routes: number;
  tests: number;
  findings: Record<string, number>;
  stack: string[];
  languages: string[];
  frameworks: string[];
  databases: string[];
  deployment: string[];
  auditStatus?: string;
  auditExitCode?: number;
  aiStatus?: string;
  aiModel?: string;
  jsonReportPath: string;
  mdReportPath: string;
  context: ContextView;
  audit: AuditView;
  apiRoutes: RouteView[];
  testSummary: TestsView;
  deploymentInfo: DeploymentView;
  ai: AIView;
  reports: ReportsView;
  loadedFromDisk?: boolean;
};

export type RecentProject = {
  repoName: string;
  repoPath: string;
  sourceType?: string;
  githubUrl?: string;
  localCachePath?: string;
  lastAnalyzed: string;
  files: number;
  routes: number;
  tests: number;
  findings: Record<string, number>;
  auditStatus?: string;
  aiStatus?: string;
  aiModel?: string;
  jsonReportPath: string;
  mdReportPath: string;
};

export type OllamaModelView = {
  name: string;
  modifiedAt?: string;
  size?: number;
};

export type OllamaModelsResponse = {
  available: boolean;
  models: OllamaModelView[];
  message?: string;
};

export type DesktopSettings = {
  defaultRunAudit: boolean;
  defaultUseAI: boolean;
  defaultModel: string;
};

export type DesktopPaths = {
  recentProjectsPath: string;
  githubCacheRoot: string;
  settingsPath: string;
};

export type PathActionRequest = {
  path: string;
};

export type ReportFileResponse = {
  path: string;
  name: string;
  content: string;
};

export type CLICommandRequest = {
  repoPath: string;
  sourceType?: string;
  localCachePath?: string;
  auditStatus?: string;
  aiStatus?: string;
  aiModel?: string;
};

export type ContextView = {
  purpose: string;
  confidence: string;
  evidence: string[];
  readmeTitle?: string;
  readmeSummary?: string;
  packageName?: string;
  packageDescription?: string;
};

export type AuditView = {
  status: string;
  exitCode?: number;
  blockers: string[];
  warnings: string[];
  mode?: string;
  hasBackendSurface: boolean;
  requiresHealthEndpoint: boolean;
};

export type RouteView = {
  method: string;
  path: string;
  sourceFile: string;
  confidence: string;
  note?: string;
};

export type TestsView = {
  hasTestFiles: boolean;
  hasTestScript: boolean;
  frameworks: string[];
  testFiles: string[];
  testScript?: string;
  playwrightDetected: boolean;
};

export type DeploymentView = {
  hasReadme: boolean;
  hasEnvExample: boolean;
  hasDockerfile: boolean;
  hasVercelConfig: boolean;
  hasHealthEndpoint: boolean;
  hasMigrationFiles: boolean;
  readmeMentionsDeploy: boolean;
  readmeMentionsMigrations: boolean;
  deploymentFiles: string[];
  migrationFiles: string[];
};

export type AIView = {
  status: string;
  model?: string;
  attemptedModels: string[];
  projectSummary?: string;
  architectureOverview?: string;
  keyStrengths: string[];
  potentialRisks: string[];
  recommendedNextSteps: string[];
  localNotes?: string;
  deterministicSummary: string;
  warning?: string;
};

export type ReportsView = {
  jsonPath: string;
  markdownPath: string;
  fullMarkdownPath: string;
  directory: string;
  history: SnapshotView[];
  changes: ChangeView;
};

export type SnapshotView = {
  timestamp: string;
  directory: string;
  jsonPath: string;
  markdownPath: string;
  fullMarkdownPath: string;
  auditStatus: string;
  aiStatus: string;
  generatedAt?: string;
};

export type ChangeView = {
  hasPrevious: boolean;
  message?: string;
  previousSnapshot?: string;
  currentGenerated?: string;
  summaryBullets: string[];
  addedRoutes: string[];
  removedRoutes: string[];
  addedEnvVars: string[];
  removedEnvVars: string[];
  addedFindings: string[];
  resolvedFindings: string[];
  auditStatusBefore?: string;
  auditStatusAfter?: string;
};

declare global {
  interface Window {
    go?: {
      main?: {
        App?: {
          AnalyzeGitHubRepo(request: GitHubAnalyzeRequest): Promise<AnalyzeResponse>;
          AnalyzeProject(request: AnalyzeRequest): Promise<AnalyzeResponse>;
          BrowseFolder(): Promise<string>;
          ClearGitHubCache(): Promise<void>;
          ClearRecentProjects(): Promise<void>;
          GenerateCLICommand(request: CLICommandRequest): Promise<string>;
          GetDesktopPaths(): Promise<DesktopPaths>;
          GetDesktopSettings(): Promise<DesktopSettings>;
          GetRecentProjects(): Promise<RecentProject[]>;
          ListOllamaModels(): Promise<OllamaModelsResponse>;
          OpenJSONReport(request: PathActionRequest): Promise<void>;
          OpenExistingReport(path: string): Promise<AnalyzeResponse>;
          OpenMarkdownReport(request: PathActionRequest): Promise<void>;
          ReadGeneratedFile(request: PathActionRequest): Promise<ReportFileResponse>;
          RevealProjectFolder(request: PathActionRequest): Promise<void>;
          RevealSnapshotFolder(request: PathActionRequest): Promise<void>;
          RevealStackIndexFolder(request: PathActionRequest): Promise<void>;
          RemoveRecentProject(path: string): Promise<void>;
          SaveDesktopSettings(settings: DesktopSettings): Promise<DesktopSettings>;
        };
      };
    };
  }
}

const backend = () => {
  const app = window.go?.main?.App;
  if (!app) {
    throw new Error('Wails backend is not connected. Run this app with `wails dev` from desktop/.');
  }
  return app;
};

export function analyzeProject(request: AnalyzeRequest) {
  return backend().AnalyzeProject(request);
}

export function analyzeGitHubRepo(request: GitHubAnalyzeRequest) {
  return backend().AnalyzeGitHubRepo(request);
}

export function openExistingReport(path: string) {
  return backend().OpenExistingReport(path);
}

export function getRecentProjects() {
  return backend().GetRecentProjects();
}

export function listOllamaModels() {
  return backend().ListOllamaModels();
}

export function removeRecentProject(path: string) {
  return backend().RemoveRecentProject(path);
}

export function clearRecentProjects() {
  return backend().ClearRecentProjects();
}

export function clearGitHubCache() {
  return backend().ClearGitHubCache();
}

export function getDesktopSettings() {
  return backend().GetDesktopSettings();
}

export function saveDesktopSettings(settings: DesktopSettings) {
  return backend().SaveDesktopSettings(settings);
}

export function getDesktopPaths() {
  return backend().GetDesktopPaths();
}

export function revealProjectFolder(request: PathActionRequest) {
  return backend().RevealProjectFolder(request);
}

export function revealStackIndexFolder(request: PathActionRequest) {
  return backend().RevealStackIndexFolder(request);
}

export function revealSnapshotFolder(request: PathActionRequest) {
  return backend().RevealSnapshotFolder(request);
}

export function openMarkdownReport(request: PathActionRequest) {
  return backend().OpenMarkdownReport(request);
}

export function openJSONReport(request: PathActionRequest) {
  return backend().OpenJSONReport(request);
}

export function readGeneratedFile(request: PathActionRequest) {
  return backend().ReadGeneratedFile(request);
}

export function generateCLICommand(request: CLICommandRequest) {
  return backend().GenerateCLICommand(request);
}

export function browseFolder() {
  return backend().BrowseFolder();
}
