package chatbot

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/pasunboneleve/cheap-rag/internal/config"
	"github.com/pasunboneleve/cheap-rag/internal/llm"
	"github.com/pasunboneleve/cheap-rag/internal/policy"
	"github.com/pasunboneleve/cheap-rag/internal/types"
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
	if gen.calls != 1 {
		t.Fatalf("expected generation called once to phrase refusal")
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
	if gen.calls != 1 {
		t.Fatalf("expected generation called once to phrase refusal")
	}
	if strings.Contains(strings.ToLower(gen.lastReq.Question), "threshold") || strings.Contains(strings.ToLower(gen.lastReq.Question), "score") {
		t.Fatalf("expected non-technical refusal prompt, got %q", gen.lastReq.Question)
	}
}

func TestRefusalFallsBackWhenProviderFails(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	cfg.Responses.Refusal.NoRetrieval = "Sorry, I don't know how to answer this."
	gen := &fakeGenerator{err: errors.New("provider down")}
	svc := New(cfg, fakeRetriever{}, gen, policy.NewValidator(cfg.Validation.MinEvidenceCoverage))
	out, err := svc.Ask(context.Background(), "what is ci?")
	if err != nil {
		t.Fatal(err)
	}
	if !out.Refused {
		t.Fatalf("expected refusal")
	}
	if out.RefusalReason != cfg.Responses.Refusal.NoRetrieval {
		t.Fatalf("expected configured seed fallback, got %q", out.RefusalReason)
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

func TestUsesProviderRefusalMessage(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	gen := &fakeGenerator{resp: llm.GenerationResponse{
		Answer:    "provider-worded refusal",
		Citations: nil,
	}}
	svc := New(cfg, fakeRetriever{}, gen, policy.NewValidator(cfg.Validation.MinEvidenceCoverage))
	out, err := svc.Ask(context.Background(), "question")
	if err != nil {
		t.Fatal(err)
	}
	if out.RefusalReason != "provider-worded refusal" {
		t.Fatalf("unexpected no retrieval refusal: %q", out.RefusalReason)
	}
}

func TestFallbackRefusalRotatesVariants(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	gen := &fakeGenerator{err: errors.New("provider down")}
	svc := New(cfg, fakeRetriever{}, gen, policy.NewValidator(cfg.Validation.MinEvidenceCoverage))

	out1, err := svc.Ask(context.Background(), "question one")
	if err != nil {
		t.Fatal(err)
	}
	out2, err := svc.Ask(context.Background(), "question two")
	if err != nil {
		t.Fatal(err)
	}
	if out1.RefusalReason == out2.RefusalReason {
		t.Fatalf("expected refusal variants to rotate, got identical message %q", out1.RefusalReason)
	}
}

func TestRefusalPromptDoesNotContainUserQuestion(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	gen := &fakeGenerator{resp: llm.GenerationResponse{
		Answer: "Sorry, I don't know how to answer this.",
	}}
	svc := New(cfg, fakeRetriever{}, gen, policy.NewValidator(cfg.Validation.MinEvidenceCoverage))
	_, err := svc.Ask(context.Background(), "how do I build a football team?")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(gen.lastReq.Question), "football") {
		t.Fatalf("expected refusal prompt to exclude user question, got %q", gen.lastReq.Question)
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
