# StackIndex Desktop

This folder is the Wails v2 desktop app for StackIndex.

It is a separate Go module so root CLI/TUI development and `go test ./...` are not affected by Wails dependencies. The desktop backend imports the parent module through:

```go
replace github.com/wcw-wcw/stackindex => ..
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
GOCACHE=/Users/will/Workspace/stackindex/.gocache go test ./...
```

If Wails reports only `ERROR exit status 1` at `Compiling application` inside the sandbox, compare it with the command Wails prints under `-v 2`:

```sh
GOCACHE=/Users/will/Workspace/stackindex/.gocache /Users/will/go/bin/wails build -v 2
GOCACHE=/Users/will/Workspace/stackindex/.gocache go build -buildvcs=false -tags desktop,wv2runtime.download,production -ldflags "-w -s" -o build/bin/StackIndex
```

Codex's managed sandbox may still fail native Wails builds because of system cache or macOS build restrictions. Manual validation command:

```sh
cd desktop && /Users/will/go/bin/wails build
```

The only compiler output from the direct `go build` path was a Wails macOS shim deprecation warning for `setShowsBaselineSeparator:` on recent macOS SDKs.

To rerun the real-project desktop backend validation:

```sh
STACKMAP_DESKTOP_ANALYZE_PATH=/Users/will/Workspace/stkapp go test ./backend -run TestAnalyzeProjectIntegration -count=1
```

## Design Direction

See [DESIGN.md](./DESIGN.md). The desktop UI should stay close to the Bubble Tea TUI: flat charcoal backgrounds, terminal-style panes, monospace typography, compact report density, cyan StackIndex accents, muted metadata, purple selected sidebar rows with a `>` marker, and direct severity colors. Future polish should improve clarity without drifting into a generic SaaS dashboard, portfolio site, or stock-app visual language.

## Current Scope

Implemented:

- enter or browse for a local project path
- paste a public GitHub repository URL and analyze a local cached clone
- optional deterministic audit
- optional local Ollama-backed AI summary using StackIndex's existing fallback behavior
- analyze through `internal/app.Analyze`
- export reports through `internal/app.ExportReports`
- show a clickable report workspace with overview, audit, context, routes, tests, Ask, AI notes, reports, snapshot history, and change summaries
- reports tab actions for copying paths, opening JSON/Markdown, and revealing folders
- recent projects and previous report loading through `.stackindex/analysis.json`
- local snapshot history under `.stackindex/history/<timestamp>/`
- same-repo change summaries against the most recent previous snapshot
- settings for local defaults and cache paths
- GitHub cache and recent-project clearing
- local Ollama model discovery/dropdown

Intentionally not implemented yet:

- full report viewer
- private GitHub repository auth
- GitHub tokens or GitHub API usage
- branch selection
- cross-repo, branch, or private GitHub comparison
- packaging
- cloud APIs or embeddings

## GitHub URL Support

The GitHub source mode is a local-first MVP for public repositories only. StackIndex accepts these URL forms:

```text
https://github.com/owner/repo
https://github.com/owner/repo.git
```

The desktop backend normalizes either form to `https://github.com/owner/repo.git`, clones with the local `git` binary, and then analyzes the local clone through the same `internal/app.Analyze` flow used for local folders. Reports are written inside the cached clone at:

```text
<cached repo>/.stackindex/analysis.json
<cached repo>/.stackindex/reports/repo-index.md
<cached repo>/.stackindex/history/<timestamp>/analysis.json
<cached repo>/.stackindex/history/<timestamp>/repo-index.md
<cached repo>/.stackindex/qa/latest-question.json
<cached repo>/.stackindex/qa/history.jsonl
```

Repositories are cached under the OS user cache directory:

```text
os.UserCacheDir()/StackIndex/repos/github.com/<owner>/<repo>
```

On macOS this is typically:

```text
~/Library/Caches/StackIndex/repos/github.com/<owner>/<repo>
```

Use "Refresh cached clone before analysis" to update an existing cached public GitHub clone before analysis. If the cache is missing, StackIndex clones as usual.

Current GitHub limitations:

- public HTTPS GitHub repositories only
- no SSH URLs
- no credentials embedded in URLs
- no private repo auth
- no GitHub tokens
- no GitHub API calls
- no branch selector
- no cross-repo or branch comparison

## Reports, Ask, and History

The desktop app uses the same local report files as the CLI:

```text
.stackindex/analysis.json
.stackindex/reports/repo-index.md
.stackindex/history/<timestamp>/analysis.json
.stackindex/history/<timestamp>/repo-index.md
.stackindex/qa/latest-question.json
.stackindex/qa/history.jsonl
```

Open Existing Report reads `.stackindex/analysis.json` without rerunning analysis or creating a new snapshot. New local or public-GitHub analyses write the latest report files and create a timestamped snapshot. The Reports tab lists recent snapshots and shows a deterministic same-repo change summary when at least one previous snapshot exists.

Ask uses deterministic StackIndex evidence by default. Supported examples include:

```text
Where is auth handled?
Where is the database initialized?
How do I run this locally?
What files should I read first?
What changed since last analysis?
```
