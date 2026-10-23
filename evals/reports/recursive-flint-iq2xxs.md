# RecursiveSI Flint 39.8B MoE: Bartowski IQ2_XXS and Unsloth UD-IQ2_XXS

**Setup:** Two 64k GGUF variants from `bartowski/RecursiveSI_flint-GGUF` and `unsloth/flint-GGUF`; Pi 1.3.0 with thinking off; pill 0.4.1; 24 GB Apple Silicon Mac. Each was pulled and registered only in an isolated pill root, then tested sequentially with upstream llama.cpp b11804 and `--reasoning-budget 0`; only one model was loaded at a time. **Runs:** three cold Tier 1 and Tier 2 runs per quant on 2026-10-23 UTC. Tier 3 was not run for either: neither quant achieved a stable 22/22 Tier 2 result to match MiniCPM.

## In one paragraph

The initial empty visible reply was traced to Flint's chat template ignoring `enable_thinking:false`; b11804's `--reasoning-budget 0` returned visible text in a raw probe. With that correction, both variants passed Pill's Tier 1 gate 3/3, but their informational agent-sized answers were correct only once in three runs. Bartowski IQ2_XXS scored 0/22, 13/22 and 19/22 in Tier 2; Unsloth UD-IQ2_XXS scored 22/22, 17/22 and 19/22, with the last run hitting the 15-minute cap after repeating a tool call 181 times. Unsloth had the better single Tier 2 result, but passed the champion score in only one run; neither met the stable gate. Keep MiniCPM as default. No Tier 3 result exists for either Flint quant.

## Model discovery and fit

Hugging Face API metadata and repository file trees were checked on 2026-10-23 UTC. Dates below are API `createdAt` / `lastModified` in UTC; sizes are repository file sizes. Flint's source is a text-only `FlintForCausalLM` MoE with 39,785,287,456 parameters (about 39.8B total / 2.5B active), 2 dense and 47 MoE layers, 256 routed experts with top-6 routing, 35 sliding-window and 14 full-attention layers, and documented 262,144-token context. No projector file is listed or needed for this text benchmark.

