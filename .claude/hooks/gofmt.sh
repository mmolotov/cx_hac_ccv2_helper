#!/usr/bin/env bash
# PostToolUse(Edit|Write): gofmt the edited Go file and report vet errors for its package.
set -euo pipefail
file=$(jq -r '.tool_input.file_path // empty')
[[ "$file" == *.go && -f "$file" ]] || exit 0
gofmt -w "$file"
cd "$CLAUDE_PROJECT_DIR"
if ! out=$(go vet "./$(dirname "${file#"$CLAUDE_PROJECT_DIR"/}")" 2>&1); then
  echo "$out" >&2
  exit 2 # feed vet errors back to Claude
fi
