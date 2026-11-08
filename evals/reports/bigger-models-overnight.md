# Bigger MoE models on the 24 GB M4: Gemma 4 26B-A4B Q4_K_S and Qwen3.6 35B-A3B Q3_K_S, 32k

## TLDR

**Verdict:** Not a good model, for either file. Neither beats the default: Gemma fails the memory gate in all 3 runs (6–7% free against 10%), and Qwen3.6 reaches 22/22 in only 2 of 3 runs, with the third hitting the 15-minute cap at 20/22.

| Check | Gemma 4 26B-A4B UD-Q4_K_S (16.5 GB) | Qwen3.6 35B-A3B UD-Q3_K_S (15.4 GB) |
|---|---|---|
| Tier 1 gate | fail, 0 of 3 (memory only) | pass, 3 of 3 |
| Tier 2 (22 tasks; champion bar 22/22) | not run (Tier 1 failed) | 22, 22, 20 |
| Tier 3 (7 tasks) | not run, gate failed | not run, gate failed |
| Quality | Tier 1 answers and Pi tool call correct in all 3 runs | 22/22 twice; 20/22 at the cap once |
| Time | Tier 1 26–27 s; 24.6 tok/s | Tier 1 20–21 s; Tier 2 14.6, 9.0 and 15.0 min; 29–30 tok/s |
| Memory pressure | lowest free 6, 7, 7%; swap up 729, 0, 3 MB | lowest free 11, 11, 9%; swap up 0 MB in all runs |

**Against the champion:** MiniCPM-V 4.7 IQ2 passes Tier 2 22/22 in 3.6 minutes at 25 tok/s with 14% free memory. Qwen3.6 Q3_K_S is as fast at decoding (29 tok/s) but 2.5 to 4 times slower to finish Tier 2, and missed once.

## Details

- **Setup:** llama.cpp build 12110, Pi with thinking off, 32k context, `pill bench run --runs 3` at the default limits (Tier 1 60 s and 10% free; Tier 2 15 min). Files from unsloth on Hugging Face; sizes checked on the Hub before pulling, and pill verified each sha256. The Mac was in normal use with swap already 5.6–6.3 GB used, which counts against the memory floor.
- **Nemotron 3.5 Lightning 30B-A3B Q4_0 (18.9 GB, ggml-org) was not run:** it needs `sudo sysctl iogpu.wired_limit_mb=20480`, which the captain could not run overnight. Filed as PILL-89.
- **Run on a different file than planned:** unsloth has no Gemma Q4_0, so UD-Q4_K_S (16.5 GB) stood in for "Gemma Q4". Qwen3.6 used UD-Q3_K_S (15.4 GB), the smallest Q3 file, rather than the 16.6 GB Q3_K_M.
- **Earlier results for the same models** (other quants, 64k) are in `tried-and-rejected.md`. These two files add the Q4_K_S and Q3_K_S points, not new models.
- **Caveats:** a quieter Mac would lift Gemma's free memory a few points, but it sat 3–4 points under the floor with 17.4 GB on the GPU, so it is unlikely to pass at 32k. Qwen's miss was at the time cap, with a 22/22 in 9.0 minutes in another run, so it is variance in agent speed, not a loop. Tier 3 was not run because neither passed Tier 2.

## Sources

Raw runs are in `~/.pill/benchmark`: `20261108T212659Z-gemma-4-26b-a4b-it-q4ks-32k` and `20261108T213451Z-qwen3-6-35b-a3b-q3ks-32k`. `pill bench summary` ranks them 7th and 8th of 8.
