# rater

A terminal annotation tool for research manual checks.

Papers that build a dataset or evaluate a tool often need people to check a few hundred items by
hand: are these two review comments the same finding, does this guideline sentence meet five
criteria, is this diff relevant to that rule. Web tools such as Label Studio or doccano are heavy
for that, and a spreadsheet loses track of who said what. rater keeps the whole loop in the
terminal:

- `annotate` shows one item at a time and takes one key per label, with notes, undo, hold, and
  resume. Every action is appended to the annotator's own JSONL log as it happens.
- `serve` hands the same screen to other annotators over SSH, blind to each other's labels.
- `agree` reports Cohen's kappa and Krippendorff's alpha, per criterion for checklists.
- `adjudicate` shows only the disagreements and writes the resolved `gold.jsonl`.

Items can be plain text, unified diffs, highlighted code, or Markdown, stacked or side by side.

```
guideline-criteria  1 / 3  labeled 0

id: g-001

Guideline sentence
Functions should do one thing and be named after what they return or change.

lines 1–2 of 2

[y] yes   [n] no   [?] unsure
  Names a concrete code construct  yes
  Can be judged from a diff alone  no
▶ Says what to do                  ·
  States where it applies          ·
  Covers one rule only             ·
verdict: fail

state: 2 of 5 answered

key: answer · tab: hold · ←/→: prev/next · ctrl+u: undo · ctrl+n: note · esc: quit
up/down: scroll · pgup/pgdown: page · shift+up/shift+down: criterion
```

## Install

With Nix (flakes enabled):

```
nix run github:furedea/rater -- --version
nix profile install github:furedea/rater
```

The package is built for `aarch64-darwin`, `x86_64-linux`, and `aarch64-linux`, and runs the
test suite while it builds.

With Go 1.27 or later:

```
go install github.com/furedea/rater/cmd/rater@latest
```

