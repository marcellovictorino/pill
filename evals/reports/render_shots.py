#!/usr/bin/env python3
"""Render screenshots of the saved web-* outputs for the reports.

  python3 evals/reports/render_shots.py                      # every web task, every agent in results/index.jsonl
  python3 evals/reports/render_shots.py --task web-kanban --agent claude:opus:medium

For each (task, agent) it takes the earliest saved run (the run the blind ratings refer to), rebuilds the
kept workspace, takes a 1440px screenshot, crops it to the first CROP_H pixels, shrinks it and writes
evals/reports/img/<task>/<agent-slug>.webp. A workspace that does not build gets no image (the report
shows a "build failed" placeholder). Needs the web toolchain from tasks/_web (setup.sh) and `cwebp`.
"""
import argparse
import json
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

HERE = Path(__file__).resolve().parent
ROOT = HERE.parent
sys.path.insert(0, str(ROOT))
sys.path.insert(0, str(ROOT / "tasks" / "_web"))
import pwlib  # noqa: E402

CROP_H, WIDTH, QUALITY = 2400, 800, 72  # keeps each image to a few tens of KB


def slug(agent):
    return agent.replace(":", "_").replace("/", "_").replace(".", "-")


def render(ws_kept, shots, task):
    """Rebuild a kept workspace and screenshot it; the kanban board gets a few cards added first."""
    tmp = Path(tempfile.mkdtemp(prefix="eval-shots-ws-"))
    ws = tmp / "ws"
    shutil.copytree(ws_kept, ws, symlinks=True)
    link = ws / "node_modules"
    if link.exists() or link.is_symlink():
        link.unlink()
    link.symlink_to(pwlib.WEB / "node_modules")  # the shared toolchain, as the verifiers do
    ok, _ = pwlib.build(ws)
    if ok:
        script, name = ("shot_kanban.mjs", "kanban.png") if task == "web-kanban" else ("shot.mjs", "1440.png")
        with pwlib.preview(ws) as url:
            pwlib.run_node(script, url, {"outDir": str(shots)})
    shutil.rmtree(tmp, ignore_errors=True)
    return ok, shots / name if ok else None


def to_webp(png, dest):
    from PIL import Image  # Pillow ships in the evals python venv (tasks/_pydata/.venv)
    im = Image.open(png).convert("RGB")
    im = im.crop((0, 0, im.width, min(im.height, CROP_H)))
    im = im.resize((WIDTH, round(im.height * WIDTH / im.width)), Image.LANCZOS)
    tmp = Path(tempfile.mkdtemp()) / "s.png"
    im.save(tmp)
    dest.parent.mkdir(parents=True, exist_ok=True)
    subprocess.run(["cwebp", "-quiet", "-q", str(QUALITY), str(tmp), "-o", str(dest)], check=True)
    shutil.rmtree(tmp.parent, ignore_errors=True)


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--task")
    ap.add_argument("--agent")
    a = ap.parse_args()
    rows = [json.loads(ln) for ln in (ROOT / "results" / "index.jsonl").read_text().splitlines() if ln.strip()]
    rows = [r for r in rows if r["task"].startswith("web-") and (not a.task or r["task"] == a.task)
            and (not a.agent or r["agent"] == a.agent)]
    first = {}
    for r in sorted(rows, key=lambda r: r["when"]):
        first.setdefault((r["task"], r["agent"]), r)
    for (task, agent), r in sorted(first.items()):
        shots = Path(tempfile.mkdtemp(prefix="eval-shots-"))
        ok, png = render(Path(r["rundir"]) / "workspace", shots, task)
        dest = HERE / "img" / task / f"{slug(agent)}.webp"
        if ok:
            to_webp(png, dest)
            print(f"{task} {agent}: {dest.relative_to(ROOT)} {dest.stat().st_size // 1024} KB")
        else:
            print(f"{task} {agent}: build failed, no image")
        shutil.rmtree(shots, ignore_errors=True)


if __name__ == "__main__":
    main()
