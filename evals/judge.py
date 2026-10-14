#!/usr/bin/env python3
"""LLM judge for py-insight reports: does the report find the planted patterns, justified with evidence?

  python3 evals/judge.py <run-dir-or-workspace>      # a results/.../run-N dir (has workspace/out) or a workspace (has out/)

Calls the Claude CLI with an Opus model. The hidden truth (planted patterns with the empirical
numbers) comes from the shared generator, so it never reaches the agent under test. Writes judge.json
next to the report and prints it.
"""
import json
import subprocess
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
PY = HERE / "tasks" / "_pydata" / ".venv" / "bin" / "python"
PATTERNS = ["seasonality", "growth_trend", "churn_by_channel", "churn_by_tickets", "enterprise",
            "price_increase", "early_churn"]

root = Path(sys.argv[1]).resolve()
out = root / "workspace" / "out" if (root / "workspace" / "out").exists() else root / "out"
report = out / "report.md"
if not report.exists():
    sys.exit(f"no report at {report}")
charts = sorted((out / "charts").glob("*.png"))

truth_file = Path("/tmp") / "evals-py-insight-truth.json"
subprocess.run([str(PY), str(HERE / "tasks" / "_pydata" / "gen.py"), "truth", str(truth_file)], check=True)
truth = json.loads(truth_file.read_text())

prompt = f"""You are a strict senior data scientist grading an analyst's report on a subscription business.
You know the hidden ground truth (planted patterns, with the real numbers):

{json.dumps(truth, indent=1)}

The analyst's report is at {report} (read it). Their charts are PNG files; look at them with your file reading
tool to check that they exist, are readable and support the claims:
{chr(10).join(str(c) for c in charts) or '(no charts)'}

For each pattern key below decide: "found" (the report states this pattern in substance), "justified" (it is
backed by correct-looking numbers/evidence from the data, not just asserted), and "correct" (no contradiction
with the truth; a wrong number or wrong direction means false). Then rate modeling rigor and presentation.

Score components (integers):
- per pattern (7 patterns): 0 if not found; 5 if found but unsupported or partly wrong; 10 if found, justified
  and correct. (max 70)
- modeling_rigor 0-15: temporal framing without leakage (features known at prediction time), honest
  validation, a real sensitivity analysis, model choice justified.
- presentation 0-15: clear structure, charts that actually support the claims, limitations stated honestly,
  actionable recommendations.
- penalty 0-20: subtract for claims the data does not support, fabricated numbers, or contradictions.

Reply with ONLY one JSON object, no prose, no code fence:
{{"patterns": {{"seasonality": {{"found": bool, "justified": bool, "correct": bool, "points": int, "note": "..."}}, ... all 7 keys: {", ".join(PATTERNS)} ...}},
 "modeling_rigor": int, "presentation": int, "penalty": int, "summary": "two sentences"}}"""

r = subprocess.run(["claude", "-p", "--model", "opus", "--effort", "medium", "--setting-sources", "project",
                    "--strict-mcp-config", "--disable-slash-commands", "--no-session-persistence",
                    "--dangerously-skip-permissions", prompt], capture_output=True, text=True, timeout=900)
text = r.stdout.strip()
start, end = text.find("{"), text.rfind("}")
if r.returncode != 0 or start < 0:
    sys.exit(f"judge failed: {r.stderr[-500:] or text[-500:]}")
res = json.loads(text[start:end + 1])
pts = sum(min(int(v.get("points", 0)), 10) for v in res["patterns"].values())
res["score"] = max(0, min(100, pts + int(res["modeling_rigor"]) + int(res["presentation"]) - int(res["penalty"])))
(report.parent / "judge.json").write_text(json.dumps(res, indent=1))
print(json.dumps(res))
