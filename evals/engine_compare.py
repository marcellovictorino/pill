#!/usr/bin/env python3
"""One-off engine comparison: mlx-lm against the llama.cpp champion, on pill's own Tier 1 and Tier 2.

Throwaway dev tooling under evals/, not part of pill. pill bench cannot target mlx-lm (it unloads
models through the llama-server router), so this script does the same steps by hand, one engine at
a time:

  1. refuse to start while any other local model is loaded (the pill router, another run, PILL-59)
  2. start the one engine it was asked for, on a free port, with the catalog's sampling settings
  3. run Tier 1 (agent-sized request, decode speed, Pi tool-calling turn) and Tier 2 (Pi builds the
     tic-tac-toe task, then the hidden verify.mjs) with the same gates as internal/bench/bench.go
  4. stop only the process it started, and write result.json

It never calls `pill stop` and never sends anything to the pill router on port 11435.

  python3 engine_compare.py mlx  --out DIR [--runs 1]
  python3 engine_compare.py gguf --out DIR [--runs 1] [--model-path PATH]
  python3 engine_compare.py mlx  --out DIR --check    # guard and prompt only, starts nothing
"""
import argparse
import json
import os
import signal
import subprocess
import sys
import time
import urllib.request
from datetime import datetime, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parent
REPO = ROOT.parent
sys.path.insert(0, str(ROOT))
from run import MemorySampler  # the free-memory sampler evals/run.py already uses

# The Tier 2 task and its verifier, exactly as pill embeds them into its binary.
TASKS_DIR = REPO / "internal" / "bench" / "tasks"

MLX_BIN = "/opt/homebrew/opt/mlx-lm/bin/mlx_lm.server"
LLAMA_BIN = "/opt/homebrew/bin/llama-server"
PI_BIN = "/opt/homebrew/bin/pi"
NODE_BIN = "node"
ROUTER_PORT = 11435  # pill's router: read-only status check only
DEFAULT_GGUF = Path.home() / ".pill" / "models" / "gemma-4-26B-A4B-it-UD-IQ4_XS.gguf"

# Gates from internal/bench/bench.go (DefaultThresholds) and the Tier 2 cap.
TIER1_MAX_SECONDS = 60
MIN_FREE_PCT = 10
TIER2_MAX_MINUTES = 15
DECODE_TOKENS = 128
PROMPT_FUNCTIONS = 130  # internal/bench/bench.go: PromptFunctions

# gemma4-26b settings from internal/catalog/catalog.toml and pill's models.ini header.
CTX = 65536
MAX_TOKENS = 16384
SAMPLING = {"temp": "1.0", "top-p": "0.95", "top-k": "64", "min-p": "0.0"}

ENGINES = {
    "mlx": {"model_id": "mlx-community/gemma-4-26b-a4b-it-4bit", "port": 8093},
    "gguf": {"model_id": "gemma4-26b-ud-iq4_xs", "port": 8094},
}


def engine_cmd(engine, port, gguf_path):
    """The command line that starts one engine by hand, mirroring pill's settings for it."""
    if engine == "mlx":
        return [MLX_BIN, "--model", ENGINES["mlx"]["model_id"], "--host", "127.0.0.1", "--port", str(port),
                "--temp", SAMPLING["temp"], "--top-p", SAMPLING["top-p"], "--top-k", SAMPLING["top-k"],
                "--min-p", SAMPLING["min-p"], "--max-tokens", str(MAX_TOKENS), "--prompt-cache-size", "1"]
    return [LLAMA_BIN, "--host", "127.0.0.1", "--port", str(port), "--jinja", "--log-colors", "off",
            "--n-gpu-layers", "99", "--flash-attn", "on", "--cache-type-k", "q8_0", "--cache-type-v", "q8_0",
            "--parallel", "1", "--cache-ram", "512", "--ctx-checkpoints", "4", "--ctx-size", str(CTX),
            "--temp", SAMPLING["temp"], "--top-p", SAMPLING["top-p"], "--top-k", SAMPLING["top-k"],
            "--min-p", SAMPLING["min-p"], "--alias", ENGINES["gguf"]["model_id"], "--model", str(gguf_path)]


