# Changelog

All notable changes to pill are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/) (0.x while the interface settles).

## [Unreleased]

## [0.1.0] - 2026-10-08

### Added

- `pill` / `pill pi`: start the llama.cpp router if needed and open Pi on the default model with thinking off.
- `pill serve`, `pill stop [--all]`, `pill ps`: manage the detached router.
- `pill add <name> --unverified` and `pill default`: register local GGUFs with the router and Pi.
- `pill doctor` and `pill version`.
- Generated `models.ini` for llama-server router mode; the `pill` provider is merged into Pi's `models.json` without touching other providers.
- Built-in catalog with Gemma 4 26B-A4B (UD-IQ4_XS and UD-Q3_K_XL, 64K context).
- Human output on a terminal; TOON output, `help[n]:` next steps and exit codes 0/1/2 otherwise.
