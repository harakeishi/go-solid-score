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

| LCOM4 | Penalty |
|-------|---------|
| ≤ 1   | None    |
| 2     | −40     |
| ≥ 3   | −70     |

Additional penalties: cyclomatic complexity > 20 (−10) or > 40 (−20), method count > 10 (−5) or > 15 (−15).

</details>

<details>
<summary>OCP — Open/Closed Principle</summary>

Detects code patterns that require modification when adding new types.

| Pattern | Per Instance | Max Penalty |
|---------|-------------|-------------|
| Type switch | −15 | −40 |
| Type assertion | −10 | −40 |
| Reflect usage | −5 | −20 |

A density penalty applies if type-check statements exceed 15% (−10) or 30% (−20) of total statements. Interface parameters in methods earn a bonus (+5 each, max +20).

</details>

<details>
<summary>LSP — Liskov Substitution Principle</summary>

Checks whether interface implementations honour their contracts.

| Violation | Penalty |
|-----------|---------|
| Method calls `panic()` | −20 |
| No-op implementation | −15 |
| Missing override of embedded interface method | −10 each |

Only methods that satisfy a declared interface are evaluated.

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

Constructor accepting interfaces earns +15 bonus. Standard library types and user-configured whitelist types are excluded.

</details>

<details>
<summary>Total Score Calculation</summary>

```
Total = Σ(principle_score × weight) / Σ(weights)
```

Weights are configurable (see [Configuration](#configuration)). Zero-weighted principles are excluded. The result is rounded to one decimal place.

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
  "line": 10
}
```

Because the `id` is derived from the import path rather than the absolute file
path, it stays the same when a type is renamed-by-file or moved to another file
within the same package. This makes it suitable as a join key when comparing
two runs (e.g. a base commit vs. a PR) to detect score regressions, rather than
relying on absolute paths that differ across machines and refactors.

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
