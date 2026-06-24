# go-solid-score

[![CI](https://github.com/harakeishi/go-solid-score/actions/workflows/ci.yml/badge.svg)](https://github.com/harakeishi/go-solid-score/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/harakeishi/go-solid-score.svg)](https://pkg.go.dev/github.com/harakeishi/go-solid-score)
[![Go Report Card](https://goreportcard.com/badge/github.com/harakeishi/go-solid-score)](https://goreportcard.com/report/github.com/harakeishi/go-solid-score)

A static analysis tool that scores Go source code against the five SOLID design principles.

## Features

- **SRP** (Single Responsibility Principle) — LCOM4-based cohesion analysis
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

### Default Weights

| Principle | Weight |
|-----------|--------|
| SRP       | 0.30   |
| OCP       | 0.15   |
| LSP       | 0.10   |
| ISP       | 0.20   |
| DIP       | 0.25   |

## Scoring Logic

Each struct is scored 0–100 per principle. The total score is a weighted average of all five.

<details>
<summary>SRP — Single Responsibility Principle</summary>

Uses **LCOM4** (Lack of Cohesion of Methods) to measure struct cohesion.

- Builds a graph where two methods are connected if they share a field or call each other
- Counts connected components via BFS — more components = more responsibilities
- A method that accesses **no** receiver field and is uncoupled from siblings (e.g. an `errors.Is`/`As` convention method, or a stateless adapter method) is excluded from the count: LCOM measures cohesion *over fields*, so a stateless method neither adds nor removes a data responsibility. Oversized types are still flagged by the method-count penalty below.

| LCOM4 | Base penalty |
|-------|--------------|
| ≤ 1   | None         |
| 2     | −40          |
| 3     | −55          |
| 4     | −70          |
| ≥ 5   | −70 (capped) |

The base penalty ramps `40 + 15×(LCOM4−2)` (capped at −70) so that more
disconnected groups read as progressively worse rather than stepping straight to
the maximum at three. The cohesion penalty is then **graduated by average
component size** (methods ÷
LCOM4). LCOM4 counts how many disconnected method groups a struct splits into,
but the same count means very different things at different sizes: two groups
among 3 methods is genuine fragmentation (most methods are islands), whereas two
groups among 70 methods is a large aggregate whose methods cluster into a couple
of cohesive areas — the facade pattern (`cobra.Command`, `gin.Engine`,
`fasthttp.Request`, `logrus.Logger`). Because the classic SRP smell is *many
small* disconnected pieces, the base penalty is attenuated toward 25% as the
average group grows from 3 to ≥10 methods, and confidence is lowered to mark the
weaker signal. Small fragmented types take the full penalty; large structured
aggregates are still kept below the SRP threshold by the method-count and
complexity penalties below, but are no longer floored.

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
