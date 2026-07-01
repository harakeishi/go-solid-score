# go-solid-score

[![CI](https://github.com/harakeishi/go-solid-score/actions/workflows/ci.yml/badge.svg)](https://github.com/harakeishi/go-solid-score/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/harakeishi/go-solid-score.svg)](https://pkg.go.dev/github.com/harakeishi/go-solid-score)
[![Go Report Card](https://goreportcard.com/badge/github.com/harakeishi/go-solid-score)](https://goreportcard.com/report/github.com/harakeishi/go-solid-score)

A static analysis tool that scores Go source code against the five SOLID design principles.

## Features

- **SRP** (Single Responsibility Principle) — LSCC-based cohesion analysis
- **OCP** (Open/Closed Principle) — Detects type switches, type assertions, and reflect usage
- **LSP** (Liskov Substitution Principle) — Detects panics, no-ops, and interface contract violations
- **ISP** (Interface Segregation Principle) — Evaluates interface sizes following Go idioms
- **DIP** (Dependency Inversion Principle) — Measures interface vs concrete type dependency ratio
- **golangci-lint integration** — Available as a `go/analysis` plugin

## Installation

```bash
go install github.com/harakeishi/go-solid-score@latest
```

Or download a binary from the [Releases](https://github.com/harakeishi/go-solid-score/releases) page.

## GitHub Action

A composite action ships in this repo, so you can gate CI on a SOLID score
without installing a Go toolchain or running `go install` on every run — it
downloads the released binary for the runner and invokes it.

> The action is available from **v0.3.0** onward. Pin a tag that contains
> `action.yml` (v0.2.0 and earlier do not).

### Gate on a minimum score

```yaml
# .github/workflows/solid.yml
name: SOLID
on: [push, pull_request]
jobs:
  solid:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: harakeishi/go-solid-score@v0.3.0   # pin to a released tag
        with:
          min-score: "70"   # fail the run if any target scores below 70
          paths: ./...
```

### Comment a PR with the score diff (and fail on regressions)

```yaml
# .github/workflows/solid-diff.yml
name: SOLID diff
on: pull_request
permissions:
  contents: read
  pull-requests: write   # required to post the comment
jobs:
  diff:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0   # diff mode checks out the base commit
      - uses: harakeishi/go-solid-score@v0.3.0   # pin to a released tag
        with:
          mode: diff
          fail-on-regression: "true"   # exit 1 if a target regresses
          max-drop: "5.0"
```

In `diff` mode the action scores the PR's base commit, diffs the current code
against it, and posts a sticky comment with the per-target/per-principle delta.
On fork PRs (which run with a read-only token) it falls back to the job summary
automatically.

### Inputs

| Input | Default | Description |
|-------|---------|-------------|
| `mode` | `check` | `check` (gate on `min-score`) or `diff` (compare vs base, comment). |
| `version` | `latest` | Release tag to use, e.g. `v1.2.3`, or `latest`. |
| `paths` | `./...` | Package patterns to analyze (space-separated). |
| `config` | (none) | Path to a `.go-solid-score.yaml` config file. |
| `min-score` | `0` | `check`: fail below this. `diff`: flag new targets below this as NEW-LOW. `0` disables. |
| `max-drop` | `5.0` | *(diff)* A total-score drop greater than this counts as a regression. |
| `fail-on-regression` | `false` | *(diff)* Exit 1 on any regression or new-low. |
| `comment` | `true` | *(diff)* Post a sticky PR comment (else use the job summary). |
| `github-token` | `${{ github.token }}` | *(diff)* Token used to post the comment. |

The action fails the step (non-zero exit) when a gate is breached — a score
below `min-score`, or a regression with `fail-on-regression: true` — so gate on
the step/job status as usual (e.g. `continue-on-error` + `steps.<id>.outcome`).

## Usage

```bash
# Analyze the current package
go-solid-score ./...

# Specify output format
go-solid-score -f json ./...

# Set minimum score threshold (exit code 1 if any score is below)
go-solid-score --min-score 70 ./...

# Use a custom config file
go-solid-score -c myconfig.yaml ./...

# Verbose output with detailed breakdown per struct
go-solid-score -v ./...
```

## Configuration

Create a `.go-solid-score.yaml` file in your project root:

```yaml
paths:
  - ./...
exclude:
  - "**/*_test.go"
format: text  # text or json
min_score: 70

weights:
  SRP: 0.30
  OCP: 0.15
  LSP: 0.10
  ISP: 0.20
  DIP: 0.25

thresholds:
  total: 70
  SRP: 60
  OCP: 50
  LSP: 50
  ISP: 50
  DIP: 60

dip:
  whitelist:
    - "mypackage.MyConcreteType"
```

The `weights` and `thresholds` above control aggregation and pass/fail gating.
To change the **scoring rules themselves** — retune a preset, disable one, or
add your own — see [Customizing Scoring Rules](#customizing-scoring-rules).

### Default Weights

| Principle | Weight |
|-----------|--------|
| SRP       | 0.30   |
| OCP       | 0.15   |
| LSP       | 0.10   |
| ISP       | 0.20   |
| DIP       | 0.25   |

## Customizing Scoring Rules

The scoring logic is **data-driven**. Every penalty, bonus, and threshold lives
in a declarative rule set rather than in code, so you can retune the built-in
presets, switch any of them off, or add entirely new rules of your own — all
from `.go-solid-score.yaml`, without recompiling.

A rule reads one **metric**, optionally checks a `when` condition (and `where`
preconditions), then applies an **effect** to the target's running score. Rules
for a principle run top to bottom; each starts from the score the previous rule
left, and the final score is clamped to `[0, 100]`.

```yaml
# Turn a preset off completely.
disable_rules:
  - ocp-type-switch

# Override a preset (same id) or add a new rule (new id).
# NOTE: overriding by id replaces the ENTIRE rule — copy all of the preset's
# fields and change what you need. A rule that ends up doing nothing (e.g. only
# `id` + `cap`) is rejected with an error rather than silently dropping the
# preset, and a misspelled `metric` is rejected too.
rules:
  # Soften the SRP cohesion penalty to half strength (halve each band value).
  - id: srp-cohesion          # matches a preset id -> replaces it in place
    principle: SRP
    metric: lscc              # LSCC cohesion in [0,1]; lower = worse
    where: ["cohesion_method_count >= 2", "has_fields == 1"]
    bands:                    # first matching band applies
      - { when: "< 0.2", value: 20, message: "very low cohesion (LSCC=%v)" }
      - { when: "< 0.4", value: 12, message: "low cohesion (LSCC=%v)" }
      - { when: "< 0.6", value: 5,  message: "moderate cohesion (LSCC=%v)" }

  # A brand-new rule: penalize any struct with more than 6 methods.
  - id: custom-too-many-methods
    principle: SRP
    metric: method_count
    when: "> 6"
    effect: penalty           # penalty | bonus | set | none
    value: 5

# Optionally change the starting score/confidence for a principle. Either field
# may be set on its own; the unspecified one keeps its preset value.
rule_defaults:
  SRP: { base_score: 100, base_confidence: 1.0 }
```

### Rule fields

| Field | Meaning |
|-------|---------|
| `id` | Unique id. A user rule with a **matching id replaces the whole preset** in place (copy all its fields); a **new id is appended**. |
| `principle` | `SRP` / `OCP` / `LSP` / `ISP` / `DIP`. |
| `target` | `struct` (default), `interface`, or `both`. |
| `metric` | The metric the rule reads (see below). |
| `when` | Condition on the metric, e.g. `"> 40"`, `">= 0.15"`, `"== 0"`. Omit for "always". |
| `where` | List of extra preconditions, e.g. `["structural_dep_total == 0"]`; all must hold. |
| `effect` | `penalty` (subtract, default), `bonus` (add), `set` (assign), `none` (confidence-only). |
| `value` | Literal amount for the effect. |
| `from_metric` / `scale` | Derive the amount from the metric value × `scale` (default 1). |
| `cap` | Maximum magnitude of a penalty/bonus. |
| `bands` | Ordered `{ when, value, effect?, message? }` list; the first match applies. |
| `confidence` | When the rule matches, sets the result confidence. |
| `stop` | Stop evaluating further rules for the target when matched. |
| `enabled` | Set `false` to disable (same as listing the id in `disable_rules`). |

### Available metrics

Structs expose: `method_count`, `public_method_count`, `field_count`,
`has_fields`, `lscc`, `cohesion_method_count`,
`total_complexity`, `type_switch_count`, `type_assert_count`, `reflect_count`,
`total_stmts`, `type_check_density`, `iface_param_count`,
`implements_interface`, `unconditional_panic_count`, `noop_count`,
`embed_missing_override_count`, `is_decorator`, `public_lcom4`,
`isp_large_iface_penalty`, `isp_composition_bonus`, `weighted_dep_total`,
`weighted_dep_iface`, `structural_dep_total`, `iface_dep_ratio`,
`has_constructor_injection`. Interfaces expose: `total_methods`,
`direct_methods`, `embed_count`. Boolean metrics are `0` or `1`.

The complete set of built-in rules — and the reference for the schema above —
is [`rules/presets.yaml`](rules/presets.yaml). Copy any rule from there into
your config to retune it.

## Scoring Logic

Each struct is scored 0–100 per principle. The total score is a weighted average of all five.

The five principles below ship as the **default rule set** (see
[Customizing Scoring Rules](#customizing-scoring-rules)); the descriptions
reflect the preset values.

<details>
<summary>SRP — Single Responsibility Principle</summary>

Uses **LSCC** (Low-level Similarity-based Class Cohesion, Al Dallal & Briand
2012) to measure struct cohesion. LSCC is a normalized ratio in `[0, 1]` where
**1 is maximally cohesive** — for each named field accessed by `x` methods it
sums `x·(x−1)` and normalizes by `k·l·(l−1)` (`l` methods, `k` named fields).
Unlike a component-count metric, a single stateless method *dilutes* the score
rather than fragmenting it, which makes the signal robust across struct sizes.

- The `errors.Is`/`As`/`Unwrap` convention methods are excluded from the method
  set: they are framework-protocol methods that inspect their argument rather
  than the receiver's fields, so counting them would artificially deflate
  cohesion. The count of remaining methods is exposed as `cohesion_method_count`.
- The cohesion penalty only applies to structs with fields and at least two
  cohesion methods (`has_fields == 1`, `cohesion_method_count >= 2`); below that
  LSCC is undefined and no false positive is emitted.

| LSCC        | Penalty |
|-------------|---------|
| ≥ 0.6       | None    |
| 0.4 – < 0.6 | −10     |
| 0.2 – < 0.4 | −25     |
| < 0.2       | −45     |

The thresholds live in [`rules/presets.yaml`](rules/presets.yaml) (the
`srp-cohesion` rule) rather than in code, so they can be retuned per repo
without a rebuild.

Additional penalties: cyclomatic complexity > 20 (−10) or > 40 (−20), method count > 10 (−5) or > 15 (−15).

</details>

<details>
<summary>OCP — Open/Closed Principle</summary>

Detects code patterns that require modification when adding new types.

| Pattern | Per Instance | Max Penalty |
|---------|-------------|-------------|
| Type switch | −15 | −40 |
| Type assertion (to a concrete type) | −10 | −40 |
| Reflect usage | −5 | −20 |

A density penalty applies if type-check statements exceed 15% (−10) or 30% (−20) of total statements. Interface parameters in methods earn a bonus (+5 each, max +20).

Type assertions whose target is an **interface** (capability/feature detection,
e.g. `if f, ok := w.(http.Flusher); ok { … }`) are *not* penalized: they are
open for extension — a new type implementing the interface needs no change here
— unlike a downcast to a concrete type, which is the OCP smell.

</details>

<details>
<summary>LSP — Liskov Substitution Principle</summary>

Checks whether interface implementations honour their contracts.

| Violation | Penalty |
|-----------|---------|
| Method panics *unconditionally* (e.g. a "not implemented" stub) | −20 |
| No-op implementation | −15 |
| Missing override of embedded interface method | −10 each |

Only methods that satisfy a declared interface are evaluated. Panics that fire
only inside an argument/state guard (`if bad { panic(...) }`) are idiomatic
fail-fast in Go and are **not** penalized — only panics on the method's
straight-line path count.

</details>

<details>
<summary>ISP — Interface Segregation Principle</summary>

Scores based on public method count (Go idiom favours small interfaces).

| Public Methods | Base Score |
|----------------|-----------|
| ≤ 5  | 100 |
| 6–10 | 80  |
| 11–15 | 60 |
| 16–20 | 40 |
| > 20 | 20  |

Penalties for implementing large interfaces (6–8 methods: −10, 9–12: −20, >12: −30). Decorator/adapter patterns are auto-detected and scored at 85. LCOM4 > 2 on ≥ 4 public methods adds −15.

</details>

<details>
<summary>DIP — Dependency Inversion Principle</summary>

Measures the ratio of interface dependencies to total dependencies.

```
Score = (weighted interface deps / weighted total deps) × 100
```

| Dependency Source | Weight |
|-------------------|--------|
| Struct fields | 1.0 |
| Constructor params | 1.0 |
| Exported method params | 0.3 |

Constructor accepting interfaces earns +15 bonus. Collections of an interface
(e.g. `[]Handler`) count as abstraction dependencies.

Only *owned collaborators* count toward the ratio. The following are **not**
treated as dependencies, because penalizing them produced false positives on
idiomatic aggregate/config types: standard-library and user-whitelisted types
(including collections of them), function-typed fields (callbacks/strategies),
pure-data value types — those whose element is a builtin basic type, such as
`map[string]string` or a named alias like `type FieldMap …` — and
self-references (recursive/tree structures). A collection of a concrete struct
(e.g. `[]*Worker`) *does* count as a concrete dependency. A type that owns no
structural dependency at all is reported as *DIP not applicable* (top score,
low confidence) rather than penalized via its method parameters. See
[`docs/scoring-analysis.md`](docs/scoring-analysis.md) for the benchmarking that
motivated these rules.

</details>

<details>
<summary>Total Score Calculation</summary>

```
Total = Σ(principle_score × weight) / Σ(weights)
```

Weights are configurable (see [Configuration](#configuration)). Zero-weighted principles are excluded. The result is rounded to one decimal place.

Only principles that were actually evaluated for a target contribute to the
average. Interface definitions are scored on ISP alone, so their Total equals
their ISP score. Because that is not comparable to a struct's five-principle
Total, the text output lists interfaces in a **separate section** from structs,
and the JSON output tags each target with `is_interface` so consumers can
filter on it.

For the same reason, the JSON `summary` reports structs and interfaces
separately: `total_structs` / `average_score` cover **structs only**, while
`total_interfaces` / `interface_average_score` cover interfaces. (Earlier
versions counted all targets together under `total_structs` / `average_score`;
if you compare against an old baseline, expect these two fields to differ on a
codebase that contains interface definitions.)

</details>

## Stable Target IDs (for score diffing)

JSON output identifies each target by a stable `id` of the form
`<package import path>.<TypeName>` (e.g. `github.com/foo/bar.MyStruct`), in
addition to the human-facing `file` and `line`:

```json
{
  "id": "github.com/foo/bar.MyStruct",
  "name": "MyStruct",
  "package": "github.com/foo/bar",
  "file": "/abs/path/to/bar/mystruct.go",
  "line": 10,
  "is_interface": false
}
```

Because the `id` is derived from the import path rather than the absolute file
path, it stays the same when a type is renamed-by-file or moved to another file
within the same package. This makes it suitable as a join key when comparing
two runs (e.g. a base commit vs. a PR) to detect score regressions, rather than
relying on absolute paths that differ across machines and refactors.

## Diffing scores (regression gating)

Compare the current scores against a baseline to detect regressions:

```bash
# 1. Capture a baseline (e.g. on the main branch)
go-solid-score -f json ./... > base.json

# 2. After making changes, diff against it
go-solid-score diff --base base.json ./...

# Fail CI when a target regresses or a new target is below a floor
go-solid-score diff --base base.json --min-score 70 --fail-on-regression ./...

# Markdown output for PR comments
go-solid-score diff --base base.json -f markdown ./... > comment.md
```

Targets are matched by their stable `id`, so renames and file moves do not
produce false regressions. Each target is classified as REGRESSED, IMPROVED,
UNCHANGED, NEW, NEW-LOW, or REMOVED. For targets that changed, the per-principle
breakdown (e.g. `OCP 100.0->70.0 (-30.0)`) is shown alongside the total so you
can tell *which* principle moved and what to fix.

The stable `id` requires a resolvable package import path; for code that does
not type-check (unresolved packages), the `id` falls back to a file-based key
and a moved target may appear as a REMOVED/NEW pair. Run `diff` on buildable
code for accurate matching.

| Flag | Default | Meaning |
|------|---------|---------|
| `--base` | (required) | Baseline JSON to compare against |
| `--max-drop` | 5.0 | A total drop greater than this is a regression |
| `--min-score` | 0 | A new target below this is NEW-LOW (0 disables) |
| `--fail-on-regression` | false | Exit 1 on any regression or new-low |
| `-f, --format` | text | `text`, `json`, or `markdown` |

See [`.github/workflows/solid-diff.yml`](.github/workflows/solid-diff.yml) for a
PR-comment workflow (requires `pull-requests: write`; fork PRs fall back to a
job summary).

## Measuring scoring accuracy (precision / recall)

`diff` guards against *score* regressions on real code. A separate concern is
whether the scorer's verdicts are *correct* — and in particular whether it has
started **missing** genuine violations (a recall regression). Two complementary
harnesses cover the two error directions:

| Harness | Measures | Ground truth | Direction it protects |
|---------|----------|--------------|-----------------------|
| [`scripts/benchmark.sh`](scripts/benchmark.sh) | mean per-principle scores over pinned OSS libraries | "good libraries score well" assumption | **precision** — flags over-penalizing sound code |
| [`scripts/evaluate.sh`](scripts/evaluate.sh) (`gss evaluate`) | per-principle precision/recall/F1 vs labelled `testdata` | inline `// solid:want` labels | **recall** — flags missing genuine violations |

The split matters: past calibration only ever *relaxed* penalties (improving
precision) without ever measuring whether the tool started missing real
violations. `evaluate` makes recall measurable so relaxations can be checked for
that side effect.

### Ground-truth labels

Each labelled `testdata` type carries its expected verdict inline, following the
inline-annotation convention of SonarSource and Checkstyle rule tests:

```go
// FatImpl has a bloated public interface.
// solid:want ISP=violation reason="11 public methods forcing clients to depend on methods they don't use"
type FatImpl struct { /* ... */ }
```

`PRINCIPLE=violation|ok|na`, an optional `reason`, and an optional
`split=train|test` (default `test`). `train` labels are used while calibrating
heuristics and are excluded from the reported baseline so accuracy is never
reported on the same types used to tune it. Corpora that cannot be annotated
inline (e.g. third-party OSS) can supply an external YAML label file via
`--labels`.

### Running and gating

```bash
# Per-principle P/R/F table with bootstrap F1 confidence intervals
go-solid-score evaluate ./testdata/srp ./testdata/ocp ./testdata/lsp ./testdata/isp ./testdata/dip

# The CI gate: fail if any principle regressed against the committed baseline
scripts/evaluate.sh

# After an *intended* accuracy change, regenerate and review the baseline
scripts/evaluate.sh --update   # then commit testdata/eval_baseline.json
```

The regression check compares the per-principle confusion matrix against the
committed [`testdata/eval_baseline.json`](testdata/eval_baseline.json) and fails
when a known violation stops being caught (recall floor breached, `TP` drops) or
a sound type starts being flagged (`FP` rises). Following the practice of
static-analysis rule tests (PMD, Semgrep, `go/analysis`), the gate works on
**absolute case counts**, not rates — the per-principle samples are too small for
a rate delta to separate a real regression from one case of natural wobble. The
bootstrap F1 confidence interval is reported for human context but is **not** a
fail condition, because percentile intervals are systematically too narrow at
these sample sizes.

> **Note:** the testdata packages are listed explicitly rather than with a
> `./...` glob. The go tool excludes directories named `testdata` from `./...`,
> so a glob would silently match nothing and the gate would pass having measured
> nothing. `gss evaluate` errors out if its patterns match no labels, to make
> that mistake loud rather than silent.

The CI [`accuracy` job](.github/workflows/ci.yml) runs `scripts/evaluate.sh` on
every PR.

## golangci-lint Integration

go-solid-score can be used as a `go/analysis` plugin with golangci-lint:

```go
package main

import (
	"github.com/harakeishi/go-solid-score/plugin"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(plugin.Analyzer)
}
```

Individual principle analyzers are also available: `plugin.SRPAnalyzer`, `plugin.OCPAnalyzer`, `plugin.LSPAnalyzer`, `plugin.ISPAnalyzer`, `plugin.DIPAnalyzer`.

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/my-feature`)
3. Commit your changes (`git commit -m 'Add my feature'`)
4. Push to the branch (`git push origin feature/my-feature`)
5. Open a Pull Request

## License

[MIT](LICENSE)