def other_local_models():
    """Anything holding a model right now that this script did not start. Empty means safe to start."""
    found = []
    routers = set(subprocess.run(["pgrep", "-f", "llama-server --models-dir"], capture_output=True,
                                 text=True).stdout.split())
    out = subprocess.run(["pgrep", "-fl", "llama-server|mlx_lm"], capture_output=True, text=True).stdout
    for line in out.splitlines():
        pid = line.split(maxsplit=1)[0]
        if pid in routers:  # pill's router itself: judged by its /models status below
            continue
        ppid = subprocess.run(["ps", "-o", "ppid=", "-p", pid], capture_output=True, text=True).stdout.strip()
        if ppid in routers:  # a model server the router started: loaded or asleep, so the router's status decides
            continue
        found.append(line.strip())
    try:
        with urllib.request.urlopen(f"http://127.0.0.1:{ROUTER_PORT}/models", timeout=5) as r:
            for m in json.load(r).get("data", []):
                if m.get("status", {}).get("value") == "loaded":
                    found.append(f"pill router has {m['id']} loaded")
    except Exception:
        pass  # router not answering: nothing is loaded through it
    return found


def engine_version(engine):
    if engine == "mlx":
        out = subprocess.run(["brew", "list", "--versions", "mlx-lm"], capture_output=True, text=True).stdout
        return out.strip()
    r = subprocess.run([LLAMA_BIN, "--version"], capture_output=True, text=True)
    lines = [l for l in (r.stdout + r.stderr).splitlines() if l.strip()]
    return " | ".join(lines[:2])


def agent_prompt(functions):
    """Port of internal/bench/tier1.go agentPrompt: the same deterministic prompt and answer."""
    parts = ["Below is part of a code base. Read it, then answer the question at the end.\n\n"]
    target = functions * 6 // 10
    want = None
    for i in range(functions):
        a, c = 3 + (i * 7919) % 89, 11 + (i * 104729) % 97
        parts.append(f"// file: pkg/calc/module_{i:04d}.go\n")
        parts.append(f"// Compute{i:04d} applies the linear rule used by pipeline stage {i % 13}.\n")
        parts.append(f"func Compute{i:04d}(x int) int {{\n\treturn x*{a} + {c}\n}}\n\n")
        if i == target:
            want = 2 * a + c
    parts.append(f"Question: what does Compute{target:04d}(2) return? Reply with just the number.\n")
    return "".join(parts), want


class Engine:
    """One engine process this script started. Stop only this process group."""

    def __init__(self, cmd, log_path, port):
        self.log = open(log_path, "wb")
        self.proc = subprocess.Popen(cmd, stdout=self.log, stderr=subprocess.STDOUT,
                                     stdin=subprocess.DEVNULL, start_new_session=True)
        self.base = f"http://127.0.0.1:{port}"

    def wait_ready(self, deadline):
        while time.time() < deadline:
            if self.proc.poll() is not None:
                raise RuntimeError(f"engine exited early with code {self.proc.returncode}")
            for path in ("/health", "/v1/models"):
                try:
                    with urllib.request.urlopen(self.base + path, timeout=3) as r:
                        if r.status == 200:
                            return
                except Exception:
                    pass
            time.sleep(1)
        raise RuntimeError("engine not ready before the deadline")

    def stop(self):
        if self.proc.poll() is None:
            os.killpg(self.proc.pid, signal.SIGTERM)
            try:
                self.proc.wait(timeout=30)
            except subprocess.TimeoutExpired:
                os.killpg(self.proc.pid, signal.SIGKILL)
                self.proc.wait()
        self.log.close()
        return self.proc.returncode


def post_json(url, payload, timeout):
    req = urllib.request.Request(url, data=json.dumps(payload).encode(),
                                 headers={"Content-Type": "application/json"})
    with urllib.request.urlopen(req, timeout=timeout) as r:
        return json.load(r)


def chat(eng, model_id, content, max_tokens, timeout, extra=None):
    payload = {"model": model_id, "messages": [{"role": "user", "content": content}],
               "max_tokens": max_tokens, "stream": False,
               "chat_template_kwargs": {"enable_thinking": False}}
    payload.update(extra or {})
    start = time.time()
    try:
        body = post_json(eng.base + "/v1/chat/completions", payload, timeout)
        secs = time.time() - start
        choices = body.get("choices") or []
        if not choices:
            return {"ok": False, "seconds": secs, "error": "unreadable response"}
        usage = body.get("usage", {})
        return {"ok": True, "seconds": secs, "reply": (choices[0]["message"].get("content") or "").strip(),
                "prompt_tokens": usage.get("prompt_tokens", 0), "gen_tokens": usage.get("completion_tokens", 0),
                "engine_tok_per_sec": (body.get("timings") or {}).get("predicted_per_second")}
    except Exception as e:  # HTTP errors, timeouts, refused connections
        return {"ok": False, "seconds": time.time() - start, "error": str(e)[:300]}


