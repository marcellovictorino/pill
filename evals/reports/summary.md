# Model comparison: Tier 3

Six models on the seven Tier 3 tasks. Scores are hidden-test scores out of 100 unless noted. **Read the
run counts:** every cloud model and Gemma has a **single run** per task (one run can land well away from
the typical result); MiniCPM has **six runs** per task, so it is shown as median with range. Task
descriptions are in `../README.md`. Both Flint IQ2_XXS variants were retested with llama.cpp b11804 `--reasoning-budget 0` after the template left the default 64-token reply in reasoning. Both passed the corrected Pill Tier 1 gate 3/3, but neither met the stable Tier 2 bar: Bartowski scored 0/22, 13/22, 19/22; Unsloth scored 22/22, 17/22, 19/22 versus MiniCPM's 22/22. Neither reached Tier 3, so Flint has no task scores here. See [the Flint report](recursive-flint-iq2xxs.md) for diagnosis, per-run memory and recommendations.

## Scores

| Task | Opus 5.5 (medium) | Sonnet 5.5 (medium) | Haiku 5.5 (high) | GPT-5.6 Luna (high) | Gemma 4 UD-IQ3_S | MiniCPM-V 4.7 IQ2 |
|---|---|---|---|---|---|---|
| Runs per cell | 1 | 1 | 1 | 1 | 1 | **6** |
| `go-notes` (Go CLI feature) | 100 | 100 | 100 | 100 | 13 (timeout) | 88 median (67–94) |
| `py-clean` (data cleaning) | 100 | 100 | 100 | 100 | 68 | 90.5 median (57–96) |
| `py-insight` (automated) | 100 | 92 | 95 | 90 | 0 (timeout) | 40 median (40–60) |
| `py-insight` (Opus judge) | 96 | 87 | 91 | 64 | 0 (no report) | 17 mean (9–30) |
| `web-kanban` | 100 | 100 | 100 | 98 | 100 | 90 median (40–92) |
| `web-landing-nursery` | 100 | 96 | 100 | 89 | 92 | 93 median (18–96) |
| `web-landing-mars` | 100 | 100 | 100 | 93 | 3 (timeout) | 95 median (87–100) |
| `web-landing-tax` | 100 | 100 | 100 | 88 | 3 (timeout) | 98 median (54–100) |

Agent settings: Claude models through Claude Code with the effort shown, GPT-5.6 Luna through Codex, local models
through Pi with thinking off and a throwaway Pi config. Cloud agents ran with permissions bypassed in temporary
workspaces. The model versions behind the Claude aliases were not independently verified (the Sonnet transcript reports
`claude-sonnet-5-5`).

## Time per run (minutes, mean)

| Task | Opus | Sonnet | Haiku | GPT-5.6 Luna | MiniCPM (6 runs) |
|---|---|---|---|---|---|
| `go-notes` | 1.1 | 0.7 | 2.0 | 1.8 | 21.8 |
| `py-clean` | 1.0 | 0.7 | 2.4 | 1.6 | 17.4 |
| `py-insight` | 5.3 | 1.9 | 10.5 | 3.1 | 13.7 |
| `web-kanban` | 0.9 | 0.4 | 1.4 | 2.2 | 3.5 |
| `web-landing-nursery` | 2.9 | 1.0 | 4.3 | 1.5 | 6.1 |
| `web-landing-mars` | 3.3 | 0.8 | 4.8 | 2.6 | 6.3 |
| `web-landing-tax` | 3.0 | 0.8 | 4.2 | 2.9 | 5.9 |

Gemma timed out on 4 of 7 tasks (30 or 40 minute caps); on the three it finished it took 3.6 to 7.5 minutes.

## Blind human ratings of the landing pages

Rated blind by one person, one page per cell. MiniCPM's rated page is its first run.

| Agent | Nursery | Mars | Tax | Total /9 |
|---|---|---|---|---|
| Opus 5.5 | Excellent | Excellent | Good | 8 |
| Haiku 5.5 | Excellent | Good | Excellent | 8 |
| GPT-5.6 Luna | Excellent | Good | Excellent | 8 |
| Sonnet 5.5 | not rated | not rated | not rated | n/a |
| Gemma 4 UD-IQ3_S | Good | not built | not built | n/a |
| MiniCPM-V 4.7 IQ2 (first run: 31, 96, 98) | Bad | Ok | Ok | 2 |

## Local models: pill gate results (Tier 1 and 2, 64k context)

| Model | Result | Runs | Tier 2 | Min free memory | Swap | tok/s |
|---|---|---|---|---|---|---|
| MiniCPM IQ2 11.3 GB | pass | 5 of 5 | 22/22 ×5 | 22% | 0 | 25 |
| MiniCPM IQ3_XXS 13.7 GB | fail (20/22, swap) | 5 | 22/22 ×4, 20/22 ×1 (r5) | 11% | 7.2 GB | 24 |
| Gemma 4 UD-IQ3_S 11.3 GB | fail (two runs over the 15-minute cap) | 3 | 22/22 ×3 (two over cap) | 19% | 0 | 24 |
| Gemma 4 Q3_K_XL | passed once, then failed | 1, 3 | single: 22/22; 3-run: 22/22, skipped, skipped | 11%, 6% | 4.8–7.7 GB | 22 |
| Gemma 4 IQ4_XS | fail | 5 | 22/22 ×4, 21/22 ×1 (r5) | 9% | 765 MB | 21 |
| Flint Bartowski IQ2_XXS 12.0 GB (b11804, reasoning budget 0) | fail (Tier 2 below champion) | 3 | 0/22, 13/22, 19/22 | 11% full-run | 172–638 MB per run | 34.7–36.3 |
| Flint Unsloth UD-IQ2_XXS 11.9 GB (b11804, reasoning budget 0) | fail (unstable Tier 2) | 3 | 22/22, 17/22, 19/22 | 11–12% full-run | 67–1,663 MB per run | 34.0–37.6 |

