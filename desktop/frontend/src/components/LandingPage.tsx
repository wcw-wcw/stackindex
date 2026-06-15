import { FormEvent } from 'react';
import { RecentProject } from '../wails';

type LandingPageProps = {
  path: string;
  runAudit: boolean;
  useAI: boolean;
  model: string;
  status: string;
  error: string;
  isRunning: boolean;
  recentProjects: RecentProject[];
  onPathChange: (value: string) => void;
  onRunAuditChange: (value: boolean) => void;
  onUseAIChange: (value: boolean) => void;
  onModelChange: (value: string) => void;
  onBrowse: () => void;
  onAnalyze: (event: FormEvent) => void;
  onOpenReport: (path: string) => void;
  onAnalyzeAgain: (path: string) => void;
  onRemoveRecent: (path: string) => void;
  onClearRecent: () => void;
};

export function LandingPage({
  path,
  runAudit,
  useAI,
  model,
  status,
  error,
  isRunning,
  recentProjects,
  onPathChange,
  onRunAuditChange,
  onUseAIChange,
  onModelChange,
  onBrowse,
  onAnalyze,
  onOpenReport,
  onAnalyzeAgain,
  onRemoveRecent,
  onClearRecent,
}: LandingPageProps) {
  return (
    <section className="panel landing-panel">
      <div className="eyebrow">Local-first codebase analyzer</div>
      <h1>StackMap</h1>
      <p className="intro">Analyze a project on this machine and export the familiar `.stackmap` JSON and Markdown reports.</p>

      <form onSubmit={onAnalyze}>
        <label htmlFor="path">Project path</label>
        <div className="path-row">
          <input id="path" value={path} onChange={(event) => onPathChange(event.target.value)} placeholder="/path/to/project" />
          <button type="button" className="secondary" onClick={onBrowse}>Browse folder</button>
        </div>
        <p className="selected">Selected: <code>{path || 'No path selected'}</code></p>

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

        <label htmlFor="model">Model</label>
        <input id="model" value={model} onChange={(event) => onModelChange(event.target.value)} placeholder="Empty uses the existing default local model chain" />

        <button type="submit" disabled={isRunning}>{isRunning ? 'Analyzing...' : 'Analyze'}</button>
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
  onAnalyzeAgain: (path: string) => void;
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
                <button type="button" className="secondary" onClick={() => onAnalyzeAgain(project.repoPath)} disabled={isRunning}>
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
