# Examples

Run one with `rater annotate --task examples/<name> --as <annotator>`. Annotations are appended
to `examples/<name>/annotations/<annotator>.jsonl`, which is ignored by git.

| Task                   | Shown as                                                      |
| ---------------------- | ------------------------------------------------------------- |
| `review-comment-pairs` | two texts side by side, three labels                          |
| `guideline-criteria`   | one sentence, five y/n/? criteria, and the `all_pass` verdict |
| `diff-relevance`       | guideline text and a coloured unified diff, four labels       |
