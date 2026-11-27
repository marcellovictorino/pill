"""The runner gives Pi the same compaction rule pill writes: 80% of the model's window."""
import importlib.util
import json
import pathlib
import tempfile
import unittest

spec = importlib.util.spec_from_file_location("run", pathlib.Path(__file__).parent / "run.py")
run = importlib.util.module_from_spec(spec)
spec.loader.exec_module(run)


class CompactionRule(unittest.TestCase):
    def test_rule_holds_back_a_fifth_of_the_window_and_keeps_other_settings(self):
        d = pathlib.Path(tempfile.mkdtemp())
        (d / "settings.json").write_text(json.dumps({"compaction": {"reserveTokens": 400000}}))
        run.write_compaction_rule(d, "qwen-64k", 65536)
        cfg = json.loads((d / "settings.json").read_text())
        self.assertEqual(cfg["compaction"]["modelOverrides"]["pill/qwen-64k"]["reserveTokens"], 13107)
        self.assertEqual(cfg["compaction"]["reserveTokens"], 400000)

    def test_setup_hash_ignores_the_per_model_rule(self):
        a, b = (pathlib.Path(tempfile.mkdtemp()) for _ in range(2))
        run.write_compaction_rule(a, "one", 65536)
        run.write_compaction_rule(b, "two", 32768)
        self.assertEqual(run.setup_hash(a), run.setup_hash(b))


if __name__ == "__main__":
    unittest.main()