| Repository | URL | Created / modified (UTC) | Relevant fit details |
|---|---|---|---|
| RecursiveSI source | [RecursiveSI/flint](https://huggingface.co/RecursiveSI/flint) | 2026-10-21 14:20:58 / 14:24:03 | BF16, 79,587,116,933 bytes; too large for this machine. |
| ggml-org conversions | [ggml-org/Flint-GGUF](https://huggingface.co/ggml-org/Flint-GGUF) | 2026-10-22 12:16:33 / 12:19:07 | BF16 ~79.6 GB, Q8_0 ~42.4 GB, Q4_K_M ~23.4 GB; none leaves room for cache/runtime. |
| bartowski conversions | [bartowski/RecursiveSI_flint-GGUF](https://huggingface.co/bartowski/RecursiveSI_flint-GGUF) | 2026-10-22 06:13:36 / 07:41:08 | IQ2_XXS 12,029,007,072 bytes; IQ2_XS 12,742,325,472; IQ2_S 13,394,982,112; IQ2_M 14,883,566,816. README describes IQ2_XXS as very low quality but usable and reports model-specific imatrix/layout work. No projector in file list. |
| Unsloth conversions | [unsloth/flint-GGUF](https://huggingface.co/unsloth/flint-GGUF) | 2026-10-20 03:26:14 / 2026-10-21 23:54:26 | UD-IQ1_M 11,352,241,600 bytes; UD-IQ2_XXS 11,855,541,696; UD-IQ3_XXS 14,535,427,520; UD-Q4_K_XL 25,054,688,704. No projector listed. The tested file was `flint-UD-IQ2_XXS.gguf`; SHA-256 `c1fa4a6d5ec67fbeff460c68829984e4aa3eea95dc9d2067691f19e5406deeee`. |
| LM Studio Community conversions | [lmstudio-community/flint-GGUF](https://huggingface.co/lmstudio-community/flint-GGUF) | 2026-10-20 23:43:57 / 2026-10-21 15:47:03 | Q4_K_M/Q6_K/Q8_0; Q4_K_M alone is 24,166,841,184 bytes, so it does not fit with context/runtime. |
| RecursiveSI INT4 | [RecursiveSI/flint-int4-gguf](https://huggingface.co/RecursiveSI/flint-int4-gguf) | created date not returned by API / 2026-10-21 15:54:08 | Official Q4_1 is 25,992,966,976 bytes; card says Apple Silicon 48 GB+; too large here. |
| RecursiveSI NVFP4 | [RecursiveSI/flint-nvfp4-gguf](https://huggingface.co/RecursiveSI/flint-nvfp4-gguf) | 2026-10-21 13:51:14 / 15:44:41 | 22,863,084,160 bytes and NVIDIA-focused; too large for safe 24 GB Apple Silicon use. |
| Other GGUFs | [romanerm/flint-GGUF](https://huggingface.co/romanerm/flint-GGUF), [terelabs/flint-GGUF](https://huggingface.co/terelabs/flint-GGUF) | 2026-10-22 14:41:39 / 16:32:22; 2026-10-21 17:25:39 / 20:39:34 | Listed IQ4/Q4/Q5 variants are about 22 GB or larger; not shortlisted. |

At 64k, an approximate fp16 KV cache is 1.79 GiB: about 1.75 GiB for full-attention layers plus 0.04 GiB for sliding-window layers. This excludes runtime/scratch buffers and other apps. Approximate weight + KV budgets are:

| Best-first shortlist | File size | Approx. weights + 64k fp16 KV | Rationale / test status |
|---|---:|---:|---|
| bartowski IQ2_XXS | 12.03 GB | 13.0 GiB | Model-specific imatrix/layout; tested first. |
| Unsloth UD-IQ2_XXS | 11.86 GB | 12.8 GiB | Smaller dynamic alternative; tested second. |
| Unsloth UD-IQ1_M | 11.35 GB | 12.4 GiB | Smallest plausible file found, with higher quality risk; not tested. |

These are estimates, not safe-fit guarantees. The machine was already under other-app memory pressure during testing; swap readings are system-wide deltas, not attributable to the model alone.

## Tier 1 and 2 results

The installed `/opt/homebrew/bin/llama-server` was build 11669. Its first attempt failed to load the architecture (`unknown model architecture: 'flint'`), so no inference ran. Upstream llama.cpp PR [#30928](https://github.com/ggml-org/llama.cpp/pull/30928) added `FlintForCausalLM` and merged at 2026-10-21 21:26:22 UTC; the official [b11804 macOS arm64 release](https://github.com/ggml-org/llama.cpp/releases/tag/b11804), published at 2026-10-22 01:26:20 UTC, loaded both variants. The compatibility build was downloaded and SHA-256 checked against the release digest, then used only in an isolated pill root/port.

### Raw-response diagnosis and corrected setting

Before the corrected benchmark, three b11804 Bartowski attempts using the normal `enable_thinking:false` request setting returned an empty visible `message.content`. A raw chat-completions response showed the model had spent the 64-token output budget in `reasoning_content`, with `finish_reason="length"` and only a partial answer; the loaded chat template did not consume the `enable_thinking` kwarg. Repeating the same probe with the upstream server option `--reasoning-budget 0` returned the answer in visible content and stopped after 27 tokens. This identified a serving/template configuration issue, not a missing model or unsupported API. No pill Go source was changed.

The superseded pre-correction Bartowski outcomes are retained here for completeness; Tier 2 was skipped for all three because the visible-reply check failed:

| Baseline run (b11804, no reasoning-budget option) | Tier 1 wall time | Direct visible reply | Pi tool-call check | Minimum free | Swap growth | Throughput probe | Tier 2 |
|---|---:|---|---|---:|---:|---:|---|
| 1 | 43.7 s | Empty after 64 generated tokens | Passed (1 tool call) | 12% | 7,129.98 MB | 32.65 tok/s | Skipped |
| 2 | 19.4 s | Empty after 64 generated tokens | Passed (1 tool call) | 12% | 51.84 MB | 35.60 tok/s | Skipped |
| 3 | 19.3 s | Empty after 64 generated tokens | Passed (1 tool call) | 12% | 59.57 MB | 36.21 tok/s | Skipped |

The corrected three-run Bartowski `pill bench run recursivesi-flint-iq2xxs-64k --runs 3` used that server option. Pill marks informational direct-answer correctness separately from the Tier 1 pass boolean: all 3 runs passed the Tier 1 gate, but only run 1 returned the correct agent-sized answer. Run 2's informational answer was just `2`; run 3 was unfinished within the output budget. All three completed within 60 seconds, passed the Pi tool-call check, and met the 10% memory threshold. Full-run samples saw 11% minimum free in each run; the Tier 1 checks recorded 12%, 11% and 13% respectively.

| Bartowski corrected run (b11804, reasoning budget 0) | Tier 1 wall time | Informational direct answer | Pi tool-call check | Full-run min free | Full-run swap growth | Sampled peak router-tree RSS / GPU memory | Tier 1 throughput probe | Tier 2 |
|---|---:|---|---|---:|---:|---:|---:|---|
| 1 | 28.9 s | Correct (`68`) | Passed | 11% (Tier 1 check: 12%) | 638.26 MB | 8.36 / 15.25 GB | 34.73 tok/s | 0/22 in 15:00; workspace incomplete (`engine.mjs` and `cli.mjs` absent), hit cap |
| 2 | 17.4 s | Incorrect (`2`) | Passed | 11% (Tier 1 check: 11%) | 171.89 MB | 13.31 / 15.35 GB | 34.69 tok/s | 13/22 in 15:00; failed verifier checks and hit cap |
| 3 | 19.0 s | Incorrect / unfinished within output budget | Passed | 11% (Tier 1 check: 13%) | 318.12 MB | 13.27 / 15.25 GB | 36.29 tok/s | 19/22 in 11:32; completed with verifier failures |

Memory uses decimal GB. Router-tree RSS peaks were sampled about once per second; `pill ps` GPU/unified-memory readings were sampled about once per five seconds. Free-memory and swap figures are system-wide. Tier 2 took about 41.5 minutes in total, in addition to about 65 seconds of Tier 1 time and model startup. The first cold run added 7.13 GB system-wide swap; later baseline runs added about 52–60 MB each.

### Unsloth UD-IQ2_XXS corrected run

The Unsloth file was downloaded by exact reference `unsloth/flint-GGUF:UD-IQ2_XXS` (11,855,541,696 bytes), SHA-256 verified, and registered as `flint-iq2xxs-64k` only in the isolated benchmark root. It used the same 64k context, Pi settings, b11804 server, `--reasoning-budget 0`, and `pill bench run flint-iq2xxs-64k --runs 3`.

| Unsloth corrected run (b11804, reasoning budget 0) | Tier 1 wall time | Informational direct answer | Pi tool-call check | Full-run min free | Full-run swap growth | Sampled peak router-tree RSS / GPU memory | Tier 1 throughput probe | Tier 2 |
|---|---:|---|---|---:|---:|---:|---:|---|
| 1 | 34.3 s | Correct (`68`) | Passed | 12% | 1,662.84 MB | 8.00 / 14.82 GB | 33.98 tok/s | 22/22 in 46.19 s |
| 2 | 19.0 s | Incorrect / unfinished after 64 tokens | Passed | 12% | 78.97 MB | 13.10 / 14.93 GB | 37.64 tok/s | 17/22 in 46.23 s; five checks failed |
| 3 | 19.2 s | Incorrect / unfinished within output budget | Passed | 11% | 66.86 MB | 13.16 / 15.46 GB | 36.41 tok/s | 19/22 in 15:00; hit cap after 181 repeated identical tool calls |

The Unsloth Tier 1 gate passed 3/3 (11–12% free, over the 10% floor), while its informational direct answer was correct only in run 1. The three full-run memory samples recorded maximum pressure level 2. Tier 2 used about 16.5 minutes total, plus about 72 seconds of Tier 1 time and startup. RSS and GPU memory are sampled peaks; free memory and swap growth are system-wide readings.

The run-3 Tier 2 result is 19/22, not a zero: the verifier completed with 19 checks passing, but the agent repeated the same tool call 181 times and reached the 15-minute cap. The model's Tier 2 result was therefore not stable across cold starts even though run 1 scored a perfect 22/22.

## Side-by-side gate comparison

| Quantisation | Tier 1 gate | Informational answer correct | Tier 2 scores by run | Full-run min free | Swap growth | Throughput probe |
|---|---|---:|---|---:|---:|---:|
| Bartowski IQ2_XXS (12.03 GB) | 3/3 pass | 1/3 | 0/22, 13/22, 19/22 | 11% each | 172–638 MB | 34.69–36.29 tok/s |
| Unsloth UD-IQ2_XXS (11.86 GB) | 3/3 pass | 1/3 | 22/22, 17/22, 19/22 | 11–12% | 67–1,663 MB | 33.98–37.64 tok/s |
| MiniCPM champion (11.3 GB) | 5/5 pass | not a comparable sample here | 22/22 in its gate runs | 22% | 0 | 25 tok/s |

The champion's score and memory values are the existing local gate results, not a new Flint-session measurement. Flint's two variants were run one at a time on the same isolated router port; the corrected Unsloth run began after the Bartowski server had been stopped.

## Tier 3 scores per task

**Not run for either Flint quantisation. Gate 2 decision: stop before Tier 3.** Bartowski scored 0/22, 13/22 and 19/22; Unsloth scored 22/22, 17/22 and 19/22. The MiniCPM champion is 22/22 and passed its five-run gate. Unsloth matched that score once, but missed it twice; Bartowski missed it in all three runs. Neither candidate demonstrated the stable Tier 2 performance required to proceed. There are no Flint Tier 3 task scores, screenshots, blind ratings or Opus-judge results.

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

**Tier 1:** Both quantisations passed the Pill gate 3/3 (18–34 seconds; all Pi tool-call checks passed; full-run minimum free memory 11–12%). Their informational direct-answer checks were correct only once in three runs each.

**Tier 2 / Gate 2:** Neither met the stable champion bar. Bartowski's scores were 0/22, 13/22 and 19/22. Unsloth's were 22/22, 17/22 and 19/22; its third run hit the cap after a 181-call loop. Full-run minimum free memory was 11–12%; swap grew by 67–1,663 MB across Unsloth's runs and by 172–638 MB across corrected Bartowski runs. Throughput probes were 34–38 tok/s, above MiniCPM's 25 tok/s, but did not offset the Tier 2 inconsistency. Tier 3 was not run for either quant.

**Recommendation:** keep `minicpm-v-4-7-35b-a3b-ad-iq2-xs-iq2-xxs-64k` as the default; do not switch to Flint. Unsloth's best result (22/22) is stronger than Bartowski's best (19/22), but its other two runs scored below the champion and one reached the time cap. That is a promising single run, not stable evidence that Unsloth is better than MiniCPM. Both Flint quantisations lack Tier 3 task-quality evidence.

The owner subsequently removed the original Bartowski registration with `pill rm recursivesi-flint-iq2xxs-64k`; pill reported the IQ2_XXS GGUF deleted and 12.0 GB freed. It was not re-registered, and its completed benchmark results remain documented above. The Unsloth GGUF remains at `/Users/marcellovictorino/.pill/models/flint-UD-IQ2_XXS.gguf` (11,855,541,696 bytes; SHA-256 above). It is registered only in the isolated task benchmark root as `flint-iq2xxs-64k` (failed; not default); it was not added to the owner's actual `pill ls` registry. At final check the owner's actual registry had these ten entries, with MiniCPM still the default:

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

The isolated b11804 router was stopped after benchmarking. The compatibility runtime, diagnostic scripts, and benchmark outputs remain in the authorized task scratch directory. No Tier 3 screenshots were produced.

## Caveats

The evidence is three cold Tier 1/2 runs per quant, not a multi-run Tier 3 comparison. One Unsloth run reached 22/22; this alone is not evidence of stable task performance. Memory pressure and swap samples describe the whole machine, including other apps; router-tree RSS was sampled externally about once per second and GPU memory via `pill ps` about once per five seconds. Only the listed two IQ2 variants were tested; UD-IQ1_M and other conversions remain untested. The HF API did not return a created date for the official INT4 repo. Exact conversion quality beyond the README claims was not independently evaluated. Tier 3 scores, screenshots, human ratings and Opus-judge results are unverified because Tier 3 was not run.
