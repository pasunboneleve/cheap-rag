package llm

import "context"

// EmbeddingsProvider creates embedding vectors.
type EmbeddingsProvider interface {
	Embed(ctx context.Context, texts []string, model string) ([][]float64, error)
	Name() string
}

// GenerationRequest is the generation input.
type GenerationRequest struct {
	Question     string
	Evidence     []EvidenceChunk
	Model        string
	SystemPolicy string
}

// EvidenceChunk is grounding evidence passed to the model.
type EvidenceChunk struct {
	ID   string
	Path string
	Text string
}

// GenerationResponse is a structured answer with citations.
type GenerationResponse struct {
	Answer    string   `json:"answer"`
	Citations []string `json:"citations"`
	RawText   string   `json:"-"`
}

// GenerationProvider produces grounded answers.
type GenerationProvider interface {
	Generate(ctx context.Context, req GenerationRequest) (GenerationResponse, error)
	Name() string
}
