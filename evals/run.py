#!/usr/bin/env python3
"""evals runner: give a task to an agent in a throwaway workspace, then score it.

Standard library only. See README.md for the task layout and agent specs.

  python3 run.py list
  python3 run.py selftest <task>                     # reference must score ~100, blank fixture low
  python3 run.py run <task> --agent <spec> [--runs N] [--timeout-min M]

Agent specs:
  pi:<pill-model-name>             local model through the pill router, driven by Pi (thinking off)
  pi:<pill-model-name>:<level>     same with thinking on (Pi level minimal|low|medium|high; with pill's
                                   chat-template format this is an on/off switch for the model's reasoning)
  Add --personal to a pi: run to give Pi a copy of your own ~/.pi/agent setup (see README.md); the
  results then stay under $PILL_HOME/personal/evals and are never committed.

  claude:<model>[:<effort>]        Claude Code CLI, e.g. claude:opus:medium
  codex:<model>[:<effort>]         Codex CLI, e.g. codex:gpt-5.6-luna:high
"""
import argparse
import datetime
import hashlib
import json
import os
import shutil
import signal
import subprocess
import sys
import re
import tempfile
import threading
import time
from pathlib import Path

ROOT = Path(__file__).resolve().parent
TASKS = ROOT / "tasks"
RESULTS = ROOT / "results"

# Personal runs (the user's own full Pi setup) are never committed. Every personal record carries this
# key set to true; CI fails if a tracked file does. The marker is built here, not written out, so this
# file does not trip that check itself.
PERSONAL_KEY = "personal"
# Top-level names of ~/.pi/agent that are not part of the setup: history, caches and backups.
PERSONAL_SKIP = {"sessions", "web-search-cache"}

# Same preamble for every agent so the comparison is fair.
PREAMBLE = (
    "You are working in the current directory, which is a throwaway workspace. "
    "Do all your work inside it and do not touch anything outside it. "
    "No human is available: do not ask questions, make reasonable choices and finish the task.\n\n"
)


def load_task(task_id):
    d = TASKS / task_id
    if not (d / "task.json").exists():
        sys.exit(f"unknown task {task_id!r}; try: run.py list")
    t = json.loads((d / "task.json").read_text())
    t["dir"] = d
    return t


def all_tasks():
    return sorted(p.name for p in TASKS.iterdir() if (p / "task.json").exists())


def ensure_setup(task):
    """Run the task's idempotent one-time toolchain setup (venv, npm install, ...)."""
    setup = task.get("setup")
    if not setup:
        return
    marker = task["dir"] / ".setup-done"
    if marker.exists():
        return
    print(f"[setup] {task['id']}: {setup}", flush=True)
    r = subprocess.run(["bash", str(task["dir"] / setup)], cwd=task["dir"])
    if r.returncode != 0:
        sys.exit(f"setup failed for {task['id']}")
    marker.write_text(str(time.time()))


def task_env(task):
    env = dict(os.environ)
    extra = [str((task["dir"] / p).resolve()) for p in task.get("path_prepend", [])]
    if extra:
        env["PATH"] = os.pathsep.join(extra + [env["PATH"]])
    env.update({k: str((task["dir"] / v).resolve()) if k.endswith("_DIR_REL") else v
                for k, v in task.get("env", {}).items()})
    return env


def make_workspace(task, base, overlay_reference=False):
    ws = Path(tempfile.mkdtemp(prefix=f"eval-{task['id']}-", dir=base))
    fx = task["dir"] / task.get("fixture", "fixture")
    if fx.exists():
        shutil.copytree(fx, ws, symlinks=True, dirs_exist_ok=True)
    if task.get("prepare"):
        r = subprocess.run([sys.executable, str(task["dir"] / task["prepare"]), str(ws)],
                           env=task_env(task))
        if r.returncode != 0:
            sys.exit("prepare step failed")
    if overlay_reference:
        ref = task["dir"] / task.get("reference", "reference")
        shutil.copytree(ref, ws, symlinks=True, dirs_exist_ok=True)
    return ws


def run_cmd(cmd, ws, env, timeout_s, stdout_path, stderr_path, stdin=None):
    """Run cmd in its own process group; kill the whole group on timeout."""
    with open(stdout_path, "wb") as out, open(stderr_path, "wb") as err:
        p = subprocess.Popen(cmd, cwd=ws, env=env, stdout=out, stderr=err,
                             stdin=subprocess.DEVNULL if stdin is None else stdin,
                             start_new_session=True)
        start = time.time()
        timed_out = False
        try:
            p.wait(timeout=timeout_s)
        except subprocess.TimeoutExpired:
            timed_out = True
            os.killpg(p.pid, signal.SIGKILL)
            p.wait()
        return {"exit": p.returncode, "timed_out": timed_out, "seconds": round(time.time() - start, 1)}


