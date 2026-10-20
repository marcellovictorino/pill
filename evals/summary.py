#!/usr/bin/env python3
"""Summarise results/index.jsonl as markdown: one row per task x agent."""
import json
import statistics
import sys
from collections import defaultdict
from pathlib import Path

ROOT = Path(__file__).resolve().parent
idx = ROOT / "results" / "index.jsonl"
if not idx.exists():
    sys.exit("no results yet")
rows = [json.loads(line) for line in idx.read_text().splitlines() if line.strip()]
groups = defaultdict(list)
for r in rows:
    groups[(r["task"], r["agent"])].append(r)
tasks = sorted({k[0] for k in groups})
agents = sorted({k[1] for k in groups})

print("| task | agent | runs | mean score % | min | max | timeouts | mean minutes | min free % | swap growth MB |")
print("|---|---|---|---|---|---|---|---|---|---|")
for t in tasks:
    for a in agents:
        g = groups.get((t, a))
        if not g:
            continue
        pct = [100 * r["score"] / max(r["total"], 1) for r in g]
        mins = statistics.mean(r["seconds"] for r in g) / 60
        mem = [r["memory"] for r in g if r.get("memory", {}).get("min_free_pct") is not None]
        free = f"{min(m['min_free_pct'] for m in mem)}" if mem else ""
        swap = f"{max(m['swap_growth_mb'] for m in mem)}" if mem else ""  # worst run, local agents only
        print(f"| {t} | {a} | {len(g)} | {statistics.mean(pct):.0f} | {min(pct):.0f} | {max(pct):.0f} "
              f"| {sum(r['timed_out'] for r in g)} | {mins:.1f} | {free} | {swap} |")
