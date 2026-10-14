#!/usr/bin/env python3
"""python3 prepare.py <workspace>: write the dirty parquet tables into <workspace>/data (seed 7).

Runs gen.py with the shared venv python, because the harness python has no pandas.
"""
import subprocess
import sys
from pathlib import Path

here = Path(__file__).resolve().parent
sys.exit(subprocess.run([str(here / ".venv/bin/python"), str(here / "gen.py"), "write",
                         str(Path(sys.argv[1]) / "data")]).returncode)
