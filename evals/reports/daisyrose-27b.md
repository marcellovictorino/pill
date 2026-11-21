# DaisyRose-27B (RadixArk), IQ2 quants, 32k context, thinking off, with and without MTP, on a 16 GB M2 Pro

## TLDR

**Verdict:** Not a good model on this 16 GB Mac. No arm passed Tier 1 and Tier 2 together. The two 2-bit files that fit
the memory floor (IQ2_XXS and IQ2_XS) passed Tier 1 in 116–121 s, but Tier 2 hit the 15-minute cap at 20/22 and 21/22.
MTP doubles decoding (9 to 16 tok/s) but pushes free memory to 7%. Tier 3 was not run.

| Check | Result |
|---|---|
| Tier 1 gate (300 s, 10% free) | 2 of 4 arms passed: IQ2_XXS 116 s, IQ2_XS 121 s; IQ2_S (8%) and IQ2_XXS with MTP (7%) failed on memory |
| Tier 2 (22 tasks; bar 22/22) | IQ2_XXS 20/22 and IQ2_XS 21/22, both at the 15-minute cap; IQ2_XXS repeated one tool call 84 times |
| Tier 3 (7 tasks) | Not run: Tier 2 missed full marks |
| Quality | One run per arm; misses were all in `bestMove` (win, tie-break) |
| Time | Tier 2 15.0 min (cap) on both; decode 8–9 tok/s, 16 tok/s with MTP |
| Memory pressure | Lowest free 7–11%; swap growth 0–82 MB on the 2-bit passes, 2.2 GB on IQ2_S |

**Against the champion:** MiniCPM-V 4.7 IQ2 passes on the 24 GB M4 (22/22, about 25 tok/s); on this 16 GB Mac it
also fails Tier 1 on memory (6% free, 4 November), so no model is adopted here.

## Details

- **Model:** a reinforcement-learning fine-tune of Qwen3.8 27B (dense, apache-2.0). The same base was tried on this Mac at
  UD-IQ2_S on 4 November: Tier 1 116 s, 10% free, 8 tok/s.
- **Setup:** Pi, thinking off (`chat-template-kwargs = '{"enable_thinking":false}'`), 32k context, bartowski imatrix
  quants `hf.co/bartowski/RadixArk_DaisyRose-27B-GGUF` (IQ2_S 9.6 GB, IQ2_XS 9.0 GB, IQ2_XXS 8.8 GB), llama.cpp build 12359,
  pill 0.6.0, the current default gate (Tier 1 300 s, Tier 2 15 min, 10% free memory). One cold run per arm,
  21 November 2026, on an otherwise idle machine.
- **Quant choice:** this Mac's Metal limit is about 10.7 GB, so 3-bit files (12.2 GB and up) cannot hold the weights and
  a 32k cache. IQ2_S, the largest 2-bit file, dipped to 8% free, so the smaller IQ2_XS and IQ2_XXS were tried.
- **MTP:** DaisyRose ships a one-layer MTP head (`mtp_num_hidden_layers = 1`), and both the RadixArk and bartowski GGUFs
  keep it (`nextn_predict_layers = 1`), so `spec-type = "draft-mtp"` needs no separate file. On IQ2_XXS it raised decoding
  from 9 to 16 tok/s; Tier 1 time barely moved (116 to 119 s), because prefill dominates on this dense model.
  The model card recommends a DSpark drafter instead (`RadixArk/Qwen3.8-27B-DSpark`); it was not tried, as a second
  model would not fit beside the weights here.
- **Misses:** IQ2_XXS failed `bestMove takes the win` and `cli --best prints the chosen index`, after repeating one tool
  call 84 times; IQ2_XS failed `bestMove lowest index tie-break`.
- **Re-test only for:** a 24 GB Mac. The near-identical Arete-27B (another Qwen3.8 27B fine-tune) scored 22/22 in three
  runs at IQ4_XS on the 24 GB M4, and the 300 s Tier 1 limit now admits a dense 27B there.

## Sources

Rows: [`tried-and-rejected.md`](tried-and-rejected.md). Raw runs: `~/.pill/benchmark/` (not committed), folders
`20261121T120213Z` (IQ2_S), `20261121T121523Z` (IQ2_XXS), `20261121T123309Z` (IQ2_XS) and `20261121T125104Z` (IQ2_XXS, MTP).
