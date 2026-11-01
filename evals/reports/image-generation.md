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

**Verdict:** Not decided. Opus 5.5 cannot separate FLUX.2 klein 4B (20.6 of 25), Qwen-Image 2.1 Turbo (20.4) and Z-Image Turbo (19.9); the blind human rating is the tie-break and is not in yet. FLUX.2 klein 4B is the fastest and lightest. Qwen-Image 2512 does not run on this 24 GB Mac. Per the keep-a-model rule, no new model replaces Qwen Turbo until the human rating shows it ahead.

| Model (Draw Things, 6-bit) | Opus (0-25) | Time per image | Peak footprint | Swap growth | Download |
|---|---|---|---|---|---|
| FLUX.2 klein 4B (`flux_2_klein_4b_q6p`) | 20.6 | 53 s | 3.8 GB | 0 | 3.1 GB + 4.5 GB text encoder |
| Qwen Turbo, 4-bit S (re-run baseline) | 20.4 | 75 s | 3.1 GB | 0 | already on disk |
| Z-Image Turbo (`z_image_turbo_1.0_q6p`) | 19.9 | 150 s | 5.2 GB | 0 | 4.9 GB + 4.5 GB text encoder |
| Qwen-Image 2512 (`qwen_image_2512_q6p`) | not run | first image stalled 21 min | n/a | swap 2.7 to 4.9 GB | 17.6 GB + VAE |

### Details

- **Setup:** same three prompts, seeds 7, 42, 1234, 1024x1024, no style words, each model at Draw Things' recommended steps and guidance. 9 of 9 images for the three runs that finished. The machine was idle (pill's model unloaded), so Qwen Turbo's re-run took 75 s per image against 97 to 310 s in round 1, which confirms round 1's slow images were memory pressure.
- **Where the plan was wrong:** the shortlist put Qwen-Image 2512 at about 11 GiB peak at 6-bit. The 6-bit file is 17.6 GB, which leaves no room on 24 GB: the process held 12 GB resident at 0% CPU for 21 minutes without producing an image, so I killed it. A smaller file does not exist in Draw Things (no 4-bit for 2512), so the model cannot be compared here. The 6-bit Z-Image and FLUX.2 files are also 4.9 GB and 3.1 GB, not the sizes quoted.
- **Quality by criterion (Opus mean of 5):** Z-Image: subject 4.7, style 3.7, setting 4.4, technical 3.1, composition 4.0. FLUX.2 klein 4B: 4.2, 4.3, 4.7, 3.2, 4.1. Qwen Turbo: 4.3, 4.6, 3.6, 3.9, 4.1. Qwen Turbo remains best on technical quality; both new models are weaker there.
- **Per prompt (Opus mean of 25), Z-Image / FLUX.2 / Qwen Turbo:** coffee shop 19.3 / 20.7 / 21.0; Halloween chef 19.0 / 18.3 / 18.3; vineyard 21.3 / 22.7 / 22.0.
- **Text in images:** Opus flags garbled text in 2 of 9 Z-Image, 4 of 9 FLUX.2 and 3 of 9 Qwen Turbo images (coffee shop and vineyard signage). Neither new model fixes it.
- **Round 1 baseline:** Qwen Turbo scored 20.4 on this re-run against 20.1 in round 1, so judge noise is about a third of a point. Differences between the three models are inside that noise plus the single-pass caveat. The raw round 1 images were kept outside the repository and are not on this machine, so the human sheet holds the re-run of Qwen Turbo instead of the round 1 images.
- **Human rating:** pending. Blind sheet of 27 images (3 models x 9), key held back until rated.

## Running it

```bash
pill stop
python3 evals/imagegen/imagegen.py run flux3
python3 evals/imagegen/imagegen.py run qwen-turbo
python3 evals/imagegen/imagegen.py run z-image-turbo        # also flux2-klein-4b; qwen-image-2512 does not fit in 24 GB
python3 evals/imagegen/imagegen.py judge <run-dir> <run-dir>      # Opus 5.5, uses the plan allowance
python3 evals/imagegen/imagegen.py blind <run-dir> <run-dir>      # sheet.html to rate, key.json to open afterwards
python3 evals/imagegen/imagegen.py validate agreement ratings-imagegen.json key.json <run-dir> <run-dir>
```

## Sources

`evals/imagegen/imagegen.py` (tool and `validate`), `evals/imagegen/prompts.json`. Raw runs, judge output and the blind sheet were kept outside the repository.
