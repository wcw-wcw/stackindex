export type AnalyzeRequest = {
  path: string;
  runAudit: boolean;
  useAI: boolean;
  model: string;
};

export type AnalyzeResponse = {
  repoName: string;
  repoPath: string;
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

declare global {
  interface Window {
    go?: {
      main?: {
        App?: {
          AnalyzeProject(request: AnalyzeRequest): Promise<AnalyzeResponse>;
          BrowseFolder(): Promise<string>;
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

export function browseFolder() {
  return backend().BrowseFolder();
}
