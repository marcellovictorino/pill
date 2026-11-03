"""Tests for the image runner's standing prompt rules and the six-option command (stdlib unittest, no model).

Run: python3 -m unittest discover -s evals/imagegen
Intent source: PILL-31 (captain: no text in generated images; FLUX.2 cyberpunk looked plastic; always generate
3 variants per model so both models give six options to pick from).
"""
import argparse
import json
import os
import tempfile
import unittest
from pathlib import Path

import imagegen as ig

ROUND3 = json.loads((Path(ig.HERE) / "prompts-round3.json").read_text())


class BuildPrompt(unittest.TestCase):
    def test_no_text_sentence_on_every_prompt_and_model(self):
        # Captain: text in images is the weak side, so the rule must not depend on remembering it.
        for model in ("qwen-turbo", "flux2-klein-4b"):
            for prompt in list(ROUND3.values()) + ["watercolour style vineyard"]:
                self.assertTrue(ig.build_prompt(model, prompt).endswith(ig.NO_TEXT))
                self.assertTrue(ig.build_prompt(model, prompt).startswith(prompt))

    def test_realism_cue_only_flux2_and_only_for_realistic_prompts(self):
        # Captain: FLUX.2 cyberpunk looked cartoonish/plastic; Qwen's were fine, and stylised prompts must stay stylised.
        cyber, mars = ROUND3["cyberpunk-night"], ROUND3["mars-domes"]
        self.assertIn(ig.REALISM_CUE, ig.build_prompt("flux2-klein-4b", cyber))
        self.assertIn(ig.REALISM_CUE, ig.build_prompt("flux2-klein-4b", ROUND3["magazine-cover"]))
        self.assertNotIn(ig.REALISM_CUE, ig.build_prompt("qwen-turbo", cyber))
        self.assertNotIn(ig.REALISM_CUE, ig.build_prompt("flux2-klein-4b", mars))
        self.assertNotIn(ig.REALISM_CUE, ig.build_prompt("flux2-klein-4b", "cartoon chef, anime style"))

    def test_plain_reproduces_old_runs(self):
        self.assertEqual(ig.build_prompt("flux2-klein-4b", ROUND3["cyberpunk-night"], plain=True), ROUND3["cyberpunk-night"])


class Commands(unittest.TestCase):
    def test_run_keeps_verbatim_prompt_and_prompt_sent(self):
        tmp = tempfile.mkdtemp()
        ig.cmd_run(argparse.Namespace(model="fake", out=tmp, force=True, timeout=60, plain=False, no_cue=False))
        rows = ig.load_run(next(Path(tmp).iterdir()))
        for r in rows:
            self.assertEqual(r["prompt"], ig.PROMPTS[r["prompt_id"]])
            self.assertEqual(r["prompt_sent"], ig.build_prompt("fake", r["prompt"]))
            self.assertNotEqual(r["prompt"], r["prompt_sent"])

    def test_options_makes_variants_per_model_with_fresh_seeds_and_a_pick_page(self):
        # Captain: both models, 3 variants each, 6 options to review for one request.
        tmp = tempfile.mkdtemp()
        ns = argparse.Namespace(prompt="realistic photograph of a market", models="fake,fake", variants=3, rng=5, out=tmp,
                                plain=False, force=True, wait=0, timeout=60)
        ig.cmd_options(ns)
        run = next(Path(tmp).iterdir())
        saved = json.loads((run / "options.json").read_text())["options"]
        self.assertEqual(len(saved), 6)
        self.assertEqual(len({o["seed"] for o in saved}), 6)
        self.assertFalse({7, 42, 1234} & {o["seed"] for o in saved})  # not the benchmark seeds
        page = (run / "pick.html").read_text()
        for o in saved:
            self.assertTrue((run / o["file"]).exists())
            self.assertIn(o["file"], page)
            self.assertIn(f'seed {o["seed"]}', page)
            self.assertTrue(o["prompt_sent"].endswith(ig.NO_TEXT))

    def test_options_default_is_both_models_three_each(self):
        self.assertEqual(ig.DEFAULT_OPTION_MODELS, ["qwen-turbo", "flux2-klein-4b"])
        self.assertEqual(ig.VARIANTS, 3)

    def test_selftest_runs(self):
        ig.cmd_selftest(None)


if __name__ == "__main__":
    unittest.main()
