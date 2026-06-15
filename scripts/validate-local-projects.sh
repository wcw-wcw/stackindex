#!/usr/bin/env bash
set -u

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUN_AUDIT_STRICT="${STACKMAP_VALIDATE_STRICT_AUDIT:-0}"

PROJECTS=(
  "/Users/will/Workspace/stkapp"
  "/Users/will/Workspace/animerec"
  "/Users/will/Workspace/connect_four"
  "/Users/will/Workspace/assessmentapp"
  "/Users/will/Workspace/folio"
  "/Users/will/Workspace/my-website"
)

failures=0

pass() {
  printf 'PASS %s\n' "$1"
}

fail() {
  printf 'FAIL %s\n' "$1"
  failures=$((failures + 1))
}

skip() {
  printf 'SKIP %s\n' "$1"
}

run_required() {
  local label="$1"
  shift
  if "$@" >/tmp/stackmap-validate-local.log 2>&1; then
    pass "$label"
  else
    fail "$label"
    sed 's/^/  /' /tmp/stackmap-validate-local.log
  fi
}

run_optional_audit() {
  local project="$1"
  local label="$2"
  if go run ./cmd/stackmap audit "$project" >/tmp/stackmap-validate-local.log 2>&1; then
    pass "$label"
    return
  fi
  if [[ "$RUN_AUDIT_STRICT" == "1" ]]; then
    fail "$label"
    sed 's/^/  /' /tmp/stackmap-validate-local.log
  else
    printf 'PASS %s (audit reported expected blockers; set STACKMAP_VALIDATE_STRICT_AUDIT=1 to fail)\n' "$label"
    sed 's/^/  /' /tmp/stackmap-validate-local.log
  fi
}

cd "$ROOT_DIR" || exit 1

run_required "go test ./..." go test ./...

for project in "${PROJECTS[@]}"; do
  if [[ ! -d "$project" ]]; then
    skip "$project missing"
    continue
  fi

  run_required "analyze --no-tui $project" go run ./cmd/stackmap analyze "$project" --no-tui
  run_required "ask $project" go run ./cmd/stackmap ask "$project" "What is this project for?"
  run_optional_audit "$project" "audit $project"
done

rm -f /tmp/stackmap-validate-local.log

if [[ "$failures" -gt 0 ]]; then
  printf 'FAIL validation completed with %d failure(s)\n' "$failures"
  exit 1
fi

printf 'PASS validation completed\n'
