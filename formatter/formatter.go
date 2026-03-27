package formatter

import "github.com/harakeishi/go-solid-score/scorer"

// Formatter defines how score results are rendered to output.
type Formatter interface {
	Format(results []*scorer.ScoreResult) (string, error)
}
