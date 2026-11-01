# rater

A terminal annotation tool for research manual checks.

## Usage

```
rater annotate --task <dir> --as <annotator>
```

A task directory holds `task.yaml` and `items.jsonl`; annotations are appended to
`annotations/<annotator>.jsonl`. See [examples](examples/readme.md).

Keys: a label key (from `task.yaml`) labels and advances, `tab` holds, `←`/`→` move,
`ctrl+u` undoes the latest action, `ctrl+n` edits the note, `esc` quits. Every action is saved
as it happens, so quitting or crashing loses nothing.
