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
to the next and previous hunk; the position line shows which hunk is being read, and `]` reaches
every hunk even when the last ones fit on screen together.

## Agreement

```
rater agree --task <dir> [--format text|json] [--annotators a,b]
```

`agree` reads every `annotations/<annotator>.jsonl` (or only the listed annotators), replays each
log the way `annotate` resumes it (undo and hold included), and compares the latest labels.
Held items and incomplete checklists are left out and counted per annotator. It reports:

- Krippendorff's alpha (nominal) over all selected annotators, using every item at least two of
  them labeled.
- For exactly two annotators, Cohen's kappa with observed and expected agreement, and the
  confusion matrix of the label (or of the verdict for a checklist).
- For a checklist, one row per criterion and one for the verdict, recomputed from the task's rule.

A coefficient that cannot be computed, because no item was labeled by enough annotators or
every compared value is the same category, is shown as `n/a` with the reason (`null` with an
`undefined` field in JSON). Weighted kappa, ordinal or interval alpha, and confidence intervals
are not computed. `agree` takes no lock, so it can run while others annotate.

## Layout

`layout: stacked` (default) shows the fields one under another. `layout: side-by-side` shows
exactly two fields in two columns, each scrolling on its own: the scroll and hunk keys act on the
focused column (marked `▶` on the position line) and `ctrl+w` switches columns. On a terminal
narrower than 63 columns the fields are stacked and a notice says so.

## Labels and checklists

A task answers each item in one of two ways:

- `labels`: one single-choice label per item; its key labels the item and moves on.
- `checklist`: one answer per criterion. An answer key answers the focused criterion (`▶`) and
  moves the focus to the next unanswered one; `shift+↑`/`shift+↓` move the focus. Once every
  criterion has an answer the item counts as labeled and the next item opens. Each answer is one
  log record, so `ctrl+u` undoes one answer.

```yaml
checklist:
    criteria:
        - { name: concrete, title: Names a concrete code construct }
        - { name: checkable }
    answers:
        - { key: y, name: "yes" }
        - { key: "n", name: "no" }
        - { key: "?", name: unsure }
    verdict: { rule: all_pass, pass: "yes", fail: "no" }
```

The verdict is derived from the answers by `verdict.rule` and shown below the criteria; it is not
written to the log. `all_pass` passes when every criterion has the `pass` answer, fails as soon as
any has the `fail` answer, and is undecided otherwise.

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
    - { name: snippet, render: code, language: go, language_field: lang }
keys:
    next_hunk: "n"
```

## Keys

`keys` rebinds `scroll_up`, `scroll_down`, `page_up`, `page_down`, `next_hunk`, `prev_hunk`,
`prev_criterion`, `next_criterion`, and `switch_pane`, written as Bubble Tea names such as `down`,
`ctrl+d`, or `]`. Spellings of one key are treated alike: `shift+s` is `S`, modifiers may come in
any order (`shift+ctrl+j` is `ctrl+shift+j`), and `escape`, `return`, `pageup`, and `pagedown` mean
`esc`, `enter`, `pgup`, and `pgdown`. Label and answer keys are one character and accept the same
spellings. A key may not be bound twice or reuse a label key, an answer key, or a fixed control
key; `task.yaml` is rejected if it does.
