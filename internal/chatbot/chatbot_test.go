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
	resp    llm.GenerationResponse
	seq     []llm.GenerationResponse
	err     error
	calls   int
	lastReq llm.GenerationRequest
}

func (f *fakeGenerator) Name() string { return "fake" }
func (f *fakeGenerator) Generate(_ context.Context, req llm.GenerationRequest) (llm.GenerationResponse, error) {
	f.calls++
	f.lastReq = req
	if f.err != nil {
		return llm.GenerationResponse{}, f.err
	}
	if len(f.seq) > 0 {
		resp := f.seq[0]
		f.seq = f.seq[1:]
		return resp, nil
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

func TestReturnsAnswerWhenValidationFails(t *testing.T) {
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
	if out.Refused {
		t.Fatalf("expected answer even when validation fails")
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

func TestDoesNotNeedRetryWhenFallbackCitationsAreAvailable(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	evidence := []types.RetrievalResult{{
		Chunk:      types.Chunk{ID: "chunk_1", Citation: "chunk_1", Path: "post.md", Text: "low coupling and explicit boundaries"},
		Similarity: 0.95,
	}}
	gen := &fakeGenerator{resp: llm.GenerationResponse{Answer: "Low coupling matters.", Citations: nil}}
	svc := New(cfg, fakeRetriever{results: evidence}, gen, policy.NewValidator(cfg.Validation.MinEvidenceCoverage))
	out, err := svc.Ask(context.Background(), "question")
	if err != nil {
		t.Fatal(err)
	}
	if out.Refused {
		t.Fatalf("expected answer after retry, got refusal: %+v", out)
	}
	if gen.calls != 1 {
		t.Fatalf("expected one generation attempt, got %d", gen.calls)
	}
}

func TestUsesFallbackCitationsWhenModelProvidesNone(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	evidence := []types.RetrievalResult{
		{Chunk: types.Chunk{ID: "chunk_1", Citation: "cite-1", Path: "post.md", Text: "evidence one"}, Similarity: 0.91},
		{Chunk: types.Chunk{ID: "chunk_2", Citation: "cite-2", Path: "post.md", Text: "evidence two"}, Similarity: 0.90},
	}
	gen := &fakeGenerator{resp: llm.GenerationResponse{
		Answer:    "Use explicit boundaries and fast feedback loops.",
		Citations: nil,
	}}
	svc := New(cfg, fakeRetriever{results: evidence}, gen, policy.NewValidator(cfg.Validation.MinEvidenceCoverage))
	out, err := svc.Ask(context.Background(), "question")
	if err != nil {
		t.Fatal(err)
	}
	if out.Refused {
		t.Fatalf("expected answer with fallback citations")
	}
	if len(out.Citations) == 0 {
		t.Fatalf("expected fallback citations to be populated")
	}
}

func TestUsesConfiguredRefusalMessages(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	cfg.Responses.Refusal.NoRetrieval = "custom no retrieval"
	cfg.Responses.Refusal.LowSimilarity = "custom low similarity score={score} threshold={threshold}"
	gen := &fakeGenerator{}
	svc := New(cfg, fakeRetriever{}, gen, policy.NewValidator(cfg.Validation.MinEvidenceCoverage))
	out, err := svc.Ask(context.Background(), "question")
	if err != nil {
		t.Fatal(err)
	}
	if out.RefusalReason != "custom no retrieval" {
		t.Fatalf("unexpected no retrieval refusal: %q", out.RefusalReason)
	}

	low := New(cfg, fakeRetriever{results: []types.RetrievalResult{{
		Chunk:      types.Chunk{ID: "c1", Citation: "c1", Path: "a.md", Text: "text"},
		Similarity: 0.10,
	}}}, gen, policy.NewValidator(cfg.Validation.MinEvidenceCoverage))
	lowOut, err := low.Ask(context.Background(), "question")
	if err != nil {
		t.Fatal(err)
	}
	if lowOut.RefusalReason == cfg.Responses.Refusal.LowSimilarity {
		t.Fatalf("expected placeholder-expanded low-similarity refusal")
	}
}

func TestPassesConfiguredGenerationTemperature(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	cfg.GenerationTemperature = 0.8
	evidence := []types.RetrievalResult{{
		Chunk:      types.Chunk{ID: "chunk_1", Citation: "chunk_1", Path: "post.md", Text: "evidence"},
		Similarity: 0.95,
	}}
	gen := &fakeGenerator{resp: llm.GenerationResponse{
		Answer:    "answer",
		Citations: []string{"chunk_1"},
	}}
	svc := New(cfg, fakeRetriever{results: evidence}, gen, policy.NewValidator(cfg.Validation.MinEvidenceCoverage))
	if _, err := svc.Ask(context.Background(), "question"); err != nil {
		t.Fatal(err)
	}
	if gen.lastReq.Temperature != 0.8 {
		t.Fatalf("expected temperature 0.8, got %v", gen.lastReq.Temperature)
	}
}