Or download a release archive for macOS or Linux (amd64 or arm64) from
[Releases](https://github.com/furedea/rater/releases), check it, and put `rater` on your `PATH`.
Each release carries `checksums.txt` and a build provenance attestation for every archive:

```
sha256sum --check --ignore-missing checksums.txt   # macOS: shasum -a 256 --check --ignore-missing checksums.txt
gh attestation verify rater_<version>_<os>_<arch>.tar.gz --repo furedea/rater
tar -xzf rater_<version>_<os>_<arch>.tar.gz
```

`rater --version` prints the version.

## Quick start

The [examples](examples/readme.md) directory holds three small tasks. Label one as two people,
then compare and resolve:

```
rater annotate --task examples/review-comment-pairs --as alice
rater annotate --task examples/review-comment-pairs --as bob
rater agree --task examples/review-comment-pairs
rater adjudicate --task examples/review-comment-pairs --as carol
```

`agree` prints, for example:

```
task review-comment-pairs · 3 items

annotator  labeled  held  pending
alice      3        0     0
bob        3        0     0

measure  items  observed  expected  kappa  alpha
label    3      0.667     0.333     0.500  0.545

label confusion (rows: alice, columns: bob)
          distinct  partial  same
distinct  1         0        0
partial   0         0        1
same      0         0        1

held and incomplete items are left out of agreement.
```

## Data layout

A task is a directory. You write the first two files; rater writes the rest, and never rewrites
an annotator's or adjudicator's log.

```
<task>/
  task.yaml                          what to show and how to answer
  items.jsonl                        the items, one JSON object per line
  assignment.yaml                    optional: which annotator sees which items
  annotations/<annotator>.jsonl      append-only log, written by annotate and serve
  adjudications/<adjudicator>.jsonl  append-only log, written by adjudicate
  gold.jsonl                         rebuilt by adjudicate on quit
  host_ed25519                       serve's host key, created on its first start
```

Each line of `items.jsonl` is an object with a string `id` and any other fields; `task.yaml`
names the fields to show. A value that is not a string is shown as its compact JSON text.

```json
{
    "id": "pair-001",
    "repo": "example/web",
    "pr": 101,
    "comment_a": "Please add a null check before dereferencing user.",
    "comment_b": "user can be nil here, guard it before use."
}
```

Each line of a log is one action: `label`, `answer` (one checklist criterion), `hold`, `note`, or
`undo`, with the item, the annotator, and the time. Undo is itself a record, so a log is the full
history and is replayed, never edited:

```json
{
    "item": "pair-001",
    "annotator": "alice",
    "action": "label",
    "label": "same",
    "at": "2026-11-02T10:00:00Z"
}
```

Annotator and adjudicator names become file names, so they are limited to lowercase letters and
digits, optionally joined by single `-` or `_` (`alice`, `rater-2`).

## Task file

```yaml
name: review-comment-pairs
layout: side-by-side # or stacked (default)
header: [repo, pr] # item fields shown on the line under the title
fields:
    - { name: comment_a, title: Review comment A }
    - { name: comment_b, title: Review comment B }
labels:
    - { key: s, name: same }
    - { key: p, name: partial }
    - { key: d, name: distinct }
```

Unknown keys are rejected, so a misspelt option fails instead of being ignored.

### Labels and checklists

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

### Layout

`layout: stacked` (default) shows the fields one under another. `layout: side-by-side` shows
exactly two fields in two columns, each scrolling on its own: the scroll and hunk keys act on the
focused column (marked `▶` on the position line) and `ctrl+w` switches columns. On a terminal
narrower than 63 columns the fields are stacked and a notice says so.

### Fields

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

### Keys

`keys` rebinds `scroll_up`, `scroll_down`, `page_up`, `page_down`, `next_hunk`, `prev_hunk`,
`prev_criterion`, `next_criterion`, and `switch_pane`, written as Bubble Tea names such as `down`,
`ctrl+d`, or `]`. Spellings of one key are treated alike: `shift+s` is `S`, modifiers may come in
any order (`shift+ctrl+j` is `ctrl+shift+j`), and `escape`, `return`, `pageup`, and `pagedown` mean
`esc`, `enter`, `pgup`, and `pgdown`. Label and answer keys are one character and accept the same
spellings. A key may not be bound twice or reuse a label key, an answer key, or a fixed control
key; `task.yaml` is rejected if it does.

## Commands

```
rater annotate --task <dir> --as <annotator>
rater agree --task <dir> [--format text|json] [--annotators a,b]
rater adjudicate --task <dir> --as <adjudicator>
rater serve --task <dir> --listen <addr> --keys <file> [--host-key <path>]
rater --version
```

`rater adjudicate --help` and `rater serve --help` describe their flags and rules.

### annotate

`annotate` shows the task's items one at a time and appends what `--as` does to
`annotations/<annotator>.jsonl`. Running it again replays the log and resumes at the first
pending item, then the first held one.

Keys: a label key (from `task.yaml`) labels and advances, `tab` holds, `←`/`→` move,
`ctrl+u` undoes the latest action, `ctrl+n` edits the note, `esc` quits. Every action is saved
as it happens, so quitting or crashing loses nothing. A log has one writer at a time: a second
`annotate` (or `serve` session) for the same annotator is refused until the first quits.

Long bodies scroll: `↑`/`↓` move a line and `pgup`/`pgdown` a page. In a diff, `]` and `[` jump
to the next and previous hunk; the position line shows which hunk is being read, and `]` reaches
every hunk even when the last ones fit on screen together.

### agree

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

The coefficients are tested against published worked examples
([internal/agreement/stats_test.go](internal/agreement/stats_test.go)):

| Statistic            | Example                                                                                  | Expected                       |
| -------------------- | ---------------------------------------------------------------------------------------- | ------------------------------ |
| Cohen's kappa        | Wikipedia, "Cohen's kappa", simple example (50 grant proposals)                          | p_o 0.70, p_e 0.50, kappa 0.40 |
| Cohen's kappa        | R package `irr`, `kappa2(diagnoses[, 2:3])` (Fleiss 1971 diagnoses, 30 subjects)         | kappa 0.631                    |
| Krippendorff's alpha | R package `irr`, `kripp.alpha(nmm)` (Krippendorff's 4 raters × 12 units, missing values) | nominal alpha 0.743            |

### adjudicate

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

```json
{
    "id": "pair-003",
    "label": "partial",
    "source": "adjudicated",
    "adjudicator": "carol",
    "annotations": [
        {
            "annotator": "alice",
            "status": "labeled",
            "label": "partial",
            "records": 3
        },
        {
            "annotator": "bob",
            "status": "labeled",
            "label": "same",
            "records": 3
        }
    ]
}
```

A task has one adjudicator, because `gold.jsonl` is rebuilt from one adjudication log.
`adjudicate` refuses to start, before writing anything, when `adjudications/` holds a log of
anyone other than `--as`, and names whose log it found; the same adjudicator can always resume.
To hand over, move the previous adjudicator's log out of `adjudications/` or rename it so it no
longer ends in `.jsonl` (for example `carol.jsonl.handed-over`). The new adjudicator then starts
from no decisions, and the previous decisions leave `gold.jsonl` when the new adjudicator quits.
`rater adjudicate --help` repeats this rule. The check and the creation of the log run under an
exclusive lock on `adjudications/`, so two adjudicators starting at the same moment on a fresh
task cannot both pass it.

### serve

`serve` shows the `annotate` screen to annotators over SSH, so they need only an SSH client:

```
rater serve --task examples/review-comment-pairs --listen :2222 --keys keys.txt
ssh -p 2222 rater.example.org
```

