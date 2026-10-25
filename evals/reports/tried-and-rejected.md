# Tried and not adopted

Models that were benchmarked on this machine (24 GB M4) and did not beat the current default, so nobody
spends time re-running them. Re-test only after a significant change: a new release of the model, a
different quantisation, a faster machine, or an engine fix that changes the cause below.

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
