package policy

import (
	"testing"

	"github.com/dmvianna/cheap-rag/internal/types"
)

func TestValidateAllowsOneUnsupportedClaimWhenCoverageIsHigh(t *testing.T) {
	t.Parallel()
	v := NewValidator(0.45)
	evidence := []types.RetrievalResult{{Chunk: types.Chunk{
		ID:       "chunk_1",
		Citation: "cheap-change",
		Path:     "post.md",
		Text:     "Cheap change comes from short feedback loops, explicit boundaries, and localised change surfaces.",
	}}}
	answer := "Cheap change comes from short feedback loops and explicit boundaries. Keep organisational velocity high over time."
	report := v.Validate(answer, []string{"cheap-change"}, evidence)
	if !report.Valid {
		t.Fatalf("expected valid report, got invalid: %#v", report)
	}
	if report.Coverage < 0.45 {
		t.Fatalf("expected coverage >= 0.45, got %.2f", report.Coverage)
	}
	if len(report.UnsupportedClaims) == 0 {
		t.Fatalf("expected at least one unsupported claim to be recorded")
	}
}
