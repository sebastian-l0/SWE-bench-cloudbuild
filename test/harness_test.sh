#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

require_file() {
  local path="$1"
  test -f "$ROOT/$path" || { echo "missing file: $path" >&2; exit 1; }
}

require_executable() {
  local path="$1"
  test -x "$ROOT/$path" || { echo "missing executable: $path" >&2; exit 1; }
}

require_contains() {
  local path="$1"
  local text="$2"
  grep -Fq "$text" "$ROOT/$path" || { echo "missing text in $path: $text" >&2; exit 1; }
}

require_file Makefile
require_file lefthook.yml
require_executable scripts/check.sh
require_executable scripts/check-openspec.sh
require_executable scripts/check-secrets.sh
require_executable scripts/check-shell.sh
require_executable scripts/test-backend.sh
require_executable scripts/tdd.sh
require_executable test/harness_test.sh

require_contains Makefile "tdd"
require_contains Makefile "pre-commit"
require_contains Makefile "pre-push"
require_contains Makefile "test-backend"
require_contains lefthook.yml "./scripts/check.sh pre-commit"
require_contains lefthook.yml "./scripts/check.sh pre-push"
require_contains AGENTS.md "make tdd"
require_contains AGENTS.md "make pre-commit"
require_contains AGENTS.md "make pre-push"

"$ROOT/scripts/check.sh" tdd
"$ROOT/scripts/check.sh" pre-commit
