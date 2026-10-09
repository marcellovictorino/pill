# AGENTS.md

pill is a Go CLI (module `github.com/marcellovictorino/pill`) that runs the Pi coding agent against local models served by llama.cpp's `llama-server` router. Apple Silicon macOS only.

## Commands

```bash
go build ./...                 # compile
go test ./...                  # unit tests (no real model, no network)
go vet ./...
golangci-lint run ./...        # config in .golangci.yml (brew install golangci-lint)
go test ./cmd -update          # rewrite golden files after an intended output change
```

Run `go test ./...` and `golangci-lint run ./...` before every commit.

## Rules

- **Changelog:** every user-visible change adds a line under `## [Unreleased]` in `CHANGELOG.md` (Keep a Changelog). The version constant in `internal/version` and the top released section move together.
- **Tests never touch the real home.** Use `t.TempDir()` with `PILL_HOME`, `PI_CODING_AGENT_DIR` and `HOME` (see `newEnv` in `cmd/cmd_test.go`). Fake `llama-server`, `pi` and `launchctl` come from `internal/testutil` (test-binary re-exec); no test needs a real model.
- **No benchmark-lab references.** pill is self-contained; do not link to or depend on outside local projects or paths.
- **Pi's `models.json`:** pill writes only its own `pill` provider key, backs up first, writes atomically, and preserves every other key.
- **Idiom:** add a short comment where a Go feature is non-obvious (embedding, build tags, `syscall.Exec`, test-binary re-exec).

## Agent-facing output (axi)

When stdout is not a terminal, output is TOON on stdout: minimal fields, `help[n]:` next steps, definitive empty states (`models[0]:`), errors as `error:` plus `help[n]:` on stdout. Exit codes: 0 ok, 1 failure, 2 usage error (unknown flag or command, with the valid choices listed). No prompts, ever; repeating a command is a no-op, not an error. `--format toon|human` overrides detection; `NO_COLOR` is respected; the logo only appears on a terminal with no agent environment variable set.

## Layout

```
main.go                 calls cmd.Execute()
skills/                 the agent skill (SKILL.md), embedded in the binary
cmd/                    one cobra command per file
internal/config         paths, config.toml, models.toml, results.json (state)
internal/catalog        embedded catalog.toml
internal/preset         models.ini generation
internal/router         llama-server start/stop/probe and HTTP calls
internal/pi             Pi models.json merge, launching pi
internal/registry       joins models.toml + results.json, regenerates models.ini and Pi's provider
internal/sysinfo        memory/GPU/process info (darwin build tag) + fake
internal/output         TOON encoder, human rendering, logo, errors
internal/bench          Tier 1 and Tier 2 benchmark, memory sampler, summary; tasks/ embedded
internal/service        launchd LaunchAgent plist and launchctl calls
internal/version        version constant
internal/testutil       fake external programs for tests
```
