package chatbot

import (
	"context"
	"fmt"

	"github.com/dmvianna/chatbot-prototype/internal/config"
	"github.com/dmvianna/chatbot-prototype/internal/llm"
	"github.com/dmvianna/chatbot-prototype/internal/types"
)

type Service struct {
	cfg       config.Config
	retriever Retriever
	gen       llm.GenerationProvider
	validator Validator
}

type Retriever interface {
	Retrieve(ctx context.Context, query string, topK int) ([]types.RetrievalResult, error)
}

type Validator interface {
	Validate(answer string, citations []string, evidence []types.RetrievalResult) types.ValidationReport
}

func New(cfg config.Config, retriever Retriever, gen llm.GenerationProvider, validator Validator) *Service {
	return &Service{cfg: cfg, retriever: retriever, gen: gen, validator: validator}
}

func (s *Service) Ask(ctx context.Context, question string) (types.AskOutcome, error) {
	retrieved, err := s.retriever.Retrieve(ctx, question, s.cfg.Retrieval.TopK)
	if err != nil {
		return types.AskOutcome{}, err
	}
	if len(retrieved) == 0 {
		return refuse("I can only answer from indexed local content. I found no relevant chunks.", retrieved), nil
	}
	if retrieved[0].Similarity < s.cfg.Retrieval.MinQuerySimilarity {
		return refuse(fmt.Sprintf("I can only answer when local evidence is relevant enough (top similarity %.3f < threshold %.3f).", retrieved[0].Similarity, s.cfg.Retrieval.MinQuerySimilarity), retrieved), nil
	}
	evidence := make([]llm.EvidenceChunk, 0, len(retrieved))
	for _, r := range retrieved {
		evidence = append(evidence, llm.EvidenceChunk{ID: r.Chunk.ID, Path: r.Chunk.Path, Text: r.Chunk.Text})
	}
	genResp, err := s.gen.Generate(ctx, llm.GenerationRequest{
		Question:     question,
		Evidence:     evidence,
		Model:        s.cfg.Model,
		SystemPolicy: policyPrompt(),
	})
	if err != nil {
		return types.AskOutcome{}, fmt.Errorf("generate answer: %w", err)
	}
	if len(genResp.Citations) == 0 {
		return refuse("I could not produce a grounded answer with citations from retrieved evidence.", retrieved), nil
	}
	report := s.validator.Validate(genResp.Answer, genResp.Citations, retrieved)
	if !report.Valid {
		return types.AskOutcome{
			Refused:       true,
			RefusalReason: "I found related content but the evidence was insufficient to support a reliable answer.",
			Retrieved:     retrieved,
			Validation:    report,
		}, nil
	}
	return types.AskOutcome{
		Answer:     genResp.Answer,
		Citations:  genResp.Citations,
		Retrieved:  retrieved,
		Validation: report,
	}, nil
}

func policyPrompt() string {
	return "You are a guarded assistant for local blog content only. Use only provided evidence chunks. If evidence is insufficient, return JSON with answer explaining refusal and empty citations. Do not invent entities or facts."
}

func refuse(reason string, retrieved []types.RetrievalResult) types.AskOutcome {
	return types.AskOutcome{Refused: true, RefusalReason: reason, Retrieved: retrieved}
}
