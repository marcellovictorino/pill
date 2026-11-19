"""Tests for personal runs: python3 -m unittest discover -s evals -p 'test_*.py'

Why they exist: a personal run copies the user's own Pi folder and must never write outside the copy or
$PILL_HOME/personal, never change the real folder, and never leave a trace a commit could pick up.
A fake `pi` and `pill` stand in for the real programs; no model runs.
"""
import argparse
import hashlib
import json
import os
import subprocess
import sys
import tempfile
import time
import unittest
from pathlib import Path
from unittest import mock

sys.path.insert(0, str(Path(__file__).resolve().parent))
import run  # noqa: E402
import summary  # noqa: E402

FAKE_PI = """#!/bin/sh
# records how it was launched, then exits
{ for a in "$@"; do printf '%s\\n' "$a"; done; } > "$FAKE_LOG/argv"
printf '%s' "$PI_CODING_AGENT_DIR" > "$FAKE_LOG/agent_dir"
cp -R "$PI_CODING_AGENT_DIR" "$FAKE_LOG/agent_dir_copy"
"""

MARKER_CMD = r"""! git grep -nIE '"personal"[[:space:]]*:[[:space:]]*true'"""


def tree_hash(root):
    h = hashlib.sha256()
    for p in sorted(Path(root).rglob("*")):
        h.update(str(p.relative_to(root)).encode())
        if p.is_file() and not p.is_symlink():
            h.update(p.read_bytes())
    return h.hexdigest()


class Env(unittest.TestCase):
    def setUp(self):
        self.tmp = Path(tempfile.mkdtemp(prefix="pill-personal-test-"))
        self.addCleanup(lambda: subprocess.run(["rm", "-rf", str(self.tmp)]))
        self.home, self.pill_home, self.log = self.tmp / "home", self.tmp / "pillhome", self.tmp / "log"
        for d in (self.home, self.log, self.tmp / "bin"):
            d.mkdir()
        agent = self.home / ".pi" / "agent"
        (agent / "sessions").mkdir(parents=True)
        (agent / "sessions" / "s.jsonl").write_text("private history")
        (agent / "web-search-cache").mkdir()
        (agent / "skills" / "one").mkdir(parents=True)
        (agent / "skills" / "one" / "SKILL.md").write_text("skill")
        (agent / "AGENTS.md").write_text("my rules")
        (agent / "models.json.bak").write_text("backup")
        (agent / "settings.json.hindsight-backup").write_text("backup")
        (agent / "extensions").mkdir()
        (agent / "extensions" / "footer.ts").write_text("footer")
        (agent / "auth.json").write_text("{}")
        (agent / "settings.json").write_text(json.dumps({
            "theme": "dark",
            "extensions": ["/x/.hindsight/coding-agents/dist/pi.js", "/x/footer.ts"],
            "packages": ["npm:pi-web-access"]}))
        (agent / "models.json").write_text(json.dumps(
            {"providers": {"pill": {"models": [{"id": "other", "name": "other"}]}}}))
        for name, body in (("pi", FAKE_PI), ("pill", "#!/bin/sh\nexit 0\n")):
            p = self.tmp / "bin" / name
            p.write_text(body)
            p.chmod(0o755)
        task = self.tmp / "tasks" / "t"
        (task / "fixture").mkdir(parents=True)
        (task / "task.json").write_text(json.dumps({"id": "t", "timeout_minutes": 1}))
        (task / "spec.md").write_text("do it")
        (task / "verify.py").write_text(
            'import json\nprint(json.dumps({"score": 80, "total": 100, "checks": []}))\n')
        self.agent_src = agent
        self.results = self.tmp / "bare-results"
        patches = [mock.patch.dict(os.environ, {"HOME": str(self.home), "PILL_HOME": str(self.pill_home),
                                                 "FAKE_LOG": str(self.log),
                                                 "PATH": f"{self.tmp / 'bin'}{os.pathsep}{os.environ['PATH']}"}),
                   mock.patch.object(run, "TASKS", self.tmp / "tasks"),
                   mock.patch.object(run, "RESULTS", self.results),
                   mock.patch.object(run, "MemorySampler", mock.MagicMock())]
        for p in patches:
            p.start()
            self.addCleanup(p.stop)
        run.MemorySampler.return_value.start.return_value.stop.return_value = {}

    def go(self, personal, agent="pi:other"):
        run.cmd_run(argparse.Namespace(task="t", agent=agent, runs=1, timeout_min=0, personal=personal))

    def argv(self):
        return (self.log / "argv").read_text().split("\n")


