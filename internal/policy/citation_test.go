package policy

import (
	"testing"

	"github.com/dmvianna/chatbot-prototype/internal/types"
)

func TestCitationSetUsesCitationFieldWhenPresent(t *testing.T) {
	t.Parallel()
	evidence := []types.RetrievalResult{{Chunk: types.Chunk{ID: "chunk_1", Citation: "post-slug"}}}
	set := citationSet([]string{"post-slug"}, evidence)
	if _, ok := set["post-slug"]; !ok {
		t.Fatalf("expected citation match on citation field")
	}
}
