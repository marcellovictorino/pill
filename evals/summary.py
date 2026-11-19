#!/usr/bin/env python3
"""Summarise results/index.jsonl as markdown: one row per task x agent.

  python3 summary.py             # the bare baseline, from evals/results
  python3 summary.py --personal  # bare and personal medians side by side; reads and writes nothing else

Personal results live under $PILL_HOME/personal/evals (never in the repository); see README.md.
"""
import argparse
import json
import os
import statistics
import sys
from collections import defaultdict
from pathlib import Path

ROOT = Path(__file__).resolve().parent


def load(idx):
    if not idx.exists():
        return []
    return [json.loads(line) for line in idx.read_text().splitlines() if line.strip()]


def percent(r):
    return 100 * r["score"] / max(r["total"], 1)


def verdict(bare, personal):
    """Personal is noise unless its median falls outside the range the bare runs themselves span."""
    m = statistics.median(personal)
    if m > max(bare):
        return "setup helps"
    if m < min(bare):
        return "setup hurts"
    return "noise"


def side_by_side(bare_rows, personal_rows):
    def group(rows):
        g = defaultdict(list)
        for r in rows:
            g[(r["task"], r["agent"])].append(percent(r))
        return g
    bare, personal = group(bare_rows), group(personal_rows)
    out = ["| task | agent | bare median % (range, runs) | personal median % (range, runs) | verdict |",
           "|---|---|---|---|---|"]
    for key in sorted(personal):
        p = personal[key]
        b = bare.get(key)
        bare_cell = f"{statistics.median(b):.0f} ({min(b):.0f}-{max(b):.0f}, {len(b)})" if b else "none"
        verdict_cell = verdict(b, p) if b else "no bare runs to compare"
        out.append(f"| {key[0]} | {key[1]} | {bare_cell} "
                   f"| {statistics.median(p):.0f} ({min(p):.0f}-{max(p):.0f}, {len(p)}) | {verdict_cell} |")
    return "\n".join(out)


def bare_table(rows):
    groups = defaultdict(list)
    for r in rows:
        groups[(r["task"], r["agent"])].append(r)
    lines = ["| task | agent | runs | mean score % | min | max | timeouts | mean minutes | min free % | swap growth MB |",
             "|---|---|---|---|---|---|---|---|---|---|"]
    for t in sorted({k[0] for k in groups}):
        for a in sorted({k[1] for k in groups}):
            g = groups.get((t, a))
            if not g:
                continue
            pct = [percent(r) for r in g]
            mins = statistics.mean(r["seconds"] for r in g) / 60
            mem = [r["memory"] for r in g if r.get("memory", {}).get("min_free_pct") is not None]
            free = f"{min(m['min_free_pct'] for m in mem)}" if mem else ""
            swap = f"{max(m['swap_growth_mb'] for m in mem)}" if mem else ""  # worst run, local agents only
            lines.append(f"| {t} | {a} | {len(g)} | {statistics.mean(pct):.0f} | {min(pct):.0f} | {max(pct):.0f} "
                         f"| {sum(r['timed_out'] for r in g)} | {mins:.1f} | {free} | {swap} |")
    return "\n".join(lines)


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--personal", action="store_true", help="compare bare and personal runs")
    a = ap.parse_args()
    bare = load(ROOT / "results" / "index.jsonl")
    if not a.personal:
        if not bare:
            sys.exit("no results yet")
        print(bare_table(bare))
        return
    home = Path(os.environ.get("PILL_HOME") or Path.home() / ".pill")
    personal = load(home / "personal" / "evals" / "index.jsonl")
    if not personal:
        sys.exit("no personal results yet; run: python3 evals/run.py run <task> --agent pi:<model> --personal")
    print(side_by_side(bare, personal))


if __name__ == "__main__":
    main()
