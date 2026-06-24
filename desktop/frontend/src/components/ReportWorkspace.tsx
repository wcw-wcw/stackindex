import { useEffect, useRef, useState } from 'react';
import {
  AnalyzeResponse,
  generateCLICommand,
  openJSONReport,
  openMarkdownReport,
  revealProjectFolder,
  revealSnapshotFolder,
  revealStackIndexFolder,
  SnapshotView,
} from '../wails';
import { MetricCard } from './MetricCard';
import { ReportPath } from './ReportPath';
import { Sidebar } from './Sidebar';
import { StackChip } from './StackChip';
import { StatusBadge } from './StatusBadge';
import { SectionId } from './sections';

export function ReportWorkspace({ result, onRunAgain, onOpenSettings }: { result: AnalyzeResponse; onRunAgain: () => void; onOpenSettings: () => void }) {
  const [activeSection, setActiveSection] = useState<SectionId>('overview');
  const scrollRef = useRef<HTMLElement | null>(null);

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = 0;
    }
  }, [activeSection]);

  return (
    <section className="workspace">
      <header className="project-header">
        <div>
          <div className="eyebrow">StackIndex report workspace</div>
          <h1>{result.repoName}</h1>
          <p className="muted code-line">{result.repoPath}</p>
        </div>
        <div className="header-actions">
          <span className="generated">{result.loadedFromDisk ? 'Loaded report generated' : 'Generated'} {result.generatedAt}</span>
          <button className="secondary" onClick={onOpenSettings}>Settings</button>
          <button className="secondary" onClick={onRunAgain}>Run again</button>
        </div>
      </header>

      <div className="workspace-grid">
        <Sidebar active={activeSection} onSelect={setActiveSection} />
        <article className="detail-panel" ref={scrollRef}>
          {activeSection === 'overview' && <Overview result={result} />}
          {activeSection === 'audit' && <Audit result={result} />}
          {activeSection === 'context' && <Context result={result} />}
          {activeSection === 'routes' && <Routes result={result} />}
          {activeSection === 'tests' && <Tests result={result} />}
          {activeSection === 'ai' && <AINotes result={result} />}
          {activeSection === 'reports' && <Reports result={result} />}
        </article>
      </div>
    </section>
  );
}

function Overview({ result }: { result: AnalyzeResponse }) {
  return (
    <>
      <SectionHeader title="Overview" subtitle="Project shape, detected stack, and local analysis status." />
      <div className="metrics">
        <MetricCard label="Files" value={result.files} />
        <MetricCard label="Routes" value={result.routes} />
        <MetricCard label="Tests" value={result.tests} />
        <MetricCard label="High" value={result.findings.high ?? 0} tone="danger" />
        <MetricCard label="Medium" value={result.findings.medium ?? 0} tone="warning" />
        <MetricCard label="Low" value={result.findings.low ?? 0} />
      </div>
      <StackGroup values={result.stack} />
      <div className="summary-grid">
        <Summary label="Purpose" value={result.context.purpose || 'Unknown project purpose'} />
        <Summary label="Audit" value={<StatusBadge status={result.auditStatus || 'not run'} />} />
        <Summary label="AI" value={<StatusBadge status={aiLabel(result)} />} />
      </div>
      <p className="body-copy">{result.ai.deterministicSummary}</p>
    </>
  );
}

function Audit({ result }: { result: AnalyzeResponse }) {
  const audit = result.audit;
  return (
    <>
      <SectionHeader title="Audit" subtitle="Local deterministic checks. No cloud services involved." />
      <div className="summary-grid">
        <Summary label="Status" value={<StatusBadge status={audit.status} />} />
        <Summary label="Exit code" value={audit.status === 'not run' ? 'Not run' : String(audit.exitCode ?? 0)} />
        <Summary label="Backend surface" value={audit.hasBackendSurface ? 'Detected' : 'Not detected'} />
      </div>
      <ListBlock title="Blockers" items={audit.blockers} empty="No audit blockers reported." />
      <ListBlock title="Warnings" items={audit.warnings} empty="No audit warnings reported." />
      <CheckList
        items={[
          ['Requires health endpoint', audit.requiresHealthEndpoint],
          ['Health endpoint present', result.deploymentInfo.hasHealthEndpoint],
          ['Env example present', result.deploymentInfo.hasEnvExample],
          ['Migration files present', result.deploymentInfo.hasMigrationFiles],
        ]}
      />
    </>
  );
}

