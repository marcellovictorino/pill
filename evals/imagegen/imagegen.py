#!/usr/bin/env python3
"""Image-generation benchmark: generate, judge with Opus 5.5, build a blind human sheet, self-validate.

  python3 evals/imagegen/imagegen.py run flux3                 # 3 prompts x 3 seeds, one process per image
  python3 evals/imagegen/imagegen.py run qwen-turbo
  python3 evals/imagegen/imagegen.py judge <run-dir> [<run-dir>...]   # PAID: Opus 5.5 rubric, blind to the model
  python3 evals/imagegen/imagegen.py blind <run-dir> [<run-dir>...]   # blind human sheet + key.json
  python3 evals/imagegen/imagegen.py validate determinism flux3       # same seed twice -> same pixels?
  python3 evals/imagegen/imagegen.py validate judge <run-dir>...      # PAID: matched vs mismatched, order swap
  python3 evals/imagegen/imagegen.py validate agreement <ratings.json> <key.json> <run-dir>...
  python3 evals/imagegen/imagegen.py selftest                         # whole pipeline with a fake generator, no model

Models are the captain's: FLUX 3 Image Klein (mflux, 4-bit) and Qwen-Image 2.1 Turbo (Draw Things CLI,
4-bit S, 8 steps). Every model gets the three prompts in prompts.json verbatim, no style words added,
seeds 7/42/1234, 1024x1024. Only one local model can run on this machine at a time: run() refuses while
another model server is up (stop pill first: `pill stop`). Results go to evals/results/imagegen/ (gitignored).
"""
import argparse
import base64
import datetime
import hashlib
import json
import os
import random
import re
import struct
import subprocess
import sys
import tempfile
import time
import zlib
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE.parent))
from run import MemorySampler  # noqa: E402  the same free-memory and swap sampler the coding evals use

RESULTS = HERE.parent / "results" / "imagegen"
PROMPTS = json.loads((HERE / "prompts.json").read_text())
SEEDS = [7, 42, 1234]
SIZE = 1024
JUDGE_MODEL = "claude-opus-5-5"
CRITERIA = ["subject", "style", "setting", "technical_quality", "composition"]
MIN_FREE_PCT = 20  # personal rule for this machine: below this, raise it instead of starting a model run


def backend_cmd(model, prompt, seed, out):
    if model == "flux3":
        # 16 distilled steps (model default); -q 4 is the only variant that plausibly fits 24 GB.
        return ["mflux-generate-flux3", "--model", "flux3-image-klein", "-q", "4", "--prompt", prompt,
                "--seed", str(seed), "--width", str(SIZE), "--height", str(SIZE), "--no-exif", "--output", str(out)]
    if model == "qwen-turbo":
        return ["draw-things-cli", "generate", "--local", "--model", "qwen_image_2.1_turbo_i4x.ckpt",
                "--prompt", prompt, "--seed", str(seed), "--width", str(SIZE), "--height", str(SIZE),
                "--steps", "8", "--output", str(out)]
    if model == "fake":  # selftest only: a tiny deterministic PNG, no model
        return [sys.executable, str(HERE / "imagegen.py"), "_fake", prompt, str(seed), str(out)]
    sys.exit(f"unknown model {model}; choose flux3, qwen-turbo")


def sh(*cmd):
    try:
        return subprocess.run(cmd, capture_output=True, text=True, timeout=30).stdout
    except Exception:
        return ""


def preflight():
    """Refuse to start while another local model runs or memory is tight. Returns a list of problems."""
    problems = []
    busy = sh("pgrep", "-fl", "llama-server|mlx_lm|ollama runner|gRPCServerCLI").strip()
    if busy:
        problems.append("a model server is running (stop it first; never kill someone else's): " + busy.splitlines()[0][:120])
    m = re.search(r"free percentage: (\d+)%", sh("memory_pressure", "-Q"))
    if m and int(m.group(1)) < MIN_FREE_PCT:
        problems.append(f"free memory {m.group(1)}% is under {MIN_FREE_PCT}%")
    return problems


