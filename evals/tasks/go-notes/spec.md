This directory is a small Go CLI called `notes` (read `README.md` and `AGENTS.md` first, and follow the repo conventions in `AGENTS.md`). Dependencies are vendored; do not add any. Make these changes, then run the build, vet, gofmt and tests as `AGENTS.md` says, and stop with a one-line summary.

## 1. New command `notes stats`

Takes no arguments. Summarises all notes.

Human output (exact, one item per line, tag lines indented by two spaces):

```
total: 3
oldest: 2026-01-02T03:04:05Z
newest: 2026-03-04T05:06:07Z
tags:
  work: 2
  home: 1
```

- `oldest` / `newest` are the earliest and latest note creation times, RFC 3339 UTC.
- Tags are counted case-insensitively and printed in lowercase (`Work` and `work` are the same tag). A note counts once per distinct tag, even if it carries the tag twice.
- Tag lines are sorted by count descending, then by tag name ascending (bytewise, on the lowercase name).
- With no notes: `total: 0`, `oldest: -`, `newest: -`, `tags: none` (four lines). With notes but no tags the last line is also `tags: none`.

With `--format json` print one compact line:
`{"total":3,"oldest":"2026-01-02T03:04:05Z","newest":"2026-03-04T05:06:07Z","tags":[{"tag":"work","count":2},{"tag":"home","count":1}]}`.
With no notes: `{"total":0,"oldest":null,"newest":null,"tags":[]}`. `tags` is always an array.

## 2. New command `notes rm <id>...`

Removes the notes with the given ids. It takes one or more ids; ids are positive integers. All ids are validated first: if any argument is not a positive integer, or none is given, it is a usage error (exit 2) and nothing is removed.

Removing an id that does not exist is not an error. Ids are processed in argument order, so repeating an id removes it the first time and reports it as missing the second time. Exit code 0 whenever the arguments are valid, even if every id was missing.

Human output: one line per id in argument order, `removed #3` or `not found #7`.
JSON output: `{"removed":[3],"missing":[7]}` with ids in argument order, both arrays always present (`[]` when empty).

An id that has ever been used is never reused: after `rm`, new notes keep getting fresh, increasing ids.

## 3. Bug to fix

`notes ls --tag work` does not list notes that were added with `--tag Work` (or `WORK`). Tag filtering is documented as case-insensitive and must work that way, including for non-ASCII tags such as `Ärger` and `ärger`.

## 4. Conventions

Follow `AGENTS.md`: each new command in its own file under `cmd/`, registered in `cmd/root.go`; honour `--format`; correct exit codes and `error:` messages on stderr; add table-driven tests for `stats` and `rm` in the existing style (tests must not touch the real home); keep `gofmt`, `go vet` and `go test ./...` clean; do not touch `go.mod`, `go.sum` or `vendor/`; add lines under `## [Unreleased]` in `CHANGELOG.md` for the user-visible changes (the two new commands and the fix).
