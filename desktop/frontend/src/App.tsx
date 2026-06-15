import { FormEvent, useEffect, useState } from 'react';
import { AppShell } from './components/AppShell';
import { LandingPage } from './components/LandingPage';
import { ReportWorkspace } from './components/ReportWorkspace';
import {
  analyzeProject,
  browseFolder,
  AnalyzeResponse,
  clearRecentProjects,
  getRecentProjects,
  openExistingReport,
  RecentProject,
  removeRecentProject,
} from './wails';

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
  const [recentProjects, setRecentProjects] = useState<RecentProject[]>([]);

  useEffect(() => {
    refreshRecentProjects();
  }, []);

  async function refreshRecentProjects() {
    try {
      setRecentProjects(await getRecentProjects());
    } catch (err) {
      setRecentProjects([]);
      setError(errorMessage(err));
    }
  }

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
    await runAnalysis(path);
  }

  async function runAnalysis(projectPath: string) {
    setError('');
    setResult(null);
    setIsRunning(true);
    setStatus('Analyzing project and writing .stackmap reports...');
    try {
      const nextResult = await analyzeProject({
        path: projectPath,
        runAudit,
        useAI,
        model,
      });
      setPath(nextResult.repoPath || projectPath);
      setResult(nextResult);
      setStatus('Analysis complete.');
      await refreshRecentProjects();
    } catch (err) {
      setError(errorMessage(err));
      setStatus('Analysis failed.');
    } finally {
      setIsRunning(false);
    }
  }

  async function openReport(projectPath: string) {
    setError('');
    setIsRunning(true);
    setStatus('Opening previous StackMap report...');
    try {
      const nextResult = await openExistingReport(projectPath);
      setPath(nextResult.repoPath || projectPath);
      setResult(nextResult);
      setStatus('Loaded previous report.');
      await refreshRecentProjects();
    } catch (err) {
      setError(errorMessage(err));
      setStatus('Could not open previous report.');
    } finally {
      setIsRunning(false);
    }
  }

  async function analyzeAgain(projectPath: string) {
    setPath(projectPath);
    await runAnalysis(projectPath);
  }

  async function removeRecent(projectPath: string) {
    setError('');
    try {
      await removeRecentProject(projectPath);
      await refreshRecentProjects();
      setStatus('Removed recent project.');
    } catch (err) {
      setError(errorMessage(err));
    }
  }

  async function clearRecent() {
    setError('');
    try {
      await clearRecentProjects();
      await refreshRecentProjects();
      setStatus('Cleared recent projects.');
    } catch (err) {
      setError(errorMessage(err));
    }
  }

  function runAgain() {
    if (result?.repoPath) {
      setPath(result.repoPath);
    }
    setResult(null);
  }

  return (
    <AppShell>
      {result ? (
        <ReportWorkspace result={result} onRunAgain={runAgain} />
      ) : (
        <LandingPage
          path={path}
          runAudit={runAudit}
          useAI={useAI}
          model={model}
          status={status}
          error={error}
          isRunning={isRunning}
          recentProjects={recentProjects}
          onPathChange={setPath}
          onRunAuditChange={setRunAudit}
          onUseAIChange={setUseAI}
          onModelChange={setModel}
          onBrowse={pickFolder}
          onAnalyze={analyze}
          onOpenReport={openReport}
          onAnalyzeAgain={analyzeAgain}
          onRemoveRecent={removeRecent}
          onClearRecent={clearRecent}
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