def parse_time_l(stderr):
    """/usr/bin/time -l prints bytes on macOS: 'maximum resident set size' and 'peak memory footprint'."""
    out = {}
    for key, pat in (("peak_rss_gb", r"(\d+)\s+maximum resident set size"), ("peak_footprint_gb", r"(\d+)\s+peak memory footprint")):
        m = re.search(pat, stderr)
        if m:
            out[key] = round(int(m.group(1)) / 1024 ** 3, 2)
    return out


def cmd_run(a):
    problems = preflight()
    if problems and not a.force:
        sys.exit("refusing to start:\n  " + "\n  ".join(problems))
    stamp = datetime.datetime.now().strftime("%Y%m%dT%H%M%S")
    out = Path(a.out) / f"{stamp}-{a.model}"
    out.mkdir(parents=True)
    rows = []
    sampler = MemorySampler().start()
    first = True
    for pid, prompt in PROMPTS.items():
        for seed in SEEDS:
            img = out / f"{pid}-{seed}.png"
            cmd = ["/usr/bin/time", "-l"] + backend_cmd(a.model, prompt, seed, img)
            t0 = time.time()
            r = subprocess.run(cmd, capture_output=True, text=True, timeout=a.timeout)
            row = {"model": a.model, "prompt_id": pid, "prompt": prompt, "seed": seed, "file": img.name,
                   "seconds": round(time.time() - t0, 1), "cold": first, "exit": r.returncode,
                   **parse_time_l(r.stderr)}
            if r.returncode != 0 or not img.exists():
                row["error"] = (r.stderr or r.stdout)[-400:]
            first = False
            rows.append(row)
            print(json.dumps(row), flush=True)
    summary = {"model": a.model, "swap_and_memory": sampler.stop(), "size": SIZE, "seeds": SEEDS}
    (out / "results.json").write_text(json.dumps({"summary": summary, "images": rows}, indent=1))
    print(f"wrote {out}")


def load_run(d):
    return json.loads((Path(d) / "results.json").read_text())["images"]


def ask_judge(prompt, images, timeout=600):
    """One Opus 5.5 call through the Claude CLI; returns the parsed JSON object."""
    dirs = sorted({str(Path(i).parent) for i in images})
    cmd = ["claude", "-p", "--model", JUDGE_MODEL, "--effort", "medium", "--setting-sources", "project",
           "--strict-mcp-config", "--disable-slash-commands", "--no-session-persistence",
           "--allowedTools", "Read"]
    for d in dirs:
        cmd += ["--add-dir", d]
    r = subprocess.run(cmd + [prompt], capture_output=True, text=True, timeout=timeout)
    text = r.stdout.strip()
    s, e = text.find("{"), text.rfind("}")
    if r.returncode != 0 or s < 0:
        raise RuntimeError(f"judge failed: {(r.stderr or text)[-400:]}")
    return json.loads(text[s:e + 1])


RUBRIC = """Score each criterion as an integer 0-5 (0 absent or wrong, 3 acceptable, 5 excellent):
- subject: the requested subject(s) are clearly present and recognisable
- style: the requested style is applied faithfully (and nothing contradicts it)
- setting: the requested place, time and atmosphere are present
- technical_quality: no distortions, malformed anatomy or objects, garbled text, artefacts or blur
- composition: framing, balance, depth and overall visual appeal
Be strict and use the full range."""


def score_prompt(prompt, image):
    return (f"You are a strict image-generation reviewer. The image was generated from this prompt only:\n"
            f"\"{prompt}\"\n\nLook at the image with your Read tool: {image}\n\n{RUBRIC}\n\n"
            'Reply with ONLY one JSON object, no prose, no code fence: {"subject": int, "style": int, '
            '"setting": int, "technical_quality": int, "composition": int, "note": "one sentence"}')


def total(scores):
    return sum(int(scores[c]) for c in CRITERIA)


def cmd_judge(a):
    for d in a.run_dir:
        rows = load_run(d)
        out = {}
        for r in rows:
            if r.get("error"):
                continue
            # The judge sees the prompt and the image path, never the model name.
            res = ask_judge(score_prompt(r["prompt"], Path(d).resolve() / r["file"]), [Path(d).resolve() / r["file"]])
            res["total"] = total(res)
            out[r["file"]] = res
            print(r["file"], res["total"], flush=True)
        (Path(d) / "judge.json").write_text(json.dumps({"judge": JUDGE_MODEL, "scores": out}, indent=1))


