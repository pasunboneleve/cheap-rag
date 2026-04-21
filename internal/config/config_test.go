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
	temp := 0.9
	cfg.ApplyOverrides("", "", "", "xai", "gemini", "grok-4-1-fast-reasoning", "gemini-embedding-001", "{slug}", &temp)
	if cfg.GenerationProvider != "xai" {
		t.Fatalf("unexpected generation provider: %s", cfg.GenerationProvider)
	}
	if cfg.EmbeddingProvider != "gemini" {
		t.Fatalf("unexpected embedding provider: %s", cfg.EmbeddingProvider)
	}
	if cfg.CitationPattern != "{slug}" {
		t.Fatalf("unexpected citation pattern: %s", cfg.CitationPattern)
	}
	if cfg.GenerationTemperature != 0.9 {
		t.Fatalf("unexpected generation temperature: %v", cfg.GenerationTemperature)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid config: %v", err)
	}
}

func TestValidateRejectsOutOfRangeGenerationTemperature(t *testing.T) {
	t.Parallel()
	cfg := Default()
	cfg.GenerationTemperature = 3.0
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected out-of-range generation temperature validation error")
	}
}

func TestValidateRejectsAnthropicTemperatureAboveOne(t *testing.T) {
	t.Parallel()
	cfg := Default()
	cfg.GenerationProvider = "anthropic"
	cfg.GenerationTemperature = 1.2
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected anthropic temperature validation error")
	}
}

func TestValidateBackfillsRuntimeSocketPathFromRuntimeRoot(t *testing.T) {
	t.Parallel()
	cfg := Default()
	cfg.Runtime.SocketPath = ""
	cfg.RuntimeRoot = "/tmp/cheap-rag-runtime"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid config: %v", err)
	}
	if cfg.Runtime.SocketPath != "/tmp/cheap-rag-runtime/cheap-rag.sock" {
		t.Fatalf("unexpected socket path: %s", cfg.Runtime.SocketPath)
	}
}

func TestValidateRejectsInvalidServerLimits(t *testing.T) {
	t.Parallel()
	cfg := Default()
	cfg.Server.MaxInflightRequests = 0
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected invalid max_inflight_requests error")
	}

	cfg = Default()
	cfg.Server.MaxRequestBodyBytes = 0
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected invalid max_request_body_bytes error")
	}
}
