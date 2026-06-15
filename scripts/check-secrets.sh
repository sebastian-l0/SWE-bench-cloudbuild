#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

# Lightweight repository guard. It intentionally focuses on common committed-secret shapes
# and allows documentation that mentions variable names without assigning real values.
patterns=(
  'VOLC_ACCESS_KEY=[^[:space:]<>{}$]+'
  'VOLC_SECRET_KEY=[^[:space:]<>{}$]+'
  'VOLC_BYTEPLUS_ACCESS_KEY=[^[:space:]<>{}$]+'
  'VOLC_BYTEPLUS_SECRET_KEY=[^[:space:]<>{}$]+'
  'AK=[A-Za-z0-9_/+=-]{12,}'
  'SK=[A-Za-z0-9_/+=-]{12,}'
  'SECRET_KEY=[A-Za-z0-9_/+=-]{12,}'
  'ACCESS_TOKEN=[A-Za-z0-9_.-]{20,}'
)

exclude=(
  '--glob=!.git/**'
  '--glob=!node_modules/**'
  '--glob=!server/data/**'
  '--glob=!dist/**'
  '--glob=!build/**'
  '--glob=!coverage/**'
  '--glob=!scripts/check-secrets.sh'
)

for pattern in "${patterns[@]}"; do
  if rg -n --pcre2 "${exclude[@]}" "$pattern" . >/tmp/swe-cloudbuild-secret-scan.txt; then
    echo "potential committed secret detected for pattern: $pattern" >&2
    cat /tmp/swe-cloudbuild-secret-scan.txt >&2
    exit 1
  fi
done

echo "secret scan passed"
