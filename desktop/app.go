package main

import (
	"context"
	"errors"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/will/stackmap/desktop/backend"
)

type App struct {
	ctx context.Context
}

func NewApp() *App {
	return &App{}
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
	return backend.AnalyzeProject(context.Background(), request)
}
