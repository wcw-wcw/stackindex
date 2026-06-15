# StackMap Desktop MVP

This folder is an isolated Wails v2 scaffold for the first desktop vertical slice.

It is a separate Go module so root CLI/TUI development and `go test ./...` are not affected by Wails dependencies. The desktop backend imports the parent module through:

```go
replace github.com/will/stackmap => ..
```

## Setup

The Wails CLI is expected on `PATH`. In this workspace it was available at `/Users/will/go/bin/wails`; add `/Users/will/go/bin` to `PATH` or call that binary directly. If Wails is missing, install it with:

```sh
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

Then from this directory:

```sh
npm install --prefix frontend
wails dev
```

For a production build:

```sh
wails build
```

To rerun the real-project desktop backend validation:

```sh
STACKMAP_DESKTOP_ANALYZE_PATH=/Users/will/Workspace/stkapp go test ./backend -run TestAnalyzeProjectIntegration -count=1
```

## Current Scope

Implemented:

- enter or browse for a local project path
- optional deterministic audit
- optional local Ollama-backed AI summary using StackMap's existing fallback behavior
- analyze through `internal/app.Analyze`
- export reports through `internal/app.ExportReports`
- show counts, stack chips, audit/AI status, and generated report paths

Intentionally not implemented yet:

- full report viewer
- chat Q&A UI
- GitHub cloning
- recent projects
- model picker/settings
- packaging
- cloud APIs or embeddings
