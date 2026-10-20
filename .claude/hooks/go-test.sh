#!/usr/bin/env bash
# Stop: if Go files changed, run the test suite; block the stop on failure.
set -uo pipefail
input=$(cat)
# avoid infinite loops when Claude is already continuing because of this hook
[[ $(jq -r '.stop_hook_active' <<<"$input") == "true" ]] && exit 0
cd "$CLAUDE_PROJECT_DIR"
git status --porcelain -- '*.go' go.mod go.sum | grep -q . || exit 0
if ! out=$(go test ./... 2>&1); then
  echo "go test ./... failed:" >&2
  echo "$out" | grep -v '^ok\|no test files' | tail -40 >&2
  exit 2
fi
