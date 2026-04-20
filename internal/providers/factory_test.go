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
