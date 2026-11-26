# llama.cpp 1.0.0 (build 12359) against build 12110, 24 GB M4

## TLDR

**Verdict:** llama.cpp 1.0.0 does not fail Tier 1 by itself. On a quiet machine MiniCPM-V 4.7 passed all 6 runs on 1.0.0, Gemma 4 passed 5 of 6 (the one miss was a 80 s first run, over the 60 s limit), and Qwen3.6 passed all 6, with no Metal error, compute error or HTTP 500 in any server log. The 21 and 22 Nov failures (Metal out of memory, HTTP 500) came from the machine's state then, most likely leftover model servers, not from the build. Nothing here makes 1.0.0 faster either: speed matches 12110 within run-to-run noise.

Quiet-machine runs, 23 and 24 Nov 2026: three batches per model (1.0.0, 12110, 1.0.0 again), 3 runs each, Tier 1 limit 60 s, free-memory floor 5%, nothing else loading a model (swap 2.5 to 9.3 GB in use during the runs).

| Tier 1, per run | MiniCPM-V 4.7 IQ2 (default), 64k | Gemma 4 26B-A4B UD-Q4_K_S, 32k | Qwen3.6 35B-A3B UD-Q3_K_S, 32k |
|---|---|---|---|
| 1.0.0, first batch | 36, 21, 21 s (pass) | 80 s (fail), 34, 28 s | 44, 22, 22 s (pass) |
| 12110 | 23, 23, 24 s (pass) | 37, 28, 30 s (pass) | 32, 22, 22 s (pass) |
| 1.0.0, last batch | 24, 30, 30 s (pass) | 37, 35, 39 s (pass) | 22, 22, 22 s (pass) |
| Lowest free memory (1.0.0 / 12110) | 10% / 13% | 5% / 5% | 9% / 9% |
| Generation speed (1.0.0 / 12110) | 19 to 25 / 25 tok/s | 21 / 22 tok/s | 27 / 27 tok/s |

Tier 2 was not measured for MiniCPM and Gemma: these runs capped it at 2 minutes to keep Tier 1 the focus, so their Tier 2 scores are not comparable with earlier records. For Qwen3.6, 12110 scored 22/22 in 13.3 minutes. The Tier 2 results and the CPU-expert table in the earlier version of this report (21 and 22 Nov, machine not quiet) are superseded; the 1.0.0 CPU-expert runs passed Tier 1 only at a 300 s limit and failed the 60 s limit regardless.

**Against the champion:** MiniCPM-V 4.7 IQ2 keeps passing Tier 1 on 1.0.0 at the catalog's 64k setting, with the same times as 12110 (21 to 36 s against 23 to 24 s). It stays the default.

## Details

- **Setup:** pill 0.6.0; llama.cpp 1.0.0 build 12359 (commit b1b770d90) from Homebrew against build 12110 (commit cd959bae9, official release, checksum checked); Pi 1.6.0 (1.5.2 on 21 and 22 Nov); macOS 27.2.1; 24 GB M4. Each batch ran through `pill bench run` with an isolated pill home and port, so the installed llama.cpp and the router were untouched. A sampler logged swap, free memory and other model servers every 30 s; none ran during the batches.
- **Free-memory floor:** the runs used `--min-free-pct 5`. pill's own default is 10%, and at 10% every Qwen3.6 batch and every Gemma batch reads as failed, on 12110 as well as on 1.0.0 (lowest free 5 to 9%). That is a memory-floor result, not a build difference. pill's default floor is unchanged in this task.
- **The 21 and 22 Nov failures:** the same Qwen3.6 model failed 4 of 4 runs at 17 to 18 s with Metal "Insufficient Memory" while 12% of memory was free, and the earlier Gemma runs also hit compute errors. The notes from those batches record model servers left running from earlier ones. The raw runs are no longer on this Mac, so the cause is not proven, but three models on a quiet machine, 24 runs, produced no such error.
- **One slow run:** Gemma's first 1.0.0 run took 80 s for about 7,900 prompt tokens, then 34 and 28 s. It came right after the MiniCPM batches with 6 GB of swap in use and a 6% free-memory low, so a cold, memory-starved first load is the likely cause. It happened once in six 1.0.0 runs and not in the repeat batch.
- **Expert cache (21 Nov, not repeated):** `LLAMA_ARG_MOE_CACHE_MIB` only applies to expert weights kept on the CPU. On that sample it did not clearly change memory or speed.
- **Caveats:** three runs per batch is a small sample. Swap was 2.5 to 9.3 GB in use, above pill's 4 GB gate; the batches were back to back, so drift would show between the two 1.0.0 batches (it did not for pass or fail). Only these three models were checked on a quiet machine; the GLM family named earlier was not run.

## Sources

Raw runs of 23 and 24 Nov are not in `~/.pill/benchmark`, which holds only the 21 and 22 Nov runs. The 12110 records from earlier are in `bigger-models.md`, `tried-and-rejected.md` and the MiniCPM catalog entry.
