# Ternary Bonsai 2.5 27B, Q2_0 with and without the MTP head, 32k context

## TLDR

**Verdict:** Not adopted. The MTP file reaches 22/22 on Tier 2, but only at the 15-minute cap, Tier 1 takes 160 s against the 60 s default, and Tier 3 scored 0 on both Python tasks. MiniCPM stays the default.

| Check | Result |
|---|---|
| Tier 1 gate | Plain 144 s, MTP 160 s, both pass the 300 s limit used here and fail the 60 s default (1 run each) |
| Tier 2 (22 tasks; champion bar 22/22) | Plain 20/22 at the cap (`bestMove blocks`, `bestMove replies to centre with a corner`); MTP 22/22 at the cap |
| Tier 3 (7 tasks) | MTP, one run: Go 80, cleaning 0, insight 0, Kanban 86, nursery 96, Mars 93, tax 96. Plain not run |
| Quality | Strong on landing pages (93 to 96); no output on cleaning (30-minute cap) or insight |
| Time | Tier 2 at the 15-minute cap both ways; decode 9 tok/s plain, 11 with MTP (96% of drafted tokens accepted); Tier 3 10 to 30 minutes per task |
| Memory pressure | Tier 1 and 2 minimum free 15–18%, swap up 2.3–2.5 GB; Tier 3 minimum free 9% (insight), swap up to 2.7 GB (cleaning) |

**Against the champion:** MiniCPM-V 4.7 IQ2 passes the default gate (Tier 1 21–28 s, Tier 2 22/22 in about 5 minutes, about 25 tok/s) and its Tier 3 medians are Go 88, cleaning 91, insight 40, Kanban 90 and landing pages 93, 95, 98.

## Details

- **Setup:** `prism-ml/Ternary-Bonsai-2.5-27B-GGUF` (Qwen3.8-27B base, ternary, dense, released 24 November). `Q2_0` is 7.7 GB and `Q2_0-mtp` 7.9 GB, both run on stock llama.cpp 1.0.0 (build 12359, newer than the b11556 the card requires). The MTP file carries its own draft head, so it runs with `spec-type = draft-mtp` and no separate drafter file. pill 0.6.0, Pi 1.6.0, 24 GB M4, thinking off (`enable_thinking` false), 32k context. `PQ2_0` and `PTQ1_0` need PrismML's fork and were not run.
- **Gate:** run at Tier 1 300 s, Tier 2 15 min, free memory floor 5%, one run per file. The installed pill binary reports both runs as failed because Tier 2 hit the cap; the repository's current rule counts a capped run with full marks and no tool-call loop as a pass, which the MTP run meets (one repeated call).
- **MTP effect:** decode 9 to 11 tok/s (22% faster), Tier 1 144 to 160 s (no faster: prefill of the 7,400-token prompt dominates on a dense model). MTP changed Tier 2 from 20/22 to 22/22, but with one run each that may be noise.
- **Tier 3 misses:** cleaning ran to the 30-minute cap with no output file; insight ended after 27 minutes with no predictions and no report. Go 80 took 24.8 minutes.
- **Not run:** a second Tier 1 and 2 run, the plain file on Tier 3, personal Tier 3 (the baseline did not hold up against MiniCPM), PrismML's fork.
- **Earlier Bonsai:** Bonsai 2 27B PQ2_0 failed every Tier 1 and 2 run on the fork (see [tried-and-rejected.md](tried-and-rejected.md)).
- **Re-test only on:** faster prefill for Pi's prompt on this Mac, or a Bonsai release that fixes multi-step Python work.

## Sources

Raw runs (not committed): `~/.pill/benchmark/20261125T175659Z-ternary-bonsai-2-5-27b-q2-0-32k` (plain), `20261125T182210Z-ternary-bonsai-2-5-27b-q2-0-mtp-32k` (MTP); Tier 3 in `evals/results/` (ignored). Row: [tried-and-rejected.md](tried-and-rejected.md). Model card: https://huggingface.co/prism-ml/Ternary-Bonsai-2.5-27B-GGUF.
