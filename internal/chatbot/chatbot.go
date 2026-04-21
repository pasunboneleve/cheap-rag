package chatbot

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/dmvianna/cheap-rag/internal/config"
	"github.com/dmvianna/cheap-rag/internal/llm"
	"github.com/dmvianna/cheap-rag/internal/providerdiag"
	"github.com/dmvianna/cheap-rag/internal/types"
)

type Service struct {
	refusalSeq atomic.Uint64
	cfg        config.Config
	retriever  Retriever
	gen        llm.GenerationProvider
	validator  Validator
}

type Retriever interface {
	Retrieve(ctx context.Context, query string, topK int) ([]types.RetrievalResult, error)
}

type Validator interface {
	Validate(answer string, citations []string, evidence []types.RetrievalResult) types.ValidationReport
}

type AskError struct {
	Err              error
	ProviderStatuses map[string]int
}

func (e *AskError) Error() string { return e.Err.Error() }
func (e *AskError) Unwrap() error { return e.Err }
func (e *AskError) ProviderStatusMap() map[string]int {
	if len(e.ProviderStatuses) == 0 {
		return nil
	}
	out := make(map[string]int, len(e.ProviderStatuses))
	for k, v := range e.ProviderStatuses {
		out[k] = v
	}
	return out
}

func New(cfg config.Config, retriever Retriever, gen llm.GenerationProvider, validator Validator) *Service {
	return &Service{cfg: cfg, retriever: retriever, gen: gen, validator: validator}
}

var genericRefusalVariants = []string{
	"Sorry, but I could not relate your question to the content I have.",
	"Sorry, I could not connect that question to the material available to me.",
	"Sorry, I can’t find enough relevant material to answer that question.",
	"Sorry, that question does not seem to match the content I can use right now.",
}

func (s *Service) Ask(ctx context.Context, question string) (types.AskOutcome, error) {
	diagCtx, tracker := providerdiag.WithTracker(ctx)
	retrieved, err := s.retriever.Retrieve(diagCtx, question, s.cfg.Retrieval.TopK)
	if err != nil {
		return types.AskOutcome{}, &AskError{Err: err, ProviderStatuses: tracker.Snapshot()}
	}
	if len(retrieved) == 0 {
		out, err := s.refuseWithProvider(providerdiag.WithStage(diagCtx, "generation"), retrieved, s.cfg.Responses.Refusal.NoRetrieval)
		out.ProviderStatuses = tracker.Snapshot()
		return out, err
	}
	if retrieved[0].Similarity < s.cfg.Retrieval.MinQuerySimilarity {
		out, err := s.refuseWithProvider(providerdiag.WithStage(diagCtx, "generation"), retrieved, s.cfg.Responses.Refusal.LowSimilarity)
		out.ProviderStatuses = tracker.Snapshot()
		return out, err
	}
	evidence := make([]llm.EvidenceChunk, 0, len(retrieved))
	for _, r := range retrieved {
		evidence = append(evidence, llm.EvidenceChunk{ID: r.Chunk.ID, Citation: r.Chunk.Citation, Path: r.Chunk.Path, Text: r.Chunk.Text})
	}
	genResp, err := s.gen.Generate(providerdiag.WithStage(diagCtx, "generation"), llm.GenerationRequest{
		Question:     question,
		Evidence:     evidence,
		Model:        s.cfg.Model,
		Temperature:  s.cfg.GenerationTemperature,
		SystemPolicy: policyPrompt(),
	})
	if err != nil {
		return types.AskOutcome{}, &AskError{Err: fmt.Errorf("generate answer: %w", err), ProviderStatuses: tracker.Snapshot()}
	}
	citations := sanitizeCitations(genResp.Citations)
	if len(citations) == 0 {
		citations = fallbackCitations(retrieved, 3)
	}
	report := s.validator.Validate(genResp.Answer, citations, retrieved)
	return types.AskOutcome{
		Answer:           genResp.Answer,
		Citations:        citations,
		Retrieved:        retrieved,
		ProviderStatuses: tracker.Snapshot(),
		Validation:       report,
	}, nil
}

func policyPrompt() string {
	return "You are a guarded assistant for local blog content only. Use only provided evidence chunks. If evidence is insufficient, return JSON with answer explaining refusal and empty citations. Do not invent entities or facts."
}

func refusalPolicyPrompt() string {
	return "Rewrite the provided sentence in one short sentence. Keep the same meaning."
}

func (s *Service) refuseWithProvider(ctx context.Context, retrieved []types.RetrievalResult, seed string) (types.AskOutcome, error) {
	fallbackSeed := strings.TrimSpace(seed)
	promptSeed := fallbackSeed
	if promptSeed == "" {
		promptSeed = "Sorry, I don't know how to answer this."
	}
	prompt := fmt.Sprintf("Rephrase this sentence in one short sentence: %q", promptSeed)
	genResp, err := s.gen.Generate(ctx, llm.GenerationRequest{
		Question:     prompt,
		Evidence:     nil,
		Model:        s.cfg.Model,
		Temperature:  s.cfg.GenerationTemperature,
		SystemPolicy: refusalPolicyPrompt(),
	})
	if err != nil {
		return refuse(seedFallback(fallbackSeed, s.fallbackRefusal), retrieved), nil
	}
	msg := strings.TrimSpace(genResp.Answer)
	if msg == "" {
		msg = seedFallback(fallbackSeed, s.fallbackRefusal)
	}
	return refuse(msg, retrieved), nil
}

func (s *Service) fallbackRefusal() string {
	next := s.refusalSeq.Add(1)
	return genericRefusalVariants[(next-1)%uint64(len(genericRefusalVariants))]
}

func seedFallback(seed string, fallback func() string) string {
	seed = strings.TrimSpace(seed)
	if seed != "" {
		return seed
	}
	return fallback()
}

func refuse(reason string, retrieved []types.RetrievalResult) types.AskOutcome {
	return types.AskOutcome{Refused: true, RefusalReason: reason, Retrieved: retrieved}
}

func sanitizeCitations(citations []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, c := range citations {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		out = append(out, c)
	}
	return out
}

func fallbackCitations(retrieved []types.RetrievalResult, limit int) []string {
	if limit <= 0 {
		limit = 1
	}
	seen := map[string]struct{}{}
	var out []string
	for _, r := range retrieved {
		cite := strings.TrimSpace(r.Chunk.Citation)
		if cite == "" {
			cite = r.Chunk.ID
		}
		if cite == "" {
			continue
		}
		if _, ok := seen[cite]; ok {
			continue
		}
		seen[cite] = struct{}{}
		out = append(out, cite)
		if len(out) >= limit {
			break
		}
	}
	return out
}
