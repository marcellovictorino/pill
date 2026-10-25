#!/usr/bin/env bash
# Idempotent: create the shared venv (duckdb, pandas, scikit-learn, matplotlib, faker) with pinned versions.
# The Python version comes from .python-version, which uv reads: pyarrow==21.0.0 ships no wheel for 3.14+.
# A venv on any other Python (for example one left by an earlier failed run) is recreated.
set -euo pipefail
cd "$(dirname "$0")"
want=$(cat .python-version)
have=$(.venv/bin/python -c 'import sys; print("%d.%d" % sys.version_info[:2])' 2>/dev/null || true)
[ "$have" = "$want" ] || uv venv -q --clear .venv
uv pip install -q --python .venv/bin/python -r requirements.txt
