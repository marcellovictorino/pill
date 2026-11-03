# Image generation benchmark: FLUX 3 Image Klein and Qwen-Image 2.1 Turbo

## TLDR

**Verdict:** Not decided. Both models run on the 24 GB M4, Qwen-Image 2.1 Turbo scored higher with Opus 5.5 (20.1 against 17.7 of 25), but the human ratings that confirm the judge are not in yet.

| Check | Result |
|---|---|
| Runs | 2 models x 3 prompts x 3 seeds (7, 42, 1234), 1024x1024, 18 of 18 images produced |
| Quality (Opus 5.5, 0-25) | Qwen Turbo 20.1, FLUX 3 Klein 17.7; Qwen ahead on every criterion, most on technical quality (3.9 against 2.8) |
| Time per image | FLUX 3 Klein 107 s; Qwen Turbo 214 s mean, but 97 to 310 s because of memory pressure (see caveats) |
| Memory pressure | FLUX: min free 16%, swap grew 10.6 GB, pressure level 4. Qwen: min free 12%, swap grew 3.5 GB (started at 16 GB used), level 2 |
| Peak RSS (`/usr/bin/time -l`) | 4.0 to 4.2 GB for both. It misses GPU memory, so it understates the real footprint |
| Self-validation | Seed determinism: identical bytes (FLUX). Judge matched beats mismatched prompt: 10 of 10. Order swap: 2 of 3 consistent (one position-biased pair). Judge-human agreement: Spearman 0.54 over 18 images |

## Details

- **Setup:** Apple M4, 24 GB. FLUX 3 Klein 5.6B 4-bit S (`flux_3_klein_i4x.ckpt`) and Qwen-Image 2.1 Turbo 4-bit S (`qwen_image_2.1_turbo_i4x.ckpt`, 8 steps), both through `draw-things-cli`. The prompts are the captain's, verbatim and with no added style words (`evals/imagegen/prompts.json`).
- **Where the plan was wrong:** the plan said FLUX would run through mflux at 4-bit. The Hugging Face weights are gated and the account had no access, so on the captain's choice FLUX ran through Draw Things instead. mflux's peak memory (18.5 GB) and 7 s per step were therefore not measured. `draw-things-cli` also made the Draw Things HTTP server unnecessary.
- **Per prompt (Opus mean of 25):** coffee shop at night FLUX 17.0, Qwen 20.7; Halloween chef FLUX 14.3, Qwen 18.3; vineyard and lavender FLUX 21.7, Qwen 21.3. FLUX's weakest result is the anime chef (11 for seed 1234).
- **Human review (captain, blind, 1 to 5):** Qwen Turbo 4.33, FLUX 3 Klein 3.44. By prompt, FLUX / Qwen: coffee shop 3.00 / 3.67, Halloween chef 3.00 / 5.00, vineyard 4.33 / 4.33. Spearman between the human rating and the Opus total is 0.54 (moderate); the clearest disagreement is Qwen's coffee-shop seed 42, rated 1 by the captain (Opus 18 of 25). The captain's summary: one model is clearly better, and text inside an image comes out as gibberish.
- **Text in images:** the captain saw gibberish wherever an image contains text. Opus's notes flag garbled text (signs, machine labels) on all three FLUX coffee shops and one FLUX chef image, and on two of three Qwen coffee shops; no vineyard image is flagged for either model. The coffee-shop prompt is the one that invites signage, and FLUX is hit more often (4 of 9 images against 2 of 9). Neither prompt asks for text, so this is a model weakness the rubric only partly punishes (technical quality).
- **Cold and warm:** each image is a new process that reloads the model, so every image pays the load. The first image of a run is flagged `cold`; it was not slower in practice (FLUX 108 s against 107 s warm mean).
- **Caveats:** the machine was not idle. Qwen ran with 16 GB of swap already in use, which probably explains its slow images; the last one ran in 97 s once the pressure fell, so treat 97 s as the better estimate of its speed and 214 s as the contaminated mean. One judge pass per image: expect a few points of noise. The judge saw the prompt and image only, never the model name.

## Round 2: Z-Image Turbo, FLUX.2 klein 4B and Qwen-Image 2512

### TLDR

**Verdict:** Qwen-Image 2.1 Turbo stays the default and FLUX.2 klein 4B is kept as the second model; Z-Image Turbo is dropped. In the captain's blind rating Qwen Turbo scored 4.67 and FLUX.2 klein 4B 4.56 (a tie on 9 images each), Z-Image Turbo 3.00. Opus 5.5 scored all three within 0.7 of 25 (20.6, 20.4, 19.9), so it ranks FLUX.2 first but cannot separate them. FLUX.2 klein 4B is the fastest and lightest (53 s, 3.8 GB). Qwen-Image 2512 does not run on this 24 GB Mac. The captain plans to generate with both kept models on each request for variety.

