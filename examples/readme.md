# Examples

Run one with `rater annotate --task examples/<name> --as <annotator>`. Annotations are appended
to `examples/<name>/annotations/<annotator>.jsonl`, which is ignored by git.

| Task                   | Shown in M1 as                  | Later milestones add                                   |
| ---------------------- | ------------------------------- | ------------------------------------------------------ |
| `review-comment-pairs` | two stacked texts, three labels | side-by-side layout (M3)                               |
| `guideline-criteria`   | one sentence, one verdict label | five y/n/? criteria and the all-pass verdict rule (M3) |
| `diff-relevance`       | guideline and plain-text diff   | diff highlighting and hunk navigation (M2)             |
