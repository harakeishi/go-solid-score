# External OSS ground-truth corpus

This directory holds hand-written ground-truth labels for a pinned corpus of
well-known Go libraries, plus the per-repo accuracy baselines the CI gate
checks against. It is the *measured* counterpart to `scripts/benchmark.sh`,
which only eyeballs mean scores: here every labelled (type, principle) pair
lands in a per-principle confusion matrix, so a scoring change that newly
flags a sound real-world type (new FP) or stops catching a genuine violation
(TP drop) fails CI.

Run it with `scripts/evaluate-oss.sh` (see the script header for usage).

## Layout

| Path | Contents |
|------|----------|
| `corpus/` | A Go module that pins the corpus libraries in `go.mod`/`go.sum`. The harness analyses them by import path out of the module cache — no git clone, fully reproducible. |
| `labels/<repo>.yaml` | Ground-truth verdicts per target ID (`<pkgPath>.<TypeName>`), in the external-label schema of `gss evaluate --labels`. |
| `baselines/<repo>.json` | The committed confusion-matrix baseline per repo (regenerate with `scripts/evaluate-oss.sh --update`). |

## Labeling policy

Labels are expert judgments about the *code*, not descriptions of what the
tool currently outputs. The two must be allowed to disagree — the disagreement
is the measurement.

- **`ok`** — a competent Go reviewer would not demand a redesign of this type
  under that principle. Deliberate, widely accepted designs (facade types,
  zero-allocation buffer types, Null Objects, typed-by-design wide encoder
  surfaces) are `ok` even when the tool currently flags them: those rows are
  *recorded false positives*, and the committed baseline makes them visible
  and gated instead of anecdotal.
- **`violation`** — the type exhibits a concrete, commonly-cited design
  problem under that principle (e.g. `logrus.FieldLogger`'s 27-method
  interface, `fasthttp.fakeAddrer`'s unconditionally panicking `net.Conn`
  methods). These are the recall rows: if a scoring change stops catching one,
  the gate fails.
- **`na`** — the principle does not meaningfully apply.

Only principles with a confident verdict are labelled; contentious calls are
omitted rather than guessed. Every label carries a `reason` so a future
reader can challenge it — if you disagree with a label, change it in a PR
where the reasoning can be reviewed, then regenerate the baselines.

All labels use the default `split: test`: the corpus is evaluation-only and
must never be used to tune heuristics (that is what `split: train` inline
labels are for).

## Version pinning

Labels reference types as they exist at the versions pinned in
`corpus/go.mod`. Bumping a pin can invalidate labels (a type gets renamed,
refactored, or fixed); the gate defends this — a label whose ID no longer
matches any scored target silently leaves the join, which shrinks the recall
denominator or drops a TN/FP, and `CompareToBaseline` fails on the shrink.
After an intended bump, re-review the affected labels, then regenerate with
`scripts/evaluate-oss.sh --update`.
