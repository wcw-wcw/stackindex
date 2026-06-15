import { useState } from 'react';
import { AnalyzeResponse } from '../wails';
import { MetricCard } from './MetricCard';
import { ReportPath } from './ReportPath';
import { Sidebar } from './Sidebar';
import { StackChip } from './StackChip';
import { StatusBadge } from './StatusBadge';
import { SectionId } from './sections';

export function ReportWorkspace({ result, onRunAgain }: { result: AnalyzeResponse; onRunAgain: () => void }) {
  const [activeSection, setActiveSection] = useState<SectionId>('overview');

  return (
    <section className="workspace">
      <header className="project-header">
        <div>
          <div className="eyebrow">StackMap report workspace</div>
          <h1>{result.repoName}</h1>
          <p className="muted code-line">{result.repoPath}</p>
        </div>
        <div className="header-actions">
          <span className="generated">Generated {result.generatedAt}</span>
          <button className="secondary" onClick={onRunAgain}>Run again</button>
        </div>
      </header>

      <div className="workspace-grid">
        <Sidebar active={activeSection} onSelect={setActiveSection} />
        <article className="detail-panel">
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
  return (
    <>
      <SectionHeader title="Reports" subtitle="Files written by the local StackMap analysis run." />
      <div className="report-paths">
        <ReportPath label="JSON" path={result.reports.jsonPath} />
        <ReportPath label="Markdown" path={result.reports.markdownPath} />
        <ReportPath label="Directory" path={result.reports.directory} />
      </div>
      <p className="body-copy">Reports stay in `.stackmap` inside the analyzed project. The desktop app is only showing paths in this pass.</p>
    </>
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
