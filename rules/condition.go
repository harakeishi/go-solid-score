package rules

import (
	"fmt"
	"strconv"
	"strings"
)

// comparison is a parsed numeric comparison such as ">= 40". It is the atomic
// predicate used by both a rule's `when` clause (compared against the rule's
// metric value) and each entry of a `where` clause (compared against a named
// metric).
type comparison struct {
	op  string
	num float64
}

// operators are matched longest-first so that the two-character forms ">=",
// "<=", "==", and "!=" are recognized before the single-character ">" and "<".
var operators = []string{">=", "<=", "==", "!=", ">", "<"}

// parseComparison parses a string of the form "<op> <number>", e.g. "> 40",
// ">= 0.15", or "== 0". Whitespace around the parts is optional.
func parseComparison(s string) (comparison, error) {
	t := strings.TrimSpace(s)
	for _, op := range operators {
		if strings.HasPrefix(t, op) {
			numStr := strings.TrimSpace(t[len(op):])
			num, err := strconv.ParseFloat(numStr, 64)
			if err != nil {
				return comparison{}, fmt.Errorf("invalid number %q in condition %q", numStr, s)
			}
			return comparison{op: op, num: num}, nil
		}
	}
	return comparison{}, fmt.Errorf("condition %q must start with one of >= <= == != > <", s)
}

// eval reports whether v satisfies the comparison.
func (c comparison) eval(v float64) bool {
	switch c.op {
	case ">":
		return v > c.num
	case ">=":
		return v >= c.num
	case "<":
		return v < c.num
	case "<=":
		return v <= c.num
	case "==":
		return v == c.num
	case "!=":
		return v != c.num
	}
	return false
}

// whereClause is a parsed precondition of the form "<metric> <op> <number>",
// e.g. "structural_dep_total == 0". All of a rule's where clauses must hold for
// the rule to apply, regardless of its primary `when` condition.
type whereClause struct {
	metric string
	cmp    comparison
}

// parseWhere parses a where clause such as "method_count >= 4".
func parseWhere(s string) (whereClause, error) {
	fields := strings.Fields(s)
	if len(fields) != 3 {
		return whereClause{}, fmt.Errorf("where clause %q must be '<metric> <op> <number>'", s)
	}
	cmp, err := parseComparison(fields[1] + " " + fields[2])
	if err != nil {
		return whereClause{}, fmt.Errorf("in where clause %q: %w", s, err)
	}
	return whereClause{metric: fields[0], cmp: cmp}, nil
}
