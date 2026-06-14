# StackMap

StackMap is a local-first Go CLI/TUI for understanding a codebase before you deploy, hand it off, or wire it into CI. It scans a repository, detects common stack and structure signals, checks deployment readiness, and writes Markdown/JSON reports that are useful for humans and automation.

It is for developers reviewing their own apps, maintainers onboarding to unfamiliar repos, and teams that want a lightweight deterministic audit gate without sending source code to a hosted service.

Local-first matters because StackMap is meant to run directly beside your code:

- It does not upload repositories.
- It does not call cloud AI APIs.
- It does not run arbitrary project commands.
- Optional AI summaries use your local Ollama server only.
- Repo Q&A answers from deterministic analysis first; optional AI can only polish bounded evidence.
- Audit pass/fail is always based on deterministic static checks, not model output.

## Quickstart

```sh
go run ./cmd/stackmap --help
go run ./cmd/stackmap analyze .
go run ./cmd/stackmap analyze . --no-tui
go run ./cmd/stackmap audit .
go run ./cmd/stackmap ask . "What is this project for?"
```

After building:

```sh
go build -o stackmap ./cmd/stackmap
./stackmap analyze .
```

## Features

| Feature | What StackMap does |
| --- | --- |
| Stack detection | Detects common languages, frameworks, databases, test tools, and deployment targets. |
| API route detection | Extracts basic Express and Next.js route patterns when visible statically. |
| Env var review | Compares app env var usage with `.env.example` and warns about missing or unsafe placeholders. |
| Deployment readiness checks | Reviews README/setup hints, build/test scripts, health endpoints, migrations, Docker/Vercel signals, and related findings. |
| Markdown/JSON exports | Writes `.stackmap/reports/repo-report.md` and `.stackmap/analysis.json`. |
| TUI overview | Shows a Bubble Tea/Lip Gloss terminal overview for interactive review. |
| Deterministic audit gate | Provides CI-friendly pass/fail behavior using static findings and readiness rules. |
| Evidence-based repo Q&A | Answers local questions from StackMap's analysis data, with evidence and optional JSON output. |
| Optional local AI notes | Adds report-only Ollama summaries with deterministic fallback text. |

## Example Commands

```sh
stackmap analyze .
stackmap analyze . --no-tui
stackmap analyze . --json
stackmap analyze . --ai
stackmap analyze . --ai --model llama3.2:3b
stackmap analyze . --ai-debug
stackmap audit .
stackmap audit . --allow-missing-tests
stackmap audit . --fail-on-low
stackmap ask . "What is this project for?"
stackmap ask . "Where are the API routes?"
stackmap ask . "What should I review before deployment?"
stackmap ask . "Where are the API routes?" --json
stackmap ask . "What is this project for?" --ai
```

When running from source, prefix the same commands with `go run ./cmd/stackmap`, for example:

```sh
go run ./cmd/stackmap audit .
```

## Analyze Mode

`stackmap analyze [path]` scans a repository and opens the terminal UI by default. Use `--no-tui` for a plain export-only run, or `--json` to print JSON to stdout.

Analyze mode checks:

- Repository files and basic metadata
- JavaScript and TypeScript package scripts and dependencies
- Stack signals such as React, Vite, Next.js, Express, TypeScript, Tailwind, PostgreSQL, Prisma, Drizzle, SQLite, Vitest, Jest, Playwright, and Vercel
- Environment variable usage compared with `.env.example`
- Basic Express and Next.js route patterns
- Test infrastructure and test scripts
- Deployment readiness signals such as README, Dockerfile, Vercel config, health endpoints, migrations, and setup/deploy notes

StackMap intentionally keeps findings conservative to avoid noisy false positives.

## Audit Mode

`stackmap audit [path]` runs the same static analysis and writes the same reports, then evaluates a deterministic deployment-readiness gate. It is designed for CI and exits non-zero when blockers are found.

By default, audit fails for:

- High findings
- Medium findings
- Missing stack detection
- Missing tests
- Env var usage without `.env.example`
- Backend/API deployment surfaces without a health endpoint, including Next.js API routes, Vercel-style `api/` functions, and detected server entrypoints

Low and info findings do not fail audit by default. Static/frontend-style deployments without detected backend/API surface receive a health-endpoint warning instead of a failure.

Audit flags:

- `--allow-medium`: treat medium findings as warnings.
- `--allow-missing-tests`: treat missing tests as a warning.
- `--fail-on-low`: make low findings block the audit.
- `--json`: print JSON and still use the audit exit code.
- `--ai`: include optional local AI report notes without affecting pass/fail.

## Ask Mode

`stackmap ask [path] "question"` answers repository questions from StackMap's deterministic analysis data. It does not chat over raw source files, use embeddings, or call cloud AI. Each answer includes a confidence level and evidence such as detected routes, files, stack terms, audit signals, package scripts, or graph facts.

Examples:

```sh
stackmap ask . "What is this project for?"
stackmap ask . "Where are the API routes?"
stackmap ask . "What should I review before deployment?"
stackmap ask . "Does this project have tests?"
stackmap ask . "How is the frontend connected to the backend?"
```

Ask mode supports questions about:

- Project purpose and README/package context
- Detected stack, frameworks, databases, testing, and deployment targets
- API routes and backend surface
- Important folders/files and where to start
- Lightweight dependency graph connections and highly connected files
- Deployment readiness, risks, health checks, and env examples
- Test files, test scripts, and detected test frameworks
- Environment variable usage and `.env.example` coverage