class PersonalRun(Env):
    def test_personal_gives_pi_a_copy_of_the_setup_without_history_or_hindsight(self):
        before = tree_hash(self.agent_src)
        self.go(personal=True)
        argv = self.argv()
        for flag in ("--no-extensions", "--no-skills", "--no-context-files", "--offline"):
            self.assertNotIn(flag, argv)  # the setup is the point, and personal runs are online
        self.assertIn("--no-session", argv)  # no session is written
        copy = self.log / "agent_dir_copy"
        self.assertTrue((copy / "AGENTS.md").exists() and (copy / "skills" / "one" / "SKILL.md").exists())
        self.assertTrue((copy / "extensions" / "footer.ts").exists())
        for gone in ("sessions", "web-search-cache", "models.json.bak", "settings.json.hindsight-backup"):
            self.assertFalse((copy / gone).exists(), gone)
        cfg = json.loads((copy / "settings.json").read_text())
        self.assertEqual(cfg["extensions"], ["/x/footer.ts"])  # Hindsight would read the real memory
        self.assertEqual(cfg["packages"], ["npm:pi-web-access"])
        self.assertEqual(tree_hash(self.agent_src), before)  # the real folder is only read

    def test_links_and_hindsight_in_the_extensions_folder_cannot_reach_the_original(self):
        outside = self.tmp / "outside"
        outside.mkdir()
        (outside / "settings.json").write_text(json.dumps(
            {"extensions": ["/x/.hindsight/pi.js"], "theme": "dark"}))
        (outside / "linked-skill").mkdir()
        (outside / "linked-skill" / "SKILL.md").write_text("linked skill")
        (self.agent_src / "settings.json").unlink()
        (self.agent_src / "settings.json").symlink_to(outside / "settings.json")
        (self.agent_src / "skills" / "linked").symlink_to(outside / "linked-skill")
        (self.agent_src / "extensions" / "hindsight.ts").write_text("memory")
        (self.agent_src / "extensions" / "hindsight").mkdir()
        before = tree_hash(outside), tree_hash(self.agent_src)
        self.go(personal=True)
        copy = self.log / "agent_dir_copy"
        self.assertEqual((tree_hash(outside), tree_hash(self.agent_src)), before)
        self.assertFalse((copy / "settings.json").is_symlink())
        self.assertEqual(json.loads((copy / "settings.json").read_text())["extensions"], [])
        self.assertEqual((copy / "skills" / "linked" / "SKILL.md").read_text(), "linked skill")
        self.assertFalse((copy / "extensions" / "hindsight.ts").exists())
        self.assertFalse((copy / "extensions" / "hindsight").exists())
        self.assertTrue((copy / "extensions" / "footer.ts").exists())

    def test_the_pill_model_is_cloned_into_the_copy_like_a_bare_run(self):
        self.go(personal=True, agent="pi:not-offered-to-pi")
        models = json.loads((self.log / "agent_dir_copy" / "models.json").read_text())["providers"]["pill"]["models"]
        self.assertIn("not-offered-to-pi", [m["id"] for m in models])

    def test_records_stay_under_pill_home_stamped_and_hashed(self):
        self.go(personal=True)
        idx = self.pill_home / "personal" / "evals" / "index.jsonl"
        rec = json.loads(idx.read_text())
        self.assertIs(rec[run.PERSONAL_KEY], True)
        self.assertEqual(len(rec["setup_hash"]), 16)
        result = json.loads((Path(rec["rundir"]) / "result.json").read_text())
        self.assertIs(result[run.PERSONAL_KEY], True)
        self.assertFalse(self.results.exists())  # nothing reaches the bare results folder
        self.assertFalse((Path(rec["rundir"]) / "pi-agent").exists())  # the large copy is removed

    def test_setup_hash_changes_when_the_setup_changes(self):
        self.go(personal=True)
        first = json.loads((self.pill_home / "personal/evals/index.jsonl").read_text().splitlines()[0])["setup_hash"]
        (self.agent_src / "AGENTS.md").write_text("changed rules")
        time.sleep(1.1)  # run folders are named by the second
        self.go(personal=True)
        second = json.loads((self.pill_home / "personal/evals/index.jsonl").read_text().splitlines()[1])["setup_hash"]
        self.assertNotEqual(first, second)

    def test_editing_an_external_extension_changes_the_setup_hash(self):
        ext = self.tmp / "footer.ts"
        ext.write_text("v1")
        cfg = json.loads((self.agent_src / "settings.json").read_text())
        cfg["extensions"] = [str(ext)]
        (self.agent_src / "settings.json").write_text(json.dumps(cfg))
        self.go(personal=True)
        ext.write_text("v2")
        time.sleep(1.1)
        self.go(personal=True)
        hashes = [json.loads(line)["setup_hash"]
                  for line in (self.pill_home / "personal/evals/index.jsonl").read_text().splitlines()]
        self.assertNotEqual(hashes[0], hashes[1])

    def test_a_relative_pill_home_gives_pi_an_absolute_config_path(self):
        os.chdir(self.tmp)
        self.addCleanup(os.chdir, "/")
        with mock.patch.dict(os.environ, {"PILL_HOME": "relhome"}):
            self.go(personal=True)
        self.assertTrue(Path((self.log / "agent_dir").read_text()).is_absolute())
        self.assertTrue((self.tmp / "relhome" / "personal" / "evals" / "index.jsonl").exists())

    def test_a_bare_run_keeps_its_isolation_and_carries_no_marker(self):
        self.go(personal=False)
        argv = self.argv()
        for flag in ("--no-extensions", "--no-skills", "--no-context-files", "--offline", "--no-session"):
            self.assertIn(flag, argv)
        rec = json.loads((self.results / "index.jsonl").read_text())
        self.assertNotIn(run.PERSONAL_KEY, rec)
        self.assertFalse(self.pill_home.exists())
        copy = self.log / "agent_dir_copy"
        self.assertFalse((copy / "AGENTS.md").exists())  # bare Pi sees only the provider files

    def test_personal_refuses_other_agents(self):
        with self.assertRaises(SystemExit):
            self.go(personal=True, agent="claude:opus")

    def test_a_results_folder_inside_a_git_work_tree_is_refused_before_anything_is_written(self):
        repo = self.tmp / "repo"
        repo.mkdir()
        subprocess.run(["git", "init", "-q", str(repo)], check=True)
        with mock.patch.dict(os.environ, {"PILL_HOME": str(repo / "inner")}):
            with self.assertRaises(SystemExit) as cm:
                self.go(personal=True)
        self.assertIn("git work tree", str(cm.exception))
        self.assertEqual(list(repo.iterdir()), [repo / ".git"])
        self.assertFalse((self.log / "argv").exists())  # pi never started


