import { FormEvent, useEffect, useState } from 'react';
import { AppShell } from './components/AppShell';
import { LandingPage } from './components/LandingPage';
import { ReportWorkspace } from './components/ReportWorkspace';
import { SettingsPage } from './components/SettingsPage';
import {
  analyzeGitHubRepo,
  analyzeProject,
  browseFolder,
  AnalyzeResponse,
  clearGitHubCache,
  clearRecentProjects,
  DesktopPaths,
  DesktopSettings,
  getDesktopPaths,
  getDesktopSettings,
  getRecentProjects,
  listOllamaModels,
  OllamaModelsResponse,
  openExistingReport,
  RecentProject,
  removeRecentProject,
  saveDesktopSettings,
} from './wails';

const defaultPath = '/Users/will/Workspace/stkapp';
type SourceMode = 'local' | 'github';
type ViewMode = 'landing' | 'report' | 'settings';

const defaultSettings: DesktopSettings = {
  defaultRunAudit: true,
  defaultUseAI: false,
  defaultModel: '',
};

export default function App() {
  const [viewMode, setViewMode] = useState<ViewMode>('landing');
  const [sourceMode, setSourceMode] = useState<SourceMode>('local');
  const [path, setPath] = useState(defaultPath);
  const [githubUrl, setGithubUrl] = useState('');
  const [githubRefresh, setGithubRefresh] = useState(false);
  const [runAudit, setRunAudit] = useState(true);
  const [useAI, setUseAI] = useState(false);
  const [model, setModel] = useState('');
  const [settings, setSettings] = useState<DesktopSettings>(defaultSettings);
  const [desktopPaths, setDesktopPaths] = useState<DesktopPaths | null>(null);
  const [status, setStatus] = useState('Ready to analyze a local project.');
  const [error, setError] = useState('');
  const [result, setResult] = useState<AnalyzeResponse | null>(null);
  const [isRunning, setIsRunning] = useState(false);
  const [isSavingSettings, setIsSavingSettings] = useState(false);
  const [recentProjects, setRecentProjects] = useState<RecentProject[]>([]);
  const [ollamaModels, setOllamaModels] = useState<OllamaModelsResponse>({
    available: false,
    models: [],
    message: 'AI disabled',
  });
  const [isLoadingModels, setIsLoadingModels] = useState(false);

  useEffect(() => {
    loadDesktopState();
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

  async function loadDesktopState() {
    try {
      const [nextSettings, nextPaths] = await Promise.all([getDesktopSettings(), getDesktopPaths()]);
      setSettings(nextSettings);
      setRunAudit(nextSettings.defaultRunAudit);
      setUseAI(nextSettings.defaultUseAI);
      setModel(nextSettings.defaultModel);
      setDesktopPaths(nextPaths);
    } catch (err) {
      setError(errorMessage(err));
    }
  }

  async function refreshOllamaModels(shouldUseAI = useAI) {
    if (!shouldUseAI) {
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
      await runGitHubAnalysis(githubUrl, githubRefresh);
      return;
    }
    await runLocalAnalysis(path);
  }

  async function runLocalAnalysis(projectPath: string) {
    setError('');
    setResult(null);
    setIsRunning(true);
    setStatus('Analyzing project and writing .stackindex reports...');
    try {
      const nextResult = await analyzeProject({
        path: projectPath,
        runAudit,
        useAI,
        model,
      });
      setPath(nextResult.repoPath || projectPath);
      setResult(nextResult);
      setViewMode('report');
      setStatus('Reports written to .stackindex.');
      await refreshRecentProjects();
    } catch (err) {
      setError(errorMessage(err));
      setStatus('Analysis failed.');
    } finally {
      setIsRunning(false);
    }
  }

  async function runGitHubAnalysis(repoUrl: string, refresh = false) {
    setError('');
    setResult(null);
    setIsRunning(true);
    setStatus('Validating GitHub URL...');
    const cloneStatus = window.setTimeout(() => {
      setStatus(refresh ? 'Refreshing cached clone...' : 'Using cached clone...');
    }, 250);
    const missingCacheStatus = window.setTimeout(() => {
      if (!refresh) {
        setStatus('Cloning repo if cache is missing...');
      }
    }, 750);
    const analyzeStatus = window.setTimeout(() => {
      setStatus('Analyzing local cached files...');
    }, 1500);
    try {
      const nextResult = await analyzeGitHubRepo({
        url: repoUrl,
        runAudit,
        useAI,
        model,
        refresh,
      });
      setPath(nextResult.repoPath);
      setGithubUrl(nextResult.githubUrl || repoUrl);
      setResult(nextResult);
      setViewMode('report');
      setStatus(refresh ? 'Refreshed cached clone and wrote reports.' : 'Reports written to the cached clone.');
      await refreshRecentProjects();
    } catch (err) {
      setError(errorMessage(err));
      setStatus('GitHub analysis error.');
    } finally {
      window.clearTimeout(cloneStatus);
      window.clearTimeout(missingCacheStatus);
      window.clearTimeout(analyzeStatus);
      setIsRunning(false);
    }
  }

  async function openReport(projectPath: string) {
    setError('');
    setIsRunning(true);
    setStatus('Opening previous StackIndex report...');
    try {
      const nextResult = await openExistingReport(projectPath);
      setPath(nextResult.repoPath || projectPath);
      setResult(nextResult);
      setViewMode('report');
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
      setGithubRefresh(false);
      await runGitHubAnalysis(project.githubUrl, false);
      return;
    }
    await analyzeAgain(project.repoPath);
  }

  async function refreshRecentGitHub(project: RecentProject) {
    if (!project.githubUrl) {
      return;
    }
    setSourceMode('github');
    setGithubUrl(project.githubUrl);
    setGithubRefresh(true);
    await runGitHubAnalysis(project.githubUrl, true);
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

  async function clearGitHubCacheAction() {
    setError('');
    try {
      await clearGitHubCache();
      await loadDesktopState();
      setStatus('Cleared StackIndex GitHub cache.');
    } catch (err) {
      setError(errorMessage(err));
      setStatus('Could not clear GitHub cache.');
    }
  }

  async function saveSettings(event: FormEvent) {
    event.preventDefault();
    setError('');
    setIsSavingSettings(true);
    try {
      const saved = await saveDesktopSettings(settings);
      setSettings(saved);
      setRunAudit(saved.defaultRunAudit);
      setUseAI(saved.defaultUseAI);
      setModel(saved.defaultModel);
      setStatus('Saved desktop defaults.');
    } catch (err) {
      setError(errorMessage(err));
      setStatus('Could not save desktop defaults.');
    } finally {
      setIsSavingSettings(false);
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
    setViewMode('landing');
  }

  function openSettings() {
    setError('');
    setViewMode('settings');
    loadDesktopState();
  }

  function backFromSettings() {
    setError('');
    setViewMode(result ? 'report' : 'landing');
  }

  return (
    <AppShell>
      {viewMode === 'settings' ? (
        <SettingsPage
          settings={settings}
          paths={desktopPaths}
          status={status}
          error={error}
          isSaving={isSavingSettings}
          isRunning={isRunning}
          ollamaModels={ollamaModels}
          isLoadingModels={isLoadingModels}
          onSettingsChange={setSettings}
          onSaveSettings={saveSettings}
          onClearRecent={clearRecent}
          onClearGitHubCache={clearGitHubCacheAction}
          onRefreshModels={() => refreshOllamaModels(settings.defaultUseAI)}
          onBack={backFromSettings}
        />
      ) : viewMode === 'report' && result ? (
        <ReportWorkspace result={result} onRunAgain={runAgain} onOpenSettings={openSettings} />
      ) : (
        <LandingPage
          sourceMode={sourceMode}
          path={path}
          githubUrl={githubUrl}
          githubRefresh={githubRefresh}
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
          onGitHubRefreshChange={setGithubRefresh}
          onRunAuditChange={setRunAudit}
          onUseAIChange={setUseAI}
          onModelChange={setModel}
          onRefreshModels={refreshOllamaModels}
          onBrowse={pickFolder}
          onAnalyze={analyze}
          onOpenReport={openReport}
          onAnalyzeAgain={analyzeRecentAgain}
          onRefreshGitHub={refreshRecentGitHub}
          onRemoveRecent={removeRecent}
          onClearRecent={clearRecent}
          onOpenSettings={openSettings}
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
