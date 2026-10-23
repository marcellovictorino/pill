# RecursiveSI Flint 39.8B MoE, IQ2_XXS (12.03 GB)

**Setup:** `bartowski/RecursiveSI_flint-GGUF` IQ2_XXS GGUF, 64k context, Pi 1.3.0 with thinking off, pill 0.4.1, 24 GB Apple Silicon Mac. The corrected three-run gate used upstream llama.cpp b11804 in an isolated pill root and port, with `--reasoning-budget 0`; the owner's router/default were not changed. **Runs:** three cold Tier 1 and Tier 2 runs on 2026-10-23 UTC. Tier 3 was not run because Tier 2 did not reach the MiniCPM champion's 22/22 score.

## In one paragraph

The serving/template issue behind Flint's initial empty reply was confirmed and corrected, but Flint still is not a champion candidate in this test. With llama.cpp b11804's `--reasoning-budget 0`, Pill's Tier 1 gate passed 3/3 runs (17.4–28.9 s, working Pi tool turns, and at least 11% free memory); the informational agent-sized answer was correct in only 1/3. Tier 2 scored 0/22, 13/22 and 19/22, versus MiniCPM's 22/22 champion result, and the machine added 172–638 MB of swap in each full run. Tier 3 was therefore not run. There is no Flint Tier 3 evidence to compare, and the recommendation is to keep MiniCPM as default.

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

These are estimates, not safe-fit guarantees. The tested machine was already under other-app memory pressure: system swap was about 10.7 GB at the start of the corrected series. The measured results below are the fit evidence.

## Tier 1 and 2 results