class MemorySampler:
    """Samples free memory and swap every few seconds while a local model runs (macOS).

    Same sources as pill's own benchmark: `memory_pressure` for the system-wide free percentage and
    `sysctl vm.swapusage`. Other apps on the machine count too, so read the numbers as the whole
    system's state while the run was going, not the model's share alone.
    """

    def __init__(self, interval=5.0):
        self.interval, self.free, self.swap, self.level = interval, [], [], []
        self._stop = threading.Event()
        self._t = threading.Thread(target=self._loop, daemon=True)

    @staticmethod
    def _run(*cmd):
        try:
            return subprocess.run(cmd, capture_output=True, text=True, timeout=20).stdout
        except Exception:
            return ""

    def _take(self):
        m = re.search(r"free percentage: (\d+)%", self._run("memory_pressure"))
        if m:
            self.free.append(int(m.group(1)))
        m = re.search(r"used = ([\d.]+)([MG])", self._run("sysctl", "-n", "vm.swapusage"))
        if m:
            self.swap.append(float(m.group(1)) * (1024 if m.group(2) == "G" else 1))
        m = re.search(r"\d+", self._run("sysctl", "-n", "kern.memorystatus_vm_pressure_level"))
        if m:
            self.level.append(int(m.group(0)))

    def _loop(self):
        while not self._stop.is_set():
            self._take()
            self._stop.wait(self.interval)

    def start(self):
        self._t.start()
        return self

    def stop(self):
        self._stop.set()
        self._t.join(timeout=30)
        return {"samples": len(self.free), "min_free_pct": min(self.free, default=None),
                "swap_start_mb": round(self.swap[0]) if self.swap else None,
                "swap_growth_mb": round(max(self.swap) - self.swap[0]) if self.swap else None,
                "max_pressure_level": max(self.level, default=None)}


def pill_home():
    # Absolute: Pi runs in a temporary workspace and would resolve a relative path from there.
    return Path(os.environ.get("PILL_HOME") or Path.home() / ".pill").expanduser().resolve()


def personal_results_dir():
    return pill_home() / "personal" / "evals"


def inside_git_work_tree(path):
    """True when path (which may not exist yet) lies inside a git work tree."""
    p = Path(path).resolve()
    while not p.exists():
        p = p.parent
    r = subprocess.run(["git", "-C", str(p), "rev-parse", "--is-inside-work-tree"],
                       capture_output=True, text=True)
    return r.returncode == 0 and r.stdout.strip() == "true"


def guard_personal_results(path):
    """Refuse a personal results folder inside a git work tree, so a commit cannot pick it up."""
    if inside_git_work_tree(path):
        sys.exit(f"refusing to write personal results inside a git work tree: {path}\n"
                 "personal results must stay outside every repository; unset PILL_HOME or point it elsewhere")


def _is_hindsight(entry):
    text = entry if isinstance(entry, str) else json.dumps(entry)
    return "hindsight" in text.lower()


def _personal_ignore(src):
    def ignore(directory, names):
        d = Path(directory)
        # Dangling links are skipped here, judged from the link's own folder: copytree's own flag
        # checks relative targets against the launching directory and would drop valid links.
        skip = [n for n in names if (d / n).is_symlink() and not (d / n).exists()]
        if d == src:  # only the top level: node_modules and the like keep their own names
            skip += [n for n in names if n in PERSONAL_SKIP or ".bak" in n or n.endswith("-backup")
                     or ("hindsight" in n.lower() and n != "settings.json")]
        elif d in (src / "extensions", src / "skills"):  # Pi loads these folders automatically
            skip += [n for n in names if "hindsight" in n.lower()]
        return skip
    return ignore


def copy_personal_pi(src, dest):
    """Copy the user's Pi folder into dest, leaving out history, caches, backups and Hindsight.

    The Hindsight add-on talks to its own memory service, so a copy of it would read and write the
    user's real memory; its entries are dropped from settings.json. The source is only read.
    """
    src = Path(src)
    # symlinks=False copies what a link points to, so nothing in the copy can write through to the
    # original (settings.json and models.json are rewritten below).
    shutil.copytree(src, dest, symlinks=False,
                    ignore=_personal_ignore(src), dirs_exist_ok=True)
    settings = Path(dest) / "settings.json"
    if settings.exists():
        cfg = json.loads(settings.read_text())
        for key in ("extensions", "packages"):
            if isinstance(cfg.get(key), list):
                cfg[key] = [e for e in cfg[key] if not _is_hindsight(e)]
        settings.write_text(json.dumps(cfg, indent=2))


