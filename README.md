# StackIndex

StackIndex is a local-first repo orientation file generator for coding agents. It creates a compact, deterministic search plan so agents know where to start, what to avoid, and when to broaden.

The MVP condenses a repository into an agent-facing Markdown index so coding agents can search with better context and fewer wasted tokens. It scans a repo, detects important project signals, and writes a stable glossary-style file at:

```text
.stackindex/reports/repo-index.md
```

The generated index is meant to be read before broad exploration. It summarizes purpose, stack, key directories, read-first files, dependency hubs, API routes, env var names, package scripts, tests, findings, and search-budget hints.

## Quickstart

```sh
go run ./cmd/stackindex analyze . --no-tui
open .stackindex/reports/repo-index.md
```

After building:

```sh
go build -o stackindex ./cmd/stackindex
./stackindex analyze /path/to/repo --no-tui
```

## MVP Scope

| Feature | Status |
| --- | --- |
| Repo walking with ignore rules | Implemented |
| Index quality/trustworthiness summary | Implemented |
| Agent-oriented Markdown index | Implemented |
| Feature map with start files, tests, terms, and avoid-first guidance | Implemented |
| Task-specific search recipes | Implemented |
| Compact route-to-code chains | Implemented |
| JSON analysis output | Implemented |
| Key file and directory role detection | Implemented |
| Lightweight dependency graph for source files | Implemented |
| Stack, package script, route, env, test, and deployment signals | Implemented |
| Same-repo snapshot history and change summary | Implemented |
| Deterministic repo Q&A from indexed evidence | Implemented |
| Optional local Ollama notes | Implemented, local only |
| Desktop app | Inherited from StackMap and not yet reworked for StackIndex |

## Commands

```sh
stackindex analyze .
stackindex analyze . --no-tui
stackindex analyze . --json
stackindex ask . "What files should I read first?"
stackindex ask . "Where are the API routes?"
stackindex audit .
```

When running from source, prefix commands with `go run ./cmd/stackindex`:

```sh
go run ./cmd/stackindex analyze . --no-tui
```

## Output Contract

StackIndex writes generated artifacts under the analyzed repository:

```text
.stackindex/
  analysis.json
  history/
    YYYYMMDD-HHMMSS/
      analysis.json
      repo-index.md
      repo-index.full.md
  qa/
    latest-question.json
    history.jsonl
  reports/
    repo-index.md
    repo-index.full.md
```

The main artifact is `.stackindex/reports/repo-index.md`, a compact search plan. `.stackindex/reports/repo-index.full.md` carries verbose routes, env vars, scripts, findings, and file counts. `.stackindex/analysis.json` keeps the same information in a structured form for future automation.

`.stackindex/` is ignored by this repository and should usually be ignored by projects being analyzed unless generated indexes are intentionally committed.

## What The Markdown Index Contains

- **Repository Snapshot**: repo name, path, indexed file count, and finding counts.
- **Project Context**: likely purpose inferred from README, package metadata, scripts, env names, and docs.
- **Agent Search Guide**: read-first files, directory roles, and dependency hubs.
- **Search Budget Hints**: practical guidance for narrowing future searches.
- **Detected Stack**: languages, frameworks, libraries, databases, testing, and deployment signals.
- **Project Structure**: important folders and their likely roles.
- **Key Files**: important manifests, entrypoints, configs, docs, routes, scripts, and tests.
- **File Connections**: highly connected files that are likely worth inspecting before leaves.
- **Operational Signals**: scripts, env var names, routes, tests, deployment readiness, and findings.
- **Important Exports**: lightweight TypeScript/JavaScript, Go, and Python symbol summaries for high-priority files.

## Local-First Behavior

StackIndex is designed to run beside source code:

- It does not upload repositories.
- It does not call cloud AI APIs.
- It does not run arbitrary project commands.
- Optional AI notes use a local Ollama server only.
- Q&A answers from deterministic analysis first; local AI can only polish bounded evidence.

## Development

```sh
GOCACHE="$(pwd)/.gocache" go test ./...
GOCACHE="$(pwd)/.gocache" go build ./cmd/stackindex
```

The root CLI module has been renamed for StackIndex. The `desktop/` module still contains inherited StackMap desktop code and needs a separate product pass before it should be considered part of the StackIndex MVP.
