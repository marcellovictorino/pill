# notes

A tiny note-taking CLI. Data lives in `$NOTES_HOME/notes.json` (default `~/.notes`).

```
notes add <text> [--tag t]...   add a note; prints "added #<id>"
notes ls [--tag t]              list notes, oldest first; --tag matches case-insensitively
```

Global flag `--format human|json` (default human). Exit codes: 0 ok, 1 failure, 2 usage error.

`ls` human output is one line per note, `#<id> <text>` followed by ` [tag1,tag2]` when the note has tags,
or `no notes`. Timestamps are RFC 3339 UTC.
