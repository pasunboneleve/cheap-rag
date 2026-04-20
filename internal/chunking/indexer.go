package chunking

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/dmvianna/chatbot-prototype/internal/fsguard"
	"github.com/dmvianna/chatbot-prototype/internal/llm"
	"github.com/dmvianna/chatbot-prototype/internal/store"
)

type Indexer struct {
	guard        *fsguard.Guard
	embeddings   llm.EmbeddingsProvider
	embedModel   string
	citePattern  string
	store        *store.SQLiteStore
	chunkSize    int
	chunkOverlap int
}

func NewIndexer(guard *fsguard.Guard, embeddings llm.EmbeddingsProvider, embedModel string, citationPattern string, st *store.SQLiteStore) *Indexer {
	return &Indexer{
		guard:        guard,
		embeddings:   embeddings,
		embedModel:   embedModel,
		citePattern:  citationPattern,
		store:        st,
		chunkSize:    800,
		chunkOverlap: 120,
	}
}

func (i *Indexer) Run(ctx context.Context) (int, error) {
	var records []store.Record
	err := i.guard.WalkContent(func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".md" && ext != ".txt" {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		rel, err := filepath.Rel(i.guard.ContentRoot(), path)
		if err != nil {
			return fmt.Errorf("rel path: %w", err)
		}
		chunks := SplitText(filepath.ToSlash(rel), string(b), i.chunkSize, i.chunkOverlap)
		if len(chunks) == 0 {
			return nil
		}
		inputs := make([]string, 0, len(chunks))
		for _, c := range chunks {
			inputs = append(inputs, c.Text)
		}
		vectors, err := i.embeddings.Embed(ctx, inputs, i.embedModel)
		if err != nil {
			return fmt.Errorf("embed %s: %w", path, err)
		}
		if len(vectors) != len(chunks) {
			return fmt.Errorf("embedding count mismatch for %s", path)
		}
		for idx := range chunks {
			chunks[idx].Citation = FormatCitation(i.citePattern, chunks[idx], idx)
			records = append(records, store.Record{Chunk: chunks[idx], Vector: vectors[idx]})
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	if err := i.store.ReplaceAll(ctx, records); err != nil {
		return 0, fmt.Errorf("write index: %w", err)
	}
	return len(records), nil
}
