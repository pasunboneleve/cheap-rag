package types

// Chunk is an indexed document fragment.
type Chunk struct {
	ID       string
	Citation string
	Path     string
	Text     string
}

// RetrievalResult contains a retrieved chunk and similarity score.
type RetrievalResult struct {
	Chunk      Chunk
	Similarity float64
}

// AskOutcome captures a guarded model response.
type AskOutcome struct {
	Refused       bool
	RefusalReason string
	Answer        string
	Citations     []string
	Retrieved     []RetrievalResult
	Validation    ValidationReport
}

// ValidationReport contains deterministic validation signals.
type ValidationReport struct {
	Coverage            float64
	UnsupportedClaims   []string
	UnsupportedEntities []string
	Valid               bool
}
