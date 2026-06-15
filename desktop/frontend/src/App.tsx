import { FormEvent, useState } from 'react';
import { analyzeProject, browseFolder, AnalyzeResponse } from './wails';

const defaultPath = '/Users/will/Workspace/stkapp';

export default function App() {
  const [path, setPath] = useState(defaultPath);
  const [runAudit, setRunAudit] = useState(true);
  const [useAI, setUseAI] = useState(false);
  const [model, setModel] = useState('');
  const [status, setStatus] = useState('Ready to analyze a local project.');
  const [error, setError] = useState('');
  const [result, setResult] = useState<AnalyzeResponse | null>(null);
  const [isRunning, setIsRunning] = useState(false);

  async function pickFolder() {
    setError('');
    try {
      const selected = await browseFolder();
      if (selected) {
        setPath(selected);
        setStatus(`Selected ${selected}`);
      }
    } catch (err) {
      setError(errorMessage(err));
    }
  }

  async function analyze(event: FormEvent) {
    event.preventDefault();
    setError('');
    setResult(null);
    setIsRunning(true);
    setStatus('Analyzing project and writing .stackmap reports...');
    try {
      const nextResult = await analyzeProject({
        path,
        runAudit,
        useAI,
        model,
      });
      setResult(nextResult);
      setStatus('Analysis complete.');
    } catch (err) {
      setError(errorMessage(err));
      setStatus('Analysis failed.');
    } finally {
      setIsRunning(false);
    }
  }

  if (result) {
    return (
      <main className="shell">
        <section className="panel result-panel">
          <div className="eyebrow">StackMap desktop MVP</div>
          <h1>{result.repoName}</h1>
          <p className="muted">{result.repoPath}</p>

          <div className="chips">
            {result.stack.length ? result.stack.map((item) => <span key={item}>{item}</span>) : <span>No stack detected</span>}
          </div>

          <div className="metrics">
            <Metric label="Files" value={result.files} />
            <Metric label="Routes" value={result.routes} />
            <Metric label="Tests" value={result.tests} />
          </div>

          <div className="summary-grid">
            <Summary label="Findings" value={findingSummary(result.findings)} />
            <Summary label="Audit" value={auditLabel(result)} />
            <Summary label="AI" value={aiLabel(result)} />
          </div>

          <div className="report-paths">
            <h2>Reports</h2>
            <code>{result.jsonReportPath}</code>
            <code>{result.mdReportPath}</code>
          </div>

          <button className="secondary" onClick={() => result && setResult(null)}>Run again</button>
        </section>
      </main>
    );
  }

  return (
    <main className="shell">
      <section className="panel">
        <div className="eyebrow">Local-first codebase analyzer</div>
        <h1>StackMap</h1>
        <p className="intro">Analyze a project on this machine and export the familiar `.stackmap` JSON and Markdown reports.</p>

        <form onSubmit={analyze}>
          <label htmlFor="path">Project path</label>
          <div className="path-row">
            <input id="path" value={path} onChange={(event) => setPath(event.target.value)} placeholder="/path/to/project" />
            <button type="button" className="secondary" onClick={pickFolder}>Browse folder</button>
          </div>
          <p className="selected">Selected: {path || 'No path selected'}</p>

          <div className="toggles">
            <label className="toggle">
              <input type="checkbox" checked={runAudit} onChange={(event) => setRunAudit(event.target.checked)} />
              <span>Run audit</span>
            </label>
            <label className="toggle">
              <input type="checkbox" checked={useAI} onChange={(event) => setUseAI(event.target.checked)} />
              <span>Generate local AI summary</span>
            </label>
          </div>

          <label htmlFor="model">Model</label>
          <input id="model" value={model} onChange={(event) => setModel(event.target.value)} placeholder="Empty uses the existing default local model chain" />

          <button type="submit" disabled={isRunning}>{isRunning ? 'Analyzing...' : 'Analyze'}</button>
        </form>

        <p className="status">{status}</p>
        {error && <p className="error">{error}</p>}
      </section>
    </main>
  );
}

function Metric({ label, value }: { label: string; value: number }) {
  return (
    <div>
      <strong>{value}</strong>
      <span>{label}</span>
    </div>
  );
}

function Summary({ label, value }: { label: string; value: string }) {
  return (
    <div className="summary">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function findingSummary(findings: Record<string, number>) {
  return `High ${findings.high ?? 0} / Medium ${findings.medium ?? 0} / Low ${findings.low ?? 0}`;
}

function auditLabel(result: AnalyzeResponse) {
  if (result.auditStatus === 'failed') {
    return `failed (exit ${result.auditExitCode ?? 1})`;
  }
  return result.auditStatus || 'not run';
}

function aiLabel(result: AnalyzeResponse) {
  if (result.aiStatus === 'generated' && result.aiModel) {
    return `generated with ${result.aiModel}`;
  }
  return result.aiStatus || 'not requested';
}

function errorMessage(err: unknown) {
  if (err instanceof Error) {
    return err.message;
  }
  return String(err);
}
