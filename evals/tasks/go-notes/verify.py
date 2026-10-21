#!/usr/bin/env python3
"""Hidden verifier for go-notes. Usage: verify.py <workspace>. Last stdout line is the result JSON.

Weights (total 100): build 5, vet 5, gofmt 5, go test 10, behaviour 60, agent-written tests 8, conventions 7.
Behaviour is checked against the built binary with NOTES_HOME in a temp dir and NOTES_NOW fixing the clock.
"""
import json
import os
import re
import subprocess
import sys
import tempfile
from pathlib import Path

HERE = Path(__file__).resolve().parent
FIXTURE = HERE / "fixture"
ws = Path(sys.argv[1]).resolve()

ENV = dict(os.environ, GOFLAGS="-mod=vendor", GOPROXY="off", GOTOOLCHAIN="local")
checks = []


def check(name, weight, fn):
    try:
        ok, detail = fn()
    except Exception as e:  # a crashing check is a failed check, never a crashed verifier
        ok, detail = False, f"exception: {e}"
    checks.append({"name": name, "pass": bool(ok), "detail": "" if ok else str(detail)[:400], "weight": weight})


def sh(cmd, timeout=300, **kw):
    return subprocess.run(cmd, cwd=ws, env=ENV, capture_output=True, text=True, timeout=timeout, **kw)


# ---- build, vet, gofmt, tests ---------------------------------------------------------------
binary = Path(tempfile.mkdtemp(prefix="notes-bin-")) / "notes"


def build():
    r = sh(["go", "build", "./..."])
    if r.returncode != 0:
        return False, r.stderr
    r = sh(["go", "build", "-o", str(binary), "."])
    return r.returncode == 0, r.stderr


def vet():
    r = sh(["go", "vet", "./..."])
    return r.returncode == 0, r.stderr


def gofmt():
    files = [str(p.relative_to(ws)) for p in ws.rglob("*.go") if "vendor" not in p.relative_to(ws).parts]
    r = sh(["gofmt", "-l"] + files)
    return r.returncode == 0 and r.stdout.strip() == "", r.stdout + r.stderr


def gotest():
    r = sh(["go", "test", "./..."])
    return r.returncode == 0, r.stdout[-400:] + r.stderr[-400:]


check("go build", 5, build)
built = binary.exists()
check("go vet", 5, vet)
check("gofmt clean", 5, gofmt)
check("go test ./... passes", 10, gotest)


# ---- behaviour against the built binary --------------------------------------------------------
def nt(home, args, now="2026-01-02T03:04:05Z"):
    r = subprocess.run([str(binary)] + args, env=dict(os.environ, NOTES_HOME=str(home), NOTES_NOW=now),
                       capture_output=True, text=True, timeout=30)
    return r.returncode, r.stdout, r.stderr


def fresh():
    return Path(tempfile.mkdtemp(prefix="notes-home-"))


def beh(name, weight, fn):
    check(name, weight, (lambda: (False, "binary did not build")) if not built else fn)


def populate(home, items):
    for text, tags, now in items:
        args = ["add", text]
        for t in tags:
            args += ["--tag", t]
        code, _, err = nt(home, args, now)
        assert code == 0, f"add failed: {err}"


T1, T2, T3 = "2026-01-02T03:04:05Z", "2026-03-04T05:06:07Z", "2026-02-01T00:00:00Z"
BASIC = [("a", ["work"], T1), ("b", ["work", "home"], T2), ("c", [], T3)]


def stats_human():
    h = fresh()
    populate(h, BASIC)
    code, out, _ = nt(h, ["stats"])
    want = f"total: 3\noldest: {T1}\nnewest: {T2}\ntags:\n  work: 2\n  home: 1\n"
    return code == 0 and out == want, f"code {code} out {out!r}"


def stats_json():
    h = fresh()
    populate(h, BASIC)
    code, out, _ = nt(h, ["stats", "--format", "json"])
    got = json.loads(out)
    want = {"total": 3, "oldest": T1, "newest": T2, "tags": [{"tag": "work", "count": 2}, {"tag": "home", "count": 1}]}
    return code == 0 and got == want and out.count("\n") == 1 and out.strip() == out.strip().replace("\n", ""), f"{out!r}"


