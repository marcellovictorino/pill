#!/usr/bin/env python3
"""prepare.py <workspace>: lay down the Vite+React+TS scaffold and link the shared node_modules."""
import shutil
import sys
from pathlib import Path

here = Path(__file__).resolve().parent
ws = Path(sys.argv[1])
shutil.copytree(here / "scaffold", ws, dirs_exist_ok=True)
link = ws / "node_modules"
if link.is_symlink() or link.exists():
    link.unlink() if link.is_symlink() else shutil.rmtree(link)
link.symlink_to(here / "node_modules")  # symlink so agents never need network