function Context({ result }: { result: AnalyzeResponse }) {
  return (
    <>
      <SectionHeader title="Context" subtitle="Purpose inference and evidence gathered from local files." />
      <div className="summary-grid">
        <Summary label="Purpose" value={result.context.purpose || 'Unknown'} />
        <Summary label="Confidence" value={result.context.confidence || 'Unknown'} />
        <Summary label="Package" value={result.context.packageName || result.repoName} />
      </div>
      {result.context.readmeTitle && <p className="body-copy"><strong>README:</strong> {result.context.readmeTitle}</p>}
      {result.context.readmeSummary && <p className="body-copy">{result.context.readmeSummary}</p>}
      {result.context.packageDescription && <p className="body-copy">{result.context.packageDescription}</p>}
      <ListBlock title="Evidence" items={result.context.evidence} empty="No purpose evidence captured." />
    </>
  );
}

function Routes({ result }: { result: AnalyzeResponse }) {
  return (
    <>
      <SectionHeader title="API Routes" subtitle={`${result.apiRoutes.length} route${result.apiRoutes.length === 1 ? '' : 's'} detected.`} />
      {result.apiRoutes.length === 0 ? (
        <p className="empty">No API routes detected.</p>
      ) : (
        <div className="route-list">
          {result.apiRoutes.map((route, index) => (
            <div className="route-row" key={`${route.method}-${route.path}-${route.sourceFile}-${index}`}>
              <span className="method">{route.method}</span>
              <code>{route.path}</code>
              <span className="route-meta">{route.confidence} confidence</span>
              <span className="route-source">{route.sourceFile}</span>
              {route.note && <p>{route.note}</p>}
            </div>
          ))}
        </div>
      )}
    </>
  );
}

function Tests({ result }: { result: AnalyzeResponse }) {
  const tests = result.testSummary;
  return (
    <>
      <SectionHeader title="Tests" subtitle="Detected test files, scripts, and frameworks." />
      <div className="summary-grid">
        <Summary label="Test files" value={tests.hasTestFiles ? 'Detected' : 'Not detected'} />
        <Summary label="Test script" value={tests.hasTestScript ? 'Detected' : 'Not detected'} />
        <Summary label="Playwright" value={tests.playwrightDetected ? 'Detected' : 'Not detected'} />
      </div>
      {tests.testScript && <ReportPath label="Script" path={tests.testScript} />}
      <StackGroup values={tests.frameworks} empty="No test frameworks detected." />
      <ListBlock title="Test files" items={tests.testFiles} empty="No test files detected." />
    </>
  );
}

function AINotes({ result }: { result: AnalyzeResponse }) {
  const ai = result.ai;
  return (
    <>
      <SectionHeader title="AI Notes" subtitle="Optional local AI summary status and deterministic fallback." />
      <div className="summary-grid">
        <Summary label="Status" value={<StatusBadge status={ai.status} />} />
        <Summary label="Model" value={ai.model || result.aiModel || 'Not requested'} />
        <Summary label="Attempted" value={ai.attemptedModels.length ? ai.attemptedModels.join(', ') : 'None'} />
      </div>
      {ai.warning && <p className="warning-note">{ai.warning}</p>}
      {ai.projectSummary && <p className="body-copy">{ai.projectSummary}</p>}
      {ai.architectureOverview && <p className="body-copy">{ai.architectureOverview}</p>}
      {ai.localNotes ? <p className="body-copy pre-line">{ai.localNotes}</p> : <p className="empty">No local AI notes available for this run.</p>}
      <ListBlock title="Strengths" items={ai.keyStrengths} empty="No local strengths returned." />
      <ListBlock title="Risks" items={ai.potentialRisks} empty="No local risks returned." />
      <ListBlock title="Recommended next steps" items={ai.recommendedNextSteps} empty="No AI next steps returned." />
      <h3>Deterministic Summary</h3>
      <p className="body-copy">{ai.deterministicSummary}</p>
    </>
  );
}

