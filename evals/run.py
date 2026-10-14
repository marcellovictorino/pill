#!/usr/bin/env python3
"""evals runner: give a task to an agent in a throwaway workspace, then score it.

Standard library only. See README.md for the task layout and agent specs.

  python3 run.py list
  python3 run.py selftest <task>                     # reference must score ~100, blank fixture low
  python3 run.py run <task> --agent <spec> [--runs N] [--timeout-min M]

Agent specs:
  pi:<pill-model-name>             local model through the pill router, driven by Pi
  claude:<model>[:<effort>]        Claude Code CLI, e.g. claude:opus:medium
  codex:<model>[:<effort>]         Codex CLI, e.g. codex:gpt-5.6-luna:high
"""
import argparse
import datetime
import json
import os
import shutil
import signal
import subprocess
import sys
import tempfile
import time
from pathlib import Path

ROOT = Path(__file__).resolve().parent
TASKS = ROOT / "tasks"
RESULTS = ROOT / "results"

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


def pi_agent_dir(outdir, model):
    """Throwaway Pi config: a copy of the user's models.json (the pill provider).

    pill drops a model from Pi's config when its benchmark failed, though the router still serves
    it. Evals compare such models too, so the entry is cloned into this throwaway copy only; the
    user's real Pi config is never touched.
    """
    d = Path(outdir) / "pi-agent"
    d.mkdir(parents=True, exist_ok=True)
    src = Path.home() / ".pi" / "agent"
    for f in ("models.json", "models-store.json", "auth.json"):
        if (src / f).exists():
            shutil.copy(src / f, d / f)
    cfg = json.loads((d / "models.json").read_text())
    models = cfg["providers"]["pill"]["models"]
    if not any(m["id"] == model for m in models):
        entry = dict(models[0])
        entry["id"] = entry["name"] = model
        models.append(entry)
        (d / "models.json").write_text(json.dumps(cfg, indent=2))
    return d


def agent_command(spec, prompt, ws, outdir, env):
    kind, _, rest = spec.partition(":")
    if kind == "pi":
        subprocess.run(["pill", "stop"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)  # cold start
        env["PI_CODING_AGENT_DIR"] = str(pi_agent_dir(outdir, rest))
        env["PI_SKIP_VERSION_CHECK"] = "1"
        return ["pi", "-p", "--mode", "json", "--no-session", "--offline", "--no-extensions",
                "--no-skills", "--no-context-files", "--model", "pill/" + rest,
                "--thinking", "off", prompt]
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
    ensure_setup(task)
    slug = a.agent.replace(":", "_").replace("/", "_")
    stamp = datetime.datetime.now().strftime("%Y%m%dT%H%M%S")
    rundir = RESULTS / f"{stamp}-{task['id']}-{slug}"
    rundir.mkdir(parents=True, exist_ok=True)
    timeout_min = a.timeout_min or task.get("timeout_minutes", 30)
    prompt = PREAMBLE + (task["dir"] / task.get("prompt_file", "spec.md")).read_text()
    for i in range(1, a.runs + 1):
        out = rundir / f"run-{i}"
        out.mkdir()
        tmp = tempfile.mkdtemp(prefix="eval-run-")
        ws = make_workspace(task, tmp)
        env = task_env(task)
        cmd = agent_command(a.agent, prompt, ws, out, env)
        print(f"[run] {task['id']} {a.agent} run {i}/{a.runs} (timeout {timeout_min} min)", flush=True)
        info = run_cmd(cmd, ws, env, timeout_min * 60, out / "transcript.jsonl", out / "agent.stderr.log")
        res = verify(task, ws, out / "verify.json")
        keep_workspace(ws, out / "workspace")
        rec = {"task": task["id"], "agent": a.agent, "run": i, "rundir": str(out),
               "score": res["score"], "total": res["total"], "when": stamp, **info}
        (out / "result.json").write_text(json.dumps(rec, indent=1))
        with open(RESULTS / "index.jsonl", "a") as f:
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
    r.set_defaults(fn=cmd_run)
    a = ap.parse_args()
    a.fn(a)


if __name__ == "__main__":
    main()
