package policy

import "testing"

func TestEntityMismatchesIgnoresSentenceCaseWords(t *testing.T) {
	t.Parallel()
	answer := "Cheap change requires explicit interfaces. Keep modules small."
	evidence := "cheap change requires explicit interfaces and small modules"
	unsupported := entityMismatches(answer, evidence)
	if len(unsupported) != 0 {
		t.Fatalf("expected no unsupported entities, got %v", unsupported)
	}
}

func TestEntityMismatchesFlagsUnsupportedAcronym(t *testing.T) {
	t.Parallel()
	answer := "Cheap change is useful. NASA approves this method."
	evidence := "cheap change is useful for local reasoning"
	unsupported := entityMismatches(answer, evidence)
	if len(unsupported) == 0 {
		t.Fatalf("expected unsupported acronym to be flagged")
	}
}