def write_agent_dir(d, base_url, model_id):
    """Same provider block internal/pi/models.go writes for pill, in a scratch Pi config directory."""
    d.mkdir(parents=True, exist_ok=True)
    doc = {"providers": {"pill": {
        "api": "openai-completions", "apiKey": "local", "baseUrl": base_url + "/v1",
        "compat": {"supportsStore": False, "supportsDeveloperRole": False, "supportsReasoningEffort": False,
                   "supportsUsageInStreaming": True, "supportsStrictMode": False,
                   "maxTokensField": "max_tokens", "thinkingFormat": "qwen-chat-template"},
        "models": [{"id": model_id, "name": model_id, "reasoning": True, "input": ["text"],
                    "contextWindow": CTX, "maxTokens": MAX_TOKENS}]}}}
    (d / "models.json").write_text(json.dumps(doc, indent=2) + "\n")


def run_pi(agent_dir, workspace, model_id, prompt, timeout_s, transcript, stderr_log):
    """Port of internal/bench/pirun.go: one non-interactive Pi turn, killed as a group on timeout."""
    workspace.mkdir(parents=True, exist_ok=True)
    env = dict(os.environ, PI_CODING_AGENT_DIR=str(agent_dir), PI_SKIP_VERSION_CHECK="1")
    cmd = [PI_BIN, "-p", "--mode", "json", "--no-session", "--offline", "--no-extensions",
           "--no-skills", "--no-context-files", "--model", "pill/" + model_id, "--thinking", "off", prompt]
    with open(transcript, "wb") as out, open(stderr_log, "wb") as err:
        p = subprocess.Popen(cmd, cwd=workspace, env=env, stdin=subprocess.DEVNULL, stdout=out,
                             stderr=err, start_new_session=True)
        timed_out = False
        try:
            p.wait(timeout=timeout_s)
        except subprocess.TimeoutExpired:
            timed_out = True
            os.killpg(p.pid, signal.SIGKILL)
            p.wait()
    return {"timed_out": timed_out, "exit": p.returncode, "seconds": None}


def parse_pi(path):
    """Port of parsePiOutput: final assistant text, tool-call count, longest identical-call run."""
    answer, tool_calls, repeats, run_len, last_call = "", 0, 0, 0, None
    for line in Path(path).read_text(errors="replace").splitlines():
        try:
            e = json.loads(line)
        except ValueError:
            continue
        if e.get("type") == "tool_execution_start":
            tool_calls += 1
            call = str(e.get("toolName")) + json.dumps(e.get("args"), sort_keys=True)
            run_len = run_len + 1 if call == last_call else 1
            last_call = call
            repeats = max(repeats, run_len)
        elif e.get("type") == "message_end" and (e.get("message") or {}).get("role") == "assistant":
            content = e["message"].get("content")
            if isinstance(content, str):
                text = content
            else:
                text = "".join(p.get("text", "") for p in (content or []) if p.get("type") == "text")
            if text:
                answer = text
    return answer.strip(), tool_calls, repeats


