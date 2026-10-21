"""Shared verifier for the web-landing-* tasks. Each task's verify.py calls main(kind)."""
import json
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import pwlib  # noqa: E402

KINDS = {
    "nursery": {
        "sections": ["section-hero", "section-philosophy", "section-day", "section-teachers", "section-enrol", "section-faq"],
        "items": {"teacher-card": 3, "faq-item": 4, "lead-form": 1},
        "priced": [],
    },
    "mars": {
        "sections": ["section-hero", "section-models", "section-life", "section-safety", "section-reserve", "section-faq"],
        "items": {"dome-model": 3, "faq-item": 4, "lead-form": 1},
        "priced": ["dome-model"],
    },
    "tax": {
        "sections": ["section-hero", "section-how", "section-pricing", "section-security", "section-testimonials",
                     "section-faq", "section-signup"],
        "items": {"pricing-tier": 3, "testimonial": 2, "faq-item": 4, "lead-form": 1},
        "priced": ["pricing-tier"],
    },
}


def main(kind):
    ws = Path(sys.argv[1]).resolve()
    checks = []

    def add(name, weight, ok, detail=""):
        checks.append({"name": name, "weight": weight, "score": weight if ok else 0, "pass": bool(ok), "detail": detail})

    ok, log = pwlib.build(ws)
    add("npm run build passes (tsc --noEmit && vite build)", 15, ok, "" if ok else log)
    add("tsconfig keeps strict mode on", 3, pwlib.strict_enabled(ws))
    if ok:
        try:
            import os
            with pwlib.preview(ws) as url:
                checks += pwlib.run_node("landing.mjs", url, KINDS[kind], env={"EVAL_SCREENSHOT_DIR": os.environ.get("EVAL_SCREENSHOT_DIR", "")} if os.environ.get("EVAL_SCREENSHOT_DIR") else None)
        except Exception as e:  # noqa: BLE001
            add("playwright checks ran", 82, False, str(e)[-400:])
    else:
        add("playwright checks ran", 82, False, "skipped: build failed")
    out = [{"name": c["name"], "pass": c["pass"], "detail": f"{c['score']}/{c['weight']} {c['detail']}".strip()} for c in checks]
    print(json.dumps({"score": round(sum(c["score"] for c in checks)), "total": sum(c["weight"] for c in checks), "checks": out}))
