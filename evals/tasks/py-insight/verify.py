#!/usr/bin/env python3
"""Wrapper: run verify_impl.py with the shared venv python (the harness python has no pandas)."""
import subprocess
import sys
from pathlib import Path

here = Path(__file__).resolve().parent
py = here.parent / "_pydata" / ".venv" / "bin" / "python"
r = subprocess.run([str(py), str(here / "verify_impl.py"), *sys.argv[1:]])
sys.exit(r.returncode)
