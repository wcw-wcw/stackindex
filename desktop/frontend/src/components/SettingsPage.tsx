import { FormEvent } from 'react';
import { DesktopPaths, DesktopSettings, OllamaModelsResponse } from '../wails';

type SettingsPageProps = {
  settings: DesktopSettings;
  paths: DesktopPaths | null;
  status: string;
  error: string;
  isSaving: boolean;
  isRunning: boolean;
  ollamaModels: OllamaModelsResponse;
  isLoadingModels: boolean;
  onSettingsChange: (settings: DesktopSettings) => void;
  onSaveSettings: (event: FormEvent) => void;
  onClearRecent: () => void;
  onClearGitHubCache: () => void;
  onRefreshModels: () => void;
  onBack: () => void;
};

export function SettingsPage({
  settings,
  paths,
  status,
  error,
  isSaving,
  isRunning,
  ollamaModels,
  isLoadingModels,
  onSettingsChange,
  onSaveSettings,
  onClearRecent,
  onClearGitHubCache,
  onRefreshModels,
  onBack,
}: SettingsPageProps) {
  return (
    <section className="panel settings-panel">
      <div className="settings-header">
        <div>
          <div className="eyebrow">Desktop settings</div>
          <h1>Settings</h1>
          <p className="intro">Local paths, cache controls, and desktop-only default run options.</p>
        </div>
        <button type="button" className="secondary" onClick={onBack}>Back</button>
      </div>

      <div className="settings-grid">
        <section className="settings-section">
          <h2>Paths</h2>
          <PathLine label="Recent projects" value={paths?.recentProjectsPath || 'Loading...'} />
          <PathLine label="GitHub cache" value={paths?.githubCacheRoot || 'Loading...'} />
          <PathLine label="Settings file" value={paths?.settingsPath || 'Loading...'} />
        </section>

        <section className="settings-section">
          <h2>Local-first</h2>
          <p className="body-copy">
            StackMap analyzes local files. Public GitHub repositories are cloned into the local cache before analysis. Optional AI uses local Ollama only.
          </p>
        </section>

        <form className="settings-section" onSubmit={onSaveSettings}>
          <h2>Defaults</h2>
          <label className="toggle">
            <input
              type="checkbox"
              checked={settings.defaultRunAudit}
              onChange={(event) => onSettingsChange({ ...settings, defaultRunAudit: event.target.checked })}
            />
            <span>Run audit by default</span>
          </label>
          <label className="toggle">
            <input
              type="checkbox"
              checked={settings.defaultUseAI}
              onChange={(event) => onSettingsChange({ ...settings, defaultUseAI: event.target.checked })}
            />
            <span>Use local AI by default</span>
          </label>
          <div className={`model-picker ${settings.defaultUseAI ? '' : 'is-disabled'}`}>
            <div className="model-picker-header">
              <label htmlFor="default-model">Default model</label>
              <button type="button" className="secondary compact" onClick={onRefreshModels} disabled={!settings.defaultUseAI || isRunning || isLoadingModels}>
                {isLoadingModels ? 'Refreshing...' : 'Refresh models'}
              </button>
            </div>
            <select
              id="default-model"
              value={settings.defaultModel}
              onChange={(event) => onSettingsChange({ ...settings, defaultModel: event.target.value })}
              disabled={!settings.defaultUseAI || isRunning}
            >
              <option value="">Default local chain</option>
              {selectedModelMissing(settings.defaultModel, ollamaModels) && <option value={settings.defaultModel}>{settings.defaultModel}</option>}
              {ollamaModels.models.map((item) => (
                <option key={item.name} value={item.name}>
                  {item.name}
                </option>
              ))}
            </select>
            <p className="model-status">{modelStatusText(settings.defaultUseAI, ollamaModels, isLoadingModels)}</p>
          </div>
          <button type="submit" disabled={isSaving || isRunning}>{isSaving ? 'Saving...' : 'Save defaults'}</button>
        </form>

        <section className="settings-section">
          <h2>Cache</h2>
          <div className="settings-actions">
            <button type="button" className="secondary" onClick={onClearRecent} disabled={isRunning}>
              Clear recent projects
            </button>
            <button type="button" className="secondary" onClick={onClearGitHubCache} disabled={isRunning}>
              Clear GitHub cache
            </button>
          </div>
          <p className="selected">Refresh cached GitHub repositories is not implemented.</p>
        </section>
      </div>

      <p className="status">{status}</p>
      {error && <p className="error">{error}</p>}
    </section>
  );
}

function PathLine({ label, value }: { label: string; value: string }) {
  return (
    <div className="path-line">
      <span>{label}</span>
      <code>{value}</code>
    </div>
  );
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