| Model (Draw Things, 6-bit) | Human (1-5) | Opus (0-25) | Time per image | Peak footprint | Swap growth | Download |
|---|---|---|---|---|---|
| FLUX.2 klein 4B (`flux_2_klein_4b_q6p`) | 4.56 | 20.6 | 53 s | 3.8 GB | 0 | 3.1 GB + 4.5 GB text encoder |
| Qwen Turbo, 4-bit S (re-run baseline) | 4.67 | 20.4 | 75 s | 3.1 GB | 0 | already on disk |
| Z-Image Turbo (`z_image_turbo_1.0_q6p`) | 3.00 | 19.9 | 150 s | 5.2 GB | 0 | 4.9 GB + 4.5 GB text encoder |
| Qwen-Image 2512 (`qwen_image_2512_q6p`) | not run | not run | first image stalled 21 min | n/a | swap 2.7 to 4.9 GB | 17.6 GB + VAE |

### Details

- **Setup:** same three prompts, seeds 7, 42, 1234, 1024x1024, no style words, each model at Draw Things' recommended steps and guidance. 9 of 9 images for the three runs that finished. The machine was idle (pill's model unloaded), so Qwen Turbo's re-run took 75 s per image against 97 to 310 s in round 1, which confirms round 1's slow images were memory pressure.
- **Where the plan was wrong:** the shortlist put Qwen-Image 2512 at about 11 GiB peak at 6-bit. The 6-bit file is 17.6 GB, which leaves no room on 24 GB: the process held 12 GB resident at 0% CPU for 21 minutes without producing an image, so I killed it. A smaller file does not exist in Draw Things (no 4-bit for 2512), so the model cannot be compared here. The 6-bit Z-Image and FLUX.2 files are also 4.9 GB and 3.1 GB, not the sizes quoted.
- **Quality by criterion (Opus mean of 5):** Z-Image: subject 4.7, style 3.7, setting 4.4, technical 3.1, composition 4.0. FLUX.2 klein 4B: 4.2, 4.3, 4.7, 3.2, 4.1. Qwen Turbo: 4.3, 4.6, 3.6, 3.9, 4.1. Qwen Turbo remains best on technical quality; both new models are weaker there.
- **Per prompt (Opus mean of 25), Z-Image / FLUX.2 / Qwen Turbo:** coffee shop 19.3 / 20.7 / 21.0; Halloween chef 19.0 / 18.3 / 18.3; vineyard 21.3 / 22.7 / 22.0.
- **Text in images:** Opus flags garbled text in 2 of 9 Z-Image, 4 of 9 FLUX.2 and 3 of 9 Qwen Turbo images (coffee shop and vineyard signage). Neither new model fixes it.
- **Round 1 baseline:** Qwen Turbo scored 20.4 on this re-run against 20.1 in round 1, so judge noise is about a third of a point. Differences between the three models are inside that noise plus the single-pass caveat. The raw round 1 images were kept outside the repository and are not on this machine, so the human sheet holds the re-run of Qwen Turbo instead of the round 1 images.
- **Human rating (captain, blind, 27 images, 1 to 5):** Qwen Turbo 4.67, FLUX.2 klein 4B 4.56, Z-Image Turbo 3.00. By prompt, Qwen / FLUX.2 / Z-Image: coffee shop 4.0 / 5.0 / 2.3, Halloween chef 5.0 / 3.7 / 3.0, vineyard 5.0 / 5.0 / 3.7. FLUX.2 is better on the coffee shop and worse on the anime chef.
- **Judge against human:** Spearman 0.51 over the 27 images (round 1: 0.54), moderate. Both rank Z-Image last. Opus is lenient where the captain saw visible faults: three Z-Image images Opus scored 19 to 20 got a 2, and Qwen's coffee-shop seed 42 got 18 from Opus and 2 from the captain. Opus also separates the models by under 3%.
- **Decision (captain):** drop Z-Image Turbo, keep Qwen Turbo as default and FLUX.2 klein 4B as support. The Z-Image and Qwen-Image 2512 downloads were removed from Draw Things' model folder (Z-Image 4.5 GB, the 2512 VAE 243 MB; the 2512 checkpoint was already gone). The shared text encoders stay. The flux_1 VAE (160 MB) may belong to Z-Image and is left in place until confirmed.
- **FLUX.2 against FLUX 3:** FLUX.2 klein 4B (6-bit) beat the round 1 FLUX 3 Klein (4-bit S): human 4.56 against 3.44, Opus 20.6 against 17.7. The two were not compared in one blind sheet, and they differ in quantisation, text encoder and sampling defaults, so the cause is not isolated. A three-way blind test with new prompts is proposed to settle it.

## Round 3 and the no-text rule

### TLDR

