# AGENTS.md

`notes` is a small Go CLI (module `example.com/notes`) that keeps notes in `$NOTES_HOME/notes.json`
(default `~/.notes`).

## Commands

```bash
go build ./...      # compile (dependencies are vendored; the build works offline)
go test ./...       # unit tests
go vet ./...
gofmt -l .          # must print nothing
```

Run `go test ./...`, `go vet ./...` and `gofmt -l cmd internal main.go` before you finish.

## Rules

- **One cobra command per file** in `cmd/` (`cmd/add.go`, `cmd/ls.go`, ...), registered in `newRoot` in `cmd/root.go`.
- **Output goes through `internal/output`.** Every command honours `--format human|json`; JSON is one compact line.
- **Exit codes:** 0 ok, 1 runtime failure, 2 usage error (unknown flag or command, wrong arguments, bad `--format`).
  Return `usageErrorf(...)` for usage errors; any other error is a failure. Errors are printed to stderr as
  `error: <message>` and nothing is printed to stdout.
- **No prompts, ever.** Repeating a command that changes state to the same end result is a no-op, not an error.
- **Tests never touch the real home.** Use `t.TempDir()` with `NOTES_HOME`. `NOTES_NOW` (RFC 3339) fixes the clock.
  Tests are table-driven; see `cmd/cmd_test.go`.
- **No new dependencies.** Do not change `go.mod`, `go.sum` or `vendor/`.
- **Changelog:** every user-visible change adds a line under `## [Unreleased]` in `CHANGELOG.md`
  (Keep a Changelog).

## Layout

```
main.go                  calls cmd.Execute
cmd/                     one cobra command per file; root.go wires them
internal/store           the notes file (JSON, atomic writes)
internal/output          --format handling
```
