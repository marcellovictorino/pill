# Nous Research Arete-27B, IQ3_XXS with MTP and IQ4_XS, 32k context, thinking off

## TLDR

**Verdict:** Not adopted, but the best dense 27B tried here. IQ4_XS scored 22/22 in all three Tier 2 runs in 4.5–11
minutes, better than base Qwen3.8 27B at the same quant (21, 21, 22 in 24–30 minutes). It still takes 129–145 s on
Tier 1 against the 60 s limit, and needs the memory floor lowered to 7%. MiniCPM stays the default.

| Check | IQ3_XXS (12.1 GB) with MTP | IQ4_XS (15.2 GB), no MTP |
|---|---|---|
| Tier 1 gate | Fail at the default 60 s: 135–145 s, 3 of 3 runs | Fail at the default 60 s: 129–144 s; also below the 10% memory floor (8–9% free) |
| Tier 2 (22 tasks; champion bar 22/22) | 21, 22, 20 (relaxed 30-minute cap; the 20 hit the cap) | 22, 22, 22 (relaxed limits, memory floor 7%) |
| Tier 3 (7 tasks) | Not run | Not run |
| Quality | Lost "bestMove blocks" in 2 of 3 runs | No lost check in 3 runs |
| Time | Tier 2 29.6, 17.3 and 30.0 min; 15.2 tok/s | Tier 2 7.3, 11.0 and 4.5 min; 6.2 tok/s |
| Memory pressure | Minimum free memory 10%; swap up 2.3 GB on run 1, 0.2–0.3 GB after | Minimum free memory 8–9%; swap up 0.0–0.5 GB |

**Against the champion:** MiniCPM-V 4.7 IQ2 passes the default gate on the same day and build: Tier 1 under 60 s,
22/22 in all five runs, 8.4 minutes mean, about 28 tok/s.

## Details

- **Model:** Arete-27B is a supervised fine-tune of Qwen3.8 27B (same architecture, no architectural change), released on
  6 November 2026. Tier 1 speed is therefore the base model's: a dense 27B reading Pi's 7,400-token prompt.
- **Setup:** Pi with thinking off (`chat-template-kwargs = '{"enable_thinking":false}'`), 32k context, bartowski's
  imatrix quants (`bartowski/NousResearch_Arete-27B-GGUF`), llama.cpp build 12110, pill 0.6.0, Pi 1.5.0, 24 GB M4.
  Gate limits relaxed to `--tier1-max-seconds 240 --tier2-max-minutes 30`; IQ4_XS also used `--min-free-pct 7`.
- **MTP:** Nous disabled the MTP head during training and ships no MTP weights. The base model's head still works:
  Unsloth's `MTP/mtp-Qwen3.8-27B-Q4_0.gguf` (1.4 GB, from `unsloth/Qwen3.8-27B-GGUF`) with `spec-type = "draft-mtp"`,
  `spec-draft-model = "<path>"` and `spec-draft-ngl = 99` accepted 84.6% of drafted tokens (30,140 of 35,608 over 172
  requests, mean accepted length 3.5) across the three IQ3_XXS runs, above the 70–77% measured on base Qwen3.8.
  Decoding rose to 15.2 tok/s, against 6.2 for IQ4_XS without MTP.
- **Why IQ4_XS ran without MTP:** the file plus the head is about 16.6 GB; IQ4_XS alone already left only 8–9% free at
  32k. A first IQ4_XS gate run at the default 10% floor failed Tier 1 on memory in 3 of 3 runs, so Tier 2 never ran;
  the three runs above are the re-run with a 7% floor.
- **Runs:** 7 November 2026, 9:49 to 11:57, three runs per setup, on an otherwise idle machine. IQ3_XXS run 3 hit the
  30-minute cap at 20/22; no run looped (at most two repeated calls).
- **Speed against quality:** MTP made IQ3_XXS decode 2.5 times faster, but its Tier 2 runs took longer than IQ4_XS's
  (17–30 against 4.5–11 minutes): the lower-precision quant wrote more code and fixed more mistakes before it finished.
- **Other quants that fit:** with MTP at 32k, IQ2_M (10.3 GB), Q2_K (10.6 GB) and IQ3_XS (12.6 GB, tight); without
  MTP, Q3_K_L (13.9 GB) and IQ3_M (14.6 GB). Q4_0 (16.1 GB) and larger do not fit. Not tested.
- **Caveats:** three runs per setup; Tier 3 not run. Both setups would need the relaxed Tier 1 limit, and IQ4_XS also
  a lower memory floor, to pass.

## What would justify more work

- **Tier 3 on IQ4_XS**, the one setup with a clean Tier 2: it would show whether the fine-tune also beats the base
  model's single Tier 3 run (Go 100, nursery 100, Mars 100, insight 0, Kanban 71).
- **A pill change that relaxes Tier 1** for slow, high-quality models; on this machine no Arete quant reaches 60 s.

## Sources

Raw runs are in `~/.pill/benchmark/` (`20261107T084925Z-nousresearch-arete-27b-iq3-xxs-32k`,
`20261107T101539Z-nousresearch-arete-27b-iq4-xs-32k` at the 10% floor and
`20261107T102531Z-nousresearch-arete-27b-iq4-xs-32k` at 7%). Base model results:
[the Qwen3.8 27B report](qwen38-27b.md). The catalog records and their `tried-and-rejected.md` rows are not added yet.
