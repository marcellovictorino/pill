# Tried and not adopted

Models that were benchmarked on this machine (24 GB M4) and did not beat the current default, so nobody
spends time re-running them. Re-test only after a significant change: a new release of the model, a
different quantisation, a faster machine, or an engine fix that changes the cause below.

## Seen issues

Every model that lost or could not run, with the numbers pill recorded. Tier 1 is the time for a 7,400-token agent prompt (limit 60 s
by default). Tier 2 is the 22-task hidden test. "Lowest free" is the lowest free memory during the run. The nine models with a
build 11429 result were deleted from disk on 25 October; re-pull one with `pill pull` and register it with `pill add` to test it again.

Issue kinds: latency (Tier 1 time, or Tier 2 hitting its cap), accuracy (Tier 2 score, or a loop), memory (lowest free, swap
growth), load (the engine cannot open the file), thinking (the model thinks although thinking is off).

| Model | Result and build | Tier 1 time | Tier 2 | Decode tok/s | Lowest free | Issues seen |
|---|---|---|---|---|---|---|
| Devstral Small 2 24B Q3_K_M, 32k and 64k | Failed, 3 runs, build 11429, 14 Oct | 128–131 s (32k); 131–141 s (64k) | Not run | 5.2–5.3 | 16–17% (32k); 6–7% (64k) | Latency; swap up to 2.1 GB |
| Devstral Small 2 24B UD-Q3_K_XL, 64k | Failed, 3 runs, build 11429, 14 Oct | 130–143 s | Not run | 5.5–5.6 | 6% | Latency; swap up to 2.7 GB |
| Gemma 4 26B UD-IQ3_S, 64k | Failed, 3 runs, build 11429, 14 Oct | 27–29 s | 22/22 on all runs; runs 1 and 2 hit the 15-min cap | 24–25 | 19–22% | Latency (Tier 2 over the cap) |
| Gemma 4 26B UD-Q3_K_XL, 64k | Passed once, 1 run, build 11429, 9 Oct | 48 s | 22/22 in 294 s | 22.4 | 11% | Swap 4.75 GB in the run; not re-run on build 11669 |
| GLM-4.7 Flash UD-IQ2_M, 64k | Failed, 3 runs, build 11429, 14 Oct | 37 s | 18/22, 18/22, 14/22; all hit the cap | 2–14 | 16–19% | Accuracy and latency |
| Ornith 1.5 35B-A3B IQ3_XXS-IQ2_S, 64k | Failed, 3 runs, build 11429, 14 Oct | 22–32 s | 20/22; 22/22 in 372 s; 22/22 at the cap | 24–25 | 11% | Accuracy on run 1; swap up to 533 MB |
| Qwen3.6 35B-A3B UD-IQ2_M, 64k | Failed, 3 runs, build 11429, 14 Oct | 22–29 s | 0/22 in 528 s; 22/22 in 650 s; 22/22 at the cap | 31–32 | 18–22% | Accuracy: 0/22 on run 1 |
| Qwen3.6 27B UD-IQ3_XXS, 64k | Failed, 3 runs, build 11669, 25 Oct (PILL-8) | 130–147 s | Not run at the default limits; one relaxed run 20/22 at the 30-min cap | 6.3–6.6 | 9–10% | Latency; swap up to 2.4 GB; thinking partly on in the relaxed run |
| Qwen3.8 27B UD-IQ3_XXS, 64k, thinking on | Failed, 3 runs, build 11429, 14 Oct | 130–144 s | Not run; one relaxed run 0/22 | 6.7–6.8 | 13–18% | Thinking: an 8,192-token thinking block and no tool call (PILL-9) |
| Qwen3.8 27B UD-IQ3_XXS, 64k, thinking off | Relaxed gate, 3 runs, scratch home, 25 Oct (PILL-8) | 133–142 s (limit 300 s) | 22/22 in 4.3 min; 22/22 and 20/22 at the 30-min cap | 6.7 | 12% | Latency; Tier 1 answer wrong in every run (informational) |
| Qwen3.8 27B UD-IQ3_XXS, 64k, thinking off, MTP, 32k | Failed gate, 3 runs, 25 Oct (PILL-10) | 151–159 s (limit 300 s) | 20/22; 22/22 in 11.4 min; 20/22; two runs repeated one shell command 90–105 times | 6.7 without MTP; 14.8 with MTP | 8–10% | Latency; loops; swap up 4.5 GB in run 1 |
| flint (RecursiveSI) IQ2_XXS, 64k | Failed, 3 runs, build 11669, 23 Oct; build 11804 runs in the [Flint report](recursive-flint-iq2xxs.md) | Load failed in 0.2–0.3 s on 11669 | Not run on 11669 | Not measured on 11669 | 43–44% | Load on 11669: `unknown model architecture: 'flint'`, so no timing limit applies |
| LiquidAI LFM2.5-24B-A2B UD-Q3_K_XL (unsloth, 10.9 GB), 64k, `reasoning-budget 0` | Failed, 3 runs, build 11669, 28 Oct; GGUF deleted after the run | 13–28 s | 17/22 at the cap; 21/22 in 7.4 min; 16/22 at the cap; two runs repeated one tool call 154 and 320 times | 36 | 17–18% | Accuracy and loops; Tier 1 answer wrong in every run (informational) |
| LiquidAI LFM2.5-24B-A2B Q4_K_M (LiquidAI, 14.5 GB), 64k, `reasoning-budget 0` | Failed, 3 runs, build 11669, 28 Oct; GGUF deleted after the run | 13–30 s | Not run on run 1 (Tier 1 memory gate); 19/22 and 21/22, both at the cap, repeating one tool call 105 and 182 times | 48 | 8–9% | Memory (below the 10% limit; swap up to 1.0 GB); accuracy and loops |
| NVIDIA Nemotron 3.5 Bolt E4B UD-Q8_K_XL (unsloth, 8.2 GB), 64k | Failed, 3 runs, build 11960, 29 Oct | 26–30 s | 16/22 in 4.1 min; 19/22 in 7.4 min; 21/22 in 6.6 min | 16–17 | 32–40% | Accuracy: never 22/22; no cap, no loops |
| Meta Muse Glimmer 30B UD-IQ2_XXS (unsloth, 10.7 GB), 64k, dense | Failed, 9 runs, build 11960, 29 Oct (3 standard, 6 relaxed at 300 s / 30 min) | 113–140 s | Not run in 8 of 9; one relaxed run 22/22 in 11.1 min | 5–7 | 14–17% | Latency; thinking: empty visible reply in 8 of 9 runs |
| Meta Muse Glimmer 30B UD-IQ2_XXS, 64k, dense, thinking-off template | Failed relaxed gate, 3 runs, build 11960, 30 Oct (300 s / 30 min) | 108–127 s | 22/22, 20/22, 22/22 in 17.7–18.2 min | 7 | 14–16% | Latency (about double the default limits); accuracy on run 2 |
| Meta Muse Glimmer 30B UD-IQ2_XXS, 64k, dense, thinking-off template, DFlash drafter | Passed relaxed gate only, 3 runs, build 11960, 30 Oct (300 s / 30 min) | 107–111 s | 22/22 in 5.9, 6.3 and 13.2 min | 12.8 in a 300-token probe (6.8 without) | 11% | Latency: Tier 1 still about twice the 60 s limit; memory close to the 10% floor; Tier 1 answer wrong in every run (informational) |