**Verdict:** FLUX 3 Klein is dropped (captain's call; round 3 human mean 3.44 against Qwen Turbo 4.44 and FLUX.2 klein 4B 4.33). Qwen Turbo and FLUX.2 klein 4B are both used for every request: three variants each, six options, through `imagegen.py options`. A standing no-text sentence is now added to every prompt, but the target (lettering on 2 or fewer of 18 images) was **not met**: the best wording leaves lettering on 6 of 18, and cyberpunk-type scenes still show text.

| Check | Result |
|---|---|
| Round 3 prompts | cyberpunk night, fashion magazine cover, Mars domes; 2 models x 3 seeds = 18 images per variant |
| Determinism | Qwen Turbo and FLUX.2 reproduced their round 3 image byte for byte, so round 3 is the baseline |
| Lettering, no sentence (Opus 5.5, per image yes or no) | Qwen 4 of 9, FLUX.2 8 of 9: 12 of 18 |
| First wording ("The image contains no text of any kind: signs, screens, labels...") | Qwen 4, FLUX.2 5: 9 of 18 |
| Strict wording ("IMPORTANT: NEVER WRITE ANY TEXT", naming signs, menus, screens) | Qwen 8, FLUX.2 5: 13 of 18, worse |
| Short wording ("No text, letters, numbers or logos anywhere.") | Qwen 3, FLUX.2 3: 6 of 18 |
| Short wording plus "All surfaces are plain and unmarked; nothing is written on anything." (shipped, e-combo) | Qwen 3, FLUX.2 3: 6 of 18 |
| Realism cue on FLUX.2 (cyberpunk and magazine, 3 seeds each) | Opus judged the cue image more photographic in 6 of 6 pairs, both orders |

### Details

- **Method:** autoresearch-style loop, one prompt lever per variant, same 18 prompts and seeds, same Opus lettering judge ("any legible or garbled text, signs, labels, logos or watermarks"), FLUX.2 realism cue off to isolate the wording. Results per variant, Qwen / FLUX.2 of 9: a-short 3 / 3; b "all surfaces plain and unmarked" 2 / 6; c "clean, text-free image" 3 / 7; d short wording as a prefix 4 / 6; e-combo 3 / 3; f "No text." 3 / 5; g short wording at both ends 3 / 4; j abstract pictograms 9 / 9; k no signage on storefronts 3 / 8; l "no signs, banners or billboards" only 4 / 7; m e-combo plus no signs 3 / 3; n e-combo plus a negative prompt at `--cfg 2` 3 / 3.
- **What helped:** short wording that names nothing (the long first and strict wordings were worse, and the strict one made Qwen worse); a suffix rather than a prefix; "plain and unmarked surfaces" helped Qwen most (2 of 9 on its own).
- **What did not:** naming signs, menus or screens (it appears to prompt them), asking for pictograms (every image then carries lettering), repetition, a negative prompt at `--cfg 2`.
- **The floor:** the cyberpunk prompt keeps lettering in all three seeds for both models under every suffix (food stalls and neon storefronts invite signs); magazine covers and Mars domes are nearly clean once a suffix is present. No wording tried removes it. Use `--plain` or reword the scene per request when a scene is sign-heavy.
- **No clear winner:** e-combo ties the best count (6 of 18) with the shortest wording and is shipped as the default. With per-model mixing the best is 5 of 18 (Qwen "plain and unmarked" 2, FLUX.2 e-combo 3), still above the target.
- **Judge:** one Opus 5.5 call per image, so expect a count to move by one or two between runs; the first before/after blind sheet for the captain's own picks was built but not rated.
- **Realism cue:** FLUX.2 on realistic or photographic prompts gets a documentary-photo cue; Qwen gets none.
- **Magazine covers:** with the sentence a cover gets no masthead or cover lines.

## Running it

```bash
pill stop
python3 evals/imagegen/imagegen.py run qwen-turbo
python3 evals/imagegen/imagegen.py run flux2-klein-4b       # also z-image-turbo; qwen-image-2512 does not fit in 24 GB; add --plain to send prompts verbatim
python3 evals/imagegen/imagegen.py options "your prompt"    # six options: 3 Qwen Turbo then 3 FLUX.2, one pick page (pick.html)
python3 -m unittest discover -s evals/imagegen              # unit tests, no model
python3 evals/imagegen/imagegen.py judge <run-dir> <run-dir>      # Opus 5.5, uses the plan allowance
python3 evals/imagegen/imagegen.py blind <run-dir> <run-dir>      # sheet.html to rate, key.json to open afterwards
python3 evals/imagegen/imagegen.py validate agreement ratings-imagegen.json key.json <run-dir> <run-dir>
```

## Sources

`evals/imagegen/imagegen.py` (tool and `validate`), `evals/imagegen/prompts.json`, `evals/imagegen/prompts-round3.json`. FLUX 3 Image Klein was dropped from the runner after round 3 (below); its numbers above are history and cannot be re-run from this tool. Raw runs, judge output and the blind sheet were kept outside the repository.