def b64(path):
    return "data:image/png;base64," + base64.b64encode(Path(path).read_bytes()).decode()


SHEET = """<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Blind rating: image generation</title><style>
body{font-family:system-ui,sans-serif;margin:0;background:#f4f4f5;color:#18181b}
header{padding:16px 24px;background:#fff;border-bottom:1px solid #d4d4d8;position:sticky;top:0;z-index:5}
h1{font-size:1.2rem;margin:0 0 4px}p{margin:4px 0}section{margin:24px;padding:16px;background:#fff;border-radius:10px}
.imgs{display:flex;gap:16px;flex-wrap:wrap}figure{margin:0;flex:1 1 320px;max-width:480px}figure img{width:100%;display:block;border:1px solid #d4d4d8}
.rate{margin-top:8px;display:flex;gap:6px;flex-wrap:wrap}.rate label{border:2px solid #71717a;border-radius:8px;padding:6px 12px;cursor:pointer}
.rate input{margin-right:4px}.rate label:has(input:checked){background:#18181b;color:#fff;border-color:#18181b}
button{font:inherit;padding:10px 18px;border-radius:8px;border:0;background:#2563eb;color:#fff;cursor:pointer}
</style></head><body><header><h1>Blind rating: image generation</h1>
<p>Rate each image 1 (poor) to 5 (excellent) for how well it fits the prompt shown and how good it looks. Labels are shuffled; the key is in a separate file.</p>
<p><span id="done">0</span> of __N__ rated. <button id="dl" type="button">Download ratings JSON</button></p></header>
__SECTIONS__
<script>
function upd(){document.getElementById("done").textContent=document.querySelectorAll("input[type=radio]:checked").length}
document.addEventListener("change",upd);
document.getElementById("dl").addEventListener("click",()=>{const r={};
document.querySelectorAll("input[type=radio]:checked").forEach(i=>{r[i.name]=Number(i.value)});
const a=document.createElement("a");a.href=URL.createObjectURL(new Blob([JSON.stringify({ratings:r},null,1)],{type:"application/json"}));
a.download="ratings-imagegen.json";a.click();});
</script></body></html>"""


def cmd_blind(a):
    rng = random.Random(a.seed)
    items = [(d, r) for d in a.run_dir for r in load_run(d) if not r.get("error")]
    out = Path(a.out) / datetime.datetime.now().strftime("%Y%m%dT%H%M%S")
    out.mkdir(parents=True)
    key, sections, n = {}, [], 0
    for pid, prompt in PROMPTS.items():
        group = [x for x in items if x[1]["prompt_id"] == pid]
        rng.shuffle(group)
        figs = []
        for d, r in group:
            label = f"{pid[0].upper()}{len(key) + 1}"
            key[label] = {"model": r["model"], "prompt_id": pid, "seed": r["seed"], "file": str(Path(d).resolve() / r["file"])}
            figs.append(f'<figure><img alt="Image {label}" src="{b64(Path(d) / r["file"])}"><figcaption>{label}</figcaption>'
                        '<div class="rate" role="radiogroup" aria-label="Rating for ' + label + '">'
                        + "".join(f'<label><input type="radio" name="{label}" value="{v}">{v}</label>' for v in range(1, 6))
                        + "</div></figure>")
            n += 1
        sections.append(f'<section><h2>Prompt: {prompt}</h2><div class="imgs">{"".join(figs)}</div></section>')
    (out / "sheet.html").write_text(SHEET.replace("__N__", str(n)).replace("__SECTIONS__", "\n".join(sections)))
    (out / "key.json").write_text(json.dumps(key, indent=1))
    print(f"wrote {out / 'sheet.html'} ({n} images); key (do not open before rating): {out / 'key.json'}")


