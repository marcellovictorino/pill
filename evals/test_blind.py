"""Tests for the blind rating sheets (stdlib unittest, no browser, no model).

Run: python3 -m unittest discover -s evals -p 'test_blind.py'
Intent source: Kanban polish work, piece 2 (maintainer: rate Kanban polish blind like the landing pages; personal runs join the
same shuffled sheet) and the maintainer's ask that the Kanban task be judged as a polished customer-facing app.
"""
import json
import tempfile
import unittest
from pathlib import Path

import blind

PNG = bytes.fromhex("89504e470d0a1a0a")


def fake_shoot(calls):
    def shoot(ws, shots, script):
        calls.append((Path(ws).parent.name, script))
        for n in ("1440.png", "390.png", "kanban.png"):
            (shots / n).write_bytes(PNG)
        return True, ""
    return shoot


def row(task, agent, run, personal=False):
    r = {"task": task, "agent": agent, "run": run, "rundir": f"/x/{agent}-{run}", "score": 0.5}
    if personal:
        r["personal"] = True
    return r


class KanbanSheet(unittest.TestCase):
    def build(self, rows, seed=3, task=None):
        out = Path(tempfile.mkdtemp()) / "sheet"
        calls = []
        key = blind.build_sheets(rows, out, seed, task, fake_shoot(calls))
        return out, key, calls

    def test_kanban_is_rated_blind_with_its_own_screenshot_and_polish_question(self):
        # Maintainer: Kanban polish should be rated blind like the landing pages.
        out, key, calls = self.build([row("web-kanban", "pi:a", 1), row("web-kanban", "pi:b", 1)])
        html = (out / "web-kanban.html").read_text()
        self.assertEqual({s for _, s in calls}, {"shot_kanban.mjs"})
        self.assertEqual(html.count('type="radio"'), 8)  # two submissions, four ratings each
        self.assertIn("good and polished", html)
        self.assertNotIn("pi:a", html)
        self.assertEqual(set(key["web-kanban"]), {"A", "B"})

    def test_landing_still_uses_both_viewports(self):
        out, _, calls = self.build([row("web-landing-mars", "pi:a", 1)])
        self.assertEqual(calls[0][1], "shot.mjs")
        html = (out / "web-landing-mars.html").read_text()
        self.assertIn("Desktop screenshot", html)
        self.assertIn("Mobile screenshot", html)

    def test_personal_run_joins_the_same_shuffled_sheet_without_a_tell(self):
        # Maintainer: personal runs can join the same blind sheet; the sheet must not reveal which one is which.
        rows = [row("web-kanban", "pi:a", i) for i in range(3)] + [row("web-kanban", "pi:a", 9, personal=True)]
        out, key, _ = self.build(rows)
        html = (out / "web-kanban.html").read_text()
        self.assertEqual(html.count("<section>"), 4)
        self.assertNotIn("personal", html.lower().split("<body>")[1])
        self.assertEqual(sum(v["personal"] for v in key["web-kanban"].values()), 1)

    def test_same_seed_gives_same_labels(self):
        rows = [row("web-kanban", "pi:a", i) for i in range(5)]
        self.assertEqual(self.build(rows, seed=7)[1], self.build(rows, seed=7)[1])

    def test_task_filter_and_non_visual_tasks_are_ignored(self):
        rows = [row("web-kanban", "pi:a", 1), row("go-notes", "pi:a", 1), row("web-landing-tax", "pi:a", 1)]
        out, key, _ = self.build(rows, task="web-kanban")
        self.assertEqual(list(key), ["web-kanban"])
        self.assertFalse((out / "go-notes.html").exists())

    def test_read_rows_merges_indexes_in_order(self):
        d = Path(tempfile.mkdtemp())
        (d / "a.jsonl").write_text(json.dumps(row("web-kanban", "pi:a", 1)) + "\n")
        (d / "b.jsonl").write_text(json.dumps(row("web-kanban", "pi:b", 2, personal=True)) + "\n")
        rows = blind.read_rows([d / "a.jsonl", d / "b.jsonl"])
        self.assertEqual([r["agent"] for r in rows], ["pi:a", "pi:b"])
        with self.assertRaises(SystemExit):
            blind.read_rows([d / "missing.jsonl"])


if __name__ == "__main__":
    unittest.main()
