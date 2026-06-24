# StackIndex

StackIndex is a local-first repo orientation file generator for coding agents. It creates a compact, deterministic Markdown search plan so agents know where to start, what to avoid, and when to broaden.

The MVP condenses a repository into an agent-facing Markdown index so coding agents can search with better context and fewer wasted tokens. It scans a repo, detects important project signals, and writes a stable glossary-style file at:

```text
.stackindex/reports/repo-index.md
```

The generated index is meant to be read before broad exploration. It summarizes purpose, stack, feature start points, route implementation chains, task search recipes, key directories, read-first files, tests, findings, freshness, and search-budget hints.

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

## Core Workflow

1. Select a repository in the desktop app or pass one to the CLI.
2. Run analysis.
3. Open `.stackindex/reports/repo-index.md`.
4. Give that compact Markdown file to a coding agent before broad searches.
5. Open `.stackindex/reports/repo-index.full.md` only when the compact index is not enough.

The desktop app should stay focused on this workflow: select/analyze a repo, open the compact or full index, and copy the compact index path or agent instructions. StackIndex is not a repo chat product.

## Generated Artifacts

StackIndex writes generated artifacts under the analyzed repository:

```text
.stackindex/
  analysis.json
  AGENTS.md                 # optional with --agents-md
  history/
    YYYYMMDD-HHMMSS/
      analysis.json
      repo-index.md
      repo-index.full.md
  reports/
    repo-index.md
    repo-index.full.md
```

The main artifact is `.stackindex/reports/repo-index.md`, a compact search plan. `.stackindex/reports/repo-index.full.md` carries verbose routes, env vars, scripts, findings, and file counts. `.stackindex/analysis.json` keeps the structured analysis, file fingerprints, and index freshness metadata.

`stackindex analyze <repo> --agents-md` writes `.stackindex/AGENTS.md`, a concise helper that tells coding agents how to use the generated index. Root `AGENTS.md` export is opt-in with `--agents-md-root`; StackIndex refuses to overwrite an existing root `AGENTS.md` unless `--agents-md-force` is provided.

`.stackindex/` is ignored by this repository and should usually be ignored by projects being analyzed unless generated indexes are intentionally committed.

## Feature Status

| Feature | Status |
| --- | --- |
| Repo walking with ignore rules | Implemented |
| Index quality/trustworthiness summary | Implemented |
| Agent-oriented Markdown index | Implemented |
| Feature map with start files, tests, terms, and avoid-first guidance | Implemented |
| Task-specific search recipes | Implemented |
| Compact route-to-code chains | Implemented |
| Index freshness/staleness reporting | Implemented |
| JSON analysis output | Implemented |
| Key file and directory role detection | Implemented |
| Lightweight dependency graph for source files | Implemented |
| Deterministic agent search planner | Developer validation tool |
| Built-in agent usefulness eval fixtures | Developer validation tool |
| Stack, package script, route, env, test, and deployment signals | Implemented |
| Same-repo snapshot history and change summary | Implemented |
| Deterministic repo Q&A from indexed evidence | CLI-only legacy utility |
| Optional local Ollama notes | Implemented, local only |
| Desktop app | Focused on selecting repos and opening/copying generated artifacts |

## Commands

```sh
stackindex analyze .
stackindex analyze . --no-tui
stackindex analyze . --no-tui --agents-md
stackindex analyze . --json
stackindex audit .
```

When running from source, prefix commands with `go run ./cmd/stackindex`:

```sh
go run ./cmd/stackindex analyze . --no-tui
```

## Agent Workflow

Use `.stackindex/reports/repo-index.md` as the first read before opening source files. It is intentionally compact and should answer: what kind of project is this, what features exist, which files are worth opening first, what routes lead to which implementation files, and what paths should be avoided at the start.

Use `.stackindex/reports/repo-index.full.md` when the compact index omits detail. The full index keeps the complete feature and route-chain set so an agent can broaden with structure instead of falling back to whole-repo search.

The compact index includes an **Agent Usage Instructions** section near the top. It tells agents to use Feature Map for feature work, Route Implementation Chains for API work, Task Search Recipes before whole-repo search, and the full index only when compact output is insufficient.

The compact index also reports **Index stale: yes/no**. If indexed files changed after generation, rerun:

```sh
stackindex analyze <repo> --no-tui
```

## Developer Validation Tools

`stackindex plan <repo> "task"` and `stackindex eval <repo>` are developer validation tools for tuning StackIndex itself. They are useful for checking whether generated feature maps and route chains produce good first-file recommendations, but they are not the main app experience and are intentionally kept out of the desktop UI.

`stackindex plan` reads `.stackindex/analysis.json` and prints deterministic recommended first files, related tests, search terms, directories to inspect, avoid-first paths, and reasons. If the analysis is missing or stale, run `stackindex analyze <repo> --no-tui` first.

`stackindex eval` runs built-in or local fixtures against the latest analysis and reports precision@5, recall@10, top-hit status, and warnings for broad-search or under-search risk. Projects can add `.stackindex/eval-fixtures.json` to replace the built-in stkapp-style fixture set with project-specific tasks.

## What The Markdown Index Contains

- **Repository Snapshot**: repo name, path, indexed file count, and finding counts.
- **Agent Usage Instructions**: short guidance for coding agents using the index.
- **Index Freshness**: stale yes/no, changed file count, and rerun guidance when needed.
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

## Development

```sh
GOCACHE="$(pwd)/.gocache" go test ./...
GOCACHE="$(pwd)/.gocache" go build ./cmd/stackindex
```

The root CLI module has been renamed for StackIndex. The `desktop/` module still contains inherited StackMap desktop code and needs a separate product pass before it should be considered part of the StackIndex MVP.
