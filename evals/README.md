# evals: tier 3 model evaluations

Dev tooling for comparing models on realistic coding work. It is **not part of pill**: nothing in the
`pill` binary imports, embeds or reads it, and it only talks to pill through the public CLI (`pill pull`,
`pill add`, `pill stop`) and the router's OpenAI-compatible API (through Pi). Go fixtures are their own
modules, so `go build ./...`, `go test ./...` and `golangci-lint run ./...` skip this folder.

Tier 1 and 2 (`pill bench run`) answer "is this model usable on this machine". Tier 3 answers "what is it
good at": one capability profile per domain, with objective hidden tests carrying most of the score.

## Run

```bash
python3 evals/run.py list
python3 evals/run.py selftest go-notes                       # reference solution must score ~100, blank fixture low
python3 evals/run.py run go-notes --agent pi:minicpm-v-4-7-35b-a3b-ad-iq2-xs-iq2-xxs-64k --runs 3
python3 evals/run.py run go-notes --agent claude:opus:medium
python3 evals/run.py run go-notes --agent codex:gpt-5.6-luna:high
python3 evals/summary.py                                      # markdown table over results/index.jsonl
```

Only one local model can be loaded at a time on a 24 GB machine: run local agents one at a time
(the runner calls `pill stop` first so every run is a cold start).

## Tasks

| id | domain | what the agent builds | score |
|---|---|---|---|
| `go-notes` | 3B Go CLI | adds `stats` and `rm` and fixes a bug in a small cobra CLI, following the repo's conventions | hidden `go test` + binary behaviour + lint/convention checks |
| `py-clean` | 3A1 Python + DuckDB | cleans, joins and aggregates four dirty parquet tables | exact output tables vs generator truth |
| `py-insight` | 3A2 Python + DuckDB + scikit-learn | EDA, features, churn model, written insights with charts | holdout AUC (leak-free) + planted-pattern checklist (LLM judge) |
| `web-kanban` | 3C React + TS | Trello-style kanban SPA | hidden Playwright tests + `tsc` + build |
| `web-landing-nursery`, `web-landing-mars`, `web-landing-tax` | 3C taste | landing page for three fictional companies, each with a different audience | build + structure + a11y + responsive checks; screenshots for **blind human rating** (`blind.py`) |

## Task layout

```
tasks/<id>/
  task.json     id, title, timeout_minutes, setup (script), prepare (script), path_prepend, fixture, reference
  spec.md       the prompt, given to the agent verbatim after a short shared preamble
  fixture/      files copied into the throwaway workspace
  prepare.py    optional: python3 prepare.py <workspace>  (e.g. generate seeded data into the workspace)
  setup.sh      optional, idempotent, run once (venv, npm install); marker file .setup-done
  verify.py     python3 verify.py <workspace>  -> last stdout line is JSON {"score","total","checks":[{"name","pass","detail"}]}
  reference/    overlay on the fixture that makes a perfect solution (selftest proves the verifier works)
```

Hidden tests live in `verify.py` and its helpers, outside the workspace: the agent runs in a copy of
`fixture/` under the system temp dir (so it cannot see them, nor this repo's `AGENTS.md`).

## Scoring and caveats

- Hidden tests carry most of the score; an LLM or human rubric is a secondary signal. One run says little:
  use at least 3 runs per cell when a conclusion matters.
- Cloud baselines run with permissions bypassed inside the temp workspace; local models run through Pi.
- Results (`results/`) are gitignored.

## Reports

`reports/` holds the written results: one report per model, a comparison across models, screenshots of the
web outputs and a readable HTML page. See `reports/README.md` for how to add a model.
