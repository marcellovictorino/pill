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
- **Human review (captain, blind, 1 to 5):** Qwen Turbo 4.33, FLUX 3 Klein 3.44. By prompt, FLUX / Qwen: coffee shop 3.00 / 3.67, Halloween chef 3.00 / 5.00, vineyard 4.33 / 4.33. Spearman between the human rating and the Opus total is 0.54 (moderate); the clearest disagreement is Qwen's coffee-shop seed 1234, rated 1 by the captain. The captain's summary: one model is clearly better, and text inside an image comes out as gibberish.
- **Text in images:** the captain saw gibberish wherever an image contains text. Opus's notes flag garbled text (signs, machine labels) on all three FLUX coffee shops and one FLUX chef image, and on two of three Qwen coffee shops; no vineyard image is flagged for either model. The coffee-shop prompt is the one that invites signage, and FLUX is hit more often (4 of 9 images against 2 of 9). Neither prompt asks for text, so this is a model weakness the rubric only partly punishes (technical quality).
- **Cold and warm:** each image is a new process that reloads the model, so every image pays the load. The first image of a run is flagged `cold`; it was not slower in practice (FLUX 108 s against 107 s warm mean).
- **Caveats:** the machine was not idle. Qwen ran with 16 GB of swap already in use, which probably explains its slow images; the last one ran in 97 s once the pressure fell, so treat 97 s as the better estimate of its speed and 214 s as the contaminated mean. One judge pass per image: expect a few points of noise. The judge saw the prompt and image only, never the model name.

## Running it

```bash
pill stop
python3 evals/imagegen/imagegen.py run flux3
python3 evals/imagegen/imagegen.py run qwen-turbo
python3 evals/imagegen/imagegen.py judge <run-dir> <run-dir>      # Opus 5.5, uses the plan allowance
python3 evals/imagegen/imagegen.py blind <run-dir> <run-dir>      # sheet.html to rate, key.json to open afterwards
python3 evals/imagegen/imagegen.py validate agreement ratings-imagegen.json key.json <run-dir> <run-dir>
```

## Sources

`evals/imagegen/imagegen.py` (tool and `validate`), `evals/imagegen/prompts.json`. Raw runs, judge output and the blind sheet were kept outside the repository.
