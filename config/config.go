// Package config handles loading and merging of configuration for
// go-solid-score, including scoring weights, thresholds, and output format.
package config

import (
	"os"

	"gopkg.in/yaml.v3"

	"github.com/harakeishi/go-solid-score/analyzer"
	"github.com/harakeishi/go-solid-score/rules"
)

// Config holds the configuration for go-solid-score.
type Config struct {
	Paths      []string           `yaml:"paths"`
	Exclude    []string           `yaml:"exclude"`
	Format     string             `yaml:"format"`
	Weights    map[string]float64 `yaml:"weights"`
	Thresholds map[string]float64 `yaml:"thresholds"`
	MinScore   float64            `yaml:"min_score"`
	DIP        DIPConfig          `yaml:"dip"`

	// Rules holds user-defined scoring rules. A rule whose `id` matches a
	// built-in preset replaces that preset (in place); a new `id` adds a rule.
	// See rules/presets.yaml for the schema and the built-in rules to copy.
	Rules []rules.Rule `yaml:"rules"`
	// DisableRules lists preset rule ids to switch off without redefining them.
	DisableRules []string `yaml:"disable_rules"`
	// RuleDefaults overrides the per-principle starting score/confidence.
	RuleDefaults map[string]rules.Defaults `yaml:"rule_defaults"`
}

// RuleSet returns the effective rule set: the built-in presets with the user's
// custom rules, disabled ids, and default overrides merged in.
func (c *Config) RuleSet() rules.RuleSet {
	user := rules.RuleSet{Defaults: c.RuleDefaults, Rules: c.Rules}
	return rules.Merge(rules.DefaultRuleSet(), user, c.DisableRules)
}

// Engine builds the scoring engine from the effective rule set. It validates
// the merged rules — surfacing malformed conditions, no-op rules, and
// references to unknown (e.g. misspelled) metrics — so configuration mistakes
// are reported rather than silently mis-scoring.
func (c *Config) Engine() (*rules.Engine, error) {
	return rules.NewEngine(c.RuleSet(), analyzer.MetricNames()...)
}

// DIPConfig holds DIP-specific configuration.
type DIPConfig struct {
	Whitelist []string `yaml:"whitelist"`
}

// DefaultWeights returns the default scoring weights for each principle.
func DefaultWeights() map[string]float64 {
	return map[string]float64{
		"SRP": 0.30,
		"OCP": 0.15,
		"LSP": 0.10,
		"ISP": 0.20,
		"DIP": 0.25,
	}
}

// DefaultThresholds returns the default minimum score thresholds.
func DefaultThresholds() map[string]float64 {
	return map[string]float64{
		"total": 70,
		"SRP":   60,
		"OCP":   50,
		"LSP":   50,
		"ISP":   50,
		"DIP":   60,
	}
}

// Default returns a Config with default values.
func Default() *Config {
	return &Config{
		Paths:      []string{"./..."},
		Format:     "text",
		Weights:    DefaultWeights(),
		Thresholds: DefaultThresholds(),
		MinScore:   0,
	}
}

// Load reads a config from a YAML file and merges with defaults.
func Load(path string) (*Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	var fileCfg Config
	if err := yaml.Unmarshal(data, &fileCfg); err != nil {
		return nil, err
	}

	if len(fileCfg.Paths) > 0 {
		cfg.Paths = fileCfg.Paths
	}
	if len(fileCfg.Exclude) > 0 {
		cfg.Exclude = fileCfg.Exclude
	}
	if fileCfg.Format != "" {
		cfg.Format = fileCfg.Format
	}
	if fileCfg.MinScore > 0 {
		cfg.MinScore = fileCfg.MinScore
	}
	for k, v := range fileCfg.Weights {
		cfg.Weights[k] = v
	}
	for k, v := range fileCfg.Thresholds {
		cfg.Thresholds[k] = v
	}
	if len(fileCfg.DIP.Whitelist) > 0 {
		cfg.DIP.Whitelist = fileCfg.DIP.Whitelist
	}
	if len(fileCfg.Rules) > 0 {
		cfg.Rules = fileCfg.Rules
	}
	if len(fileCfg.DisableRules) > 0 {
		cfg.DisableRules = fileCfg.DisableRules
	}
	if len(fileCfg.RuleDefaults) > 0 {
		cfg.RuleDefaults = fileCfg.RuleDefaults
	}

	return cfg, nil
}
