import { FormEvent } from 'react';
import { OllamaModelsResponse, RecentProject } from '../wails';

type SourceMode = 'local' | 'github';

type LandingPageProps = {
  sourceMode: SourceMode;
  path: string;
  githubUrl: string;
  runAudit: boolean;
  useAI: boolean;
  model: string;
  status: string;
  error: string;
  isRunning: boolean;
  recentProjects: RecentProject[];
  ollamaModels: OllamaModelsResponse;
  isLoadingModels: boolean;
  onSourceModeChange: (value: SourceMode) => void;
  onPathChange: (value: string) => void;
  onGitHubUrlChange: (value: string) => void;
  onRunAuditChange: (value: boolean) => void;
  onUseAIChange: (value: boolean) => void;
  onModelChange: (value: string) => void;
  onRefreshModels: () => void;
  onBrowse: () => void;
  onAnalyze: (event: FormEvent) => void;
  onOpenReport: (path: string) => void;
  onAnalyzeAgain: (project: RecentProject) => void;
  onRemoveRecent: (path: string) => void;
  onClearRecent: () => void;
  onOpenSettings: () => void;
};

export function LandingPage({
  sourceMode,
  path,
  githubUrl,
  runAudit,
  useAI,
  model,
  status,
  error,
  isRunning,
  recentProjects,
  ollamaModels,
  isLoadingModels,
  onSourceModeChange,
  onPathChange,
  onGitHubUrlChange,
  onRunAuditChange,
  onUseAIChange,
  onModelChange,
  onRefreshModels,
  onBrowse,
  onAnalyze,
  onOpenReport,
  onAnalyzeAgain,
  onRemoveRecent,
  onClearRecent,
  onOpenSettings,
}: LandingPageProps) {
  return (
    <section className="panel landing-panel">
      <div className="landing-header">
        <div>
          <div className="eyebrow">Local-first codebase analyzer</div>
          <h1>StackMap</h1>
          <p className="intro">Analyze a project on this machine and export the familiar `.stackmap` JSON and Markdown reports.</p>
        </div>
        <button type="button" className="secondary" onClick={onOpenSettings} disabled={isRunning}>Settings</button>
      </div>

      <form onSubmit={onAnalyze}>
        <label>Source</label>
        <div className="source-tabs" role="tablist" aria-label="Project source">
          <button type="button" className={sourceMode === 'local' ? 'active' : ''} onClick={() => onSourceModeChange('local')} disabled={isRunning}>
            Local
          </button>
          <button type="button" className={sourceMode === 'github' ? 'active' : ''} onClick={() => onSourceModeChange('github')} disabled={isRunning}>
            GitHub
          </button>
        </div>

        {sourceMode === 'local' ? (
          <>
            <div className="path-row">
              <input id="path" value={path} onChange={(event) => onPathChange(event.target.value)} placeholder="/path/to/project" disabled={isRunning} />
              <button type="button" className="secondary" onClick={onBrowse} disabled={isRunning}>Browse folder</button>
            </div>
            <p className="selected">Selected: <code>{path || 'No path selected'}</code></p>
          </>
        ) : (
          <div className="github-source">
            <label htmlFor="github-url">GitHub URL</label>
            <input
              id="github-url"
              value={githubUrl}
              onChange={(event) => onGitHubUrlChange(event.target.value)}
              placeholder="https://github.com/owner/repo"
              disabled={isRunning}
            />
            <p className="selected">Public HTTPS github.com repositories only. Cloned locally into the StackMap cache. Refresh is not implemented.</p>
          </div>
        )}

        <div className="toggles">
          <label className="toggle">
            <input type="checkbox" checked={runAudit} onChange={(event) => onRunAuditChange(event.target.checked)} />
            <span>Run audit</span>
          </label>
          <label className="toggle">
            <input type="checkbox" checked={useAI} onChange={(event) => onUseAIChange(event.target.checked)} />
            <span>Generate local AI summary</span>
          </label>
        </div>

        <div className={`model-picker ${useAI ? '' : 'is-disabled'}`}>
          <div className="model-picker-header">
            <label htmlFor="model">Model</label>
            <button type="button" className="secondary compact" onClick={onRefreshModels} disabled={!useAI || isRunning || isLoadingModels}>
              {isLoadingModels ? 'Refreshing...' : 'Refresh models'}
            </button>
          </div>
          <select id="model" value={model} onChange={(event) => onModelChange(event.target.value)} disabled={!useAI || isRunning}>
            <option value="">Default local chain</option>
            {selectedModelMissing(model, ollamaModels) && <option value={model}>{model}</option>}
            {ollamaModels.models.map((item) => (
              <option key={item.name} value={item.name}>
                {item.name}
              </option>
            ))}
          </select>
          <p className="model-status">{modelStatusText(useAI, ollamaModels, isLoadingModels)}</p>
        </div>

        <button type="submit" disabled={isRunning}>{isRunning ? 'Analyzing...' : sourceMode === 'github' ? 'Analyze GitHub Repo' : 'Analyze'}</button>
      </form>

      <p className="status">{status}</p>
      {error && <p className="error">{error}</p>}
      <RecentProjects
        projects={recentProjects}
        isRunning={isRunning}
        onOpenReport={onOpenReport}
        onAnalyzeAgain={onAnalyzeAgain}
        onRemoveRecent={onRemoveRecent}
        onClearRecent={onClearRecent}
      />
    </section>
  );
}

