package config

import "testing"

func TestValidateUsesLegacyProviderFallback(t *testing.T) {
	t.Parallel()
	cfg := Default()
	cfg.Provider = "xai"
	cfg.GenerationProvider = ""
	cfg.EmbeddingProvider = ""
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected legacy fallback to validate: %v", err)
	}
	if cfg.GenerationProvider != "xai" || cfg.EmbeddingProvider != "xai" {
		t.Fatalf("expected fallback providers to be set from legacy provider")
	}
}

func TestApplyOverridesSplitProviders(t *testing.T) {
	t.Parallel()
	cfg := Default()
	cfg.ApplyOverrides("", "", "", "xai", "gemini", "grok-4-1-fast-reasoning", "gemini-embedding-001", "{slug}")
	if cfg.GenerationProvider != "xai" {
		t.Fatalf("unexpected generation provider: %s", cfg.GenerationProvider)
	}
	if cfg.EmbeddingProvider != "gemini" {
		t.Fatalf("unexpected embedding provider: %s", cfg.EmbeddingProvider)
	}
	if cfg.CitationPattern != "{slug}" {
		t.Fatalf("unexpected citation pattern: %s", cfg.CitationPattern)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid config: %v", err)
	}
}
