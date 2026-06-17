// Package eval provides the ground-truth labeling and precision/recall/F1
// measurement harness for go-solid-score. It reads expected verdicts ("this
// type should be flagged as an ISP violation") from inline `// solid:want`
// comments on type declarations — following the inline-annotation convention
// of SonarSource and Checkstyle rule tests — and from an optional external
// YAML file keyed by type ID for corpora whose source cannot be annotated
// (the separate-file convention of code-smell datasets such as MLCQ/Landfill).
package eval

import (
	"fmt"
	"strings"

	"github.com/harakeishi/go-solid-score/analyzer"
)

// Expectation is the ground-truth verdict for one (type, principle) pair.
type Expectation string

const (
	// Violation means the type genuinely violates the principle and SHOULD be
	// flagged (scored below threshold). It is a true positive when caught and a
	// false negative when missed — the recall denominator.
	Violation Expectation = "violation"
	// OK means the type is sound and should NOT be flagged. Flagging it is a
	// false positive.
	OK Expectation = "ok"
	// NA means the principle is not meaningfully applicable to this type; it is
	// excluded from the confusion matrix for that principle.
	NA Expectation = "na"
)

// Split partitions labels into calibration (train) and evaluation (test) sets
// so that types used to tune heuristics are not also used to report accuracy.
type Split string

const (
	// SplitTest marks an evaluation-only label (the default). Baseline P/R/F is
	// reported over the test split.
	SplitTest Split = "test"
	// SplitTrain marks a label used while calibrating heuristics; excluded from
	// the reported baseline to avoid optimistic bias.
	SplitTrain Split = "train"
)

// Label is one ground-truth expectation for a (type, principle) pair.
type Label struct {
	// ID is the stable target identifier (`<pkgPath>.<TypeName>`) joining a
	// label to a scored result. Empty for labels parsed from doc comments,
	// where the caller supplies the ID from the surrounding type.
	ID        string
	Principle analyzer.Principle
	Expect    Expectation
	Reason    string
	Split     Split
}

const docLabelPrefix = "solid:want"

var validPrinciples = map[analyzer.Principle]bool{
	analyzer.SRP: true, analyzer.OCP: true, analyzer.LSP: true,
	analyzer.ISP: true, analyzer.DIP: true,
}

// ParseDocLabels extracts `solid:want` labels from a type's doc comment text
// (as returned by go/ast's CommentGroup.Text, i.e. without the `// ` markers).
// Each `solid:want` line yields one Label. Lines without the prefix are
// ignored. The ID field is left empty for the caller to populate.
//
// Line grammar:
//
//	solid:want <PRINCIPLE>=<violation|ok|na> [reason="free text"] [split=train|test]
//
// PRINCIPLE is one of SRP/OCP/LSP/ISP/DIP. split defaults to test.
func ParseDocLabels(doc string) ([]Label, error) {
	var labels []Label
	for _, line := range strings.Split(doc, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, docLabelPrefix) {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(line, docLabelPrefix))
		label, err := parseLabelFields(rest)
		if err != nil {
			return nil, fmt.Errorf("solid:want %q: %w", rest, err)
		}
		labels = append(labels, label)
	}
	return labels, nil
}

// parseLabelFields parses the tokens after `solid:want`. The first token is the
// mandatory `PRINCIPLE=expectation`; the remainder are optional `key=value`
// pairs where reason's value may be a double-quoted string containing spaces.
func parseLabelFields(s string) (Label, error) {
	tokens, err := tokenize(s)
	if err != nil {
		return Label{}, err
	}
	if len(tokens) == 0 {
		return Label{}, fmt.Errorf("empty label")
	}

	label := Label{Split: SplitTest}
	sawPrinciple := false
	for i, tok := range tokens {
		key, val, ok := strings.Cut(tok, "=")
		if !ok {
			return Label{}, fmt.Errorf("token %q is not key=value", tok)
		}
		if i == 0 {
			// First token is PRINCIPLE=expectation.
			p := analyzer.Principle(key)
			if !validPrinciples[p] {
				return Label{}, fmt.Errorf("unknown principle %q", key)
			}
			exp := Expectation(val)
			if exp != Violation && exp != OK && exp != NA {
				return Label{}, fmt.Errorf("unknown expectation %q", val)
			}
			label.Principle = p
			label.Expect = exp
			sawPrinciple = true
			continue
		}
		switch key {
		case "reason":
			label.Reason = val
		case "split":
			sp := Split(val)
			if sp != SplitTest && sp != SplitTrain {
				return Label{}, fmt.Errorf("unknown split %q", val)
			}
			label.Split = sp
		default:
			return Label{}, fmt.Errorf("unknown field %q", key)
		}
	}
	if !sawPrinciple {
		return Label{}, fmt.Errorf("missing principle")
	}
	return label, nil
}

// tokenize splits a label field string on spaces, keeping double-quoted
// segments (e.g. reason="a b c") as a single token with the quotes removed.
func tokenize(s string) ([]string, error) {
	var tokens []string
	var cur strings.Builder
	inQuote := false
	hasCur := false
	for _, r := range s {
		switch {
		case r == '"':
			inQuote = !inQuote
			hasCur = true
		case r == ' ' && !inQuote:
			if hasCur {
				tokens = append(tokens, cur.String())
				cur.Reset()
				hasCur = false
			}
		default:
			cur.WriteRune(r)
			hasCur = true
		}
	}
	if inQuote {
		return nil, fmt.Errorf("unterminated quote")
	}
	if hasCur {
		tokens = append(tokens, cur.String())
	}
	return tokens, nil
}