function Reports({ result }: { result: AnalyzeResponse }) {
  const [actionStatus, setActionStatus] = useState('');
  const [actionError, setActionError] = useState('');
  const history = result.reports.history ?? [];

  async function runAction(label: string, action: () => Promise<void>) {
    setActionError('');
    setActionStatus(`${label}...`);
    try {
      await action();
      setActionStatus(`${label} done.`);
    } catch (err) {
      setActionError(errorMessage(err));
      setActionStatus(`${label} failed.`);
    }
  }

  async function copyText(label: string, value: string) {
    const text = value.trim();
    if (!text) {
      throw new Error(`${label} is not available.`);
    }
    if (!navigator.clipboard?.writeText) {
      throw new Error('Clipboard is not available in this desktop view.');
    }
    await navigator.clipboard.writeText(text);
  }

  async function copyCLICommand() {
    const command = await generateCLICommand({
      repoPath: result.repoPath,
      sourceType: result.sourceType,
      localCachePath: result.localCachePath,
      auditStatus: result.auditStatus,
      aiStatus: result.aiStatus,
      aiModel: result.aiModel || result.ai.model,
    });
    await copyText('CLI command', command);
  }

  async function copyAgentInstructions() {
    await copyText('agent instructions', agentInstructions(result));
  }

  return (
    <>
      <SectionHeader title="Reports" subtitle="Files written by the local StackIndex analysis run." />
      <div className="report-paths">
        <ReportPath label="JSON" path={result.reports.jsonPath} />
        <ReportPath label="Compact Markdown" path={result.reports.markdownPath} />
        <ReportPath label="Full Markdown" path={result.reports.fullMarkdownPath} />
        <ReportPath label="Directory" path={result.reports.directory} />
      </div>
      <div className="report-actions">
        <button type="button" className="secondary compact" onClick={() => runAction('Copy project path', () => copyText('Project path', result.repoPath))}>
          Copy project path
        </button>
        <button type="button" className="secondary compact" onClick={() => runAction('Copy JSON report path', () => copyText('JSON report path', result.reports.jsonPath))}>
          Copy JSON report path
        </button>
        <button type="button" className="secondary compact" onClick={() => runAction('Copy Markdown report path', () => copyText('Markdown report path', result.reports.markdownPath))}>
          Copy Markdown report path
        </button>
        <button type="button" className="secondary compact" onClick={() => runAction('Copy agent instructions', copyAgentInstructions)}>
          Copy agent instructions
        </button>
        <button type="button" className="secondary compact" onClick={() => runAction('Copy full Markdown report path', () => copyText('Full Markdown report path', result.reports.fullMarkdownPath))}>
          Copy full Markdown report path
        </button>
        <button type="button" className="secondary compact" onClick={() => runAction('Reveal project folder', () => revealProjectFolder({ path: result.repoPath }))}>
          Reveal project folder
        </button>
        <button type="button" className="secondary compact" onClick={() => runAction('Reveal .stackindex', () => revealStackIndexFolder({ path: result.repoPath }))}>
          Reveal .stackindex
        </button>
        <button type="button" className="secondary compact" onClick={() => runAction('Open Markdown report', () => openMarkdownReport({ path: result.reports.markdownPath }))}>
          Open Markdown report
        </button>
        <button type="button" className="secondary compact" onClick={() => runAction('Open full Markdown report', () => openMarkdownReport({ path: result.reports.fullMarkdownPath }))}>
          Open full Markdown report
        </button>
        <button type="button" className="secondary compact" onClick={() => runAction('Open JSON report', () => openJSONReport({ path: result.reports.jsonPath }))}>
          Open JSON report
        </button>
        <button type="button" className="secondary compact" onClick={() => runAction('Copy CLI command', copyCLICommand)}>
          Copy CLI command
        </button>
      </div>
      {result.sourceType === 'github' && (
        <p className="selected">Desktop GitHub repositories are analyzed from the local cached clone, so the copied CLI command uses that cache path.</p>
      )}
      <ChangesPanel changes={result.reports.changes} />
      <div className="history-section">
        <div className="history-header">
          <h3>History</h3>
          <span>{history.length} snapshot{history.length === 1 ? '' : 's'}</span>
        </div>
        {history.length === 0 ? (
          <p className="body-copy">No local snapshots found in `.stackindex/history`.</p>
        ) : (
          <div className="snapshot-list">
            {history.slice(0, 8).map((snapshot) => (
              <SnapshotRow key={snapshot.directory} snapshot={snapshot} runAction={runAction} />
            ))}
          </div>
        )}
      </div>
      {actionStatus && <p className="status">{actionStatus}</p>}
      {actionError && <p className="error">{actionError}</p>}
      <p className="body-copy">Reports stay in `.stackindex` inside the analyzed project.</p>
    </>
  );
}

function agentInstructions(result: AnalyzeResponse) {
  return [
    `Read ${result.reports.markdownPath} before broad searches.`,
    'Use Feature Map for feature work.',
    'Use Route Implementation Chains for API work.',
    'Use Task Search Recipes before searching the whole repo.',
    `Open ${result.reports.fullMarkdownPath} only when compact output is insufficient.`,
    'Avoid generated/cache folders such as .stackindex, node_modules, dist, build, and framework caches.',
  ].join('\n');
}

