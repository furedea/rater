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

Long bodies scroll: `↑`/`↓` move a line and `pgup`/`pgdown` a page. In a diff, `]` and `[` jump
to the next and previous hunk.

## Fields

Each field in `task.yaml` sets how its text is shown with `render`:

| `render`         | Shows                                                                           |
| ---------------- | ------------------------------------------------------------------------------- |
| `text` (default) | plain text, wrapped                                                             |
| `diff`           | a unified diff coloured by line kind (file header, hunk header, added, removed) |
| `code`           | source highlighted by Chroma                                                    |
| `markdown`       | Markdown rendered by Glamour                                                    |

A `code` field takes its language from `language` (a Chroma language name, alias, or file name)
or, per item, from the item field named by `language_field`; an item value Chroma does not know
falls back to `language`, then to a guess from the source.

```yaml
fields:
  - {name: snippet, render: code, language: go, language_field: lang}
keys:
  next_hunk: "n" # scroll_up, scroll_down, page_up, page_down, next_hunk, prev_hunk
```

`keys` rebinds the scroll and hunk keys, written as Bubble Tea names such as `down`, `ctrl+d`, or
`]`. A key may not be bound twice or reuse a label key or a fixed control key; `task.yaml` is
rejected if it does.
