package retrieval

import (
	"context"
	"fmt"

	"github.com/pasunboneleve/cheap-rag/internal/llm"
	"github.com/pasunboneleve/cheap-rag/internal/providerdiag"
	"github.com/pasunboneleve/cheap-rag/internal/store"
	"github.com/pasunboneleve/cheap-rag/internal/types"
)

type Retriever struct {
	embeddings llm.EmbeddingsProvider
	store      *store.SQLiteStore
	model      string
}

func New(embeddings llm.EmbeddingsProvider, st *store.SQLiteStore, embeddingModel string) *Retriever {
	return &Retriever{embeddings: embeddings, store: st, model: embeddingModel}
}

func (r *Retriever) Retrieve(ctx context.Context, query string, topK int) ([]types.RetrievalResult, error) {
	vectors, err := r.embeddings.Embed(providerdiag.WithStage(ctx, "embedding"), []string{query}, r.model)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	if len(vectors) != 1 {
		return nil, fmt.Errorf("expected one query vector, got %d", len(vectors))
	}
	results, err := r.store.Query(ctx, vectors[0], topK)
	if err != nil {
		return nil, fmt.Errorf("query store: %w", err)
	}
	return results, nil
}
