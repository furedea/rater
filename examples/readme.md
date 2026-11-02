# Examples

Run one with `rater annotate --task examples/<name> --as <annotator>`. Annotations are appended
to `examples/<name>/annotations/<annotator>.jsonl`, which is ignored by git. With two or more
annotators, `rater adjudicate --task examples/<name> --as <adjudicator>` resolves their
disagreements into `adjudications/` and `gold.jsonl`, which are ignored as well.
`rater serve --task examples/<name> --listen 127.0.0.1:2222 --keys <keys file>` serves one over
SSH; the host key it creates, `examples/<name>/host_ed25519`, is ignored too.

| Task                   | Shown as                                                      |
| ---------------------- | ------------------------------------------------------------- |
| `review-comment-pairs` | two texts side by side, three labels                          |
| `guideline-criteria`   | one sentence, five y/n/? criteria, and the `all_pass` verdict |
| `diff-relevance`       | guideline text and a coloured unified diff, four labels       |