def run_once(engine_name, n, out_dir, gguf_path):
    cfg = ENGINES[engine_name]
    d = out_dir / f"run-{n}"
    d.mkdir(parents=True, exist_ok=True)
    cmd = engine_cmd(engine_name, cfg["port"], gguf_path)
    (d / "engine-command.txt").write_text(" ".join(cmd) + "\n")
    rr = {"run": n, "engine": engine_name, "model": cfg["model_id"]}

    sampler = MemorySampler(interval=5.0).start()
    t_launch = time.time()  # pill's first request also pays for the model load, so the clock starts here
    eng = Engine(cmd, d / "engine.log", cfg["port"])
    try:
        write_agent_dir(d / "pi-agent", eng.base, cfg["model_id"])
        eng.wait_ready(deadline=t_launch + 3 * TIER1_MAX_SECONDS)
        rr["ready_seconds"] = round(time.time() - t_launch, 1)

        # --- Tier 1a: agent-sized request, cold start included ---
        prompt, want = agent_prompt(PROMPT_FUNCTIONS)
        remaining = 3 * TIER1_MAX_SECONDS - (time.time() - t_launch)
        chat1 = chat(eng, cfg["model_id"], prompt, 64, timeout=max(remaining, 1))
        rr["tier1_seconds"] = round(time.time() - t_launch, 1)
        rr["tier1_prompt_tokens"] = chat1.get("prompt_tokens")
        rr["tier1_reply"] = chat1.get("reply", "")[:80]
        rr["tier1_correct_informational"] = str(want) in chat1.get("reply", "")
        req_ok = chat1["ok"] and rr["tier1_seconds"] <= TIER1_MAX_SECONDS
        rr["tier1_request_ok"] = req_ok
        rr["tier1_request_detail"] = chat1.get("error") or f"{rr['tier1_seconds']}s (limit {TIER1_MAX_SECONDS}s)"

        # --- Decode speed on the warm model (informational, as in pill) ---
        if chat1["ok"]:
            dec = chat(eng, cfg["model_id"], "Count upwards from 1, one number per line.", DECODE_TOKENS,
                       timeout=TIER1_MAX_SECONDS, extra={"ignore_eos": True})
            if dec["ok"] and dec["gen_tokens"]:
                rr["decode_tok_per_sec_wall"] = round(dec["gen_tokens"] / dec["seconds"], 1)
                rr["decode_tokens"] = dec["gen_tokens"]
                rr["decode_tok_per_sec_engine"] = dec.get("engine_tok_per_sec")
            else:
                rr["decode_error"] = dec.get("error", "no tokens")

        # --- Tier 1b: a real Pi turn that needs a tool call ---
        token = "PILL-CMP-" + os.urandom(3).hex().upper()
        ws1 = d / "tier1-workspace"
        ws1.mkdir(parents=True, exist_ok=True)
        (ws1 / "facts.txt").write_text(token + "\n")
        pi1 = run_pi(d / "pi-agent", ws1, cfg["model_id"],
                     "Read the file facts.txt with your tools and reply with only the exact code it contains.",
                     600, d / "tier1-pi.jsonl", d / "tier1-pi.stderr.log")
        answer1, calls1, _ = parse_pi(d / "tier1-pi.jsonl")
        tool_ok = (not pi1["timed_out"]) and pi1["exit"] == 0 and calls1 >= 1 and token in answer1
        rr["tier1_tool_turn"] = {"ok": tool_ok, "tool_calls": calls1, "timed_out": pi1["timed_out"],
                                 "exit": pi1["exit"], "answer": answer1[:60]}
        n_free_t1 = len(sampler.free)
        min_free_t1 = min(sampler.free[:n_free_t1], default=None)
        mem_t1 = min_free_t1 is not None and min_free_t1 >= MIN_FREE_PCT
        rr["tier1_min_free_pct"] = min_free_t1
        tier1_pass = req_ok and tool_ok and mem_t1
        rr["tier1_pass"] = tier1_pass

        # --- Tier 2: Pi builds the task, then the hidden verifier runs ---
        if tier1_pass:
            task = (TASKS_DIR / "tictactoe.md").read_text()
            ws2 = d / "tier2-workspace"
            t2_start = time.time()
            pi2 = run_pi(d / "pi-agent", ws2, cfg["model_id"], task, TIER2_MAX_MINUTES * 60,
                         d / "tier2-pi.jsonl", d / "tier2-pi.stderr.log")
            rr["tier2_seconds"] = round(time.time() - t2_start, 1)
            rr["tier2_timed_out"] = pi2["timed_out"]
            _, calls2, repeats2 = parse_pi(d / "tier2-pi.jsonl")
            rr["tier2_tool_calls"], rr["tier2_repeats"] = calls2, repeats2
            ws2.mkdir(parents=True, exist_ok=True)
            (ws2 / "verify.mjs").write_bytes((TASKS_DIR / "verify.mjs").read_bytes())
            v = subprocess.run([NODE_BIN, "verify.mjs"], cwd=ws2, capture_output=True, text=True, timeout=120)
            lines = [l for l in v.stdout.splitlines() if l.strip()]
            try:
                report = json.loads(lines[-1])
                rr["tier2_score"], rr["tier2_total"] = report["score"], report["total"]
                rr["tier2_checks"] = report["checks"]
                (d / "tier2-verify.json").write_text(lines[-1] + "\n")
            except (ValueError, KeyError, IndexError):
                rr["tier2_score"], rr["tier2_total"] = 0, 0
                rr["tier2_checks"] = [{"name": "verifier", "pass": False, "detail": "no result"}]
        else:
            rr["tier2_skipped"] = True
    except Exception as e:  # engine never ready, or a harness error: record it, stop cleanly
        rr["error"] = str(e)[:300]
    finally:
        rr["engine_exit"] = eng.stop()
        stats = sampler.stop()
        rr["min_free_pct_whole_run"] = stats["min_free_pct"]
        rr["swap_growth_mb"] = stats["swap_growth_mb"]
        rr["max_pressure_level"] = stats["max_pressure_level"]
        rr["passed"], rr["reason"] = verdict(rr, stats)
    return rr