function RecentProjects({
  projects,
  isRunning,
  onOpenReport,
  onAnalyzeAgain,
  onRemoveRecent,
  onClearRecent,
}: {
  projects: RecentProject[];
  isRunning: boolean;
  onOpenReport: (path: string) => void;
  onAnalyzeAgain: (project: RecentProject) => void;
  onRemoveRecent: (path: string) => void;
  onClearRecent: () => void;
}) {
  return (
    <section className="recent-projects">
      <div className="recent-header">
        <div>
          <h2>Recent Projects</h2>
          <p>Open an existing `.stackmap` report without rerunning analysis.</p>
        </div>
        {projects.length > 0 && (
          <button type="button" className="secondary compact" onClick={onClearRecent} disabled={isRunning}>
            Clear all
          </button>
        )}
      </div>
      {projects.length === 0 ? (
        <p className="empty">No recent projects yet.</p>
      ) : (
        <div className="recent-list">
          {projects.map((project) => (
            <div className="recent-row" key={project.repoPath}>
              <div className="recent-main">
                <strong>{project.repoName || project.repoPath}</strong>
                {project.sourceType === 'github' && project.githubUrl && <span className="source-tag">github {project.githubUrl}</span>}
                <code>{project.repoPath}</code>
                <div className="recent-meta">
                  <span>{project.lastAnalyzed || 'unknown time'}</span>
                  <span>{project.files} files</span>
                  <span>{project.routes} routes</span>
                  <span>{project.tests} tests</span>
                  <span>{findingText(project.findings)}</span>
                  <span>audit: {project.auditStatus || 'not run'}</span>
                  <span>AI: {aiText(project)}</span>
                </div>
              </div>
              <div className="recent-actions">
                <button type="button" onClick={() => onOpenReport(project.repoPath)} disabled={isRunning}>
                  Open Report
                </button>
                <button type="button" className="secondary" onClick={() => onAnalyzeAgain(project)} disabled={isRunning}>
                  Analyze Again
                </button>
                <button type="button" className="secondary" onClick={() => onRemoveRecent(project.repoPath)} disabled={isRunning}>
                  Remove
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </section>
  );
}

function findingText(findings: Record<string, number>) {
  return `findings H${findings.high ?? 0}/M${findings.medium ?? 0}/L${findings.low ?? 0}`;
}

function aiText(project: RecentProject) {
  if (project.aiStatus === 'generated' && project.aiModel) {
    return `${project.aiStatus} ${project.aiModel}`;
  }
  return project.aiStatus || 'not requested';
}

function selectedModelMissing(model: string, ollamaModels: OllamaModelsResponse) {
  if (!model) {
    return false;
  }
  return !ollamaModels.models.some((item) => item.name === model);
}

function modelStatusText(useAI: boolean, ollamaModels: OllamaModelsResponse, isLoadingModels: boolean) {
  if (!useAI) {
    return 'AI disabled';
  }
  if (isLoadingModels) {
    return 'Checking local Ollama models...';
  }
  if (ollamaModels.message) {
    return ollamaModels.message;
  }
  if (ollamaModels.available) {
    return 'Ollama models loaded.';
  }
  return 'Ollama unavailable - default analysis still works without AI.';
}
