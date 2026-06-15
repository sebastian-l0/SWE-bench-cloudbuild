#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MODE="${1:-pre-commit}"

cd "$ROOT"

run() {
  echo "+ $*"
  "$@"
}

case "$MODE" in
  tdd)
    run ./scripts/tdd.sh
    ;;
  pre-commit)
    run ./scripts/check-shell.sh
    run ./scripts/check-secrets.sh
    run ./scripts/check-openspec.sh
    ;;
  pre-push)
    run ./scripts/check-shell.sh
    run ./scripts/check-secrets.sh
    run ./scripts/check-openspec.sh
    run ./test/harness_test.sh
    ;;
  *)
    echo "usage: $0 {tdd|pre-commit|pre-push}" >&2
    exit 2
    ;;
esac
