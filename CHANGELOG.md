# Changelog

All notable changes to pill are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/) (0.x while the interface settles).

## [Unreleased]

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
