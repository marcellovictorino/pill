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
