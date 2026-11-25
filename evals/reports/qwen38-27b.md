# Qwen3.8 27B, UD-IQ3_XXS (10.9 GB), 32k context, thinking off, MTP

## Update: DFlash2 drafter (25 November, build 12359)

With z-lab's DFlash2 Q4_K_M drafter (1.1 GB, `spec-draft-n-max = 7`) the UD-IQ3_XXS file decodes at 29 tok/s (MTP 14.8) and
scored 22/22 on Tier 2 in all three runs, two of them at the 15-minute cap, with Tier 1 at 132–144 s, lowest free memory 8%
and swap up at most 1.4 GB. The pill 0.6.0 binary that ran it printed `failed` for the capped runs; the repository's gate
(since 20 November) counts them as passes. This is the first Qwen3.8 27B setup that clears the gate. Details and caveats in
[`tried-and-rejected.md`](tried-and-rejected.md#qwen38-27b-with-the-dflash2-drafter-2026-11-25). Tier 3 not run.

## Update: Unsloth Dynamic 3.1 quants (UD-IQ4_XS and UD-Q3_K_XL) tested and not adopted

**Verdict:** Not a good model on this machine (16–17 November, build 12359). Unsloth's Dynamic 3.1 files
(posted 10–12 November) do not beat the 4–5 November UD-IQ4_XS result, and neither comes near MiniCPM. Tier 1
still takes 137–155 s against the 60 s default, and no arm scored 22/22 in all three relaxed runs. MiniCPM stays
the default. Tier 3 was not run.

| Check | MiniCPM control (default) | UD-IQ4_XS, Dynamic 3.1 (14.4 GB) | UD-Q3_K_XL, Dynamic 3.1 (13.1 GB) | UD-IQ4_XS, 4–5 Nov (build 11960) |
|---|---|---|---|---|
| Default gate (60 s, 15 min, 10%) | Pass, 1 of 1 | Fail: Tier 1 155 s | Fail: Tier 1 152 s | Fail, 0 of 2 (memory) |
| Tier 1, relaxed (limit 300 s) | 45 s at the default gate | 144, 137, 137 s | 152, 146, 146 s | 148, 140, 135 s |
| Tier 2 (22 tasks; bar 22/22) | 22/22 in 4.9 min | 22, 22, 20; all three at the 30-min cap | 22 in 8.5 min, 21 in 25.4 min, 22 at the 30-min cap | 21 in 24.5 min, 21 at the cap, 22 in 28.3 min |
| Decode | 27.5 tok/s | 5.8–6.1 tok/s | 6.3–6.6 tok/s | 6.3 tok/s |
| Lowest free memory | 14% | 9–10% | 12–18% | 9–10% |
| Swap growth | 7.9 GB | 3.8 GB at the default gate; 0.2–1.0 GB relaxed | 0 in every run | 0–0.9 GB |

- **Setup:** Pi, thinking off (`chat-template-kwargs = '{"enable_thinking":false}'`), 32k context, no MTP, llama.cpp
  build 12359, the same server settings as the 4–5 November run. Each arm had one run at the default gate, then
  three runs at the relaxed gate (300 s, 30 min, memory floor 5%). MiniCPM ran once on the same build as the control.
  The new UD-IQ4_XS is a different file from the 4 November one (other checksum, 14.35 GB against 14.25 GB).
- **Misses:** UD-IQ4_XS run 3 failed `bestMove blocks` and `bestMove replies to centre with a corner`; UD-Q3_K_XL
  run 2 failed `bestMove blocks`. No run repeated a tool call in a loop. The Tier 1 answer (informational) was right in two of four UD-IQ4_XS
  runs; UD-Q3_K_XL wrote an explanation instead of the bare number in every run, which the check marks wrong.
- **UD-Q3_K_XL fits better:** at least 12% free memory and no swap growth, against 9–10% for the 4-bit files. Its one
  quick run (22/22 in 8.5 min) is the fastest Qwen3.8 27B Tier 2 result on this machine, but the other two took 25 and
  30 minutes.
- **Not run:** Tier 3, because no arm passed Tier 2 at 22/22 in every run. The UD-IQ4_XS arm with the MTP head was
  skipped: MTP speeds up decoding only, and Tier 1 is bound by prompt processing on this dense model, so it could not
  pass the default gate. The MiniCPM control swap growth (7.9 GB) came while other apps were in use at the start of
  the night.
- **Re-test only for** the reasons below: a faster machine or a pill change that relaxes Tier 1.

Raw runs: `~/.pill/benchmark/` (not committed), folders `20261116T211724Z` (MiniCPM), `20261116T213822Z` and
`20261116T215447Z` (UD-IQ4_XS), `20261116T215059Z` and `20261116T233501Z` (UD-Q3_K_XL). Rows:
[`tried-and-rejected.md`](tried-and-rejected.md).

## Update: UD-IQ4_XS (4-bit, 14.3 GB) tested and discarded

**Verdict:** Tested and discarded (PILL-47, 4–5 November, build 11960). The 4-bit quant from unsloth, 32k, thinking off,
no MTP, does not fix the problems above. Accuracy is stable but not perfect (21, 21 and 22 of 22 on the last three
runs), Tier 2 takes 24–30 minutes, and Tier 1 takes 135–183 s, 2.3 to 3 times the 60 s default. Tier 3 was not run.
MiniCPM stays the default. Runs and numbers: [`tried-and-rejected.md`](tried-and-rejected.md).

## TLDR

**Verdict:** Not adopted. The model has potential but is limited on this 24 GB machine: in one Tier 3 run it beat
MiniCPM's median on three of seven tasks (Go 100, nursery 100, Mars 100), but it scored 0 on insight, runs at half
MiniCPM's speed and still fails the 60 s Tier 1 limit at 151–159 s. MiniCPM stays the default.

| Check | Result |
|---|---|
| Tier 1 gate | Fail at the default 60 s: 151–159 s with MTP (133–142 s without), 3 of 3 runs on 25 October |
| Tier 2 (22 tasks; champion bar 22/22) | With MTP: 20, 22, 20; without MTP: 22, 22, 20 (relaxed 30-minute cap, 25 October) |
| Tier 3 (7 tasks) | One run: Go 100, cleaning 80, insight 0, Kanban 71, nursery 100, Mars 100, tax 93 |
| Quality | Ahead of MiniCPM's 6-run median on 3 of 7 tasks, behind on 4; single run |
| Time | Tier 3: 7.5 minutes on Kanban, 18–24 minutes elsewhere, none at its cap; decode 12–14 tok/s on code with MTP (6.7 without) |
| Memory pressure | Minimum free memory 12–14%; swap up 0.35 GB on Mars, under 0.1 GB on every other task |

**Against the champion:** MiniCPM-V 4.7 IQ2 medians over 6 runs are Go 88, cleaning 90.5, insight 40, Kanban 90,
nursery 93, Mars 95, tax 98, at 3.5–22 minutes per task, and it passes the default gate.

## Details

- **Setup:** Pi with thinking off (`chat-template-kwargs = '{"enable_thinking":false}'`), 32k context, Unsloth
  UD-IQ3_XXS with Unsloth's MTP head (`MTP/` folder, Q4_0, 1.4 GB) through llama.cpp build 11960 and pill 0.4.1:

  ```toml
  [model.preset]
    chat-template-kwargs = '{"enable_thinking":false}'
    spec-type = "draft-mtp"
    spec-draft-model = "~/.pill/drafts/mtp-Qwen3.8-27B-Q4_0.gguf"   # full path in models.toml
    spec-draft-ngl = 99
  ```

  A 300-token coding answer ran at 12.2 and 13.7 tok/s with 70–77% of drafts accepted.
- **Runs:** one run per task, 1 to 2 November 2026 (22:35 to 00:49), on an otherwise idle machine. No task hit its cap.
- **Insight 0:** after 25 tool calls exploring the data, two replies hit the 8,192-token output limit (`max_tokens`)
  and Pi ended the session before any report, predictions or charts were written. A higher `max_tokens` might fix it;
  not tested.
- **Kanban 71:** the board renders and adds cards, but the verifier could not click Delete or use the move select
  (Playwright timed out on the locators), so delete, move, order and persistence checks scored 0.
- **Cleaning 80:** country codes matched 26.5% and the churn flag 60.9%; the other columns matched 82–97%.
- **Caveats:** single run, so differences of a few points are not findings; MiniCPM's own runs vary by tens of points.
  The gate failure is prompt processing on a dense model, which MTP does not speed up; a smaller quant or a
  different offload setting would not bring Tier 1 near 60 s either.

## Tested so far, and what would justify a re-test

All on this 24 GB M4 with Unsloth's UD-IQ3_XXS (10.9 GB). The files were deleted on 2 November; `pill pull` them again
to re-test.

| Setting | When | Result |
|---|---|---|
| Thinking on, 64k | 14 Oct, build 11429 | Tier 1 130–144 s; Tier 2 0/22 (8,192-token thinking block, no tool call) |
| Thinking off, 64k | 25 Oct (PILL-8) | Tier 1 133–142 s; Tier 2 22, 22, 20 at 6.7 tok/s |
| Thinking off, MTP, 16k | 25 Oct (PILL-8) | Decoding 16–18 tok/s; lowest free memory 7%; no full gate run |
| Thinking off, MTP, 32k | 25 Oct (PILL-10) | Tier 1 151–159 s; Tier 2 20, 22, 20 with two runs looping; 14.8 tok/s |
| Thinking off, MTP, 32k, Tier 3 | 1 Nov, build 11960 | The scores above, one run per task |

Do not re-test for the Tier 1 gate on this machine. It is a dense model, so the 7,400-token prompt is bound by prompt
processing: a smaller quant, a different offload setting or MTP does not bring it near 60 s, and a larger quant does
not fit. Re-test only for one of these:

- **A faster or larger machine**, where Tier 1 could pass at all.
- **A pill change that relaxes Tier 1** for slow, high-quality models.
- **Insight with a higher reply limit** (`max_tokens` above 8,192), the one cheap check left: it would show whether the
  0 is the limit or the model.

## What it looks like

| Task | Screenshot | Automated | Blind rating |
|---|---|---|---|
| Kanban (five cards added through the UI) | ![Qwen3.8 27B kanban](img/web-kanban/pi_qwen38-27b-iq3xxs-mtp-32k.webp) | 71 | not rated |
| Nursery landing page | ![Qwen3.8 27B nursery](img/web-landing-nursery/pi_qwen38-27b-iq3xxs-mtp-32k.webp) | 100 | not rated |
| Mars landing page | ![Qwen3.8 27B Mars](img/web-landing-mars/pi_qwen38-27b-iq3xxs-mtp-32k.webp) | 100 | not rated |
| Tax landing page | ![Qwen3.8 27B tax](img/web-landing-tax/pi_qwen38-27b-iq3xxs-mtp-32k.webp) | 93 | not rated |

## Sources

Gate runs: [`tried-and-rejected.md`](tried-and-rejected.md#qwen38-27b-thinking-and-mtp). Comparison:
[`summary.md`](summary.md). Raw runs are in `evals/results/` (not committed), folders `20261101T223508` to
`20261102T003036` for agent `pi:qwen38-27b-iq3xxs-mtp-32k`.