## Qwen3.8-27B: thinking and MTP

- With thinking off, the model writes correct code when it finishes: 22/22 in 4.3 minutes in one of three relaxed runs, and 22/22 or
  20/22 at the 30-minute cap in the other two. Its quality matches MiniCPM's 22/22 when it finishes; its speed does not (6.7 tok/s
  against 25).
- MTP (`--spec-type draft-mtp`) raised decoding from 6.7 to 14.8 tok/s at 32k, with 94% of drafts accepted (llama-server's own
  timings). PILL-8 measured 16–18 tok/s at a 16k context. Even so, it is well under MiniCPM's 25 tok/s.
- The MTP gate failed: Tier 1 took 151–159 s, and two of the three Tier 2 runs repeated one shell command 90–105 times without
  fixing a `bestMove` bug. Without MTP, two of the three relaxed runs also kept re-running their own tests until the cap, so MTP
  is not proven to cause the loops.
- Memory: MTP cut the lowest free memory from about 30% to 7% at 16k (PILL-8), and it was 8–10% at 32k.
- Not tried: MTP with the thinking leak fixed (PILL-9), or MTP at 16k context through a full gate run.

## LiquidAI LFM2.5-24B-A2B: Q3_K_XL and Q4_K_M

- Both ran with `temp 0.1`, `top-k 50`, `repeat-penalty 1.05` (model card) and `reasoning-budget 0`, set under `[model.preset]` in
  `models.toml`; the chat template always opens `<think>` and ignores `enable_thinking:false`. Neither is adopted: MiniCPM scores
  22/22 every run; LFM2.5 never did.
- It is fast: 36 tok/s (Q3) and 48 tok/s (Q4) against MiniCPM's 25, and Tier 1 takes 13–30 s. The loss is accuracy, not speed.
- Tier 2 scores were 17, 21, 16 (Q3) and 19, 21 (Q4; run 1 failed Tier 1 on memory before Tier 2). Four of the five scored runs
  ended at the 15-minute cap with the agent repeating one tool call 105–320 times in a row.