def setup_hash(d):
    """Short hash of what steers the agent in a copied Pi folder, so a changed setup shows later."""
    d = Path(d)
    h = hashlib.sha256()
    parts = [d / "settings.json", d / "AGENTS.md", d / "npm" / "package.json", d / "npm" / "package-lock.json"]
    for sub in ("skills", "extensions"):
        parts += sorted(p for p in (d / sub).rglob("*") if p.is_file()) if (d / sub).is_dir() else []
    # Settings can name extensions, skills and packages that live outside the folder: hash their contents too.
    if (d / "settings.json").is_file():
        cfg = json.loads((d / "settings.json").read_text())
        for key in ("extensions", "skills", "packages"):
            for e in cfg.get(key, []) if isinstance(cfg.get(key), list) else []:
                ext = Path(os.path.expanduser(e)) if isinstance(e, str) else None
                if ext and ext.is_file():
                    parts.append(ext)
                elif ext and ext.is_dir():
                    parts += sorted(p for p in ext.rglob("*") if p.is_file() and "node_modules" not in p.parts)
    for p in parts:
        if p.is_file():
            name = p.relative_to(d) if d in p.parents else p
            h.update(str(name).encode() + b"\0" + p.read_bytes() + b"\0")
    return h.hexdigest()[:16]


def pi_agent_dir(outdir, model, reasoning=False, personal=False):
    """Throwaway Pi config: the user's models.json (the pill provider), or a full copy when personal.

    pill drops a model from Pi's config when its benchmark failed, though the router still serves
    it. Evals compare such models too, so the entry is cloned into this throwaway copy only; the
    user's real Pi config is never touched.
    """
    d = Path(outdir) / "pi-agent"
    d.mkdir(parents=True, exist_ok=True)
    src = Path.home() / ".pi" / "agent"
    if personal:
        copy_personal_pi(src, d)
    else:
        for f in ("models.json", "models-store.json", "auth.json"):
            if (src / f).exists():
                shutil.copy(src / f, d / f)
    cfg = json.loads((d / "models.json").read_text())
    models = cfg["providers"]["pill"]["models"]
    if not any(m["id"] == model for m in models):
        entry = dict(models[0])
        entry["id"] = entry["name"] = model
        models.append(entry)
    if reasoning:
        # Pi only sends the thinking switch for models marked as reasoning models, and pill marks a
        # model that way only if it was registered with reasoning; the throwaway copy sets it here.
        for m in models:
            if m["id"] == model:
                m["reasoning"] = True
    (d / "models.json").write_text(json.dumps(cfg, indent=2))
    return d


