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
  directory: string;
};

export type AskRequest = {
  question: string;
};

export type AskResponse = {
  question: string;
  answer: string;
  confidence: string;
  mode: string;
  evidence: QAEvidenceView[];
  warnings?: string[];
};

export type QAEvidenceView = {
  kind: string;
  label: string;
  value: string;
  path?: string;
};

declare global {
  interface Window {
    go?: {
      main?: {
        App?: {
          AnalyzeGitHubRepo(request: GitHubAnalyzeRequest): Promise<AnalyzeResponse>;
          AnalyzeProject(request: AnalyzeRequest): Promise<AnalyzeResponse>;
          AskQuestion(request: AskRequest): Promise<AskResponse>;
          BrowseFolder(): Promise<string>;
          ClearRecentProjects(): Promise<void>;
          GetRecentProjects(): Promise<RecentProject[]>;
          ListOllamaModels(): Promise<OllamaModelsResponse>;
          OpenExistingReport(path: string): Promise<AnalyzeResponse>;
          RemoveRecentProject(path: string): Promise<void>;
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

export function askQuestion(request: AskRequest) {
  return backend().AskQuestion(request);
}

export function browseFolder() {
  return backend().BrowseFolder();
}