Tier 2 score sequences are from the saved original `result.json` files. For Gemma Q3_K_XL, the standalone run and run 1 of the later three-run series scored 22/22; series runs 2–3 failed Tier 1 on memory and did not run Tier 2. Gemma IQ4_XS's five-run series scored 22/22 in runs 1–4 and 21/22 in run 5. The old summary's final-column 22 and 21 were throughput tok/s, not Tier 2 scores. The installed llama.cpp b11669 could not load Flint. Under b11804, the template ignored `enable_thinking:false`, placing the initial 64-token output in `reasoning_content`; `--reasoning-budget 0` returned visible text. Both corrected Flint quantisations passed Pill Tier 1 3/3, but their informational direct answers were correct only once each. Bartowski Tier 2 was 0/22, 13/22, 19/22; Unsloth was 22/22, 17/22, 19/22. Sampled router-tree RSS peaks were 8.36, 13.31, 13.27 GB for Bartowski and 8.00, 13.10, 13.16 GB for Unsloth; sampled GPU/unified-memory peaks were 15.25, 15.35, 15.25 GB and 14.82, 14.93, 15.46 GB, respectively. Whole-machine free memory reached 11%; swap grew 172–638 MB per Bartowski run and 67–1,663 MB per Unsloth run. Neither was stable at MiniCPM's 22/22, so Tier 3 was gated off for both. Flint has no Tier 3 scores, screenshots, judge score or blind ratings; see the report for all run details.

## Reading the table

- The three Claude models and GPT-5.6 Luna are near the ceiling on most tasks, so the build tasks separate local
  models but barely separate cloud ones. The insight task is the best discriminator (judge 64 to 96).
- Sonnet 5.5 was the fastest cloud model on every task (under 2 minutes).
- MiniCPM never timed out except twice on Go (2 of 42 runs), but about one run in five outside the insight
  task scored under 70.
- Gemma is as good as MiniCPM on the web tasks it finished, but timed out on long ones.

## MiniCPM: thinking on or off

The runs above used thinking off. llama.cpp supports reasoning for this model (`enable_thinking` in the chat template);
Pi turns it on with `--thinking medium` (`--agent pi:<model>:medium` in `run.py`). The same tasks were rerun with thinking on,
interleaved with a fresh thinking-off round, so drift in machine load affects both. Thinking on: 22 runs (3 per task, 4 for Kanban).
Thinking off: the 7 runs per task of this model (the six above plus one taken with the memory sampler).

| Task | Thinking on: runs | median | range | timeouts | mean min | Thinking off: runs | median | range | timeouts | mean min |
|---|---|---|---|---|---|---|---|---|---|---|
| `go-notes` | 3 | 82 | 15–86 | 1 | 24.4 | 7 | 89 | 67–94 | 2 | 20.7 |
| `py-clean` | 3 | 91 | 80–91 | 0 | 11.2 | 7 | 92 | 57–99 | 0 | 17.0 |
| `py-insight` (automated) | 3 | 48 | 45–54 | 0 | 11.5 | 7 | 40 | 35–60 | 0 | 13.2 |
| `web-kanban` | 4 | 88 | 34–90 | 0 | 3.8 | 7 | 90 | 40–92 | 0 | 3.5 |
| `web-landing-nursery` | 3 | 90 | 31–96 | 0 | 5.5 | 7 | 94 | 18–96 | 0 | 6.0 |
| `web-landing-mars` | 3 | 91 | 38–100 | 0 | 5.2 | 7 | 96 | 87–100 | 0 | 6.3 |
| `web-landing-tax` | 3 | 94 | 48–94 | 0 | 6.5 | 7 | 98 | 54–100 | 0 | 6.3 |
Runs under 70 outside the insight task: 5 of 19 with thinking on, 8 of 42 with it off. Opus judge on the insight reports: thinking on 17, 7, 15;
thinking off about 20 on average (7 reports).

**Result: no measurable gain from thinking.** The thinking-on median is equal or lower on six of seven tasks, one Go run hit the
30-minute cap, and times were otherwise about the same. With three runs per cell a large effect is ruled out but a small one is not.
**Recommendation: keep thinking off.**

### Memory pressure

Sampled system-wide every 5 seconds during each local run (`memory_pressure`, `sysctl vm.swapusage`):

| | Thinking on (22 runs) | Thinking off (7 runs) |
|---|---|---|
| Lowest free memory seen | 9% | 10% |
| Average of each run's minimum | 12.4% | 12.9% |
| Swap growth during a run (mean, max) | 0.6 GB, 2.4 GB | 0.9 GB, 2.2 GB |
| Swap in use at the start of a run (mean) | 12.5 GB | 11.8 GB |
| Highest pressure level | 2 (warn) | 2 (warn) |

Thinking does not change the memory the model needs: the weights are the same and llama.cpp reserves the cache for the full context
at load time, so extra reasoning tokens cost time, not memory. These readings cover the **whole machine**: during all of these runs other
apps already held swap near the 13 GB limit and pressure at "warn", so they describe a loaded machine, not the model alone. The earlier pill
gate runs on a quiet machine saw 22% free memory and no swap with thinking off.