Use `--json` to print the Q&A result as JSON:

```sh
stackmap ask . "Where are the API routes?" --json
```

The JSON shape is:

```json
{
  "question": "Where are the API routes?",
  "answer": "This project exposes detected API routes...",
  "confidence": "high",
  "evidence": [
    {
      "kind": "route",
      "label": "GET /api/health",
      "value": "high",
      "path": "src/app/api/health/route.ts"
    }
  ],
  "mode": "deterministic",
  "model": "",
  "attemptedModels": [],
  "warnings": []
}
```

With `--ai`, StackMap sends only the deterministic Q&A answer and evidence factsheet to local Ollama for polishing:

```sh
ollama serve
stackmap ask . "What is this project for?" --ai
```

If Ollama is unavailable or returns unsupported text, ask mode falls back to the deterministic answer. `--model` selects a local model, and `--ai-debug` writes bounded diagnostics under `.stackmap/ai-debug/ask/`.

## Report Outputs

Reports are written under the analyzed repository:

```text
.stackmap/
  analysis.json
  reports/
    repo-report.md
```

Add `.stackmap/` to your project `.gitignore` unless you deliberately want to commit generated reports.

On macOS, Finder hides folders that start with a dot. If you do not see `.stackmap`, press `Cmd+Shift+.` in Finder or open it directly:

```sh
open /path/to/your/repo/.stackmap
```

## Example Report Snippets

Clean audit pass, as validated against `stkapp`:

```md
## Audit Result

- Status: passed
- Exit code: 0
- Blocking issues: none
```

Warning/pass audit, as validated against `animerec --allow-missing-tests`:

```md
## Audit Result

- Status: passed
- Exit code: 0
- Blocking issues: none
- Warnings:

  - 1 low finding detected.
  - Deployment target detected without a health endpoint; this may be acceptable for static frontend apps.
  - Tests were not detected.
```

AI summary shape with deterministic summary first and local notes second:

```md
## AI Project Summary

StackMap detected this as a Next.js/React application using TypeScript, JavaScript, PostgreSQL, Vitest, and Vercel. The project appears deployment-aware: tests, health endpoints, migration files, deployment docs, and an env example are present. No actionable findings were detected.

### Local AI Notes

This TypeScript Next.js/React app has PostgreSQL and Vercel signals in the StackMap factsheet.

- Vitest is detected for testing.
- Migration files and an env example are present.
```

## Local AI Summaries

AI is disabled by default. To enable local Ollama summaries:

```sh
ollama serve
ollama pull llama3.2:3b
go run ./cmd/stackmap analyze . --ai
```

When enabled, StackMap sends only a compact factsheet of the deterministic analysis to the local Ollama server. It does not send the entire repository or `.env` files. If Ollama is unavailable or a model returns unusable text, StackMap records a friendly warning and continues with static analysis and a deterministic fallback summary.

Model recommendations:

- Start with `llama3.2:3b` for fast local summaries.
- Try `qwen:7b` if you already have it installed or want a second local model option.
- Use `--model <name>` to force a specific Ollama model.

By default, StackMap tries `llama3.2:3b`, then `qwen:7b`, then falls back to the deterministic StackMap summary. AI status, model failures, attempted models, and local model availability never affect audit pass/fail.

The same local-only AI rule applies to `stackmap ask --ai`: the model receives a compact Q&A factsheet, not the full repository.

## AI Debug Mode

Use `--ai-debug` to inspect the local prompt, factsheet, and model responses:

```sh
stackmap analyze . --ai-debug
```

Diagnostics are written under `.stackmap/ai-debug/`. Debug mode is for troubleshooting local Ollama behavior; StackMap still avoids reading `.env` values.

## GitHub Actions

This is intentionally simple. It runs the deterministic audit from source and lets the command exit code decide the job result.

```yaml
name: StackMap Audit

on:
  pull_request:
  push:
    branches: [main]

jobs:
  audit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.22"
      - run: go run ./cmd/stackmap audit .
```

Use `--allow-missing-tests` or `--allow-medium` only when that tradeoff is intentional for the repository.

## Limitations and Tradeoffs

- StackMap uses static heuristics, not full program execution.
- It does not run project build, test, install, or migration commands.
- Ask mode is evidence-based over StackMap's current analysis; unsupported questions get suggested examples instead of speculative answers.
- Ask mode does not implement embeddings, semantic vector search, or full raw source chat.
- Local AI quality depends on installed Ollama models.
- AI notes are report-only and never determine audit pass/fail.
- Frontend-only apps may not need health endpoints; StackMap warns instead of failing when no backend/API surface is detected. Hybrid frontend plus serverless/API projects are treated as having backend/API surface.
- Some framework, route, deployment, and monorepo detection will need future expansion.
- Findings are intentionally conservative, so StackMap may miss project-specific readiness issues.

## Not Included Yet

- Web app
- Database
- Embeddings
- GitHub auth
- OpenAI or cloud APIs
- Cloud storage
- Arbitrary command execution inside analyzed projects

## Development

```sh
go fmt ./...
go test ./...
go run ./cmd/stackmap --help
go run ./cmd/stackmap analyze . --no-tui
go run ./cmd/stackmap analyze . --json
go run ./cmd/stackmap audit .
go run ./cmd/stackmap
```
