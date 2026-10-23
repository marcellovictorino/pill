# RecursiveSI Flint 39.8B MoE, IQ2_XXS (12.03 GB)

**Setup:** `bartowski/RecursiveSI_flint-GGUF` IQ2_XXS GGUF, 64k context, Pi 1.3.0 with thinking off, pill 0.4.1, 24 GB Apple Silicon Mac. The installed llama.cpp build 11669 could not load Flint, so the three measured runs used upstream llama.cpp b11804 in an isolated pill root and port; the owner's service and default were not changed. **Runs:** three cold Tier 1 attempts on 2026-10-23 UTC. Tier 2 was skipped after Tier 1 failed. No Tier 3 runs.

## In one paragraph

Flint is not a champion candidate on this machine in the tested configuration. The model loaded and Pi tool calls returned the requested token in all three runs, but all three Tier 1 attempts failed because the separate 64-token agent-sized chat request returned an empty visible reply. Minimum free memory was 12% (above pill's 10% threshold), but the first cold run grew swap by 7.13 GB. Tier 2 and Tier 3 were therefore not run; there are no task-quality scores to compare with MiniCPM.

## Model discovery and fit

Hugging Face API metadata and repository file trees were checked on 2026-10-23 UTC. Dates below are API `createdAt` / `lastModified` in UTC; sizes are repository file sizes. Flint's source is a text-only `FlintForCausalLM` MoE with 39,785,287,456 parameters (about 39.8B total / 2.5B active), 2 dense and 47 MoE layers, 256 routed experts with top-6 routing, 35 sliding-window and 14 full-attention layers, and documented 262,144-token context. No projector file is listed or needed for this text benchmark.

| Repository | URL | Created / modified (UTC) | Relevant fit details |
|---|---|---|---|
| RecursiveSI source | [RecursiveSI/flint](https://huggingface.co/RecursiveSI/flint) | 2026-10-21 14:20:58 / 14:24:03 | BF16, 79,587,116,933 bytes; too large for this machine. |
| ggml-org conversions | [ggml-org/Flint-GGUF](https://huggingface.co/ggml-org/Flint-GGUF) | 2026-10-22 12:16:33 / 12:19:07 | BF16 ~79.6 GB, Q8_0 ~42.4 GB, Q4_K_M ~23.4 GB; none leaves room for cache/runtime. |
| bartowski conversions | [bartowski/RecursiveSI_flint-GGUF](https://huggingface.co/bartowski/RecursiveSI_flint-GGUF) | 2026-10-22 06:13:36 / 07:41:08 | IQ2_XXS 12,029,007,072 bytes; IQ2_XS 12,742,325,472; IQ2_S 13,394,982,112; IQ2_M 14,883,566,816. README describes IQ2_XXS as very low quality but usable and reports model-specific imatrix/layout work. No projector in file list. |
| Unsloth conversions | [unsloth/flint-GGUF](https://huggingface.co/unsloth/flint-GGUF) | 2026-10-20 03:26:14 / 2026-10-21 23:54:26 | UD-IQ1_M 11,352,241,600 bytes; UD-IQ2_XXS 11,855,541,696; UD-IQ3_XXS 14,535,427,520; UD-Q4_K_XL 25,054,688,704. No projector listed. |
| LM Studio Community conversions | [lmstudio-community/flint-GGUF](https://huggingface.co/lmstudio-community/flint-GGUF) | 2026-10-20 23:43:57 / 2026-10-21 15:47:03 | Q4_K_M/Q6_K/Q8_0; Q4_K_M alone is 24,166,841,184 bytes, so it does not fit with context/runtime. |
| RecursiveSI INT4 | [RecursiveSI/flint-int4-gguf](https://huggingface.co/RecursiveSI/flint-int4-gguf) | created date not returned by API / 2026-10-21 15:54:08 | Official Q4_1 is 25,992,966,976 bytes; card says Apple Silicon 48 GB+; too large here. |
| RecursiveSI NVFP4 | [RecursiveSI/flint-nvfp4-gguf](https://huggingface.co/RecursiveSI/flint-nvfp4-gguf) | 2026-10-21 13:51:14 / 15:44:41 | 22,863,084,160 bytes and NVIDIA-focused; too large for safe 24 GB Apple Silicon use. |
| Other GGUFs | [romanerm/flint-GGUF](https://huggingface.co/romanerm/flint-GGUF), [terelabs/flint-GGUF](https://huggingface.co/terelabs/flint-GGUF) | 2026-10-22 14:41:39 / 16:32:22; 2026-10-21 17:25:39 / 20:39:34 | Listed IQ4/Q4/Q5 variants are about 22 GB or larger; not shortlisted. |

At 64k, an approximate fp16 KV cache is 1.79 GiB: about 1.75 GiB for full-attention layers plus 0.04 GiB for sliding-window layers. This excludes runtime/scratch buffers and other apps. Approximate weight + KV budgets are:

| Best-first shortlist | File size | Approx. weights + 64k fp16 KV | Rationale / test status |
|---|---:|---:|---|
| bartowski IQ2_XXS | 12.03 GB | 13.0 GiB | Model-specific imatrix/layout; selected first and tested. |
| Unsloth UD-IQ2_XXS | 11.86 GB | 12.8 GiB | Smaller dynamic alternative; not tested. |
| Unsloth UD-IQ1_M | 11.35 GB | 12.4 GiB | Smallest plausible file found, with higher quality risk; not tested. |

These are estimates, not safe-fit guarantees. At discovery the machine had 45% free memory and 4.30 GiB swap in use; the benchmark below is the observed fit evidence.

## Tier 1 and 2 results

The installed `/opt/homebrew/bin/llama-server` was build 11669. Its first `pill bench run` attempt failed at model load with `unknown model architecture: 'flint'`; no inference ran, Tier 2 was skipped, minimum free memory was 43%, swap growth 0, and speed was 0. Upstream llama.cpp PR [#30928](https://github.com/ggml-org/llama.cpp/pull/30928) added `FlintForCausalLM` and merged at 2026-10-21 21:26:22 UTC; the official [b11804 macOS arm64 release](https://github.com/ggml-org/llama.cpp/releases/tag/b11804), published at 2026-10-22 01:26:20 UTC, was then tested in an isolated pill root/port. It loaded the same model successfully. The compatibility build was downloaded and SHA-256 checked against the release digest.

| Run (b11804) | Tier 1 wall time | Agent-sized request | Pi tool-call check | Minimum free memory | Swap growth | Generation speed | Tier 2 |
|---|---:|---|---|---:|---:|---:|---|
| 1 | 43.7 s | Failed: empty reply after 64 generated tokens | Passed, 1 tool call | 12% | 7,129.98 MB | 32.65 tok/s | Skipped |
| 2 | 19.4 s | Failed: empty reply after 64 generated tokens | Passed, 1 tool call | 12% | 51.84 MB | 35.60 tok/s | Skipped |
| 3 | 19.3 s | Failed: empty reply after 64 generated tokens | Passed, 1 tool call | 12% | 59.57 MB | 36.21 tok/s | Skipped |

The model's live footprint observed during the b11804 run was approximately 12.9 GB resident, with about 13.8 GB reported as GPU/unified-memory use. System-wide sampled pressure peaked at level 2. The first-run swap jump is a measured whole-machine delta; later cold runs added about 52–60 MB each. All three requests completed under the 60-second wall-time cap, but the empty reply made the Tier 1 check fail every time. The Pi tool result was correct in all three attempts; that does not compensate for the failed direct reply check.

## Tier 3 scores per task

Not run: the candidate failed Tier 1, so the Tier 2 and Tier 3 gates were not entered. There are no automated scores, Opus-judge score, screenshots, or blind ratings to report.

| Task | Runs | Median | Range | Timeouts | Mean minutes |
|---|---:|---:|---:|---:|---:|
| `go-notes` | 0 | not run | — | — | — |
| `py-clean` | 0 | not run | — | — | — |
| `py-insight` (automated / Opus judge) | 0 | not run | — | — | — |
| `web-kanban` | 0 | not run | — | — | — |
| `web-landing-nursery` | 0 | not run | — | — | — |
| `web-landing-mars` | 0 | not run | — | — | — |
| `web-landing-tax` | 0 | not run | — | — | — |

## Gate decision and recommendation

**Gate 2: do not proceed to Tier 3.** Flint failed Tier 1 in 3/3 attempts. It did meet the memory floor (12% free versus the 10% minimum) and decoded at 32.7–36.2 tok/s, but had empty direct replies in every run and 7.13 GB swap growth on the first run. Tier 2 has no Flint score to compare: the current MiniCPM champion passed 5/5 gate runs with Tier 2 at 22/22, 22% minimum free memory, no swap growth, and 25 tok/s. Flint therefore has no evidence of matching the champion and is not better on the available gate evidence.

**Recommendation:** keep MiniCPM as the default; do not switch to Flint. The Tier 3 task-level comparison and visual quality remain untested, not negative scores. The installed server cannot load Flint; b11804 can load it but did not pass the direct-reply check. Whether a smaller quant would reduce memory pressure or fix the empty reply is unverified. A later retest should first establish a serving/configuration path that returns visible text on the Tier 1 agent-sized prompt, then rerun the pill gate before considering Tier 3.

The existing default was not changed. The only new model left registered is `recursivesi-flint-iq2xxs-64k` (`bartowski/RecursiveSI_flint-GGUF:IQ2_XXS`, 64k, unverified; state `failed`, not default). The downloaded GGUF remains in place. The ten pre-existing model registrations were left unchanged.

## Caveats

Three cold attempts are enough to establish a repeatable Tier 1 failure in this setup, not a general quality judgement. Tier 2 and Tier 3 were not run; no task scores, five-run Tier 3 statistics, Opus judge result, screenshots, or blind ratings exist. The candidate was evaluated only in IQ2_XXS; the other shortlisted quants were not tested. Memory readings are system-wide and reflect the machine's other usage as well as the model. The 12.9 GB resident / 13.8 GB GPU-unified readings are live observations, not per-run peak fields in the saved benchmark JSON. The HF API did not return a created date for the official INT4 repo. Exact conversion quality beyond the README's claims was not independently evaluated.