def stats_fold_sort():
    h = fresh()
    populate(h, [
        ("1", ["Work", "work"], T1),          # same tag twice: counts once
        ("2", ["WORK"], T1),
        ("3", ["Ärger"], T1),
        ("4", ["ärger", "zeta"], T1),
        ("5", ["Big Idea"], T1),
        ("6", ["big idea"], T1),
        ("7", ["alpha"], T1),
        ("8", ["beta"], T1),
    ])
    code, out, _ = nt(h, ["stats"])
    want = ("total: 8\noldest: %s\nnewest: %s\ntags:\n  big idea: 2\n  work: 2\n  ärger: 2\n"
            "  alpha: 1\n  beta: 1\n  zeta: 1\n") % (T1, T1)
    code2, out2, _ = nt(h, ["stats", "--format", "json"])
    tags = [(t["tag"], t["count"]) for t in json.loads(out2)["tags"]]
    wt = [("big idea", 2), ("work", 2), ("ärger", 2), ("alpha", 1), ("beta", 1), ("zeta", 1)]
    return code == 0 and out == want and tags == wt, f"{out!r} {tags}"


def stats_empty():
    h = fresh()
    c1, o1, _ = nt(h, ["stats"])
    c2, o2, _ = nt(h, ["stats", "--format", "json"])
    populate(h, [("x", [], T1)])
    c3, o3, _ = nt(h, ["stats"])
    c4, _, e4 = nt(h, ["stats", "extra"])
    ok = (c1 == 0 and o1 == "total: 0\noldest: -\nnewest: -\ntags: none\n"
          and c2 == 0 and json.loads(o2) == {"total": 0, "oldest": None, "newest": None, "tags": []}
          and c3 == 0 and o3 == f"total: 1\noldest: {T1}\nnewest: {T1}\ntags: none\n"
          and c4 == 2 and e4.startswith("error:"))
    return ok, f"{o1!r} {o2!r} {o3!r} {c4}"


def rm_basic():
    h = fresh()
    populate(h, [("a", [], T1), ("b", [], T1), ("c", [], T1)])
    c1, o1, _ = nt(h, ["rm", "2"])
    _, ls, _ = nt(h, ["ls"])
    c2, o2, _ = nt(h, ["rm", "1", "--format", "json"])
    j = json.loads(o2)
    ok = (c1 == 0 and o1 == "removed #2\n" and ls == "#1 a\n#3 c\n"
          and c2 == 0 and j == {"removed": [1], "missing": []})
    return ok, f"{o1!r} {ls!r} {o2!r}"


def rm_idempotent():
    h = fresh()
    populate(h, [("a", [], T1), ("b", [], T1)])
    nt(h, ["rm", "2"])
    c1, o1, e1 = nt(h, ["rm", "2"])                      # again: no-op, not an error
    c2, o2, _ = nt(h, ["rm", "1", "9", "1"])             # argument order, repeats
    c3, o3, _ = nt(h, ["rm", "7", "5", "--format", "json"])
    ok = (c1 == 0 and o1 == "not found #2\n" and e1 == ""
          and c2 == 0 and o2 == "removed #1\nnot found #9\nnot found #1\n"
          and c3 == 0 and json.loads(o3) == {"removed": [], "missing": [7, 5]})
    return ok, f"{o1!r} {o2!r} {o3!r}"


def rm_invalid():
    h = fresh()
    populate(h, [("a", [], T1), ("b", [], T1)])
    bad = []
    # rm must exist and work at all, otherwise "usage error" below is just an unknown command.
    if nt(h, ["rm", "2"])[:2] != (0, "removed #2\n"):
        return False, "rm does not remove a valid id"
    for args in (["rm"], ["rm", "abc"], ["rm", "0"], ["rm", "1.5"], ["rm", "1", "oops"]):
        code, out, err = nt(h, args)
        if code != 2 or out != "" or not err.startswith("error:"):
            bad.append((args, code, out, err))
    _, ls, _ = nt(h, ["ls"])
    if ls != "#1 a\n":
        bad.append(("note removed by a failed rm", ls))
    return not bad, f"{bad}"


def bug_fixed():
    h = fresh()
    populate(h, [("a", ["Work"], T1), ("b", ["Ärger"], T1), ("c", ["other"], T1)])
    r = [nt(h, ["ls", "--tag", t])[1] for t in ("work", "WORK", "Work", "ärger", "ÄRGER", "nope")]
    ok = r[0] == r[1] == r[2] == "#1 a [Work]\n" and r[3] == r[4] == "#2 b [Ärger]\n" and r[5] == "no notes\n"
    return ok, f"{r}"


