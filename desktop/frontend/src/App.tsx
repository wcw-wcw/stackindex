import { FormEvent, useState } from 'react';
import { AppShell } from './components/AppShell';
import { LandingPage } from './components/LandingPage';
import { ReportWorkspace } from './components/ReportWorkspace';
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

  return (
    <AppShell>
      {result ? (
        <ReportWorkspace result={result} onRunAgain={() => setResult(null)} />
      ) : (
        <LandingPage
          path={path}
          runAudit={runAudit}
          useAI={useAI}
          model={model}
          status={status}
          error={error}
          isRunning={isRunning}
          onPathChange={setPath}
          onRunAuditChange={setRunAudit}
          onUseAIChange={setUseAI}
          onModelChange={setModel}
          onBrowse={pickFolder}
          onAnalyze={analyze}
        />
      )}
    </AppShell>
  );
}

function errorMessage(err: unknown) {
  if (err instanceof Error) {
    return err.message;
  }
  return String(err);
}
