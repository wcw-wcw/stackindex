export type AnalyzeRequest = {
  path: string;
  runAudit: boolean;
  useAI: boolean;
  model: string;
};

export type AnalyzeResponse = {
  repoName: string;
  repoPath: string;
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
