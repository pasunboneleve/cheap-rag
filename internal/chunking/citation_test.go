package chunking

import (
	"testing"

	"github.com/pasunboneleve/cheap-rag/internal/types"
)

func TestFormatCitationSlugPattern(t *testing.T) {
	t.Parallel()
	chunk := types.Chunk{ID: "chunk_abc", Path: "posts/2026-03-03-optimise-for-the-cheapest-change.md"}
	got := FormatCitation("{slug}", chunk, 2)
	want := "2026-03-03-optimise-for-the-cheapest-change"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestFormatCitationComposedPattern(t *testing.T) {
	t.Parallel()
	chunk := types.Chunk{ID: "chunk_abc", Path: "posts/cost-of-change.md"}
	got := FormatCitation("{slug}#{chunk_index}", chunk, 4)
	if got != "cost-of-change#4" {
		t.Fatalf("unexpected citation: %s", got)
	}
}
