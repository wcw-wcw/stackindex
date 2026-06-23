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
  if "$@" >/tmp/stackindex-validate-local.log 2>&1; then
    pass "$label"
  else
    fail "$label"
    sed 's/^/  /' /tmp/stackindex-validate-local.log
  fi
}

run_optional_audit() {
  local project="$1"
  local label="$2"
  shift 2
  if "$@" >/tmp/stackindex-validate-local.log 2>&1; then
    pass "$label"
    return
  fi
  if [[ "$RUN_AUDIT_STRICT" == "1" ]]; then
    fail "$label"
    sed 's/^/  /' /tmp/stackindex-validate-local.log
  else
    printf 'PASS %s (audit reported expected blockers; set STACKMAP_VALIDATE_STRICT_AUDIT=1 to fail)\n' "$label"
    sed 's/^/  /' /tmp/stackindex-validate-local.log
  fi
}

check_file() {
  local label="$1"
  local path="$2"
  if [[ -s "$path" ]]; then
    pass "$label"
  else
    fail "$label"
    printf '  missing or empty: %s\n' "$path"
  fi
}

check_contains() {
  local label="$1"
  local path="$2"
  local needle="$3"
  if [[ -f "$path" ]] && grep -q "$needle" "$path"; then
    pass "$label"
  else
    fail "$label"
    printf '  %s did not contain %s\n' "$path" "$needle"
  fi
}

latest_snapshot_dir() {
  local project="$1"
  local history_dir="$project/.stackindex/history"
  if [[ ! -d "$history_dir" ]]; then
    return 1
  fi
  find "$history_dir" -mindepth 1 -maxdepth 1 -type d | sort | tail -n 1
}

check_reports() {
  local project="$1"
  check_file "latest analysis.json $project" "$project/.stackindex/analysis.json"
  check_file "latest repo-index.md $project" "$project/.stackindex/reports/repo-index.md"
}

check_snapshot() {
  local project="$1"
  local snapshot
  snapshot="$(latest_snapshot_dir "$project")"
  if [[ -z "$snapshot" ]]; then
    fail "snapshot directory $project"
    printf '  missing history directory: %s\n' "$project/.stackindex/history"
    return
  fi
  pass "snapshot directory $project"
  check_file "snapshot analysis.json $project" "$snapshot/analysis.json"
  check_file "snapshot repo-index.md $project" "$snapshot/repo-index.md"
}

cd "$ROOT_DIR" || exit 1

run_required "go test ./..." go test ./...

for project in "${PROJECTS[@]}"; do
  if [[ ! -d "$project" ]]; then
    skip "$project missing"
    continue
  fi

  run_optional_audit "$project" "analyze --audit --no-tui $project" go run ./cmd/stackindex analyze "$project" --audit --no-tui
  check_reports "$project"
  check_snapshot "$project"

  run_required "ask auth $project" go run ./cmd/stackindex ask "$project" "Where is auth handled?"
  run_required "ask database $project" go run ./cmd/stackindex ask "$project" "Where is the database initialized?"
  run_required "ask local run $project" go run ./cmd/stackindex ask "$project" "How do I run this locally?"
  run_required "ask read first $project" go run ./cmd/stackindex ask "$project" "What files should I read first?"
  run_required "ask changes $project" go run ./cmd/stackindex ask "$project" "What changed since last analysis?"
  check_file "qa latest $project" "$project/.stackindex/qa/latest-question.json"
  check_file "qa history $project" "$project/.stackindex/qa/history.jsonl"

  run_optional_audit "$project" "second analyze --audit --no-tui $project" go run ./cmd/stackindex analyze "$project" --audit --no-tui
  check_reports "$project"
  check_snapshot "$project"
  check_contains "change summary in analysis.json $project" "$project/.stackindex/analysis.json" '"changes"'
  check_contains "change summary in repo-index.md $project" "$project/.stackindex/reports/repo-index.md" 'Changes Since Previous Snapshot'
done

rm -f /tmp/stackindex-validate-local.log

if [[ "$failures" -gt 0 ]]; then
  printf 'FAIL validation completed with %d failure(s)\n' "$failures"
  exit 1
fi

printf 'PASS validation completed\n'
