# llama.cpp follow-up batch: DSpark wall clock, MiniCPM sampling and Qwen thinking off (28 Nov)

## TLDR

**Verdicts:**
- **DSpark drafter (MiniCPM, 7 tokens ahead): Undecided.** Tier 2 averaged 250 s against 276 s for MiniCPM base over five pairs, but the medians were 252 s and 225 s, and single runs ranged from 73 s to 577 s. Decode is about 27% faster per token (30.9 against 24.4 tok/s), so the wall-clock result depends on how many tokens the agent writes. Next: five more pairs, reporting tokens per run beside the time.
- **Model-card sampling (MiniCPM): Undecided.** Two of three runs scored 22/22, against two of five for the base arm. Three runs are too few to call it. Next: five more sampling runs.
- **Thinking off with 32k replies (Qwen3.6): No, for now.** None of three runs passed the gate: 21/22, 22/22 at the 15-minute cap with free memory down to 1%, and 21/22.
- **Ngram-mod:** not in this batch. The verdict from the retest stands (no).

| Check | Result |
|---|---|
| Tier 1 gate | Passed in every run of every arm |
| Tier 2 (22 tasks; champion bar 22/22) | DSpark 7: 22, 22, 22, 22, 22. MiniCPM base: 22, 20, 22, 17, 17. Sampling: 17, 22, 22. Qwen thinking off: 21, 22 (capped), 21 |
| Tier 2 gate passes | DSpark 7: 4 of 5 (run 5 failed on memory). MiniCPM base: 2 of 5. Sampling: 2 of 3. Qwen thinking off: 0 of 3 |
| Time (Tier 2 mean, median) | DSpark 7: 250 s, 252 s. MiniCPM base: 276 s, 225 s. Sampling: 316 s, 321 s. Qwen thinking off: 502 s, 406 s |
| Prompt reading against decode | Prompt reading was 7 to 76 s per run (mean 13 to 40 s), against 64 to 793 s of decode. Decode is about 95% of Tier 2 time on every arm |
| Memory pressure | Lowest free memory: DSpark 4% (run 5), Qwen 1% (run 2). Both fell below the 5% floor. Swap growth up to 1.7 GB, in MiniCPM base run 1 and the two low-memory runs |

**Against the champion:** MiniCPM-V 4.7 IQ2 base (default) scored 22/22 in two of five runs in this batch and 22/22 in the retest's clean run. Its Tier 2 misses (20/22 and 17/22) are new in this batch.

## Details

- **Setup:** Pi with no extensions, skills or context files; 24 GB M4; llama.cpp 1.1.0 (build 12774); pill 0.6.0. All four arms at 64k context. DSpark drafter: `dspark-MiniCPM-V-4_7-35B-A3B-Q8_0.gguf` (`spec-type draft-dspark`, 7 tokens ahead). Sampling: temperature 0.7, top-p 0.95, top-k 100, repeat penalty 1.03. Qwen: `enable_thinking` false, 32,768 max tokens.
- **Runs:** each arm ran one cold start at a time, serially, with pill's bench lock held and nothing else loading a model. Gate: Tier 1 300 s, Tier 2 15 minutes, free memory 5%.
- **Prompt reading and decode, per Tier 2 run (seconds):**

| Arm | Run | Tier 2 | Prompt reading | Decode | Tokens generated | Decode tok/s | Turns | Lowest free |
|---|---|---|---|---|---|---|---|---|
| DSpark 7 | 1 | 258 (22/22) | 10.2 | 243.8 | 6,610 | 27.1 | 24 | 19% |
| DSpark 7 | 2 | 73 (22/22) | 6.8 | 64.0 | 2,536 | 39.6 | 14 | 19% |
| DSpark 7 | 3 | 577 (22/22) | 24.1 | 544.1 | 14,098 | 25.9 | 51 | 12% |
| DSpark 7 | 4 | 87 (22/22) | 10.8 | 72.5 | 2,665 | 36.7 | 13 | 13% |
| DSpark 7 | 5 | 252 (22/22, memory fail) | 29.9 | 211.0 | 5,310 | 25.2 | 29 | 4% |
| MiniCPM base | 1 | 504 (22/22, swap 1.7 GB) | 15.5 | 482.9 | 11,627 | 24.1 | 31 | 6% |
| MiniCPM base | 2 | 225 (20/22) | 14.1 | 207.0 | 5,337 | 25.8 | 23 | 35% |
| MiniCPM base | 3 | 228 (22/22) | 15.0 | 209.5 | 5,386 | 25.7 | 25 | 33% |
| MiniCPM base | 4 | 218 (17/22) | 12.1 | 199.5 | 4,621 | 23.2 | 20 | 19% |
| MiniCPM base | 5 | 206 (17/22) | 8.5 | 192.5 | 4,450 | 23.1 | 20 | 16% |
| Sampling | 1 | 370 (17/22) | 22.0 | 343.2 | 8,297 | 24.2 | 32 | 21% |
| Sampling | 2 | 321 (22/22) | 21.6 | 294.8 | 7,127 | 24.2 | 27 | 18% |
| Sampling | 3 | 256 (22/22) | 10.0 | 241.9 | 6,012 | 24.8 | 21 | 17% |
| Qwen thinking off | 1 | 406 (21/22) | 29.3 | 366.6 | 11,142 | 30.4 | 49 | 16% |
| Qwen thinking off | 2 | 900 (22/22, capped) | 76.1 | 793.0 | 21,350 | 26.9 | 110 | 1% |
| Qwen thinking off | 3 | 199 (21/22) | 14.6 | 179.0 | 5,661 | 31.6 | 31 | 24% |

- **Reading the split:** prompt reading is small on every arm, because most of each prompt is reused from the cache. The slower DSpark run (577 s) was slow because it wrote 14,098 tokens over 51 turns, not because its prompt reading or decode rate was worse (25.9 tok/s, the same as base).
- **Why the speed probe overstated DSpark:** the retest's 74 tok/s came from a fixed 128-token reply. In Tier 2, DSpark's decode rate ranged from 25.2 to 39.6 tok/s.
- **Caveats:**
  - DSpark run 5 scored 22/22 but dipped to 4% free memory, so the gate counts it as a fail.
  - Qwen run 2 reached 22/22 at the 15-minute cap, with free memory down to 1% and 1.0 GB of swap growth. Pill reported it as a time cap, not a memory failure.
  - MiniCPM base run 1 had 1.7 GB of swap growth and 6% free memory, so its 504 s is not a clean comparison.
  - Three further folders from this batch carry partial Tier 2 workspaces and no result file. They are not counted.
  - Not run: Ngram-mod with these arms, DSpark with 3 tokens ahead, and the personal Tier 3.

## Sources

The raw run folders, the router log and the per-run summaries are kept outside the repository, in the task's evidence folder. Nothing is quoted from personal runs.
