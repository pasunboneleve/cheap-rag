package providers

import "testing"

func TestFactorySupportsXAI(t *testing.T) {
	t.Parallel()
	embed, err := NewEmbeddings("xai")
	if err != nil {
		t.Fatalf("expected xai embeddings provider: %v", err)
	}
	if embed.Name() != "xai" {
		t.Fatalf("unexpected provider name: %s", embed.Name())
	}

	gen, err := NewGenerator("xai")
	if err != nil {
		t.Fatalf("expected xai generator provider: %v", err)
	}
	if gen.Name() != "xai" {
		t.Fatalf("unexpected generator name: %s", gen.Name())
	}
}

func TestFactorySupportsAnthropicGenerationOnly(t *testing.T) {
	t.Parallel()
	gen, err := NewGenerator("anthropic")
	if err != nil {
		t.Fatalf("expected anthropic generator provider: %v", err)
	}
	if gen.Name() != "anthropic" {
		t.Fatalf("unexpected generator name: %s", gen.Name())
	}

	_, err = NewEmbeddings("anthropic")
	if err == nil {
		t.Fatalf("expected anthropic embeddings provider to be unsupported")
	}
}