function ChangesPanel({ changes }: { changes: AnalyzeResponse['reports']['changes'] }) {
  if (!changes?.hasPrevious) {
    return (
      <div className="changes-section">
        <div className="history-header">
          <h3>Changes since previous snapshot</h3>
        </div>
        <p className="body-copy">{changes?.message || 'No previous snapshot yet. Run StackIndex again after another analysis to see changes.'}</p>
      </div>
    );
  }
  return (
    <div className="changes-section">
      <div className="history-header">
        <h3>Changes since previous snapshot</h3>
        <span>{changes.previousSnapshot}</span>
      </div>
      <div className="change-meta">
        <span>current: {changes.currentGenerated || 'unknown'}</span>
        <span>audit: {changes.auditStatusBefore || 'unknown'} -&gt; {changes.auditStatusAfter || 'unknown'}</span>
      </div>
      <ul className="change-bullets">
        {(changes.summaryBullets.length > 0 ? changes.summaryBullets : ['No deterministic changes were detected since the previous snapshot.']).map((item) => (
          <li key={item}>{item}</li>
        ))}
      </ul>
      <div className="change-lists">
        <ChangeList label="Added routes" values={changes.addedRoutes} />
        <ChangeList label="Removed routes" values={changes.removedRoutes} />
        <ChangeList label="Added env vars" values={changes.addedEnvVars} />
        <ChangeList label="Removed env vars" values={changes.removedEnvVars} />
        <ChangeList label="Added findings" values={changes.addedFindings} />
        <ChangeList label="Resolved findings" values={changes.resolvedFindings} />
      </div>
    </div>
  );
}

function ChangeList({ label, values }: { label: string; values: string[] }) {
  return (
    <div className="change-list">
      <span>{label}</span>
      <code>{values.length > 0 ? values.join(', ') : 'none'}</code>
    </div>
  );
}

function SnapshotRow({ snapshot, runAction }: { snapshot: SnapshotView; runAction: (label: string, action: () => Promise<void>) => Promise<void> }) {
  const generated = snapshot.generatedAt && snapshot.generatedAt !== 'unknown' ? snapshot.generatedAt : snapshot.timestamp;
  return (
    <div className="snapshot-row">
      <div className="snapshot-meta">
        <strong>{generated}</strong>
        <span>{snapshot.timestamp}</span>
      </div>
      <div className="snapshot-statuses">
        <StatusBadge status={snapshot.auditStatus || 'unknown'} />
        <StatusBadge status={snapshot.aiStatus || 'unknown'} />
      </div>
      <div className="snapshot-paths">
        <ReportPath label="JSON" path={snapshot.jsonPath} />
        <ReportPath label="Compact Markdown" path={snapshot.markdownPath} />
        <ReportPath label="Full Markdown" path={snapshot.fullMarkdownPath} />
      </div>
      <div className="report-actions snapshot-actions">
        <button type="button" className="secondary compact" onClick={() => runAction('Open snapshot Markdown', () => openMarkdownReport({ path: snapshot.markdownPath }))}>
          Open Markdown
        </button>
        <button type="button" className="secondary compact" onClick={() => runAction('Open snapshot full Markdown', () => openMarkdownReport({ path: snapshot.fullMarkdownPath }))}>
          Open full Markdown
        </button>
        <button type="button" className="secondary compact" onClick={() => runAction('Open snapshot JSON', () => openJSONReport({ path: snapshot.jsonPath }))}>
          Open JSON
        </button>
        <button type="button" className="secondary compact" onClick={() => runAction('Reveal snapshot folder', () => revealSnapshotFolder({ path: snapshot.directory }))}>
          Reveal folder
        </button>
      </div>
    </div>
  );
}

function SectionHeader({ title, subtitle }: { title: string; subtitle: string }) {
  return (
    <div className="section-header">
      <h2>{title}</h2>
      <p>{subtitle}</p>
    </div>
  );
}

function Summary({ label, value }: { label: string; value: string | JSX.Element }) {
  return (
    <div className="summary">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function StackGroup({ values, empty = 'No stack detected.' }: { values: string[]; empty?: string }) {
  if (!values.length) {
    return <p className="empty">{empty}</p>;
  }
  return (
    <div className="chips">
      {values.map((item) => <StackChip key={item}>{item}</StackChip>)}
    </div>
  );
}

function ListBlock({ title, items, empty }: { title: string; items: string[]; empty: string }) {
  return (
    <section className="list-block">
      <h3>{title}</h3>
      {items.length ? (
        <ul>
          {items.map((item) => <li key={item}>{item}</li>)}
        </ul>
      ) : (
        <p className="empty">{empty}</p>
      )}
    </section>
  );
}

function CheckList({ items }: { items: [string, boolean][] }) {
  return (
    <div className="check-list">
      {items.map(([label, checked]) => (
        <div key={label}>
          <span className={checked ? 'dot success' : 'dot muted'} />
          <span>{label}</span>
          <strong>{checked ? 'Yes' : 'No'}</strong>
        </div>
      ))}
    </div>
  );
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
