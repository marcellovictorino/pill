#!/usr/bin/env python3
"""Hidden verifier for web-landing-nursery. Usage: verify.py <workspace>"""
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent.parent / "_web"))
import landing_verify  # noqa: E402

landing_verify.main("nursery")
