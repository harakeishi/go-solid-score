// Package analyzer provides static analysis of Go source code against the
// SOLID design principles. Each principle (SRP, OCP, LSP, ISP, DIP) has a
// dedicated analyzer that implements the [Analyzer] interface.
package analyzer

import "github.com/harakeishi/go-solid-score/model"

// Principle identifies a SOLID principle.
type Principle string

const (
	SRP Principle = "SRP"
	OCP Principle = "OCP"
	LSP Principle = "LSP"
	ISP Principle = "ISP"
	DIP Principle = "DIP"
)

// Confidence levels for analysis results.
const (
	ConfidenceLow        = 0.3
	ConfidenceLowMedium  = 0.5
	ConfidenceMedium     = 0.7
	ConfidenceMediumHigh = 0.85
	ConfidenceHigh       = 1.0
)

// Result holds the analysis output for one principle on one target.
type Result struct {
	Principle Principle
	// TargetPkg is the import path of the package the target belongs to.
	// It is used (together with TargetName) as a stable identity for the
	// target that survives file renames and moves. TargetFile/TargetLine
	// remain for human-facing display only.
	TargetPkg  string
	TargetName string
	TargetFile string
	TargetLine int
	// TargetIsInterface marks the target as an interface definition rather than
	// a struct. The ISP analyzer is the sole authority for this flag, since it
	// is the only analyzer that visits interface definitions. It lets the
	// formatters present interfaces separately from structs, because an
	// interface is scored on ISP alone and its Total is therefore not
	// comparable to a struct's five-principle Total.
	TargetIsInterface bool
	Score             float64
	Confidence        float64
	Details           []string
}

// Analyzer is the interface every SOLID principle analyzer implements.
type Analyzer interface {
	Principle() Principle
	Analyze(pkg *model.PackageInfo) []Result
}

// Clamp constrains a score to [0, 100].
func Clamp(score float64) float64 {
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}
