#!/usr/bin/env bash
# Idempotent: create the shared venv (duckdb, pandas, scikit-learn, matplotlib, faker) with pinned versions.
set -euo pipefail
cd "$(dirname "$0")"
[ -x .venv/bin/python ] || uv venv -q .venv
uv pip install -q --python .venv/bin/python -r requirements.txt