- Q4_K_M (14.5 GB) does not fit with 64k context and q8_0 KV: lowest free memory 8–9% and swap up 0.3–1.0 GB. Q3 fit (17–18%).
- Not tried: a run with thinking on; the unsloth UD-Q4_K_XL (14.9 GB) and LiquidAI QAD-Q4_0 (13.6 GB) files; a smaller context.
  Tier 3 was not run, as Tier 2 never reached 22/22.
- Both GGUFs were deleted on 28 October; re-pull with `pill pull hf.co/unsloth/LFM2.5-24B-A2B-GGUF:UD-Q3_K_XL` or
  `pill pull hf.co/LiquidAI/LFM2.5-24B-A2B-GGUF:Q4_K_M`.

## Nemotron 3.5 Bolt E4B and Muse Glimmer 30B, 2026-10-29

Neither is adopted. Bolt E4B fits easily but never scores 22/22. Muse Glimmer's chat template cannot turn thinking off
(a template change, below, mostly does); with its DFlash drafter it passes a relaxed gate 3 of 3, but Tier 1 still
takes about twice the default limit.

| | MiniCPM (default), rerun today | Bolt E4B UD-Q8_K_XL | Muse Glimmer 30B UD-IQ2_XXS |
|---|---|---|---|
| Gate | Passed, 1 of 1 | Failed, 0 of 3 | Failed, 0 of 3 standard and 0 of 6 relaxed |
| Tier 1 time (limit 60 s) | 36 s | 26–30 s | 113–140 s |
| Tier 1 answer (informational) | Wrong (`58`) | Right in 3 of 3 | Empty in 8 of 9; right in 1 |
| Tier 2 (22) | 22/22 in 3.6 min | 16, 19, 21; 4.1–7.4 min | 22/22 in 11.1 min, in its one relaxed run that reached Tier 2 |
| Decoding speed | 25.8 tok/s | 16–17 tok/s | 5–7 tok/s |
| Lowest free memory | 14% | 32–40% | 14–17% |
| Swap growth | 3.4 GB | 0 | 0–1.8 GB per run |

- **Comparability:** all runs on 29 October, same machine, llama.cpp build 11960, Pi 1.4.0, pill 0.4.1, 64k
  context, q8_0 KV, thinking off in Pi. MiniCPM was rerun once as the reference: it still scores 22/22 at 25.8 tok/s,
  but with 14% free memory and 3.4 GB swap growth against 22% and 0 on 13 October, so other apps held about
  8 points more memory today. Swap readings are system-wide; swap was already 5.4 GB used before the first run.
- **Bolt E4B** (Gemma 4 E4B post-trained by NVIDIA, 7.5B total): every Tier 2 run finished inside 7.4 minutes
  with no repeated tool calls, but missed checks each time: `bestMove`, `render` and CLI checks on run 1
  (16/22), `render` and CLI on run 2 (19/22), draw detection on run 3 (21/22). It is slower to decode than
  MiniCPM (16–17 against 25.8 tok/s) despite the smaller file.
- **Muse Glimmer** (Meta, 29.6B dense): its chat template ignores `enable_thinking:false` and has no off switch;
  it always reasons, at `reasoning_strength` `high` by default. `--reasoning-budget 0` did not change that
  (3 relaxed runs, all empty). With `chat-template-kwargs = '{"reasoning_strength":"low"}'` under
  `[model.preset]` in `models.toml`, a short probe answered in 42 tokens, but two of three relaxed runs still
  spent Tier 1's 64-token reply on reasoning. Its 64 generated tokens take about 10 s of the 113–140 s;
  the rest is load and prefill of the 5,800-token prompt on a dense model, as with the dense 27B models above,
  so a lower reasoning setting would not bring Tier 1 under 60 s.
- **Muse with thinking off (30 October):** the template writes reasoning as an assistant message addressed
  `to=self`, and its generation prompt (`<|start|>assistant`) lets the model choose it. A copy of the template
  that appends an empty reasoning message when `enable_thinking` is false,
  `{%- if enable_thinking is defined and enable_thinking is false -%}{{- ' to=self<|message|><|eom|><|start|>assistant' -}}{%- endif -%}`,
  set as `chat-template-file` under `[model.preset]`, answered a probe in 5 tokens instead of 42 and kept tool calls
  working; thinking on still reasons. In multi-turn Tier 2 the model still writes short `to=self` notes between
  tool calls (23–43 per run), so thinking is reduced, not off. Three clean relaxed runs: Tier 1 108–127 s with a
  visible reply every run (correct in 2 of 3); Tier 2 22/22, 20/22 (missed both `bestMove` checks), 22/22, each in
  17.7–18.2 minutes; 7 tok/s; lowest free 14–16%; swap up 3.5 GB in run 1. It is accurate, but needs about twice
  the 60 s and 15-minute limits, and MiniCPM does the same task in 3.6 minutes.
