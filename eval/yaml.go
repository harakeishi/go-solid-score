package eval

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/harakeishi/go-solid-score/analyzer"
)

// yamlLabelFile is the on-disk schema for external ground-truth labels, used
// for corpora (e.g. third-party OSS) whose source we cannot annotate inline.
// Each entry is keyed by the target ID (`<pkgPath>.<TypeName>`), mirroring the
// FQN-keyed separate-file convention of code-smell datasets.
type yamlLabelFile struct {
	Labels []yamlEntry `yaml:"labels"`
}

type yamlEntry struct {
	ID   string        `yaml:"id"`
	Want []yamlVerdict `yaml:"want"`
}

type yamlVerdict struct {
	Principle string `yaml:"principle"`
	Expect    string `yaml:"expect"`
	Reason    string `yaml:"reason"`
	Split     string `yaml:"split"`
}

// ParseYAMLLabels decodes external ground-truth labels from YAML into a flat
// []Label (one per (id, principle) pair). split defaults to test when omitted.
func ParseYAMLLabels(data []byte) ([]Label, error) {
	var f yamlLabelFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parsing label YAML: %w", err)
	}
	var labels []Label
	for _, e := range f.Labels {
		if e.ID == "" {
			return nil, fmt.Errorf("label entry missing id")
		}
		for _, v := range e.Want {
			p := analyzer.Principle(v.Principle)
			if !validPrinciples[p] {
				return nil, fmt.Errorf("id %q: unknown principle %q", e.ID, v.Principle)
			}
			exp := Expectation(v.Expect)
			if exp != Violation && exp != OK && exp != NA {
				return nil, fmt.Errorf("id %q: unknown expect %q", e.ID, v.Expect)
			}
			split := Split(v.Split)
			switch split {
			case "":
				split = SplitTest
			case SplitTest, SplitTrain:
			default:
				return nil, fmt.Errorf("id %q: unknown split %q", e.ID, v.Split)
			}
			labels = append(labels, Label{
				ID:        e.ID,
				Principle: p,
				Expect:    exp,
				Reason:    v.Reason,
				Split:     split,
			})
		}
	}
	return labels, nil
}