The installed `/opt/homebrew/bin/llama-server` was build 11669. Its first attempt failed to load the architecture (`unknown model architecture: 'flint'`), so no inference ran. Upstream llama.cpp PR [#30928](https://github.com/ggml-org/llama.cpp/pull/30928) added `FlintForCausalLM` and merged at 2026-10-21 21:26:22 UTC; the official [b11804 macOS arm64 release](https://github.com/ggml-org/llama.cpp/releases/tag/b11804), published at 2026-10-22 01:26:20 UTC, loaded the model. The compatibility build was downloaded and SHA-256 checked against the release digest, then used only in an isolated pill root/port.

### Raw-response diagnosis and corrected setting

Before the corrected benchmark, three b11804 attempts using the normal `enable_thinking:false` request setting returned an empty visible `message.content`. A raw chat-completions response showed the model had spent the 64-token output budget in `reasoning_content`, with `finish_reason="length"` and only a partial answer; the loaded chat template did not consume the `enable_thinking` kwarg. Repeating the same probe with the upstream server option `--reasoning-budget 0` returned the answer in visible content and stopped after 27 tokens. This identified a serving/template configuration issue, not a missing model or unsupported API. No pill Go source was changed.

The superseded pre-correction outcomes are retained here for completeness; Tier 2 was skipped for all three because the visible-reply check failed:

| Baseline run (b11804, no reasoning-budget option) | Tier 1 wall time | Direct visible reply | Pi tool-call check | Minimum free | Swap growth | Throughput probe | Tier 2 |
|---|---:|---|---|---:|---:|---:|---|
| 1 | 43.7 s | Empty after 64 generated tokens | Passed (1 tool call) | 12% | 7,129.98 MB | 32.65 tok/s | Skipped |
| 2 | 19.4 s | Empty after 64 generated tokens | Passed (1 tool call) | 12% | 51.84 MB | 35.60 tok/s | Skipped |
| 3 | 19.3 s | Empty after 64 generated tokens | Passed (1 tool call) | 12% | 59.57 MB | 36.21 tok/s | Skipped |

The corrected three-run `pill bench run recursivesi-flint-iq2xxs-64k --runs 3` used that server option. Pill marks the informational direct-answer correctness check separately from the Tier 1 pass boolean; all 3 runs passed Pill's Tier 1 gate, but only run 1 returned the correct agent-sized answer. Run 2's informational answer was just `2`; run 3 used the output budget on an unfinished response. All three completed within 60 seconds, passed the Pi tool-call check, and met the 10% memory threshold in the Tier 1 gate reading. The table's min-free and swap figures are from the full per-run memory samples; those samples saw 11% minimum free in every run, while the Tier 1 checks recorded 12%, 11% and 13% respectively.

| Corrected run (b11804, reasoning budget 0) | Tier 1 wall time | Informational direct answer | Pi tool-call check | Full-run min free | Full-run swap growth | Sampled peak router-tree RSS / GPU memory | Tier 1 throughput probe | Tier 2 |
|---|---:|---|---|---:|---:|---:|---:|---|
| 1 | 28.9 s | Correct (`68`) | Passed | 11% (Tier 1 check: 12%) | 638.26 MB | 8.36 / 15.25 GB | 34.73 tok/s | 0/22 in 15:00; workspace incomplete (`engine.mjs` and `cli.mjs` absent), hit cap |
| 2 | 17.4 s | Incorrect (`2`) | Passed | 11% (Tier 1 check: 11%) | 171.89 MB | 13.31 / 15.35 GB | 34.69 tok/s | 13/22 in 15:00; failed verifier checks and hit cap |
| 3 | 19.0 s | Incorrect / unfinished within output budget | Passed | 11% (Tier 1 check: 13%) | 318.12 MB | 13.27 / 15.25 GB | 36.29 tok/s | 19/22 in 11:32; completed with verifier failures |

Memory uses decimal GB. Router-tree RSS peaks were sampled about once per second; `pill ps` GPU/unified-memory readings were sampled about once per five seconds. Free-memory and swap figures are system-wide (including other apps), not attributable to Flint alone. The maximum sampled pressure level was 2. The corrected runs used about 41.5 minutes in Tier 2 total, in addition to about 65 seconds of Tier 1 elapsed time and model startup; this is three gate attempts, not a Tier 3 cost estimate.

The initial b11804 runs without the reasoning-budget setting were superseded by the corrected run above: their empty visible reply was traced to the template/reasoning-budget behavior, not a valid final gate verdict. Their first cold run grew swap by 7.13 GB; later runs grew it by about 52–60 MB. The corrected configuration fixes empty visible content only on the first of its three agent-sized attempts and does not fix the Tier 2 quality/reliability shortfall.

## Tier 3 scores per task

**Not run. Gate 2 decision: do not proceed.** The current MiniCPM champion passed 5/5 Tier 1/2 gate runs and scores 22/22 in Tier 2. Corrected Flint Tier 2 scores were 0/22, 13/22 and 19/22: none reached the champion's 22/22, so the candidate did not satisfy the required promising-result gate. The corrected memory samples also showed 172–638 MB of swap growth and only 11% minimum free memory per full run. No Tier 3 run, screenshot, blind rating or Opus judge result exists.

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

**Tier 1:** Pill gate passed 3/3 in the corrected configuration (17.4–28.9 s; all Pi tool checks passed; Tier 1 minimum-free readings 11–13%, above the 10% floor). The direct-answer correctness check is informational and succeeded in only 1/3 runs. The 34.7–36.3 tok/s field is Pill's throughput probe, not the speed of each complete agent task.

**Tier 2 / Gate 2:** failed to match the champion: 0/22 in 15:00, 13/22 in 15:00 and 19/22 in 11:32, versus MiniCPM's 22/22. Whole-run minimum free memory was 11% in all three samples, with 172–638 MB swap growth. Consequently, Tier 3 was not run. This is an explicit stop at the Tier 2 gate, not a Tier 3 score of zero.

**Recommendation:** keep `minicpm-v-4-7-35b-a3b-ad-iq2-xs-iq2-xxs-64k` as the default; do not switch to Flint. Flint's measured token throughput is higher than MiniCPM's 25 tok/s, but the three Tier 2 runs did not equal the champion and Tier 1's informational answer was inconsistent. Whether another Flint quant or additional context/runtime tuning could change its quality or memory behavior is unverified. The report cannot say how Flint compares on the seven Tier 3 task domains because that stage was gated off.

The existing default was not changed. The new model registration is `recursivesi-flint-iq2xxs-64k` (`bartowski/RecursiveSI_flint-GGUF:IQ2_XXS`, 64k; marked failed by the benchmark, not default). The retained model file is `/Users/marcellovictorino/.pill/models/RecursiveSI_flint-IQ2_XXS.gguf` (12,029,007,072 bytes); other shortlisted GGUFs were not downloaded. The ten pre-existing registrations were unchanged. Registry names at handoff:

- `devstral-small-2-24b-instruct-2512-q3-k-m-32k`
- `devstral-small-2-24b-instruct-2512-q3-k-m-64k`
- `devstral-small-2-24b-instruct-2512-ud-q3-k-xl-64k`
- `gemma-4-26b-a4b-it-ud-iq3-s-64k`
- `gemma4-26b-q3kxl-64k`
- `glm-4-7-flash-ud-iq2-m-64k`
- `minicpm-v-4-7-35b-a3b-ad-iq2-xs-iq2-xxs-64k` (default)
- `ornith-1-5-35b-a3b-ad-iq3-xxs-iq2-s-64k`
- `qwen3-6-35b-a3b-ud-iq2-m-64k`
- `qwen3-8-27b-ud-iq3-xxs-64k`
- `recursivesi-flint-iq2xxs-64k` (failed; not default)

The isolated b11804 router was stopped after benchmarking. The downloaded compatibility runtime and diagnostic scripts remain in the authorized task scratch directory; no Tier 3 screenshots were produced.

## Caveats

The gate evidence is three cold runs of one quant, not a multi-run Tier 3 comparison. The per-run memory samples describe whole-machine pressure and swap, while the router-tree RSS is an external one-second sample; GPU memory is `pill ps` sampled about every five seconds. The 12.03 GB file is bartowski IQ2_XXS only. Other shortlisted quants were not tested. The HF API did not return a created date for the official INT4 repo. Exact conversion quality beyond the README's claims was not independently evaluated. Tier 3 scores, visual outputs, blind ratings and Opus-judge scores are unverified because Tier 3 was not run.
