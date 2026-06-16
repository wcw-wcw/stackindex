package main

import (
	"context"
	"errors"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/will/stackmap/desktop/backend"
)

type App struct {
	ctx     context.Context
	session *backend.Session
}

func NewApp() *App {
	return &App{session: backend.NewSession()}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) BrowseFolder() (string, error) {
	if a.ctx == nil {
		return "", errors.New("desktop runtime is not ready")
	}
	return wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Select a project folder",
	})
}

func (a *App) AnalyzeProject(request backend.AnalyzeRequest) (*backend.AnalyzeResponse, error) {
	return a.session.AnalyzeProject(context.Background(), request)
}

func (a *App) AnalyzeGitHubRepo(request backend.GitHubAnalyzeRequest) (*backend.AnalyzeResponse, error) {
	return a.session.AnalyzeGitHubRepo(context.Background(), request)
}

func (a *App) OpenExistingReport(path string) (*backend.AnalyzeResponse, error) {
	return a.session.OpenExistingReport(path)
}

func (a *App) GetRecentProjects() ([]backend.RecentProject, error) {
	return a.session.GetRecentProjects()
}

func (a *App) RemoveRecentProject(path string) error {
	return a.session.RemoveRecentProject(path)
}

func (a *App) ClearRecentProjects() error {
	return a.session.ClearRecentProjects()
}

func (a *App) GetDesktopSettings() (*backend.DesktopSettings, error) {
	return a.session.GetDesktopSettings()
}

func (a *App) SaveDesktopSettings(settings backend.DesktopSettings) (*backend.DesktopSettings, error) {
	return a.session.SaveDesktopSettings(settings)
}

func (a *App) GetDesktopPaths() (*backend.DesktopPaths, error) {
	return a.session.GetDesktopPaths()
}

func (a *App) ClearGitHubCache() error {
	return a.session.ClearGitHubCache()
}

func (a *App) RevealProjectFolder(request backend.PathActionRequest) error {
	return a.session.RevealProjectFolder(context.Background(), request)
}

func (a *App) RevealStackMapFolder(request backend.PathActionRequest) error {
	return a.session.RevealStackMapFolder(context.Background(), request)
}

func (a *App) RevealSnapshotFolder(request backend.PathActionRequest) error {
	return a.session.RevealSnapshotFolder(context.Background(), request)
}

func (a *App) OpenMarkdownReport(request backend.PathActionRequest) error {
	return a.session.OpenMarkdownReport(context.Background(), request)
}

func (a *App) OpenJSONReport(request backend.PathActionRequest) error {
	return a.session.OpenJSONReport(context.Background(), request)
}

func (a *App) GenerateCLICommand(request backend.CLICommandRequest) (string, error) {
	return a.session.GenerateCLICommand(request)
}

func (a *App) AskQuestion(request backend.AskRequest) (*backend.AskResponse, error) {
	return a.session.AskQuestion(context.Background(), request)
}

func (a *App) ListOllamaModels() (*backend.OllamaModelsResponse, error) {
	return a.session.ListOllamaModels(context.Background())
}
