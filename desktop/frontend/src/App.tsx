import { FormEvent, useEffect, useState } from 'react';
import { AppShell } from './components/AppShell';
import { LandingPage } from './components/LandingPage';
import { ReportWorkspace } from './components/ReportWorkspace';
import {
  analyzeGitHubRepo,
  analyzeProject,
  browseFolder,
  AnalyzeResponse,
  clearRecentProjects,
  getRecentProjects,
  listOllamaModels,
  OllamaModelsResponse,
  openExistingReport,
  RecentProject,
  removeRecentProject,
} from './wails';

const defaultPath = '/Users/will/Workspace/stkapp';
type SourceMode = 'local' | 'github';

export default function App() {
  const [sourceMode, setSourceMode] = useState<SourceMode>('local');
  const [path, setPath] = useState(defaultPath);
  const [githubUrl, setGithubUrl] = useState('');
  const [runAudit, setRunAudit] = useState(true);
  const [useAI, setUseAI] = useState(false);
  const [model, setModel] = useState('');
  const [status, setStatus] = useState('Ready to analyze a local project.');
  const [error, setError] = useState('');
  const [result, setResult] = useState<AnalyzeResponse | null>(null);
  const [isRunning, setIsRunning] = useState(false);
  const [recentProjects, setRecentProjects] = useState<RecentProject[]>([]);
  const [ollamaModels, setOllamaModels] = useState<OllamaModelsResponse>({
    available: false,
    models: [],
    message: 'AI disabled',
  });
  const [isLoadingModels, setIsLoadingModels] = useState(false);

  useEffect(() => {
    refreshRecentProjects();
  }, []);

  useEffect(() => {
    if (useAI) {
      refreshOllamaModels();
    } else {
      setOllamaModels({ available: false, models: [], message: 'AI disabled' });
    }
  }, [useAI]);

  async function refreshRecentProjects() {
    try {
      setRecentProjects(await getRecentProjects());
    } catch (err) {
      setRecentProjects([]);
      setError(errorMessage(err));
    }
  }

  async function refreshOllamaModels() {
    if (!useAI) {
      setOllamaModels({ available: false, models: [], message: 'AI disabled' });
      return;
    }
    setIsLoadingModels(true);
    try {
      setOllamaModels(await listOllamaModels());
    } catch {
      setOllamaModels({
        available: false,
        models: [],
        message: 'Ollama unavailable - default analysis still works without AI.',
      });
    } finally {
      setIsLoadingModels(false);
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
    if (sourceMode === 'github') {
      await runGitHubAnalysis(githubUrl);
      return;
    }
    await runLocalAnalysis(path);
  }

  async function runLocalAnalysis(projectPath: string) {
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

  async function runGitHubAnalysis(repoUrl: string) {
    setError('');
    setResult(null);
    setIsRunning(true);
    setStatus('Validating GitHub URL...');
    await Promise.resolve();
    setStatus('Cloning repository or using cached clone, then analyzing local files...');
    try {
      const nextResult = await analyzeGitHubRepo({
        url: repoUrl,
        runAudit,
        useAI,
        model,
        refresh: false,
      });
      setPath(nextResult.repoPath);
      setGithubUrl(nextResult.githubUrl || repoUrl);
      setResult(nextResult);
      setStatus('Done. Reports written to the cached clone.');
      await refreshRecentProjects();
    } catch (err) {
      setError(errorMessage(err));
      setStatus('GitHub analysis failed.');
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
    setSourceMode('local');
    await runLocalAnalysis(projectPath);
  }

  async function analyzeRecentAgain(project: RecentProject) {
    if (project.sourceType === 'github' && project.githubUrl) {
      setSourceMode('github');
      setGithubUrl(project.githubUrl);
      await runGitHubAnalysis(project.githubUrl);
      return;
    }
    await analyzeAgain(project.repoPath);
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
    if (result?.sourceType === 'github' && result.githubUrl) {
      setSourceMode('github');
      setGithubUrl(result.githubUrl);
    } else if (result?.repoPath) {
      setSourceMode('local');
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
          sourceMode={sourceMode}
          path={path}
          githubUrl={githubUrl}
          runAudit={runAudit}
          useAI={useAI}
          model={model}
          status={status}
          error={error}
          isRunning={isRunning}
          recentProjects={recentProjects}
          ollamaModels={ollamaModels}
          isLoadingModels={isLoadingModels}
          onSourceModeChange={setSourceMode}
          onPathChange={setPath}
          onGitHubUrlChange={setGithubUrl}
          onRunAuditChange={setRunAudit}
          onUseAIChange={setUseAI}
          onModelChange={setModel}
          onRefreshModels={refreshOllamaModels}
          onBrowse={pickFolder}
          onAnalyze={analyze}
          onOpenReport={openReport}
          onAnalyzeAgain={analyzeRecentAgain}
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
