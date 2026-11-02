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

## Adjudication

```
rater adjudicate --task <dir> --as <adjudicator>
```

`adjudicate` compares the annotators' latest labels the way `agree` does and shows only the
items they disagree on: at least two annotators completed the item and their labels, or for a
checklist their answers to some criterion, differ. Held items and incomplete checklists count as
missing, so an item that fewer than two annotators completed is neither shown nor written to
gold. Each annotator's label (or answers per criterion, with their verdict) and note appear next
to the item; the header shows `resolved n / total`.

The adjudicator answers with the same keys as `annotate`: a label or one answer per criterion
(the verdict follows from the task's rule), `tab` to hold, `ctrl+n` for a note, and `ctrl+u` to
undo. Decisions are appended to `adjudications/<adjudicator>.jsonl` as they happen, so a later run
resumes at the first undecided disagreement. Each decision records a `basis`, a digest of the
annotations shown when it was made; if an annotator later changes their label, answers, or note
on that item, or another annotator joins, the item is shown as undecided again.

On quit, `gold.jsonl` is rewritten from the logs, one line per item in `items.jsonl` order:

- `"source": "agreed"` for an item every completing annotator answered the same way.
- `"source": "adjudicated"` for a disagreement the adjudicator decided, with `adjudicator` and
  `note`.

Each line carries `label`, or `answers` and `verdict`, and `annotations`: every annotator's
status, answer, note, and `records`, the number of lines of their append-only log that were
replayed, so the compared annotations can be found again. Undecided disagreements are left out.

A task has one adjudicator, because `gold.jsonl` is rebuilt from one adjudication log.
`adjudicate` refuses to start, before writing anything, when `adjudications/` holds a log of
anyone other than `--as`, and names whose log it found; the same adjudicator can always resume.
To hand over, move the previous adjudicator's log out of `adjudications/` or rename it so it no
longer ends in `.jsonl` (for example `carol.jsonl.handed-over`). The new adjudicator then starts
from no decisions, and the previous decisions leave `gold.jsonl` when the new adjudicator quits.
`rater adjudicate --help` repeats this rule. The check and the creation of the log run under an
exclusive lock on `adjudications/`, so two adjudicators starting at the same moment on a fresh
task cannot both pass it.

## Assignment

An optional `assignment.yaml` in the task directory limits the items each annotator sees, in
both `annotate` and `serve`. Annotators it does not name see every item.

```yaml
split: # deal items round-robin to a group
    annotators: [alice, bob, carol]
    copies: 2 # each item goes to 2 of them
items: # or list the items of one annotator
    dave: [item-1, item-7]
```

`split` gives item number `i` (in `items.jsonl` order, from 0) to annotators `i`, `i+1`, ...,
`i+copies-1` of the list, wrapping around, so every item gets `copies` annotators and, with
`copies` of 2 or more, every pair of neighbours in the list shares items for `agree`. `copies`
must be between 1 and the number of annotators, and every annotator must get at least one item.
An annotator may appear in `split` or `items`, not both, and listed items must exist. The file
lives apart from `task.yaml` because it is about people, not the task: it changes as annotators
join or leave, while `task.yaml` and the logs stay as they are. If the assignment changes,
records about items no longer assigned stay in the log and still count in `agree` and
`adjudicate`; the annotator just no longer sees those items.

## Serving over SSH

```
rater serve --task <dir> --listen <addr> --keys <file> [--host-key <path>]
```

`serve` shows the `annotate` screen to annotators over SSH, so they need only an SSH client:

```
ssh -p 2222 rater.example.org
```

Each connection runs its own screen at the client's terminal size, capped at 1000 columns by 500
rows, and follows its resizes. In the note field, the terminal's own paste works but `ctrl+v` is
off, since it would read the server's clipboard. The
server gives no shell: commands (`ssh host ls`), subsystems such as SFTP, port forwarding, and
sessions without a terminal (`ssh -T`) are refused.

The keys file registers who may connect. Each line holds an annotator name and one public key in
`authorized_keys` form; an annotator may have several lines, and `#` starts a comment:

```
alice ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA... alice@laptop
alice ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA... alice@desktop
bob   ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ...
```

Only public-key logins with a registered key are accepted; passwords are not. The key decides the
annotator, so the SSH user name does not matter and nobody can type another name. Key options such
as `from="..."` are rejected rather than ignored, and each key may appear once.

Annotation is blind: a session replays and shows only its own annotator's log, so nobody sees
another annotator's labels, notes, or progress. `adjudicate` is not served; run it locally.

An annotator has one session at a time. Each session holds the exclusive lock on its annotator's
log (the same lock that keeps two local `annotate` runs apart), so a second connection, or a local
`annotate --as` for the same name, is refused with a message until the first quits. A dropped
connection ends its session and releases the lock.

The host key (`--host-key`, default `<dir>/host_ed25519`) is generated as Ed25519 with mode
`0600` on the first start and reused afterwards, so clients see the same host key; the server
refuses a key file that group or others can access. Keep it out of version control. The first
line `serve` prints includes the key's SHA256 fingerprint for annotators to compare.

`SIGINT` or `SIGTERM` stops accepting connections, quits every session, and returns once each has
closed its log; a second signal kills the server. Every label is already on disk when it is made,
so nothing is lost either way.

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
