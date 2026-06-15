# Design: `diff` subcommand for relative (regression) quality gating

Date: 2026-06-15
Status: Approved (pending spec review)
Branch: `feat/diff-command`

## Background / Motivation

`go-solid-score` produces a SOLID score per struct, but the **absolute value of
the score has weak grounding** (penalty magnitudes like −40/−70 are heuristic).
The tool is therefore most useful for **relative evaluation**: detecting whether
a change made the design *worse*, rather than asserting an absolute "good/bad".

A prior PR (#3, merged) added a **stable target identity** — each result carries
an `id` of the form `<pkgPath>.<TypeName>` (e.g.
`github.com/foo/bar.MyStruct`), independent of the absolute file path, so the
same target can be matched across two runs even after file renames/moves. This
spec builds the regression-detection layer on top of that foundation.

## Industry research (informing this design)

Surveyed SonarQube, Codecov, Coveralls, golangci-lint, betterer, reviewdog,
octocov. Consistent "winning patterns":

1. **Two axes: overall vs. diff.** Gate on the *changed* set, not the whole
   codebase ("Clean as You Code"). Don't punish a developer for pre-existing
   legacy.
2. **Absolute threshold AND relative-drop, both supported.**
3. **A tolerance ("wiggle room") to ignore tiny drops is mandatory** — Coveralls
   famously red-X'd on `-0.0%`; Codecov's `project` default threshold is 5%;
   SonarQube has a 20-line fudge factor. Especially important for our *discrete*
   scores.
4. **Report-only vs. blocking is a flag, expressed via exit code** (reviewdog
   `-fail-level`, golangci-lint `issues-exit-code`).
5. **baseline delivery**: file-based (betterer/tsc-baseline) vs. git-rev
   (golangci-lint). File-based is reviewable, works under shallow clone, and —
   since we have stable IDs — avoids the line-shift false positives that plague
   git-rev/line-based diffs.
6. **Output distinguishes increase / decrease / unchanged at a glance**
   (Codecov `+`/`-`/`ø`).
7. **octocov split**: the CLI computes and emits Markdown; PR comment *posting*
   (and "update previous comment") is a thin CI layer, not the CLI's job. The
   "where is the baseline stored" problem is solved by a datastore abstraction
   in octocov — we deliberately leave that to CI (see Non-goals).

## Goals

- A `diff` subcommand that compares a baseline JSON against a freshly-analyzed
  head and classifies each target.
- Separate handling of **existing targets (relative drop)** and **new targets
  (absolute min-score)**.
- A tolerance flag so tiny/zero drops don't cause noise.
- Report-only by default; opt-in CI failure via a flag.
- Text and Markdown output (Markdown for PR comments, octocov-style).
- A GitHub Actions workflow sample that posts/updates a PR comment.

## Non-goals (YAGNI, deliberately deferred)

- **Datastore abstraction** (artifact/S3/GCS/BigQuery) for fetching the
  baseline. Acquiring `base.json` is the CI workflow's responsibility; the CLI
  only takes `--base <file>`. May be added later.
- **git-rev based baseline** (`--base-rev HEAD~1`). File-only for now.
- **Per-principle gating** as a default. Per-principle deltas may be shown in
  verbose/details, but the gate decision is on `total`.
- Self-committing badges / SVG output.

## CLI

```
go-solid-score diff --base <base.json> [flags] [packages...]
```

`base.json` is produced beforehand by `go-solid-score -f json ...`. The head is
analyzed in-process by reusing the existing parse→analyze→score core.

### Flags

| Flag | Default | Meaning |
|------|---------|---------|
| `--base <file>` | (required) | Baseline JSON to compare against. |
| `--max-drop <float>` | **5.0** | A `total` drop strictly greater than this on an existing target is a REGRESSED. (Industry-aligned wiggle room.) |
| `--min-score <float>` | 0 (disabled) | A NEW target scoring below this is NEW-LOW. |
| `--fail-on-regression` | false | Exit 1 if any REGRESSED or NEW-LOW exists. Default is report-only (exit 0). |
| `-f, --format` | text | `text`, `json`, or `markdown`. |
| `-c, --config` | `.go-solid-score.yaml` | Reuses existing config loading (weights/thresholds/dip whitelist) for analyzing head. |

The global root flags relevant to analysis (config, weights) apply to the head
analysis so that base and head are scored with the same rules. **Caveat:** the
baseline must have been generated with the same weights; mismatched weights are
the user's responsibility (documented), not validated by the tool.

### Classification

Match base targets and head targets by `id` (the stable `<pkgPath>.<name>`,
falling back to `<file>:<name>` when pkg path is empty — same rule as the merge
key, already centralized in `scorer.targetID`).

For each id:

- **In both** → compare `total`:
  - `base.total - head.total > maxDrop` → **REGRESSED**
  - `head.total > base.total` → **IMPROVED**
  - otherwise → **UNCHANGED** (includes small drops within tolerance)
- **Head only** → **NEW**, or **NEW-LOW** if `minScore > 0 && head.total < minScore`
- **Base only** → **REMOVED** (informational; never a regression)

A run is "regressed" (for exit-code purposes) iff it contains ≥1 REGRESSED or
≥1 NEW-LOW.

**Sign convention:** the displayed `diff` is `head.total - base.total`, so a
drop is negative (e.g. `-14.0`) and an improvement positive (e.g. `+20.0`).
The REGRESSED test `base.total - head.total > maxDrop` is equivalent to
`diff < -maxDrop`.

### Exit code

- Report-only (default): always **0**.
- With `--fail-on-regression`: **1** if regressed, else **0**.
- Usage/IO errors: non-zero (existing cobra behavior).

## Output

### text (Codecov-style markers)

```
go-solid-score diff (base: base.json)
====================================================
REGRESSED  github.com/foo/bar.Handler  72.0 -> 58.0 (-14.0)
NEW-LOW    github.com/foo/baz.Worker   45.0 (< min 70.0)
IMPROVED   github.com/foo/svc.Svc      60.0 -> 80.0 (+20.0)
NEW        github.com/foo/x.Y          90.0
REMOVED    github.com/foo/old.Thing
----------------------------------------------------
1 regressed, 1 new-low, 1 improved, 1 new, 1 removed, 12 unchanged
```

UNCHANGED targets are summarized in the count line, not listed individually.

### markdown (octocov-style, for PR comments)

A leading HTML marker comment lets the CI layer find & update the previous
comment:

```markdown
<!-- go-solid-score-diff -->
## go-solid-score

**1 regressed**, 1 new-low, 1 improved, 1 new, 1 removed, 12 unchanged.

| | target | base | head | diff |
|--|--|--|--|--|
| 🔻 REGRESSED | `github.com/foo/bar.Handler` | 72.0 | 58.0 | -14.0 |
| ⚠️ NEW-LOW | `github.com/foo/baz.Worker` | – | 45.0 | (< min 70.0) |
| 🔺 IMPROVED | `github.com/foo/svc.Svc` | 60.0 | 80.0 | +20.0 |
| ✨ NEW | `github.com/foo/x.Y` | – | 90.0 | – |
| 🗑 REMOVED | `github.com/foo/old.Thing` | 72.0 | – | – |

<details><summary>All targets (incl. 12 unchanged)</summary>

| target | base | head | diff |
|--|--|--|--|
... full table ...

</details>
```

Notable/changed targets are shown in the top table; the full list (including
UNCHANGED) is folded in `<details>`, matching this repo's existing README style
and octocov.

### json

Machine-readable, for tooling: an array of `{id, name, package, status, base,
head, diff}` plus a summary block of counts and a `regressed` boolean.

## Components (responsibility separation)

New package **`differ`** — pure comparison logic, no I/O, no scoring:

- `type Snapshot struct { ID, Name, Package string; Total float64 }`
  (the minimal projection needed for diffing; decoded from base JSON and
  produced from head `ScoreResult`s).
- `type Status string` with constants REGRESSED / IMPROVED / UNCHANGED /
  NEW / NEW_LOW / REMOVED.
- `type Entry struct { ID, Name, Package string; Status Status; Base, Head *float64 }`
- `type Report struct { Entries []Entry; Counts map[Status]int; Regressed bool }`
- `func Diff(base, head []Snapshot, opts Options) Report` — **pure function**,
  the core unit under test. `Options{MaxDrop, MinScore float64}`.

Supporting wiring (kept thin, outside `differ`):

- **Base decoding**: reuse the JSON shape emitted by `formatter`. To avoid
  duplicating the struct, extract the JSON result shape into a small shared
  type the formatter writes and the diff path reads, OR add a minimal decoder
  in the diff command. Decision: extract a shared `formatter` type
  (`formatter.JSONResult`) so the contract has one source of truth.
- **Head scoring**: extract the parse→analyze→score core of `cmd.run()` into a
  reusable function (e.g. `cmd.analyze(cfg, patterns) ([]*scorer.ScoreResult, error)`)
  so both `run` and `diff` call it. This is the one pre-existing-code refactor
  required, and it improves `root.go`'s current "does everything" shape.
- **Diff output formatting**: add text/markdown/json rendering for
  `differ.Report`. Placed in `formatter` (new `diff_text.go`, `diff_markdown.go`,
  `diff_json.go`) to match the existing formatter location, or rendered by a
  small `differ` printer. Decision: put rendering in `formatter` next to the
  existing formatters for discoverability.

## GitHub Actions sample

`.github/workflows/solid-diff.yml` (sample, documented in README):

1. Checkout PR.
2. Obtain `base.json` for the merge base — sample uses `actions/cache` or a
   checkout of the base ref + `go-solid-score -f json` (documented; the
   "datastore" concern lives here, not in the CLI).
3. `go-solid-score diff --base base.json -f markdown ./... > comment.md`
   (optionally `--fail-on-regression`).
4. Post/update PR comment using the `<!-- go-solid-score-diff -->` marker
   (e.g. a maintained marketplace action or a small `gh pr comment` script).
5. Requires `permissions: pull-requests: write`. Note fork-PR limitation:
   no write token → fall back to job summary (documented, octocov-style).

## Testing

- `differ.Diff`: table-driven unit tests covering every status, the `maxDrop`
  boundary (drop == maxDrop → UNCHANGED; drop just over → REGRESSED), `minScore`
  on/off for NEW vs NEW-LOW, REMOVED, and the `Regressed` flag.
- Base decoding round-trip: a `formatter` JSON output decodes back into
  `Snapshot`s with ids intact (guards the shared-type contract).
- Formatters: text/markdown/json output for a representative `Report`
  (assert markers, marker comment, counts line, details fold).
- `cmd diff`: golden/end-to-end test — write a base.json, run against a testdata
  package, assert exit code under `--fail-on-regression` with and without a
  regression.

## Rollout

Single PR on `feat/diff-command`: `differ` package + formatter renderers +
`cmd diff` + shared analyze refactor + Actions sample + README section + tests.
The Actions sample is documentation/CI config and does not block the CLI.