def spearman(xs, ys):
    def ranks(v):
        order = sorted(range(len(v)), key=lambda i: v[i])
        r = [0.0] * len(v)
        i = 0
        while i < len(order):
            j = i
            while j + 1 < len(order) and v[order[j + 1]] == v[order[i]]:
                j += 1
            for k in range(i, j + 1):
                r[order[k]] = (i + j) / 2 + 1
            i = j + 1
        return r
    rx, ry = ranks(xs), ranks(ys)
    mx, my = sum(rx) / len(rx), sum(ry) / len(ry)
    num = sum((a - mx) * (b - my) for a, b in zip(rx, ry))
    den = (sum((a - mx) ** 2 for a in rx) * sum((b - my) ** 2 for b in ry)) ** 0.5
    return round(num / den, 3) if den else None


def validate_determinism(a):
    """Same model, prompt and seed twice: are the pixels identical? Free (local only)."""
    problems = preflight()
    if problems and not a.force:
        sys.exit("refusing to start:\n  " + "\n  ".join(problems))
    tmp = Path(tempfile.mkdtemp(prefix="imagegen-det-"))
    prompt = PROMPTS["coffee-night"]
    hashes = []
    for i in (1, 2):
        img = tmp / f"d{i}.png"
        subprocess.run(backend_cmd(a.model, prompt, SEEDS[0], img), capture_output=True, text=True, timeout=a.timeout, check=True)
        hashes.append(hashlib.sha256(img.read_bytes()).hexdigest())
    print(json.dumps({"model": a.model, "identical_bytes": hashes[0] == hashes[1], "sha256": hashes, "dir": str(tmp)}))


def validate_judge(a):
    """Paid. (1) matched vs mismatched: each image scored against its own prompt and a different one, the
    matched total should be higher in most pairs. (2) order swap: pairwise A/B judged in both orders; a bias
    shows as the same position winning both times."""
    items = [(Path(d).resolve(), r) for d in a.run_dir for r in load_run(d) if not r.get("error")]
    rng = random.Random(1)
    pids = list(PROMPTS)
    sample = rng.sample(items, min(a.pairs, len(items)))
    matched_wins, rows = 0, []
    for d, r in sample:
        other = rng.choice([p for p in pids if p != r["prompt_id"]])
        img = d / r["file"]
        m = total(ask_judge(score_prompt(r["prompt"], img), [img]))
        x = total(ask_judge(score_prompt(PROMPTS[other], img), [img]))
        matched_wins += m > x
        rows.append({"file": r["file"], "matched": m, "mismatched_prompt": other, "mismatched": x})
        print(rows[-1], flush=True)
    swaps = []
    by_prompt = {}
    for d, r in items:
        by_prompt.setdefault(r["prompt_id"], []).append((d, r))
    for pid, group in by_prompt.items():
        if len(group) < 2:
            continue
        (d1, r1), (d2, r2) = group[0], group[-1]
        i1, i2 = d1 / r1["file"], d2 / r2["file"]
        def pick(first, second):
            p = (f"Prompt: \"{PROMPTS[pid]}\"\nTwo images generated from it. Look at both with your Read tool: "
                 f"first image {first}, second image {second}. Which better fits the prompt and looks better?\n"
                 'Reply with ONLY one JSON object: {"winner": "first" or "second", "note": "one sentence"}')
            return ask_judge(p, [first, second])["winner"]
        w1, w2 = pick(i1, i2), pick(i2, i1)
        # consistent = the same image won in both orders; position bias = the same slot won both times
        swaps.append({"prompt_id": pid, "order1": w1, "order2": w2, "consistent": w1 != w2})
        print(swaps[-1], flush=True)
    res = {"matched_beats_mismatched": f"{matched_wins}/{len(sample)}", "pairs": rows, "order_swap": swaps,
           "order_swap_consistent": f"{sum(s['consistent'] for s in swaps)}/{len(swaps)}"}
    out = Path(a.run_dir[0]) / "validate-judge.json"
    out.write_text(json.dumps(res, indent=1))
    print(json.dumps({k: v for k, v in res.items() if k not in ("pairs", "order_swap")}))


