# Changelog

All notable changes to pill are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/) (0.x while the interface settles).

## [Unreleased]

### Added

- `pill catalog` records two more 24 GB M4 results: Gemma 4 26B-A4B UD-Q4_K_S failed Tier 1 on memory (6% free) and Qwen3.6 35B-A3B UD-Q3_K_S passed Tier 1 but Tier 2 scored 22, 22 and 20 of 22 with one run at the cap. Neither replaces MiniCPM. See `evals/reports/bigger-models-overnight.md`.
- `pill catalog` records Nemotron 3.5 Lightning 30B-A3B Q4_0 (ggml-org) on the 24 GB M4 as `nemotron3.5-30b`: it failed Tier 1 on memory in all three runs (3 to 4% free against the 10% floor), so Tier 2 was not run. See `evals/reports/tried-and-rejected.md`.
- `pill catalog` lists Nous Research Arete-27B on the 24 GB M4: `arete-27b` (IQ4_XS, Tier 2 22/22 in all three runs but Tier 1 129-144 s) and `arete-27b-mtp` (IQ3_XXS with the base model's MTP head, Tier 2 21, 22 and 20 of 22). Neither passes the default gate; MiniCPM stays the default. See `evals/reports/arete-27b.md`.

### Changed

- `pill catalog` shows the default MiniCPM model's Tier 1 and Tier 2 times again (`pass (36s)`, `22/22 3.6m`), taken from its 29 October 2026 benchmark run.

### Fixed

- pill is built with Go 1.27.4, which fixes a `net/http` vulnerability (GO-2026-6859) present in the Go 1.27.3 standard library that the router, Hugging Face and catalog downloads use.
- The router log `~/.pill/logs/server.log` no longer grows without limit. Whenever pill starts or checks the router (`pill serve`, `pill`, `pill bench run`, `pill add`, `pill rm`, `pill sync`, `pill service install`), a log over 64 MiB has its newest 16 MiB kept in `server.log.1` and is emptied in place, which also works while a router (detached or the launchd service) keeps running. An existing oversized log is trimmed the first time.
- When a llama.cpp upgrade removes the build the running router was started from, `pill doctor` now fails its `router` check and names the removed file, and `pill`, `pill serve`, `pill bench run`, `pill add`, `pill sync` and `pill rm` stop with the same message, instead of every model load timing out (which made `pill bench run` mark the model failed). The fix it gives is `pill stop --all`, then `pill serve`.

## [0.6.0] - 2026-11-06

### Changed

- `pill catalog` is one table with one row per model and short IDs. The columns are `id`, `quant`, `ctx`, `tier_1`, `tier_2` (x/22), `tier_3` (Go|cleaning|insight|Kanban|(nursery,Mars,tax)), `tok_s` and `free_mem`; the default is marked `*` to the left of its ID. Rows stay under 90 characters. The reasons, notes, run counts, swap, the separate download table and the `recommendations` and `tried_on_this_mac` blocks are gone, and the agent format adds a `source` field with the Hugging Face reference. A model measured on this Mac now shows its result even when its memory minimum says it is too small, so a failed run no longer appears as `too-small` in one place and `failed` in another.
- Catalog models have short IDs (`minicpm` for MiniCPM-V 4.7). `pill pull`, `pill add`, `pill bench run`, `pill default` and `pill rm` accept the ID, `id:quant` for a model with two files (`gemma4-26b:q3kxl`), and the old long names as before. The ID-to-source mapping lives in `catalog.toml`.
- `pill catalog` shows the measured Tier 1 time and Tier 2 time again, inside the existing columns (`pass (48s)`, `22/22 4.9m`), and marks a model's verdict after the ID: `✓` when it passed, `✗` only on a row whose Tier 1 reads `fail`, and nothing on a run that passed Tier 1 but failed another limit (such as swap growth), so it no longer reads as a pass. The agent format adds a `verdict` field, which says `FAIL` for every failed run. Tables print one space between columns, which keeps rows under 90 characters.
- `pill catalog` prints the human table by default, also when piped. The agent format is now `--format agent` (`--format toon` still works) and is accepted by every command. Callers that parsed the piped TOON of `pill catalog` must pass `--format agent`; the pill skill now says so.
- `pill catalog` reads as a report for people: it opens with `Chip` and `Memory` (`24GB RAM`) instead of `chip` and `ram_gb`, marks the default with `*` to the left of its ID, and shows Tier 1 as `pass (29s)`. On a terminal those facts, the column headings and the verdict marks (`✓` green, `✗` red) are in colour; piped output and `NO_COLOR` stay plain. The agent format keeps its field names.
- `pill catalog` has no relaxed verdict. A run that passed only with looser limits than pill's is a fail: Tier 1 reads `fail (111s)`, the mark is `✗` and the agent `verdict` is `FAIL`.

## [0.5.0] - 2026-11-06

### Added

- Homebrew source builds can include their source commit in `pill version`.
- `pill catalog` rows (recommendations and the "tried on this Mac" block) now carry `memory` (lowest free memory and swap growth) and `tier3` (score per task) when the measurement has them, and `-` when it does not. The "tried on this Mac" block also always lists a failed catalog model (such as the default MiniCPM-V 4.7, which fails Tier 1 on a 16 GB M2 Pro) even when it ranks below the closest five.
- Catalog measurements for a 16 GB Apple M2 Pro (Tier 1 and 2, llama.cpp build 11960, 2026-11-04): 12 runs including the default MiniCPM-V 4.7, which fails Tier 1 on memory there. `pill catalog` on that Mac now lists the closest tried models. See `evals/reports/m2-pro-16gb-2026-11-04.md`.
- `pill catalog` shows a `tried_on_this_mac` block when fewer than three models pass on this Mac. It lists the closest failed or relaxed runs on your exact chip and memory (up to five), with Tier 1 time, Tier 2 score and time, tokens per second and the reason, so a 16 GB Mac sees the best it can get. Catalog measurements now also record Tier 1 seconds, Tier 2 score and minutes, lowest free memory, swap growth, the failure reason and Tier 3 scores. Results from the benchmark reports are backfilled, including models that are not offered for download. A run that pushes more than 4 GB to swap no longer counts as a recommendation, and recommendation rows carry a `note`.
- `pill catalog` is per device. It detects this Mac's chip and memory and shows up to three models that passed both standard benchmark tiers on that exact chip and memory, with strength tags, tier results, tokens per second and when they were measured. It shows an empty list, not a guess, when no result exists for the device, and `unverified` when detection fails. The `recommended` column is now `yes`, `unmeasured`, `relaxed`, `failed`, `too-small` or `unverified`.
- The catalog lists MiniCPM-V 4.7 (`pill pull minicpm-v4-7`), the tested champion on a 24 GB M4. The catalog holds only references to model files, never weights.
- README: "Use the models from another agent" explains pointing any OpenAI-compatible tool at `http://127.0.0.1:11435/v1` after `pill serve` (any API key works, no authentication, local only).
- Shell completion offers model names. `pill add`, `pill pull` and `pill bench run` complete catalog and registered names (`add` also local GGUF files); `pill rm` and `pill default` complete registered names. Load it with `source <(pill completion zsh)`.

### Fixed

- `pill catalog` no longer recommends a model below its stated memory minimum (a 16 GB Mac used to be offered the 24 GB Gemma variant).
- `pill bench run` refuses to start while another benchmark is running, and `pill` refuses to open Pi while a benchmark runs or the router serves a model other than your default, so a second model can no longer be loaded under a running benchmark. A benchmark ended by a hangup, interrupt or termination now stops its Pi agent and everything the agent started, and the next benchmark ends an agent left behind by one that was killed with SIGKILL.
- A scratch `PILL_HOME` now ignores a LaunchAgent installed for another pill home and starts a detached router. It cannot replace or remove that LaunchAgent through `pill serve` or `pill service`.
- `tok_per_s` in `pill bench run` and `bench summary` measures real decoding speed. It used to come from the agent-sized request, whose reply is only a few tokens, so the figure was mostly start-up overhead (5 tok/s for a model that decodes at 28). Tier 1 now times a 128-token generation on the warm model; results recorded before this change keep the old figure.

## [0.4.1] - 2026-10-08

### Changed

- The logo is one ASCII row, `(pi|ll)`: the word as a capsule, left half blue, right half on a red block. It sits on its own line above the output.

### Fixed

- The Tier 2 verifier ignores property order. A correct `newGame` returning `{turn, board, winner}` used to fail the shape and mutation checks and lose its model. The verifier still rejects a `legalMoves` array with a missing element.
- `pill bench run` exits 1 on any failed result, including when you decline to demote a model that passed before, so `bench run ... && ...` stops. `bench summary` puts only passing runs in the top group.
- Every change to the model list runs under a cross-process lock, including the regeneration of `models.ini` and Pi's provider. A benchmark that finishes while you `add`, `rm` or `pull` no longer overwrites your change. It records no verdict for a model removed or changed in the meantime. `pill rm` of an unknown name writes nothing.
- pill tracks each downloaded file by repository and remote path, not by name alone:
  - Two repositories with the same file name no longer share or overwrite one file, even when pulled at the same moment.
  - Explicit file references follow the same rule, and a quantisation never picks up another repository's file.
  - A name that exists in several folders of one repository needs its folder: `owner/repo/sub/model.gguf`.
  - A registered model keeps its Hugging Face source, so `pill sync` works on another machine.
  - A catalog model referenced by repository resolves to the same entry before and after download.
- `pill sync` registers every declared model whose file is present, such as two context sizes sharing one GGUF, and keeps existing benchmark verdicts.
- A context size that is not a multiple of 1024 gets its own exact name, so `8192` and `9000tok` no longer collide on `8k`. Those names resolve again. A lowercase `ud-` quantisation matches the catalog.
- `pill default` keeps `config.toml` valid when the file has an indented or quoted `default` key, nested arrays or multi-line strings. It refuses to write a file it cannot read back.
- `pill stop` and `pill bench` never unload models on, or stop, a llama-server that pill did not start. `pill stop --all` ends the router, or boots the LaunchAgent out, even when `/health` fails or unloading errors. pill reports unload failures and timeouts instead of treating them as success. pill decides ownership of the port from the process, not from `/health`.
- A pid file left by a router on another `PILL_PORT` no longer lets `pill stop` end the wrong router.
- A LaunchAgent router notices changed models, presets, removed models and changed settings (port, paths, idle time). `pill serve` refreshes the installed LaunchAgent for them, unless its router has a model loaded or another process holds the new port. `pill stop --all` on a different `PILL_PORT` leaves the service alone. `pill doctor` warns when the service is out of date.

## [0.4.0] - 2026-10-08

### Added

- `pill service install|uninstall`: an optional launchd LaunchAgent that starts the router at login and restarts it if it dies. `pill serve`, `pill stop --all`, `pill ps` and `pill doctor` understand it (`stop --all` boots the job out, because a plain SIGTERM would be undone by KeepAlive).
- `pill skill install [--dir]`: install the embedded agent skill into Pi's skills directory.
- `pill sync [--bench]`: download the models declared in `models.toml` that are missing on this machine and register them as unverified; `--bench` then benchmarks the unverified ones.
- `pill bench summary` has a `result` column: a run can score 22/22 and still fail the gate (for example by hitting the Tier 2 cap).
- `pill bench run` names a stuck agent: when Tier 2 hits its cap because the model repeated the same tool call over and over, the reason says so.

### Changed

- The Tier 1 prompt is calibrated to about 8K tokens (measured 7.9K on Gemma 4), as designed.

## [0.3.0] - 2026-10-08

### Added

- `pill bench run <name> [--runs N]`: the gate. Tier 1 sends an agent-sized request (about 8K tokens) from a cold start and runs a Pi tool-calling turn; Tier 2 has the model build a small tic-tac-toe engine and CLI from a spec under a 15 minute cap, scored by a hidden 22-check verifier run with Node. A model passes only when every run passes. Memory (free %, swap, pressure, GPU) is sampled every 5 seconds throughout. Thresholds are flags (`--tier1-max-seconds`, `--min-free-pct`, `--tier2-max-minutes`).
- A failed gate exits 1 and removes the model from Pi; a pass removes the `(unverified)` marker. Demoting a previously passed model asks for confirmation on a terminal.
- `pill bench summary [--all]`: ranking of benchmarked models, flagging results from an older llama.cpp build.
- `pill ls` shows a note column; `pill doctor` warns when a benchmark predates the installed llama.cpp.
- Results are written to `~/.pill/benchmark/<timestamp>-<name>/result.json`.

## [0.2.0] - 2026-10-08

### Added

- `pill pull <name>`: download a GGUF from Hugging Face (catalog name, `hf.co/owner/repo:QUANT`, or `owner/repo/file.gguf`) with resume and sha256 verification against Hugging Face's checksum. `HF_TOKEN` is read from the environment and never stored. Pull never registers a model.
- `pill ls`: local models with state (`pulled`, `unverified`, `passed`, `failed`, `missing`), size and default marker.
- `pill rm <name>`: remove an entry from models.toml, models.ini and Pi; delete the GGUF when no other entry uses it. Removing something already gone is a no-op.
- `pill catalog`: built-in models with the variant recommended for this machine's RAM.
- Models outside the catalog work with `--ctx` and generic defaults.

## [0.1.0] - 2026-10-08

### Added

- `pill` / `pill pi`: start the llama.cpp router if needed and open Pi on the default model with thinking off.
- `pill serve`, `pill stop [--all]`, `pill ps`: manage the detached router.
- `pill add <name> --unverified` and `pill default`: register local GGUFs with the router and Pi.
- `pill doctor` and `pill version`.
- Generated `models.ini` for llama-server router mode; the `pill` provider is merged into Pi's `models.json` without touching other providers.
- Built-in catalog with Gemma 4 26B-A4B (UD-IQ4_XS and UD-Q3_K_XL, 64K context).
- Human output on a terminal; TOON output, `help[n]:` next steps and exit codes 0/1/2 otherwise.
