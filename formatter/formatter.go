// Package formatter provides output formatters for rendering SOLID score
// results in different formats such as human-readable text and JSON.
package formatter

import "github.com/harakeishi/go-solid-score/scorer"

// Formatter defines how score results are rendered to output.
type Formatter interface {
	Format(results []*scorer.ScoreResult) (string, error)
}
