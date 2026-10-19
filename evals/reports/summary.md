# Model comparison: Tier 3

Six models on the seven Tier 3 tasks. Scores are hidden-test scores out of 100 unless noted. **Read the
run counts:** every cloud model and Gemma has a **single run** per task (one run can land well away from
the typical result); MiniCPM has **six runs** per task, so it is shown as median with range. Task
descriptions are in `../README.md`.

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

| Model | Result | Runs | Min free memory | Swap | tok/s |
|---|---|---|---|---|---|
| MiniCPM IQ2 11.3 GB | pass | 5 of 5 | 22% | 0 | 25 |
| MiniCPM IQ3_XXS 13.7 GB | fail (20/22, swap) | 5 | 11% | 7.2 GB | 24 |
| Gemma 4 UD-IQ3_S 11.3 GB | fail (two runs over the 15-minute cap) | 3 | 19% | 0 | 24 |
| Gemma 4 Q3_K_XL | passed once, then failed | 1, 3 | 11%, 6% | 4.8–7.7 GB | 22 |
| Gemma 4 IQ4_XS | fail | 5 | 9% | 765 MB | 21 |

## Reading the table

- The three Claude models and GPT-5.6 Luna are near the ceiling on most tasks, so the build tasks separate local
  models but barely separate cloud ones. The insight task is the best discriminator (judge 64 to 96).
- Sonnet 5.5 was the fastest cloud model on every task (under 2 minutes).
- MiniCPM never timed out except twice on Go (2 of 42 runs), but about one run in five outside the insight
  task scored under 70.
- Gemma is as good as MiniCPM on the web tasks it finished, but timed out on long ones.
