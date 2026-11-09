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
  - The mlx-lm run and the GGUF run were not repeated.

## Sources

Raw results are in the one-off results folder (kept outside the repository): `mlx-gemma4-26b/result.json` and `gguf-gemma4-26b-ud-iq4xs/result.json`, with each run's transcripts and verifier output. The script is `evals/engine_compare.py`.
