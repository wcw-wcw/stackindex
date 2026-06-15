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

func (a *App) AskQuestion(request backend.AskRequest) (*backend.AskResponse, error) {
	return a.session.AskQuestion(context.Background(), request)
}
