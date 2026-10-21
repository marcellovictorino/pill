"""Hidden verifier for py-insight (100 points).

  50  holdout AUC of out/predictions.csv against hidden outcomes (customers active at T that churn in the
      next 90 days). Linear: AUC <= 0.65 -> 0, AUC >= 0.75 -> 50. The reference solution gets 0.76; a
      tenure-only score (or a naive "ever churned" model) gets about 0.70, i.e. half the points.
  15  artifacts: predictions format and coverage (5), report.md length (4), >= 3 valid charts (6)
  35  weak keyword signal for the 7 planted patterns (5 each). The real judging of findings and
      justification is evals/judge.py (LLM judge); this is only a cheap automated hint.

python verify_impl.py <workspace>  -> last stdout line is JSON {"score","total","checks"}.
"""
import json
import re
import struct
import sys
from pathlib import Path

import numpy as np
import pandas as pd
from sklearn.metrics import roc_auc_score

sys.path.insert(0, str(Path(__file__).resolve().parent.parent / "_pydata"))
import gen  # noqa: E402

AUC_ZERO, AUC_FULL = 0.65, 0.75
ws = Path(sys.argv[1])
out = ws / "out"
checks = []


def add(name, weight, frac, detail=""):
    frac = max(0.0, min(1.0, float(frac)))
    checks.append({"name": name, "pass": frac >= 0.999, "detail": detail, "points": round(weight * frac, 2),
                   "weight": weight})


w = gen.build()
labels = w.labels

# ---------------- predictions: format + AUC ----------------
pred = None
p = out / "predictions.csv"
if p.exists():
    try:
        pred = pd.read_csv(p)
    except Exception as e:
        add("predictions.csv readable", 5, 0, str(e)[:200])
if pred is None or not {"customer_id", "churn_probability"} <= set(pred.columns):
    add("predictions.csv format", 5, 0, "missing or wrong columns (need customer_id, churn_probability)")
    add("holdout AUC", 50, 0, "no usable predictions")
else:
    pred["churn_probability"] = pd.to_numeric(pred["churn_probability"], errors="coerce")
    ids = set(labels["customer_id"])
    cover = len(ids & set(pred["customer_id"])) / len(ids)
    fmt_ok = (pred["customer_id"].is_unique and pred["churn_probability"].between(0, 1).all()
              and not pred["churn_probability"].isna().any() and cover >= 0.99)
    add("predictions.csv: unique ids, probabilities in [0,1], covers all active customers", 5, 1 if fmt_ok else 0.3 * cover,
        f"coverage {cover:.1%}, rows {len(pred)}, expected {len(ids)}")
    m = labels.merge(pred.drop_duplicates("customer_id"), on="customer_id", how="left")
    if cover < 0.99 or m["churn_probability"].isna().any() or m["churn_probability"].nunique() < 2:
        add("holdout AUC", 50, 0, f"coverage {cover:.1%} or constant predictions: AUC not credited")
    else:
        auc = roc_auc_score(m["label"], m["churn_probability"])
        frac = (auc - AUC_ZERO) / (AUC_FULL - AUC_ZERO)
        add(f"holdout AUC {auc:.3f} (0 at {AUC_ZERO}, full at {AUC_FULL})", 50, frac, f"AUC={auc:.4f}")

# ---------------- artifacts: report + charts ----------------
rep = out / "report.md"
text = rep.read_text() if rep.exists() else ""
add("report.md exists with substance (>= 1500 chars)", 4, min(len(text) / 1500, 1) if len(text) < 1500 else 1, f"{len(text)} chars")


def png_ok(path):
    try:
        b = path.read_bytes()
        if b[:8] != b"\x89PNG\r\n\x1a\n" or len(b) < 3000:
            return False
        wd, ht = struct.unpack(">II", b[16:24])
        return wd >= 400 and ht >= 250
    except OSError:
        return False


charts = [c for c in sorted((out / "charts").glob("*.png")) if png_ok(c)] if (out / "charts").exists() else []
add("charts: >= 3 valid PNGs (>= 400x250, >= 3 KB) in out/charts/", 6, min(len(charts), 3) / 3, f"{len(charts)} valid")

# ---------------- weak keyword signal (7 x 5) ----------------
t = text.lower()


def has(*alts):
    return any(re.search(a, t) for a in alts)


patterns = {
    "seasonality": has(r"season|q4|holiday") and has(r"nov|dec") and has(r"jul|aug|summer|dip|trough|lowest"),
    "growth trend": has(r"grow|trend|increas|upward|rose|doubl") and has(r"sign-?ups?|new customers|acquisition|new_customers"),
    "paid_social churn": has(r"paid[_ ]social") and has(r"churn") and has(r"higher|highest|more|worst|twice|most|elevated"),
    "early tickets": has(r"ticket") and has(r"\b3\b|three|3\+|>= ?3|≥ ?3") and has(r"churn"),
    "enterprise": has(r"enterprise") and has(r"arpu|revenue per|149|5x|five|price|highest|larger") and has(r"churn|retention|lowest"),
    "price increase": has(r"price (increase|rise|hike|change)|pricing|repric") and has(r"2024-09|sep"),
    "early-life churn": has(r"first (90|three|3)|within 90|early|onboarding|first quarter|new customers? churn|tenure") and has(r"churn"),
}
for name, hit in patterns.items():
    add(f"keyword hint: {name}", 5, 1 if hit else 0)

score = round(sum(c["points"] for c in checks))
print(json.dumps({"score": score, "total": 100, "checks": checks}))
