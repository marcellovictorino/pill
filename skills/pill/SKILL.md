---
name: pill
description: Manage local LLMs for the Pi coding agent with the pill CLI (llama.cpp router on Apple Silicon). Use when asked to download, register, benchmark, switch, start or stop a local model, or to diagnose why the local model is not working.
---

# pill

`pill` runs the Pi coding agent on local models served by a llama.cpp router on
`127.0.0.1:11435`. Every command is non-interactive when piped and prints
compact [TOON](https://github.com/toon-format/toon); each result ends with a
`help[n]:` list of next commands. Exit codes: `0` ok, `1` failure,
`2` usage error (the valid flags or commands are listed).

## Common tasks

| Goal | Command |
|---|---|
| What is installed, and is it proven here? | `pill ls` (state: `pulled`, `unverified`, `passed`, `failed`, `missing`) |
| Is the router up, what is loaded? | `pill ps` |
| Something is wrong | `pill doctor` (fix the `fail` and `warn` rows first) |
| Models pill can download | `pill catalog` |
| Download a model | `pill pull <name>` (catalog name or `hf.co/owner/repo:Q4_K_M`); resumes and verifies sha256 |
| Offer it to Pi | `pill add <name> --unverified` |
| Prove it works on this Mac | `pill bench run <name>` (about 25 minutes; exit `1` means it failed the gate) |
| Compare benchmarked models | `pill bench summary` |
| Change the default model | `pill default <name>` |
| Free the model's memory | `pill stop` (`pill stop --all` also stops the router) |
| Restore a fresh Mac from `~/.config/pill/models.toml` | `pill sync [--bench]` |
| Let another agent use the models | `pill serve`, then point the client at `http://127.0.0.1:11435/v1` with any API key and a model name from `pill ls` |

## Rules

- A `failed` model never reaches Pi; a model that has not passed `pill bench run`
  is labelled `(unverified)` everywhere. Do not describe an unverified model as
  proven.
- `pill bench run` takes a long time and loads a large model. Run it in the
  background and check the result with `pill bench summary`.
- Never edit `~/.pi/agent/models.json`, `models.ini` or `results.json` by hand:
  pill regenerates them. Pill only ever writes its own `pill` provider into Pi.
- Do not pass `--yes`-style flags to hide a failing gate. `pill add <name>
  --unverified` is the only override, and it keeps the model labelled.
- The router listens on `127.0.0.1` only and has no authentication. It loads a model on the first request, keeps one loaded at a time and unloads it after 15 idle minutes.
- Downloads need `HF_TOKEN` in the environment for gated repositories; pill
  never stores it.