- **Muse with the DFlash drafter (30 October):** the GGUF has no MTP layers, so `draft-mtp` does not apply. Meta ships
  a DFlash block-diffusion drafter; Unsloth's `dflash-kquant.gguf` (1.6 GB) loads in build 11960 with
  `spec-type = "draft-dflash"`, `spec-draft-model = "<path>"` and `spec-draft-ngl = 99` under `[model.preset]`. On a
  300-token coding prompt, decoding rose from 6.8 to 12.8 tok/s with 151 of 216 drafted tokens accepted and the
  same output. Three clean relaxed runs all passed: Tier 2 22/22 in 5.9, 6.3 and 13.2 minutes (inside the default
  15-minute cap); Tier 1 107–111 s, because drafting does not speed up prefill; lowest free memory 11%, with both
  models loaded; swap up to 0.8 GB. Not adopted: Tier 1 still fails the default 60 s limit, and MiniCPM is faster on
  both tiers (36 s and 3.6 minutes) with the same score.
- **Not tried:** Bolt E4B UD-Q4_K_XL (5.1 GB; a smaller quant would not fix accuracy); Muse with thinking left
  at `high` through a relaxed Tier 2; Muse's DFlash drafter; Tier 3 for either.
- Both GGUFs are still in `~/.pill/models` and registered as failed; remove with `pill rm <name>`, or re-pull with
  `pill pull hf.co/unsloth/NVIDIA-Nemotron-3.5-Bolt-E4B-GGUF:UD-Q8_K_XL` or
  `pill pull hf.co/unsloth/Muse-Glimmer-30B-GGUF:UD-IQ2_XXS`.

## flint (RecursiveSI) IQ2_XXS

- Recorded result on build 11669: three Tier 1 runs on 23 October, all failed at load. Tier 2 never ran on that build. The
  corrected build 11804 runs, in isolated roots, are in [the Flint report](recursive-flint-iq2xxs.md) (Tier 2 0–22 of 22; not adopted).
- The GGUF declares `general.architecture = flint`. The llama-server in build 11669 has no flint support, and a direct load on
  25 October failed with the same `unknown model architecture: 'flint'` in under 0.3 s.
- Relaxing the Tier 1 limits would not change this. A gate run needs an engine that supports the architecture, so check that
  before scheduling one.

## Ternary Bonsai 2 27B (PrismML), PQ2_0, 2026-10-24

Not adopted. It is slower than the default and no better in quality.

- **Model and engine:** `prism-ml/Ternary-Bonsai-2-27B-gguf`, `Ternary-Bonsai-2-27B-PQ2_0.gguf` (7.2 GB,
  about 2 bits per weight, dense 27B). Stock llama.cpp cannot load it; it ran on PrismML's fork, release
  `prism-b10787` (build 10787), through `PILL_LLAMA_SERVER`. Thinking off, the model card's non-thinking
  sampling (temp 0.7, top-p 0.8, top-k 20, presence penalty 1.5), 64k context.
- **Gate at default limits:** fails Tier 1 on time (141 s against the 60 s limit); a dense model reads all
  its weights for every token, and most of that is prefill of Pi's 7,400-token prompt at about 55 tok/s.
- **Gate with `--tier1-max-seconds 180`, 5 runs:** 0 of 5 passed.

| | MiniCPM (default) | Ternary Bonsai 2 27B |
|---|---|---|
| Gate passes | 5 of 5 | 0 of 5 |
| Tier 1 time | 21–28 s | 137–144 s |
| Tier 2 accuracy | 22/22 every run | 22, 21, 20, 22, 22 (mean 21.4) |
| Tier 2 time | 4.7 min mean (slowest 8.9) | hit the 15 min cap in all 5 |
| Output tokens per run | 7,334 | about 14,600 |
| Decoding speed | about 25 tok/s | about 10 tok/s |
| Min free memory | 22–27% | 16–19% |

- **Latency:** not only the cold start. Warm decoding is 10 tok/s, so it is not faster once loaded.
- **Quality:** it never finished Tier 2 inside the cap. Two runs repeated the same tool call 19 and 23
  times in a row, and three runs reached 22/22 but kept working until the cap. An earlier single run that
  finished in 5.4 min (20/22) did not reproduce.
- **Not tried:** the smaller `PTQ1_0` file (5.9 GB); PrismML measures it slower at reading prompts, so it
  would not fix the cold start. A faster Mac (M4 Pro or later) might fit the Tier 1 limit but would not
  change the decoding speed or the Tier 2 behaviour.