def validate_agreement(a):
    """Judge vs human: Spearman between the judge's total and the human 1-5 rating, over the rated images."""
    ratings = json.loads(Path(a.ratings).read_text())["ratings"]
    key = json.loads(Path(a.key).read_text())
    judged = {}
    for d in a.run_dir:
        j = json.loads((Path(d) / "judge.json").read_text())["scores"]
        judged.update({str((Path(d).resolve() / f)): s["total"] for f, s in j.items()})
    pairs = [(judged[key[l]["file"]], v) for l, v in ratings.items() if key[l]["file"] in judged]
    rho = spearman([p[0] for p in pairs], [p[1] for p in pairs]) if len(pairs) > 2 else None
    print(json.dumps({"rated": len(pairs), "spearman_judge_vs_human": rho}))


def fake_png(prompt, seed, path):
    """A deterministic 64x64 PNG built by hand (zlib + struct), so the selftest needs no model or imaging library."""
    h = hashlib.sha256(f"{prompt}{seed}".encode()).digest()
    w = 64
    raw = b"".join(b"\x00" + bytes((h[(x + y) % 32] for x in range(w) for _ in range(3))) for y in range(w))
    def chunk(t, d):
        return struct.pack(">I", len(d)) + t + d + struct.pack(">I", zlib.crc32(t + d))
    Path(path).write_bytes(b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", struct.pack(">IIBBBBB", w, w, 8, 2, 0, 0, 0))
                           + chunk(b"IDAT", zlib.compress(raw)) + chunk(b"IEND", b""))


def cmd_selftest(a):
    """Whole pipeline with the fake generator: run -> blind sheet. The judge needs Opus and is not called."""
    tmp = Path(tempfile.mkdtemp(prefix="imagegen-selftest-"))
    ns = argparse.Namespace(model="fake", out=str(tmp), force=True, timeout=60)
    cmd_run(ns)
    run_dir = next(tmp.iterdir())
    rows = load_run(run_dir)
    assert len(rows) == len(PROMPTS) * len(SEEDS), rows
    assert all(r["exit"] == 0 and (run_dir / r["file"]).exists() for r in rows)
    assert all(r.get("peak_rss_gb") for r in rows), "time -l memory not parsed"
    cmd_blind(argparse.Namespace(run_dir=[str(run_dir)], seed=1, out=str(tmp / "blind")))
    key = json.loads(next((tmp / "blind").iterdir()).joinpath("key.json").read_text())
    assert len(key) == 9
    assert spearman([1, 2, 3, 4], [10, 20, 30, 40]) == 1.0 and spearman([1, 2, 3], [3, 2, 1]) == -1.0
    print("selftest ok", tmp)


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = ap.add_subparsers(dest="cmd", required=True)
    p = sub.add_parser("run")
    p.add_argument("model")
    p.add_argument("--out", default=str(RESULTS))
    p.add_argument("--force", action="store_true", help="skip the preflight (another model server or low memory)")
    p.add_argument("--timeout", type=int, default=1800, help="seconds per image")
    p = sub.add_parser("judge")
    p.add_argument("run_dir", nargs="+")
    p = sub.add_parser("blind")
    p.add_argument("run_dir", nargs="+")
    p.add_argument("--seed", type=int, default=None)
    p.add_argument("--out", default=str(RESULTS / "blind"))
    p = sub.add_parser("validate")
    vs = p.add_subparsers(dest="what", required=True)
    v = vs.add_parser("determinism")
    v.add_argument("model")
    v.add_argument("--force", action="store_true")
    v.add_argument("--timeout", type=int, default=1800)
    v = vs.add_parser("judge")
    v.add_argument("run_dir", nargs="+")
    v.add_argument("--pairs", type=int, default=10)
    v = vs.add_parser("agreement")
    v.add_argument("ratings")
    v.add_argument("key")
    v.add_argument("run_dir", nargs="+")
    sub.add_parser("selftest")
    f = sub.add_parser("_fake")
    f.add_argument("prompt")
    f.add_argument("seed")
    f.add_argument("out")
    a = ap.parse_args()
    if a.cmd == "_fake":
        return fake_png(a.prompt, a.seed, a.out)
    if a.cmd == "validate":
        return {"determinism": validate_determinism, "judge": validate_judge, "agreement": validate_agreement}[a.what](a)
    {"run": cmd_run, "judge": cmd_judge, "blind": cmd_blind, "selftest": cmd_selftest}[a.cmd](a)


if __name__ == "__main__":
    main()
