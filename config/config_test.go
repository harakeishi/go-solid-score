package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/harakeishi/go-solid-score/config"
)

func TestDefault(t *testing.T) {
	cfg := config.Default()
	if cfg.Format != "text" {
		t.Errorf("expected default format 'text', got %q", cfg.Format)
	}
	if cfg.Weights["SRP"] != 0.30 {
		t.Errorf("expected SRP weight 0.30, got %f", cfg.Weights["SRP"])
	}
	if cfg.Thresholds["total"] != 70 {
		t.Errorf("expected total threshold 70, got %f", cfg.Thresholds["total"])
	}
	if len(cfg.Paths) == 0 {
		t.Error("expected default paths")
	}
}

func TestLoad_NonExistent(t *testing.T) {
	cfg, err := config.Load("/nonexistent/.go-solid-score.yaml")
	if err != nil {
		t.Fatal(err)
	}
	// Should return defaults
	if cfg.Format != "text" {
		t.Errorf("expected default format, got %q", cfg.Format)
	}
}

func TestLoad_ValidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".go-solid-score.yaml")
	content := `
format: json
min_score: 80
weights:
  SRP: 0.40
dip:
  whitelist:
    - MyType
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Format != "json" {
		t.Errorf("expected format json, got %q", cfg.Format)
	}
	if cfg.MinScore != 80 {
		t.Errorf("expected min_score 80, got %f", cfg.MinScore)
	}
	if cfg.Weights["SRP"] != 0.40 {
		t.Errorf("expected SRP weight 0.40, got %f", cfg.Weights["SRP"])
	}
	// Other weights should still have defaults
	if cfg.Weights["OCP"] != 0.15 {
		t.Errorf("expected OCP weight 0.15, got %f", cfg.Weights["OCP"])
	}
	if len(cfg.DIP.Whitelist) != 1 || cfg.DIP.Whitelist[0] != "MyType" {
		t.Errorf("expected DIP whitelist [MyType], got %v", cfg.DIP.Whitelist)
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".go-solid-score.yaml")
	if err := os.WriteFile(path, []byte("{{invalid"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := config.Load(path)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}
