# Examples

Run one with `rater annotate --task examples/<name> --as <annotator>`. Annotations are appended
to `examples/<name>/annotations/<annotator>.jsonl`, which is ignored by git. With two or more
annotators, `rater adjudicate --task examples/<name> --as <adjudicator>` resolves their
disagreements into `adjudications/` and `gold.jsonl`, which are ignored as well.

| Task                   | Shown as                                                      |
| ---------------------- | ------------------------------------------------------------- |
| `review-comment-pairs` | two texts side by side, three labels                          |
| `guideline-criteria`   | one sentence, five y/n/? criteria, and the `all_pass` verdict |
| `diff-relevance`       | guideline text and a coloured unified diff, four labels       |