Each connection runs its own screen at the client's terminal size, capped at 1000 columns by 500
rows, and follows its resizes. In the note field, the terminal's own paste works but `ctrl+v` is
off, since it would read the server's clipboard.

The keys file registers who may connect. Each line holds an annotator name and one public key in
`authorized_keys` form; an annotator may have several lines, and `#` starts a comment:

```
alice ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA... alice@laptop
alice ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA... alice@desktop
bob   ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ...
```

An annotator has one session at a time. Each session holds the exclusive lock on its annotator's
log (the same lock that keeps two local `annotate` runs apart), so a second connection, or a local
`annotate --as` for the same name, is refused with a message until the first quits. A dropped
connection ends its session and releases the lock.

`SIGINT` or `SIGTERM` stops accepting connections, quits every session, and returns once each has
closed its log; a second signal kills the server. Every label is already on disk when it is made,
so nothing is lost either way.

See [Security of serve](#security-of-serve) before exposing it beyond a trusted network.

### Assignment

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

## Security of serve

- **Keys decide who connects.** Only public-key logins with a key in the keys file are accepted;
  passwords are not. The key decides the annotator, so the SSH user name does not matter and
  nobody can type another name. Key options such as `from="..."` are rejected rather than
  ignored, and each key may appear once. Restrict where the server listens with `--listen` and
  your firewall; rater has no option of its own for source addresses.
- **No shell.** Commands (`ssh host ls`), subsystems such as SFTP, port forwarding, and sessions
  without a terminal (`ssh -T`) are refused. Unauthenticated connections are dropped after 30
  seconds.
- **Host key.** `--host-key` (default `<dir>/host_ed25519`) is generated as Ed25519 with mode
  `0600` on the first start and reused afterwards, so clients see the same host key; the server
  refuses a key file that group or others can access. Keep it out of version control. The first
  line `serve` prints includes the key's SHA256 fingerprint; send it to annotators through
  another channel so they can compare it on their first connection.
- **Blind annotation.** A session replays and shows only its own annotator's log, so nobody sees
  another annotator's labels, notes, or progress. `adjudicate` is not served; run it locally on
  the server's task directory.
- **Known issues in the SSH library** (`charm.land/ssh`), which rater cannot fix from its
  handler:
    - Connections that have not finished authenticating are not closed on shutdown; they last
      until the 30-second handshake timeout. A session opened after shutdown began is refused.
    - Reading a session's terminal size races with a concurrent `window-change`. The race
      detector has not reported it in rater's tests.
    - An authenticated client that sends `window-change` before `shell` leaves one goroutine
      blocked per connection, so repeating it grows memory. Only registered annotators can do
      this.

## Limitations

- Agreement covers nominal data only: no weighted kappa, ordinal or interval alpha, or
  confidence intervals. Fleiss' kappa is not reported; alpha covers more than two annotators.
- One single-choice label or one checklist per task; there is no multi-label, span, or free-text
  answer other than the note.
- Items are text. Images, audio, and anything rendered outside the terminal are not supported.
- `adjudicate` runs locally and has one adjudicator per task.
- Logs are plain files under the task directory, locked with `flock`, so the task directory must
  be on a local file system that supports it. On systems without `flock`, locking is not
  enforced.
- There is no user management beyond the keys file: anyone with a registered key can annotate as
  that annotator, and anyone with access to the task directory can read every log.

## Releasing

Releases are managed by Release Please; nobody tags by hand. Each push to `main` updates a release
PR from the Conventional Commits since the last release. Merging it updates `CHANGELOG.md`,
`.release-please-manifest.json`, and `cmd/rater/version.txt` (the version `rater --version`
reports and the Nix package uses), tags the merge commit `vX.Y.Z`, and creates a draft GitHub
Release. In the same `Release` workflow run, the `build` job then checks out the tag, runs the
tests, builds the four archives and `checksums.txt`, and smoke-tests the Linux amd64 archive. The
shared `release-publish` workflow of
[furedea/reusable-workflows](https://github.com/furedea/reusable-workflows) attests their
provenance, uploads them, and publishes the release. If any step fails, the release stays a draft.

To resume a draft release whose run failed, start the same workflow on the tag so the attestation
records the tagged commit:

```
tag=v0.1.0
gh workflow run release.yml --ref "$tag" -f tag_name="$tag"
```

The workflow fails before publishing when the tag does not point to the commit of the run or when
the release is already published. A published release is never rebuilt in place; ship a fix as a
new release.

The `release-pr` job signs in as the release GitHub App, so that the release PR it opens runs CI.
It needs the `RELEASE_APP_CLIENT_ID` repository variable and the `RELEASE_APP_PRIVATE_KEY`
repository secret. Enable "Enable release immutability" (Settings > General > Releases) so that
published releases cannot be changed. The attestations name `release-publish.yml` at the pinned
SHA as the builder.

## License

[MIT](LICENSE)
