# pill

Simple CLI to run the [Pi](https://github.com/badlogic/pi-mono) coding agent with local models served by [llama.cpp](https://github.com/ggml-org/llama.cpp).

> Issues are welcome. Pull requests are not accepted; please open an issue instead.

`pill` keeps a small llama.cpp router running, registers your local GGUF models with it and with Pi, and opens Pi on your default model. Running `pill` is all it takes.

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

With a GGUF already in `~/.pill/models`:

```bash
pill add gemma4-26b-iq4xs-64k --unverified   # register it with the router and Pi
pill default gemma4-26b-iq4xs-64k            # make it the default
pill doctor                                  # check prerequisites and configuration
pill                                         # start the router if needed, open Pi
```

`--unverified` is deliberate: a model that has not passed `pill bench run` on this machine is marked `(unverified)` everywhere it is shown.

If your GGUFs live somewhere else (for example `~/models`), move them into place first:

```bash
mkdir -p ~/.pill/models && mv ~/models/*.gguf ~/.pill/models/
```

## Commands

| Command | What it does |
|---|---|
| `pill`, `pill pi [pi args…]` | Start the router if needed, then open Pi on the default model (`--thinking off`). Your own `--model`/`--thinking` win. |
| `pill serve [--foreground]` | Start the router (no-op when running). |
| `pill stop [--all]` | Unload the loaded model; `--all` also stops the router. |
| `pill ps` | Router state, port, loaded model and its memory. |
| `pill add <name> --unverified` | Register a local GGUF (catalog name, `.gguf` file or Hugging Face reference). |
| `pill default [name]` | Set or show the default model. |
| `pill doctor` | Check prerequisites, router health and Pi's provider. |
| `pill version` | Print the version. |

Add `--format toon|human` to force an output format. On a terminal you get tables and colour; when piped, or run by an agent, you get compact [TOON](https://github.com/toon-format/toon) with `help[n]:` next steps, and exit codes `0` (ok), `1` (failure), `2` (usage error).

## Where things live

| Path | Purpose |
|---|---|
| `~/.config/pill/` (or `$XDG_CONFIG_HOME/pill`) | Portable, small: `config.toml`, `models.toml`. Safe to keep in dotfiles. |
| `~/.pill/` | Machine-local: `models/` (GGUFs), generated `models.ini`, `results.json`, `logs/`, `run/`. |

pill only ever writes its own `pill` provider into `~/.pi/agent/models.json` (after backing the file up to `models.json.bak`). Other providers are never touched.

Environment: `PILL_PORT` (default `11435`), `PILL_HOME` (relocates everything, used by tests), `NO_COLOR`.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Bug reports, model results and ideas are welcome as issues.

## Licence

MIT, see [LICENSE](LICENSE).
