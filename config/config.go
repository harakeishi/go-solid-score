package config

import (
	"os"

	"gopkg.in/yaml.v3"
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

	return cfg, nil
}
