#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GO_BIN="${GO_BIN:-go}"
if [[ "${GO_BIN}" == "go" ]] && command -v /opt/homebrew/bin/go >/dev/null 2>&1; then
  GO_BIN="/opt/homebrew/bin/go"
fi

mkdir -p "$ROOT/.cache/go-build" "$ROOT/.cache/go-mod"
cd "$ROOT/server"

env -u GOROOT \
  GOCACHE="$ROOT/.cache/go-build" \
  GOMODCACHE="$ROOT/.cache/go-mod" \
  "$GO_BIN" test ./...
