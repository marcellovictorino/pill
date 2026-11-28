# llama.cpp tricks retest: MiniCPM-V 4.7 and Qwen3.6 35B-A3B (28 Nov)

## TLDR

**Verdict:** Nothing adopted. The DSpark drafter (7 tokens ahead) decodes 2.6 times faster on MiniCPM, but it did not finish faster end to end, and its quality matches the baseline, so the speed-only bar is not met. Ngram-mod gives no speed gain on either model. Thinking off with sampling on Qwen failed its clean Tier 2 run (17/22), so the Qwen tricks are not decided.

| Check | Result |
|---|---|
| Tier 1 gate | Passed in every clean run; one Tier 2 gate fail (Qwen thinking off + sampling, 17/22) |
| Tier 2 (22 tasks; champion bar 22/22) | Clean runs: MiniCPM base 22/22, ngram 22/22, DSpark 7 22/22, DSpark 3 22/22; Qwen base 22/22, Qwen ngram 22/22, Qwen thinking off + sampling 17/22 |
| Tier 3 (py-clean, 100 points; two runs per arm) | MiniCPM base 94, 77; DSpark 7 77, 93 |
| Time | Tier 2 minutes, one clean run: MiniCPM base 2.1, DSpark 7 4.6, DSpark 3 5.1, ngram 9.7; Qwen base 10.7, ngram 4.4. Tier 3 py-clean seconds: MiniCPM base 493, 397; DSpark 7 1,549, 538 |
| Speed | Decode tok/s, clean: MiniCPM base 28, ngram 27, DSpark 7 74, DSpark 3 54; Qwen base 34, ngram 34, thinking off + sampling 34 |
| Memory pressure | Minimum free memory 18–37% in the clean runs (floor 5%); swap growth 0 MB in every clean run |

**Against the catalog:** MiniCPM-V 4.7 AD-IQ2_XS/IQ2_XXS (default) was 25 tok/s in the catalog and 28 tok/s in the clean base run. Qwen3.6 35B-A3B UD-IQ2_M was 29 tok/s on its last catalog record (23 Nov), where run 3 scored 0/22; the clean base run passed Tier 2 22/22 in 10.7 min at 34 tok/s.

## Details

- **Setup:** Pi agent with no extensions, skills or context files; 24 GB M4; llama.cpp 1.1.0 (build 12774); 64k context for both models. Quantisations: MiniCPM AD-IQ2_XS/IQ2_XXS (11.3 GB); Qwen3.6 UD-IQ2_M. DSpark drafter: MiniCPM-V-4.7 DSpark Q8_0 (0.42 GB), 3 and 7 tokens ahead.
- **Arms:**
  - Thinking off with 32k replies: Qwen, reasoning off, max tokens 32768.
  - Model-card sampling: Qwen (thinking off) at temp 0.7, top-p 0.8, top-k 20, presence penalty 1.5; MiniCPM at temp 0.7, top-p 0.95, top-k 100, repeat penalty 1.03.
  - Ngram-mod draftless speculation: n-min 48, n-max 64, on both models.
  - DSpark drafter: MiniCPM, 3 and 7 tokens ahead.
- **Clean runs:** the seven speed arms ran one after another from 03:51 to 04:35 with nothing else on the GPU. The screening pass (23:03 to 00:58) overlapped another run on the same GPU and memory, so its numbers are marked overlap-affected and are not used for verdicts.
- **Screening results (overlap-affected):** MiniCPM base 22/22, swap growth 5.5 GB; sampling 22/22; ngram 22/22; DSpark 3 22/22; DSpark 7 22/22. Qwen base 22/22; thinking off 20/22; thinking off + sampling 22/22; thinking off + ngram 22/22.
- **Tier 3 repeats (py-clean):** the first DSpark 7 run scored 77, and the repeat 93. The baseline scored 94 and then 77 in its repeat. Both arms span the same range, so the first 77 was run-to-run noise, not a quality loss. Tier 3 scores on this task vary by about 17 points between runs of the same arm.
- **Speed is not end-to-end speed:** DSpark 7 decodes 2.6 times faster, yet it took longer to finish: Tier 2 4.6 min against 2.1 min, and py-clean 538 s (repeat) and 1,549 s (first run) against 397 s and 493 s for the baseline. The cause is not diagnosed (longer outputs, retries or prompt-processing cost are all possible). A speed-only adoption needs a wall-clock comparison with more runs.
- **Other tier 3 (one run each):** Qwen base py-insight 72 (clean), py-clean 65 (overlap-affected, hit the cap). Qwen thinking off + sampling py-insight 47 (clean), py-clean 96 (overlap-affected). The thinking-off arm was run before its clean Tier 2 rerun failed, so these scores are not a basis for adoption.
- **Caveats:**
  - Every Tier 2 arm ran once clean, apart from the two DSpark and baseline repeats on Tier 3.
  - The Qwen ngram arm also turns thinking off and sets max tokens to 32768, so its Tier 2 time is not comparable with the base arm.
  - Not run clean: MiniCPM sampling; Qwen thinking off without sampling. Personal Tier 3 was not run for any model.

## Sources

Raw run folders and logs are kept outside the repository (the bench folders under the scratch pill home and the evidence folder for this task). Nothing is quoted from personal runs. The catalog is unchanged in this change.
