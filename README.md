# pill

Simple CLI to run the [Pi](https://github.com/badlogic/pi-mono) coding agent with local models served by [llama.cpp](https://github.com/ggml-org/llama.cpp).

> Issues are welcome. Pull requests are not accepted; please open an issue instead.

`pill` keeps a small llama.cpp router running, registers your local GGUF models with it and with Pi, and opens Pi on your default model. Running `pill` is all it takes. The router also works without Pi: see [Use the models from another agent](#use-the-models-from-another-agent).

Apple Silicon macOS only for now.

## Requirements

pill orchestrates; it never installs or patches these:

- [llama.cpp](https://github.com/ggml-org/llama.cpp) (`llama-server`): `brew install llama.cpp`
- [Pi](https://github.com/badlogic/pi-mono) (`pi`): `brew install pi-coding-agent`
- Go, to build pill: `brew install go`

## Install

```bash
go install github.com/marcellovictorino/pill@latest
# make sure Go's bin directory is on your PATH:
echo 'export PATH="$HOME/go/bin:$PATH"' >> ~/.zshrc
```

While developing, `go install .` from a checkout does the same.

## Quick start

```bash
pill catalog                                 # what pill knows how to download
pill pull gemma4-26b                         # download the recommended variant (13.6 GB)
pill add gemma4-26b-iq4xs-64k --unverified   # register it with the router and Pi
pill default gemma4-26b-iq4xs-64k            # make it the default
pill doctor                                  # check prerequisites and configuration
pill                                         # start the router if needed, open Pi
```

Models outside the catalog work too: `pill pull hf.co/owner/repo:Q4_K_M`, then `pill add hf.co/owner/repo:Q4_K_M --unverified --ctx 32768`. Gated repos need `HF_TOKEN` in your environment (it is never stored).

`--unverified` is deliberate: a model that has not passed `pill bench run` on this machine is marked `(unverified)` everywhere it is shown.

If you already have the GGUF files, skip `pill pull` and move them into place:

```bash
mkdir -p ~/.pill/models && mv ~/models/*.gguf ~/.pill/models/
```

## Use the models from another agent

Pi is optional. `pill serve` starts the router and returns; anything that speaks the OpenAI API can then use your registered models.

```bash
pill serve     # start the router (no-op when it is already running)
pill ps        # port, loaded model and its memory
```

Point the other tool at:

| Setting | Value |
|---|---|
| Base URL | `http://127.0.0.1:11435/v1` (chat completions at `/v1/chat/completions`, model list at `/v1/models`) |
| API key | anything, for example `local`. The router does not check it, but most clients insist on one. |
| Model | a name from `pill ls`, such as `gemma4-26b-iq4xs-64k` |

```bash
curl http://127.0.0.1:11435/v1/chat/completions \
  -H 'Content-Type: application/json' -H 'Authorization: Bearer local' \
  -d '{"model": "gemma4-26b-iq4xs-64k", "messages": [{"role": "user", "content": "Say hi"}]}'
```

Things to know:

- The port is `11435`, or whatever `PILL_PORT` or `port` in `config.toml` says. `pill ps` prints the one in use.
- The router listens on `127.0.0.1` only and has no authentication. Other machines cannot reach it; every program on this Mac can.
- `pill serve` starts the router, not a model. The first request to a model loads it, which takes a while on a cold start. The router unloads it after 15 idle minutes (`idle_seconds` in `config.toml`) and reloads it on the next request.
- One model is loaded at a time. A request for another model unloads the current one, so two agents on different models take turns.
- The router stays up until `pill stop --all`. For one that starts at login and restarts if it dies, run `pill service install`.

## Commands

| Command | What it does |
|---|---|
| `pill`, `pill pi [pi args…]` | Start the router if needed, then open Pi on the default model (`--thinking off`). Your own `--model`/`--thinking` win. |
| `pill serve [--foreground]` | Start the router (no-op when running). |
| `pill stop [--all]` | Unload the loaded model; `--all` also stops the router. |
| `pill ps` | Router state, port, loaded model and its memory. |
| `pill ls` | Local models: name, state (`pulled`, `unverified`, `passed`, `failed`, `missing`), size, default. |
| `pill pull <name>` | Download a GGUF (catalog name or Hugging Face reference). Resumes, verifies sha256, never registers. |
| `pill rm <name>` | Remove the entry and its Pi registration; delete the GGUF when no other entry uses it. |
| `pill catalog` | Built-in models and the variant recommended for this machine's RAM. |
| `pill bench run <name> [--runs N]` | Gate a model: cold-start agent-sized request, Pi tool-calling turn, then a Tier 2 coding task scored by a hidden verifier (needs Node). A failure exits 1 and removes the model from Pi. |
| `pill bench summary [--all]` | Rank benchmarked models by Tier 2 score, speed and memory headroom. |
| `pill sync [--bench]` | Restore a new machine from `~/.config/pill/models.toml`: download missing models and register them as unverified; `--bench` benchmarks the unverified ones. |
| `pill service install\|uninstall` | Run the router as a launchd LaunchAgent (starts at login, restarts if it dies). |
| `pill skill install [--dir]` | Install the pill skill for Pi (`~/.pi/agent/skills/pill/SKILL.md`). |
| `pill add <name> --unverified` | Register a local GGUF (catalog name, `.gguf` file or Hugging Face reference). |
| `pill default [name]` | Set or show the default model. |
| `pill doctor` | Check prerequisites, router health and Pi's provider. |
| `pill version` | Print the version. |

Add `--format toon|human` to force an output format. On a terminal you get tables and colour; when piped, or run by an agent, you get compact [TOON](https://github.com/toon-format/toon) with `help[n]:` next steps, and exit codes `0` (ok), `1` (failure), `2` (usage error).

## Where things live

| Path | Purpose |
|---|---|
| `~/.config/pill/` (or `$XDG_CONFIG_HOME/pill`) | Portable, small: `config.toml`, `models.toml`. Safe to keep in dotfiles. |
| `~/.pill/` | Machine-local: `models/` (GGUFs), generated `models.ini`, `results.json`, `benchmark/`, `logs/`, `run/`. |

pill only ever writes its own `pill` provider into `~/.pi/agent/models.json` (after backing the file up to `models.json.bak`). Other providers are never touched.

Environment: `PILL_PORT` (default `11435`), `PILL_HOME` (relocates everything, used by tests), `NO_COLOR`.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Bug reports, model results and ideas are welcome as issues.

## Licence

MIT, see [LICENSE](LICENSE).
