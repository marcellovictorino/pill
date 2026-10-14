#!/usr/bin/env python3
"""Hidden verifier for web-kanban: build + strict tsconfig + Playwright checks. Usage: verify.py <workspace>"""
import json
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent.parent / "_web"))
import pwlib  # noqa: E402

ws = Path(sys.argv[1]).resolve()
checks = []


def add(name, weight, ok, detail=""):
    checks.append({"name": name, "weight": weight, "score": weight if ok else 0, "pass": bool(ok), "detail": detail})


ok, log = pwlib.build(ws)
add("npm run build passes (tsc --noEmit && vite build)", 14, ok, "" if ok else log)
add("tsconfig keeps strict mode on", 2, pwlib.strict_enabled(ws))
if ok:
    try:
        with pwlib.preview(ws) as url:
            checks += pwlib.run_node("kanban.mjs", url)
    except Exception as e:  # noqa: BLE001
        add("playwright checks ran", 84, False, str(e)[-400:])
else:
    add("playwright checks ran", 84, False, "skipped: build failed")

score = round(sum(c["score"] for c in checks))
out = [{"name": c["name"], "pass": c["pass"], "detail": f"{c['score']}/{c['weight']} {c['detail']}".strip()} for c in checks]
print(json.dumps({"score": score, "total": sum(c["weight"] for c in checks), "checks": out}))
