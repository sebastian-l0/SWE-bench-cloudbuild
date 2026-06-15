#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

scripts=()
while IFS= read -r file; do
  scripts+=("$file")
done < <(find scripts test -type f -name '*.sh' | sort)

for file in "${scripts[@]}"; do
  case "$file" in
    *.sample) continue ;;
  esac
  echo "+ bash -n $file"
  bash -n "$file"
done

if command -v shellcheck >/dev/null 2>&1; then
  echo "+ shellcheck ${scripts[*]}"
  shellcheck "${scripts[@]}"
else
  echo "shellcheck not installed; skipped"
fi
