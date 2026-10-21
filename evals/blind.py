#!/usr/bin/env python3
"""Build blind human-rating pages for the web-landing-* runs.

  python3 evals/blind.py                 # every landing run in results/index.jsonl
  python3 evals/blind.py --task web-landing-mars --seed 7

For each landing task it rebuilds each kept workspace, takes 390px and 1440px full-page screenshots
and writes results/blind/<stamp>/<task>.html: shuffled labels (A, B, C...), both screenshots per
submission, Bad/Ok/Good/Excellent buttons and a button that downloads the ratings as JSON. The
label -> (agent, run) key goes to results/blind/<stamp>/key.json: do not open it before rating.
"""
import argparse
import base64
import datetime
import json
import random
import shutil
import sys
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parent
sys.path.insert(0, str(ROOT / "tasks" / "_web"))
import pwlib  # noqa: E402

PAGE = """<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Blind rating: __TASK__</title>
<style>
body{font-family:system-ui,sans-serif;margin:0;background:#f4f4f5;color:#18181b}
header{padding:16px 24px;background:#fff;border-bottom:1px solid #d4d4d8;position:sticky;top:0;z-index:5}
h1{font-size:1.2rem;margin:0 0 4px}.sub{color:#52525b;margin:0}
.sub2{margin:8px 0 0}
section{margin:24px;padding:16px;background:#fff;border-radius:10px}
h2{margin:0 0 8px}.shots{display:flex;gap:16px;flex-wrap:wrap;align-items:flex-start}
.desk{flex:1 1 640px;max-height:80vh;overflow:auto;border:1px solid #d4d4d8}.desk img{width:100%;display:block}
.mob{flex:0 0 390px;max-height:80vh;overflow:auto;border:1px solid #d4d4d8}.mob img{width:390px;display:block}
.rate{margin-top:12px;display:flex;gap:8px;flex-wrap:wrap}
.rate label{border:2px solid #71717a;border-radius:8px;padding:8px 14px;cursor:pointer}
.rate input{margin-right:6px}.rate label:has(input:checked){background:#18181b;color:#fff;border-color:#18181b}
button{font:inherit;padding:10px 18px;border-radius:8px;border:0;background:#2563eb;color:#fff;cursor:pointer}
</style></head><body>
<header><h1>Blind rating: __TASK__</h1>
<p class="sub">Brief: __BRIEF__</p>
<p class="sub2"><span id="done">0</span> of __N__ rated. <button id="dl" type="button">Download ratings JSON</button></p></header>
__SECTIONS__
<script>
const N=__N__;
function upd(){document.getElementById("done").textContent=document.querySelectorAll("input[type=radio]:checked").length}
document.addEventListener("change",upd);
document.getElementById("dl").addEventListener("click",()=>{
  const r={};document.querySelectorAll("input[type=radio]:checked").forEach(i=>{r[i.name]=i.value});
  const blob=new Blob([JSON.stringify({task:"__TASK__",ratings:r},null,1)],{type:"application/json"});
  const a=document.createElement("a");a.href=URL.createObjectURL(blob);a.download="ratings-__TASK__.json";a.click();});
</script></body></html>"""


def b64(path):
    return "data:image/png;base64," + base64.b64encode(Path(path).read_bytes()).decode()


def screenshot(ws_kept, shots):
    tmp = Path(tempfile.mkdtemp(prefix="eval-blind-"))
    ws = tmp / "ws"
    shutil.copytree(ws_kept, ws, symlinks=True)
    link = ws / "node_modules"
    if link.exists() or link.is_symlink():
        link.unlink()
    link.symlink_to(pwlib.WEB / "node_modules")
    ok, log = pwlib.build(ws)
    if not ok:
        shutil.rmtree(tmp, ignore_errors=True)
        return False, log
    with pwlib.preview(ws) as url:
        pwlib.run_node("shot.mjs", url, {"outDir": str(shots)})
    shutil.rmtree(tmp, ignore_errors=True)
    return True, ""


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--task", help="only this landing task")
    ap.add_argument("--seed", type=int, default=None)
    ap.add_argument("--index", default=str(ROOT / "results" / "index.jsonl"))
    ap.add_argument("--out", default=str(ROOT / "results" / "blind"))
    a = ap.parse_args()
    idx = Path(a.index)
    if not idx.exists():
        sys.exit("no results yet")
    rows = [json.loads(line) for line in idx.read_text().splitlines() if line.strip()]
    rows = [r for r in rows if r["task"].startswith("web-landing") and (not a.task or r["task"] == a.task)]
    if not rows:
        sys.exit("no web-landing runs found")
    rng = random.Random(a.seed)
    stamp = datetime.datetime.now().strftime("%Y%m%dT%H%M%S")
    out = Path(a.out) / stamp
    out.mkdir(parents=True)
    key = {}
    for task in sorted({r["task"] for r in rows}):
        subs = [r for r in rows if r["task"] == task]
        rng.shuffle(subs)
        spec = (ROOT / "tasks" / task / "spec.md").read_text()
        brief = next((ln for ln in spec.splitlines() if ln.startswith("A ") and "audience" in ln), "").strip()
        sections, labels = [], {}
        for r in subs:
            label = chr(ord("A") + len(labels))
            shots = Path(tempfile.mkdtemp(prefix="eval-shots-"))
            ok, log = screenshot(Path(r["rundir"]) / "workspace", shots)
            labels[label] = {"agent": r["agent"], "run": r["run"], "rundir": r["rundir"], "score": r["score"],
                             "build_failed": not ok}
            if not ok:
                continue
            sections.append(
                f'<section><h2>Submission {label}</h2><div class="shots"><div class="desk"><img alt="Desktop screenshot of submission {label}" src="{b64(shots / "1440.png")}"></div>'
                f'<div class="mob"><img alt="Mobile screenshot of submission {label}" src="{b64(shots / "390.png")}"></div></div>'
                f'<div class="rate" role="radiogroup" aria-label="Rating for submission {label}">'
                + "".join(f'<label><input type="radio" name="{label}" value="{v}">{v}</label>' for v in ("Bad", "Ok", "Good", "Excellent"))
                + "</div></section>")
            shutil.rmtree(shots, ignore_errors=True)
        key[task] = labels
        html = (PAGE.replace("__TASK__", task).replace("__BRIEF__", brief or task).replace("__N__", str(len(sections)))
                .replace("__SECTIONS__", "\n".join(sections)))
        (out / f"{task}.html").write_text(html)
        print(f"wrote {out / (task + '.html')} ({len(sections)} submissions)")
    (out / "key.json").write_text(json.dumps(key, indent=1))
    print(f"key (do not open before rating): {out / 'key.json'}")


if __name__ == "__main__":
    main()