def ids_not_reused():
    h = fresh()
    populate(h, [("a", [], T1), ("b", [], T1), ("c", [], T1)])
    if nt(h, ["rm", "3"])[0] != 0:
        return False, "rm does not remove a valid id"
    _, out, _ = nt(h, ["add", "d"])
    nt(h, ["rm", "1", "2", "4"])
    _, out2, _ = nt(h, ["add", "e"])
    return out == "added #4\n" and out2 == "added #5\n", f"{out!r} {out2!r}"


beh("stats: human output", 8, stats_human)
beh("stats: json output", 8, stats_json)
beh("stats: case folding, ordering, unicode, spaces in tags", 8, stats_fold_sort)
beh("stats: empty store, no tags, usage error", 6, stats_empty)
beh("rm: removes, json shape, other notes kept", 8, rm_basic)
beh("rm: idempotent, missing ids, argument order", 8, rm_idempotent)
beh("rm: invalid ids are usage errors and remove nothing", 4, rm_invalid)
beh("bug fixed: ls --tag case-insensitive (incl. non-ASCII)", 6, bug_fixed)
beh("ids are never reused after rm", 4, ids_not_reused)


# ---- tests the agent wrote ---------------------------------------------------------------------
def test_files(root):
    return {str(p.relative_to(root)): p.read_text() for p in root.rglob("*_test.go")
            if "vendor" not in p.relative_to(root).parts}


orig_tests = test_files(FIXTURE)
agent_tests = {k: v for k, v in test_files(ws).items() if orig_tests.get(k) != v}


def mentions(pattern):
    return any(re.search(r"func Test", v) and re.search(pattern, v, re.I) for v in agent_tests.values())


check("agent added tests for stats", 4, lambda: (mentions(r"stats"), f"changed test files: {sorted(agent_tests)}"))
check("agent added tests for rm", 4, lambda: (mentions(r"\brm\b"), f"changed test files: {sorted(agent_tests)}"))


# ---- repo conventions ----------------------------------------------------------------------------
def cmd_sources():
    d = ws / "cmd"
    return {p.name: p.read_text() for p in d.glob("*.go") if not p.name.endswith("_test.go")} if d.exists() else {}


def command_files():
    srcs = cmd_sources()
    has = {c: [n for n, s in srcs.items() if re.search(r'Use:\s*"%s\b' % c, s)] for c in ("stats", "rm")}
    if not (has["stats"] and has["rm"]):
        return False, f"commands not found in cmd/: {has}"
    if set(has["stats"]) & set(has["rm"]):
        return False, "stats and rm share a file"
    multi = [n for n, s in srcs.items() if len(re.findall(r"\bUse:", s)) > 1]
    return not multi, f"files defining several commands: {multi}"


def deps_untouched():
    def snap(root):
        files = {}
        for rel in ("go.mod", "go.sum", "vendor/modules.txt"):
            p = root / rel
            files[rel] = p.read_text() if p.exists() else None
        v = root / "vendor"
        files["vendor-tree"] = sorted(str(p.relative_to(root)) for p in v.rglob("*") if p.is_file()) if v.exists() else None
        return files
    a, b = snap(FIXTURE), snap(ws)
    diff = [k for k in a if a[k] != b[k]]
    return not diff, f"changed: {diff}"


def unreleased(text):
    m = re.search(r"## \[Unreleased\]\n(.*?)(?=\n## \[|\Z)", text, re.S)
    return [ln.strip() for ln in (m.group(1) if m else "").splitlines() if ln.strip().startswith("- ")]


def changelog():
    p = ws / "CHANGELOG.md"
    if not p.exists():
        return False, "CHANGELOG.md missing"
    new = [b for b in unreleased(p.read_text()) if b not in unreleased((FIXTURE / "CHANGELOG.md").read_text())]
    joined = " ".join(new).lower()
    return bool(new) and "stats" in joined and re.search(r"\brm\b", joined) is not None, f"new Unreleased bullets: {new}"


check("each new command in its own cmd/ file", 3, command_files)
check("go.mod, go.sum and vendor/ untouched", 2, deps_untouched)
check("CHANGELOG Unreleased lists the changes", 2, changelog)

total = sum(c["weight"] for c in checks)
score = sum(c["weight"] for c in checks if c["pass"])
print(json.dumps({"score": score, "total": total, "checks": checks}))
