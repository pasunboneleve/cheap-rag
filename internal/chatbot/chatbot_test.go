package chatbot

import (
	"context"
	"errors"
	"testing"

	"github.com/dmvianna/cheap-rag/internal/config"
	"github.com/dmvianna/cheap-rag/internal/llm"
	"github.com/dmvianna/cheap-rag/internal/policy"
	"github.com/dmvianna/cheap-rag/internal/types"
)

type fakeRetriever struct {
	results []types.RetrievalResult
	err     error
}

func (f fakeRetriever) Retrieve(context.Context, string, int) ([]types.RetrievalResult, error) {
	return f.results, f.err
}

type fakeGenerator struct {
	resp  llm.GenerationResponse
	err   error
	calls int
}

func (f *fakeGenerator) Name() string { return "fake" }
func (f *fakeGenerator) Generate(context.Context, llm.GenerationRequest) (llm.GenerationResponse, error) {
	f.calls++
	if f.err != nil {
		return llm.GenerationResponse{}, f.err
	}
	return f.resp, nil
}

func TestRefusesWhenNoRetrievedChunks(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	gen := &fakeGenerator{}
	svc := New(cfg, fakeRetriever{}, gen, policy.NewValidator(cfg.Validation.MinEvidenceCoverage))
	out, err := svc.Ask(context.Background(), "what is ci?")
	if err != nil {
		t.Fatal(err)
	}
	if !out.Refused {
		t.Fatalf("expected refusal")
	}
	if gen.calls != 0 {
		t.Fatalf("expected generation not called")
	}
}

func TestRefusesBelowRetrievalThreshold(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	cfg.Retrieval.MinQuerySimilarity = 0.8
	gen := &fakeGenerator{}
	svc := New(cfg, fakeRetriever{results: []types.RetrievalResult{{
		Chunk:      types.Chunk{ID: "c1", Path: "a.md", Text: "cheap change means low coupling"},
		Similarity: 0.6,
	}}}, gen, policy.NewValidator(cfg.Validation.MinEvidenceCoverage))
	out, err := svc.Ask(context.Background(), "cheap to change?")
	if err != nil {
		t.Fatal(err)
	}
	if !out.Refused {
		t.Fatalf("expected refusal below threshold")
	}
	if gen.calls != 0 {
		t.Fatalf("expected generation not called")
	}
}

func TestRefusesWhenValidationFails(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	cfg.Validation.MinEvidenceCoverage = 0.8
	evidence := []types.RetrievalResult{{
		Chunk:      types.Chunk{ID: "chunk_1", Path: "post.md", Text: "Cheap change comes from explicit interfaces and low coupling."},
		Similarity: 0.91,
	}}
	gen := &fakeGenerator{resp: llm.GenerationResponse{
		Answer:    "Cheap change means explicit interfaces. NASA approved this process.",
		Citations: []string{"chunk_1"},
	}}
	svc := New(cfg, fakeRetriever{results: evidence}, gen, policy.NewValidator(cfg.Validation.MinEvidenceCoverage))
	out, err := svc.Ask(context.Background(), "what is cheap to change?")
	if err != nil {
		t.Fatal(err)
	}
	if !out.Refused {
		t.Fatalf("expected refusal when validation fails")
	}
	if out.Validation.Valid {
		t.Fatalf("expected invalid validation report")
	}
}

func TestBubblesGenerationErrors(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	evidence := []types.RetrievalResult{{
		Chunk:      types.Chunk{ID: "chunk_1", Path: "post.md", Text: "low coupling"},
		Similarity: 0.95,
	}}
	gen := &fakeGenerator{err: errors.New("provider down")}
	svc := New(cfg, fakeRetriever{results: evidence}, gen, policy.NewValidator(cfg.Validation.MinEvidenceCoverage))
	_, err := svc.Ask(context.Background(), "question")
	if err == nil {
		t.Fatalf("expected error")
	}
}
