#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

cat <<'MSG'
TDD harness reminder:
  1. Write or update the smallest failing test first.
  2. Run the focused test and verify it fails for the expected reason.
  3. Implement the smallest code change.
  4. Run the focused test again and then the relevant suite.

Current repository harness checks are running now.
MSG

./scripts/check-shell.sh
./scripts/check-openspec.sh