class Summary(unittest.TestCase):
    def rows(self, *scores):
        return [{"task": "t", "agent": "a", "score": s, "total": 100} for s in scores]

    def test_a_difference_inside_the_bare_range_is_noise(self):
        # bare runs themselves span 40-90, so 60 is within what the bare setup does by chance
        self.assertEqual(summary.verdict([40, 90, 70], [60]), "noise")
        self.assertEqual(summary.verdict([40, 90, 70], [90]), "noise")

    def test_outside_the_bare_range_is_an_effect(self):
        self.assertEqual(summary.verdict([40, 50], [80, 90, 85]), "setup helps")
        self.assertEqual(summary.verdict([60, 70], [20, 30, 35]), "setup hurts")

    def test_table_shows_both_medians(self):
        out = summary.side_by_side(self.rows(40, 60, 50), self.rows(90))
        self.assertIn("50 (40-60, 3)", out)
        self.assertIn("90 (90-90, 1)", out)
        self.assertIn("setup helps", out)

    def test_personal_without_bare_runs_says_so(self):
        self.assertIn("no bare runs", summary.side_by_side([], self.rows(90)))


class CiMarkerStep(unittest.TestCase):
    """The shell line CI runs (test_ci_runs_it checks ci.yml carries the same command)."""

    def git_repo(self, content):
        d = Path(tempfile.mkdtemp(prefix="pill-marker-test-"))
        self.addCleanup(lambda: subprocess.run(["rm", "-rf", str(d)]))
        subprocess.run(["git", "init", "-q", str(d)], check=True)
        (d / "f.json").write_text(content)
        subprocess.run(["git", "-C", str(d), "add", "."], check=True)
        return d

    def runs(self, d):
        return subprocess.run(["bash", "-c", MARKER_CMD], cwd=d).returncode

    def test_fails_when_a_tracked_file_carries_the_marker(self):
        for text in (json.dumps({run.PERSONAL_KEY: True}), '{"' + run.PERSONAL_KEY + '":true}'):
            self.assertNotEqual(self.runs(self.git_repo(text)), 0)

    def test_passes_without_it(self):
        self.assertEqual(self.runs(self.git_repo(json.dumps({run.PERSONAL_KEY: False}))), 0)

    def test_ci_runs_it(self):
        ci = (Path(__file__).resolve().parents[1] / ".github" / "workflows" / "ci.yml").read_text()
        # the YAML escapes the quotes, so compare the parts that carry the meaning
        self.assertIn("git grep -nIE", ci)
        self.assertIn("[[:space:]]*:[[:space:]]*true", ci)


if __name__ == "__main__":
    unittest.main()
