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

### Build troubleshooting

In Codex's managed sandbox, Wails may fail while generating bindings or compiling the application because Go defaults to `/Users/will/Library/Caches/go-build`, which is outside the writable workspace. Use a workspace-local cache for sandboxed test/build commands:

```sh
GOCACHE=/Users/will/Workspace/stackmap/.gocache go test ./...
```

If Wails reports only `ERROR exit status 1` at `Compiling application` inside the sandbox, compare it with the command Wails prints under `-v 2`:

```sh
GOCACHE=/Users/will/Workspace/stackmap/.gocache /Users/will/go/bin/wails build -v 2
GOCACHE=/Users/will/Workspace/stackmap/.gocache go build -buildvcs=false -tags desktop,wv2runtime.download,production -ldflags "-w -s" -o build/bin/StackMap
```

As of the desktop MVP stabilization pass, the normal unsandboxed build succeeds:

```sh
/Users/will/go/bin/wails build
```

The only compiler output from the direct `go build` path was a Wails macOS shim deprecation warning for `setShowsBaselineSeparator:` on recent macOS SDKs.

To rerun the real-project desktop backend validation:

```sh
STACKMAP_DESKTOP_ANALYZE_PATH=/Users/will/Workspace/stkapp go test ./backend -run TestAnalyzeProjectIntegration -count=1
```

## Design Direction

See [DESIGN.md](./DESIGN.md). The desktop UI should stay close to the Bubble Tea TUI: flat charcoal backgrounds, terminal-style panes, monospace typography, compact report density, cyan StackMap accents, muted metadata, purple selected sidebar rows with a `>` marker, and direct severity colors. Future polish should improve clarity without drifting into a generic SaaS dashboard, portfolio site, or stock-app visual language.

## Current Scope

Implemented:

- enter or browse for a local project path
- paste a public GitHub repository URL and analyze a local cached clone
- optional deterministic audit
- optional local Ollama-backed AI summary using StackMap's existing fallback behavior
- analyze through `internal/app.Analyze`
- export reports through `internal/app.ExportReports`
- show a clickable report workspace with overview, audit, context, routes, tests, AI notes, and report paths
- recent projects and previous report loading
- local Ollama model discovery/dropdown

Intentionally not implemented yet:

- full report viewer
- chat Q&A UI
- private GitHub repository auth
- GitHub tokens or GitHub API usage
- branch selection
- GitHub cache refresh/pull from the UI
- packaging
- cloud APIs or embeddings

## GitHub URL Support

The GitHub source mode is a local-first MVP for public repositories only. StackMap accepts these URL forms:

```text
https://github.com/owner/repo
https://github.com/owner/repo.git
```

The desktop backend normalizes either form to `https://github.com/owner/repo.git`, clones with the local `git` binary, and then analyzes the local clone through the same `internal/app.Analyze` flow used for local folders. Reports are written inside the cached clone at:

```text
<cached repo>/.stackmap/analysis.json
<cached repo>/.stackmap/reports/repo-report.md
```

Repositories are cached under the OS user cache directory:

```text
os.UserCacheDir()/StackMap/repos/github.com/<owner>/<repo>
```

On macOS this is typically:

```text
~/Library/Caches/StackMap/repos/github.com/<owner>/<repo>
```

To force a fresh clone, quit the app if it is running and remove the cached repository folder manually. Refresh/pull is intentionally not exposed in this MVP.

Current GitHub limitations:

- public HTTPS GitHub repositories only
- no SSH URLs
- no credentials embedded in URLs
- no private repo auth
- no GitHub tokens
- no GitHub API calls
- no branch selector
- no report comparison
