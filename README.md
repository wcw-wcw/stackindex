# StackMap

StackMap is a local-first developer tool for quickly mapping a codebase from the terminal. It scans a local repository, detects common stack signals, audits deployment and readiness issues, and exports useful Markdown and JSON reports.

The MVP is built in Go with a Bubble Tea terminal UI. Static analysis works without AI. Optional AI support is local-only through Ollama.

## Install and Run

```sh
go run ./cmd/stackmap --help
go run ./cmd/stackmap
go run ./cmd/stackmap analyze .
go run ./cmd/stackmap analyze ./path-to-project
go run ./cmd/stackmap analyze . --no-tui
go run ./cmd/stackmap analyze . --json
```

After building:

```sh
go build -o stackmap ./cmd/stackmap
./stackmap analyze .
```

## What StackMap Checks

- Repository files and basic metadata
- JavaScript and TypeScript package scripts and dependencies
- Stack signals such as React, Vite, Next.js, Express, TypeScript, Tailwind, PostgreSQL, Prisma, Drizzle, SQLite, Vitest, Jest, Playwright, and Vercel
- Environment variable usage compared with `.env.example`
- Basic Express and Next.js route patterns
- Test infrastructure and test scripts
- Deployment readiness signals such as README, Dockerfile, Vercel config, health endpoints, migrations, and setup notes

StackMap intentionally keeps findings conservative to avoid noisy false positives.

## Generated Files

Reports are written under the analyzed repository:

```text
.stackmap/
  analysis.json
  reports/
    repo-report.md
```

Add `.stackmap/` to your project `.gitignore` unless you deliberately want to commit generated reports.

On macOS, Finder hides folders that start with a dot. If you do not see `.stackmap` in Finder, press `Cmd+Shift+.` to toggle hidden files, or open it directly:

```sh
open /path/to/your/repo/.stackmap
```

## Local Storage and Privacy

StackMap is local-first:

- It does not upload code.
- It does not use OpenAI or cloud APIs.
- It does not run arbitrary project commands.
- It skips common heavy folders such as `.git`, `node_modules`, `dist`, `build`, `.next`, `coverage`, and `.stackmap`.
- It avoids reading or printing real `.env` values. `.env.example` may be scanned for variable names and placeholder safety.

## Optional Ollama/Qwen Analysis

AI is disabled by default. To enable local Ollama analysis:

```sh
ollama serve
ollama pull llama3.2:3b
ollama pull qwen:7b
go run ./cmd/stackmap analyze . --ai
```

When enabled, StackMap sends only a compact factsheet of the deterministic analysis to the local Ollama server. It does not send the entire repository or `.env` files.

If Ollama is unavailable, StackMap records a friendly warning and continues with static analysis.

Local model behavior varies. By default StackMap tries `llama3.2:3b`, then `qwen:7b`, then falls back to the deterministic StackMap summary. To force one model, pass `--model <name>`. To inspect the local prompt and model responses for troubleshooting, run with `--ai-debug`; StackMap writes diagnostics under `.stackmap/ai-debug/` without reading `.env` values.

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
go run ./cmd/stackmap
```