def verdict(rr, stats):
    """Port of bench.go verdict(): Tier 1 gate, then Tier 2 cap and full score, then memory over the run."""
    if "error" in rr:
        return False, "error: " + rr["error"]
    if not rr.get("tier1_pass"):
        if not rr["tier1_request_ok"]:
            return False, "tier 1: agent-sized request: " + rr["tier1_request_detail"]
        if not rr["tier1_tool_turn"]["ok"]:
            return False, "tier 1: Pi tool-calling turn failed"
        return False, f"tier 1: free memory dipped to {rr['tier1_min_free_pct']}% (limit {MIN_FREE_PCT}%)"
    if rr.get("tier2_timed_out") and rr.get("tier2_repeats", 0) >= 10:
        return False, f"tier 2 hit the {TIER2_MAX_MINUTES}-minute cap, repeating a tool call {rr['tier2_repeats']} times"
    if rr.get("tier2_timed_out"):
        return False, f"tier 2 hit the {TIER2_MAX_MINUTES}-minute cap (score {rr.get('tier2_score')}/{rr.get('tier2_total')})"
    if not rr.get("tier2_total") or rr.get("tier2_score") != rr.get("tier2_total"):
        return False, f"tier 2 score {rr.get('tier2_score')}/{rr.get('tier2_total')}"
    if stats["min_free_pct"] is None or stats["min_free_pct"] < MIN_FREE_PCT:
        return False, f"free memory dipped to {stats['min_free_pct']}% over the run (limit {MIN_FREE_PCT}%)"
    return True, ""


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("engine", choices=sorted(ENGINES))
    ap.add_argument("--out", required=True, type=Path, help="result folder (outside the repository)")
    ap.add_argument("--runs", type=int, default=1, help="cold runs; each one starts the engine afresh")
    ap.add_argument("--model-path", type=Path, default=DEFAULT_GGUF, help="GGUF file (gguf engine only)")
    ap.add_argument("--check", action="store_true", help="run the guard and print the plan; start nothing")
    args = ap.parse_args()

    if args.engine == "gguf" and not args.model_path.exists():
        sys.exit(f"missing GGUF file: {args.model_path}")
    busy = other_local_models()
    if args.check:
        print(json.dumps({"engine": args.engine, "other_local_models": busy,
                          "engine_command": engine_cmd(args.engine, ENGINES[args.engine]["port"], args.model_path),
                          "tier1_prompt_chars": len(agent_prompt(PROMPT_FUNCTIONS)[0]),
                          "tier2_task_chars": len((TASKS_DIR / "tictactoe.md").read_text())}, indent=2))
        return
    if busy:
        sys.exit("refusing to start: another local model is loaded or running:\n  " + "\n  ".join(busy))

    args.out.mkdir(parents=True, exist_ok=True)
    meta = {"engine": args.engine, "model": ENGINES[args.engine]["model_id"], "engine_version": engine_version(args.engine),
            "started": datetime.now(timezone.utc).isoformat(), "gates": {"tier1_max_seconds": TIER1_MAX_SECONDS,
            "min_free_pct": MIN_FREE_PCT, "tier2_max_minutes": TIER2_MAX_MINUTES},
            "settings": {"ctx": CTX, "max_tokens": MAX_TOKENS, "sampling": SAMPLING, "prompt_cache_size": 1}}
    runs = []
    for n in range(1, args.runs + 1):
        rr = run_once(args.engine, n, args.out, args.model_path)
        runs.append(rr)
        (args.out / "result.json").write_text(json.dumps({**meta, "runs": runs}, indent=2) + "\n")
        print(json.dumps({k: rr.get(k) for k in ("run", "passed", "reason", "ready_seconds", "tier1_seconds",
                                                 "decode_tok_per_sec_wall", "tier2_score", "tier2_total",
                                                 "tier2_seconds", "min_free_pct_whole_run")}), flush=True)


if __name__ == "__main__":
    main()
