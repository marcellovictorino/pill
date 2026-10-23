# Model reports

One focused report per model, plus a comparison across models. These are written from Tier 3 runs
(`../README.md`) and are reference material, not tooling the `pill` binary uses.

| File | What it is |
|---|---|
| `summary.md` | Scores per task for every model that has been run, with run counts, timings and blind ratings |
| `minicpm-v-4-7-35b-a3b.md` | MiniCPM-V 4.7 (IQ2): strong points, weak points, recommended use |
| `gemma-4-26b-a4b.md` | Gemma 4 26B-A4B (UD-IQ3_S), the local comparison |
| `tried-and-rejected.md` | Models benchmarked and not adopted, with dates, numbers and what would justify a re-test |
| `recursive-flint-iq2xxs.md` | RecursiveSI Flint IQ2_XXS discovery and Tier 1/2 gate (Tier 3 not run) |
| `benchmark-report.html` | The same findings as one readable page (images in `img/`) |
| `img/<task>/<agent>.webp` | Screenshots of each model's saved web output |
| `TEMPLATE.md` | Skeleton for a new model report |
| `render_shots.py` | Rebuilds saved web workspaces and writes the screenshots |

## Adding a model

1. Run the tasks: `python3 evals/run.py run <task> --agent <spec> --runs N` for all seven tasks
   (`python3 evals/run.py list`). Use the same agent settings as the other models so the comparison is
   fair, and note them in the report. Local models need `pill add` first; `pi:<pill-model-name>`
   runs through the router.
2. Grade the insight run: `python3 evals/judge.py <results/.../run-1>` (an Opus judge; single sample, so
   treat it as plus or minus 8 points).
3. Take screenshots: `python3 evals/reports/render_shots.py --agent <spec>`. It uses the earliest saved
   run per task and writes small WebP files (width 800, cropped to 2400 px). Commit only those images.
4. Rate the landing pages by eye if you want a taste score: `python3 evals/blind.py` builds anonymised
   pages; open them, rate, download the JSON, and only then open `key.json`.
5. Copy `TEMPLATE.md` to `<model>.md` and fill it in. Add a row to `summary.md`.

## Rules for the numbers

- **Say how many runs every figure rests on.** A single run is a single run: label it, and do not present
  differences of a few points as findings. Report median and range when there are several.
- Separate **automated** scores (hidden verifiers) from **human** ratings and from the **LLM judge**.
- Keep screenshots small (about 30 to 60 KB each) and name the model, the automated score and any
  blind rating in the caption.
- Raw results (`evals/results/`) are not committed; the reports are the record. Quote the date and the
  agent settings so a rerun can be compared.
