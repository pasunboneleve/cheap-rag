package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds runtime settings for the chatbot.
type Config struct {
	ContentRoot    string           `yaml:"content_root"`
	RuntimeRoot    string           `yaml:"runtime_root"`
	Provider       string           `yaml:"provider"`
	Model          string           `yaml:"model"`
	EmbeddingModel string           `yaml:"embedding_model"`
	Retrieval      RetrievalConfig  `yaml:"retrieval"`
	Validation     ValidationConfig `yaml:"validation"`
}

type RetrievalConfig struct {
	TopK               int     `yaml:"top_k"`
	MinQuerySimilarity float64 `yaml:"min_query_similarity"`
}

type ValidationConfig struct {
	MinEvidenceCoverage float64 `yaml:"min_evidence_coverage"`
}

func Default() Config {
	return Config{
		ContentRoot:    "./content",
		RuntimeRoot:    "./.chatbot",
		Provider:       "openai-compatible",
		Model:          "gpt-4o-mini",
		EmbeddingModel: "text-embedding-3-small",
		Retrieval: RetrievalConfig{
			TopK:               5,
			MinQuerySimilarity: 0.45,
		},
		Validation: ValidationConfig{MinEvidenceCoverage: 0.55},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	if path == "" {
		return cfg, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c *Config) ApplyOverrides(contentRoot, runtimeRoot, provider, model, embeddingModel string) {
	if contentRoot != "" {
		c.ContentRoot = contentRoot
	}
	if runtimeRoot != "" {
		c.RuntimeRoot = runtimeRoot
	}
	if provider != "" {
		c.Provider = provider
	}
	if model != "" {
		c.Model = model
	}
	if embeddingModel != "" {
		c.EmbeddingModel = embeddingModel
	}
}

func (c Config) Validate() error {
	if c.ContentRoot == "" || c.RuntimeRoot == "" {
		return errors.New("content_root and runtime_root are required")
	}
	if c.Provider == "" || c.Model == "" || c.EmbeddingModel == "" {
		return errors.New("provider, model and embedding_model are required")
	}
	if c.Retrieval.TopK <= 0 {
		return errors.New("retrieval.top_k must be > 0")
	}
	if c.Retrieval.MinQuerySimilarity < -1 || c.Retrieval.MinQuerySimilarity > 1 {
		return errors.New("retrieval.min_query_similarity must be between -1 and 1")
	}
	if c.Validation.MinEvidenceCoverage < 0 || c.Validation.MinEvidenceCoverage > 1 {
		return errors.New("validation.min_evidence_coverage must be between 0 and 1")
	}
	return nil
}

func (c Config) ContentRootAbs() (string, error) {
	return filepath.Abs(c.ContentRoot)
}

func (c Config) RuntimeRootAbs() (string, error) {
	return filepath.Abs(c.RuntimeRoot)
}
