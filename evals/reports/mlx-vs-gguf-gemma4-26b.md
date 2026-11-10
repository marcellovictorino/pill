# Gemma 4 26B-A4B: mlx-lm 4-bit against the llama.cpp champion (one-off)

## TLDR

**Verdict:** Not decided. mlx-lm is faster on Tier 2 (555 s against 709 s) and scored the same, but it failed pill's memory gate: free memory fell to 5% against a 10% limit, where the llama.cpp champion stayed at 12%. One cold run each is too little to call the difference.

| Check | mlx-lm 4-bit | llama.cpp UD-IQ4_XS (champion) |
|---|---|---|
| Tier 1 gate | pass (1 of 1) | pass (1 of 1) |
| Tier 2 (22 tasks) | 22/22 in 555 s, 11 tool calls | 22/22 in 709 s, 34 tool calls |
| Tier 1 agent-sized request | 35.4 s (limit 60 s) | 36.1 s (limit 60 s) |
| Time to ready, cold | 2.0 s | 10.1 s |
| Decode speed | 25.1 tok/s wall, 58 tokens only (rough) | 22.0 tok/s wall, 23.2 tok/s engine, 128 tokens |
| Memory pressure | lowest free 5% over the run, 10% in Tier 1; swap +988 MB; pressure level 2 | lowest free 12% over the run, 18% in Tier 1; swap +0 MB; pressure level 2 |
| Gate | fail (memory) | pass |

**Against the champion:** the champion passes every gate on this run. mlx-lm is about 22% faster on Tier 2 but fails the memory gate.

## Details

- **Setup:** Pi, with the same provider settings pill writes (openai-completions, qwen-chat-template, max_tokens). Sampling as in the catalog: temp 1.0, top-p 0.95, top-k 64, min-p 0.0. Context 65,536. Prompt cache size 1 for mlx-lm. mlx-lm 0.32.2 (Homebrew). GGUF file 13.6 GB, mlx-lm weights 15.3 GB.
- **Runs:** one cold run per engine, each started by hand by `evals/engine_compare.py`, not through pill's router. Each run's engine process was started and stopped by that script. The script's guard found no other model loaded before either run.
- **Caveats:**
  - One run each is not enough to rank the two. Tier 2 times and memory figures can move by tens of seconds and several points between runs.
  - The mlx-lm memory dip may not come from the model alone. Other apps were running, and pill's router still had a sleeping model process, so the free-memory reading is the whole machine's, not the model's share.
  - mlx-lm ignores the forced-length flag, so its decode probe stopped at 58 of 128 tokens. Its decode figure is not comparable with the llama.cpp figure.
  - mlx-lm reports no engine-side speed, so only wall-clock tok/s exists for it.
  - Tier 3 was not run.
  - The one-run figures above were not repeated; the three-round series below replaces them as the comparison.

## Three cold rounds per engine (10 Nov 2026)

Each engine ran three cold rounds through `evals/engine_compare.py`, interleaved (mlx-lm 1, llama.cpp 1, and so on), one engine at a time. Both engines were started after a pressure check and the guard found nothing else loaded. The machine was already at memory pressure warning (level 2, 49% free) when the series began, from other apps, so the free-memory gate was tight for both engines.

| Round | Engine | Tier 1 time (limit 60 s) | Tier 1 lowest free (limit 10%) | Tier 2 result | Tier 2 time | Tier 2 tool calls | Lowest free, whole run | Swap growth | Peak pressure | Decode tok/s (128 tokens) |
|---|---|---|---|---|---|---|---|---|---|---|
| 1 | mlx-lm | 41.3 s | 2% (fail) | not run | — | — | 2% | +9,518 MB | 4 (critical) | 36.0 |
| 1 | llama.cpp | 37.9 s | 10% | 22/22, hit the 15 min cap, 251 identical repeats | 900 s | 267 | 8% | +972 MB | 2 | 25.4 |
| 2 | mlx-lm | 37.4 s | 7% (fail) | not run | — | — | 7% | +1,948 MB | 2 | 25.3 |
| 2 | llama.cpp | 43.0 s | 10% | 22/22 | 174.5 s | 9 | 9% | +1,181 MB | 2 | 21.1 |
| 3 | mlx-lm | 31.7 s | 7% (fail) | not run | — | — | 7% | +1,763 MB | 2 | 36.0 |
| 3 | llama.cpp | 33.3 s | 11% | 22/22, hit the 15 min cap, 291 identical repeats | 900 s | 305 | 9% | +486 MB | 2 | 24.8 |

- **Gates:** mlx-lm failed Tier 1 on free memory in all three rounds, so Tier 2 never ran. The llama.cpp champion passed Tier 1 in all three rounds, but failed Tier 2 in rounds 1 and 3 (time cap on repeated tool calls) and failed the whole-run memory gate in all three (8% to 9%, limit 10%). Neither engine passed every gate in any round.
- **Decode tok/s:** both engines were timed on the same streamed 400-word prompt, first to last token, capped at 128 tokens. For llama.cpp the engine's own reading matches the wall-clock figure to within 0.1 tok/s (25.37, 21.12, 24.83). For mlx-lm, which ignores the forced-length flag, the probe ran to the full 128 tokens here, and the rate is wall-clock only. These rates were measured under memory pressure and with swap growing, so they are not clean speeds.
- **Pressure:** the driver's pre-round check allowed every round to start, since none was worse than the start reading. The mlx-lm round 1 dropped to critical (level 4) during Tier 1, which the pre-round check does not catch.

**Verdict:** No winner. Neither engine passed every gate in any round: mlx-lm failed pill's Tier 1 memory gate every time, and the llama.cpp champion failed on Tier 2 time or memory, with repeated tool calls in two rounds. The three rounds do not rank the two engines.

## Sources

Raw results are in the one-off results folder (kept outside the repository): `mlx-gemma4-26b/result.json` and `gguf-gemma4-26b-ud-iq4xs/result.json`, with each run's transcripts and verifier output. The script is `evals/engine_compare.py`. The three-round series is in the one-off results folder, `engine-rounds/<engine>-r<round>/result.json`, one folder per round.
