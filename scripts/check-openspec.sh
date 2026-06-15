#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if ! command -v openspec >/dev/null 2>&1; then
  echo "openspec CLI is required for spec validation" >&2
  exit 1
fi

status=0
while IFS= read -r change_dir; do
  change_id="$(basename "$change_dir")"
  echo "+ openspec validate $change_id --strict"
  if ! openspec validate "$change_id" --strict; then
    status=1
  fi
done < <(find openspec/changes -mindepth 1 -maxdepth 1 -type d ! -name archive | sort)

exit "$status"
