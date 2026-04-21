#!/usr/bin/env bash
set -euo pipefail

# Build a lightweight bundle for external AI/code audit.
# Output: review_bundle.zip at repo root (ignored by git).

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="${ROOT}/review_bundle.zip"

cd "$ROOT"

rm -f "$OUT"

zip -r "$OUT" \
  llms.txt \
  README.md \
  docs \
  scripts \
  cmd \
  internal \
  tests \
  go.mod go.sum \
  -x "**/.git/**" \
  -x "**/node_modules/**" \
  -x "out/**" \
  -x "**/*.zip"

echo "OK: ${OUT}"