def agent_command(spec, prompt, ws, outdir, env, personal=False):
    kind, _, rest = spec.partition(":")
    if kind == "pi":
        model, _, level = rest.partition(":")  # pi:<model>[:<thinking level>]
        subprocess.run(["pill", "stop"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)  # cold start
        env["PI_CODING_AGENT_DIR"] = str(pi_agent_dir(outdir, model, reasoning=bool(level), personal=personal))
        env["PI_SKIP_VERSION_CHECK"] = "1"
        # Bare: no extensions, skills or context files, offline. Personal: the copied setup as the
        # user runs it day to day, online; --no-session stays so no session is written.
        isolation = [] if personal else ["--offline", "--no-extensions", "--no-skills", "--no-context-files"]
        return ["pi", "-p", "--mode", "json", "--no-session", *isolation, "--model", "pill/" + model,
                "--thinking", level or "off", prompt]
    if kind == "claude":
        model, _, effort = rest.partition(":")
        cmd = ["claude", "-p", "--model", model, "--setting-sources", "project",
               "--strict-mcp-config", "--disable-slash-commands", "--no-session-persistence",
               "--dangerously-skip-permissions", "--output-format", "stream-json", "--verbose"]
        if effort:
            cmd += ["--effort", effort]
        return cmd + [prompt]
    if kind == "codex":
        model, _, effort = rest.partition(":")
        cmd = ["codex", "exec", "-m", model, "--dangerously-bypass-approvals-and-sandbox",
               "--skip-git-repo-check", "--ephemeral", "--json", "-C", str(ws)]
        if effort:
            cmd += ["-c", f"model_reasoning_effort={effort}"]
        return cmd + [prompt]
    sys.exit(f"unknown agent kind {kind!r}")


def verify(task, ws, outpath):
    r = subprocess.run([sys.executable, str(task["dir"] / task.get("verify", "verify.py")), str(ws)],
                       env=task_env(task), capture_output=True, text=True,
                       timeout=task.get("verify_timeout_s", 900))
    outpath.with_suffix(".stderr.log").write_text(r.stderr)
    line = next((ln for ln in reversed(r.stdout.strip().splitlines()) if ln.startswith("{")), None)
    if r.returncode != 0 or line is None:
        res = {"score": 0, "total": 100, "checks": [], "error": (r.stderr or r.stdout)[-2000:]}
    else:
        res = json.loads(line)
    outpath.write_text(json.dumps(res, indent=1))
    return res


def keep_workspace(ws, dest):
    skip = shutil.ignore_patterns("node_modules", ".venv", "__pycache__", ".git", "dist")
    shutil.copytree(ws, dest, symlinks=True, ignore=skip, dirs_exist_ok=True)


def cmd_list(_):
    for t in all_tasks():
        meta = load_task(t)
        print(f"{t:22} {meta.get('title', '')}")


def cmd_selftest(a):
    task = load_task(a.task)
    ensure_setup(task)
    base = tempfile.mkdtemp(prefix="eval-selftest-")
    ok = True
    for label, overlay, want in (("reference", True, lambda s: s >= 0.95), ("blank", False, lambda s: s <= 0.5)):
        ws = make_workspace(task, base, overlay_reference=overlay)
        res = verify(task, ws, Path(base) / f"{label}.json")
        frac = res["score"] / max(res["total"], 1)
        good = want(frac)
        ok &= good
        print(f"[selftest] {a.task} {label}: {res['score']}/{res['total']} -> {'ok' if good else 'UNEXPECTED'}")
        if not good or a.verbose:
            for c in res.get("checks", []):
                print(f"    {'PASS' if c['pass'] else 'FAIL'} {c['name']} {c.get('detail', '')}")
            if res.get("error"):
                print(res["error"])
    shutil.rmtree(base, ignore_errors=True)
    sys.exit(0 if ok else 1)


def cmd_run(a):
    task = load_task(a.task)
    if a.personal and not a.agent.startswith("pi:"):
        sys.exit("--personal applies to pi: agents only")
    results = personal_results_dir() if a.personal else RESULTS
    if a.personal:
        guard_personal_results(results)
    ensure_setup(task)
    slug = a.agent.replace(":", "_").replace("/", "_")
    stamp = datetime.datetime.now().strftime("%Y%m%dT%H%M%S")
    rundir = results / f"{stamp}-{task['id']}-{slug}"
    rundir.mkdir(parents=True, exist_ok=True)
    timeout_min = a.timeout_min or task.get("timeout_minutes", 30)
    prompt = PREAMBLE + (task["dir"] / task.get("prompt_file", "spec.md")).read_text()
    for i in range(1, a.runs + 1):
        out = rundir / f"run-{i}"
        out.mkdir()
        tmp = tempfile.mkdtemp(prefix="eval-run-")
        ws = make_workspace(task, tmp)
        env = task_env(task)
        cmd = agent_command(a.agent, prompt, ws, out, env, personal=a.personal)
        print(f"[run] {task['id']} {a.agent} run {i}/{a.runs} (timeout {timeout_min} min)", flush=True)
        sampler = MemorySampler().start() if a.agent.startswith("pi:") else None
        info = run_cmd(cmd, ws, env, timeout_min * 60, out / "transcript.jsonl", out / "agent.stderr.log")
        if sampler:
            info["memory"] = sampler.stop()
        res = verify(task, ws, out / "verify.json")
        keep_workspace(ws, out / "workspace")
        rec = {"task": task["id"], "agent": a.agent, "run": i, "rundir": str(out),
               "score": res["score"], "total": res["total"], "when": stamp, **info}
        if a.personal:
            rec[PERSONAL_KEY] = True
            rec["setup_hash"] = setup_hash(out / "pi-agent")
            shutil.rmtree(out / "pi-agent", ignore_errors=True)  # the copy is large; the hash records it
        (out / "result.json").write_text(json.dumps(rec, indent=1))
        with open(results / "index.jsonl", "a") as f:
            f.write(json.dumps(rec) + "\n")
        print(f"[done] score {res['score']}/{res['total']} in {info['seconds']}s"
              f"{' (TIMED OUT)' if info['timed_out'] else ''}", flush=True)
        shutil.rmtree(tmp, ignore_errors=True)


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = ap.add_subparsers(dest="cmd", required=True)
    sub.add_parser("list").set_defaults(fn=cmd_list)
    s = sub.add_parser("selftest")
    s.add_argument("task")
    s.add_argument("-v", "--verbose", action="store_true")
    s.set_defaults(fn=cmd_selftest)
    r = sub.add_parser("run")
    r.add_argument("task")
    r.add_argument("--agent", required=True)
    r.add_argument("--runs", type=int, default=1)
    r.add_argument("--timeout-min", type=float, default=0)
    r.add_argument("--personal", action="store_true",
                   help="pi: agents only: run with a copy of your own ~/.pi/agent; results go to "
                        "$PILL_HOME/personal/evals and are never committed")
    r.set_defaults(fn=cmd_run)
    a = ap.parse_args()
    a.fn(a)


if __name__ == "__main__":
    main()
