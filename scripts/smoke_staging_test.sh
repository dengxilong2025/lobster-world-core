#!/usr/bin/env bash
set -euo pipefail

out="$(bash scripts/smoke_staging.sh --help || true)"
echo "$out" | grep -q "BASE_URL"
echo "$out" | grep -q "/healthz"
echo "$out" | grep -q "/api/v0/intents"
echo "OK"

