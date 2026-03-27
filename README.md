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
