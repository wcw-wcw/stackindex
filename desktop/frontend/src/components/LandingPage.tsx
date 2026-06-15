import { FormEvent } from 'react';

type LandingPageProps = {
  path: string;
  runAudit: boolean;
  useAI: boolean;
  model: string;
  status: string;
  error: string;
  isRunning: boolean;
  onPathChange: (value: string) => void;
  onRunAuditChange: (value: boolean) => void;
  onUseAIChange: (value: boolean) => void;
  onModelChange: (value: string) => void;
  onBrowse: () => void;
  onAnalyze: (event: FormEvent) => void;
};

export function LandingPage({
  path,
  runAudit,
  useAI,
  model,
  status,
  error,
  isRunning,
  onPathChange,
  onRunAuditChange,
  onUseAIChange,
  onModelChange,
  onBrowse,
  onAnalyze,
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
    </section>
  );
}
