package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/dmvianna/chatbot-prototype/internal/types"
)

type SQLiteStore struct {
	db *sql.DB
}

type Record struct {
	Chunk  types.Chunk
	Vector []float64
}

func Open(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	s := &SQLiteStore{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *SQLiteStore) Close() error { return s.db.Close() }

func (s *SQLiteStore) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS chunks (
  id TEXT PRIMARY KEY,
  citation TEXT NOT NULL,
  path TEXT NOT NULL,
  text TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS embeddings (
  chunk_id TEXT PRIMARY KEY,
  vector_json TEXT NOT NULL,
  norm REAL NOT NULL,
  FOREIGN KEY(chunk_id) REFERENCES chunks(id) ON DELETE CASCADE
);
`)
	if err != nil {
		return fmt.Errorf("migrate sqlite: %w", err)
	}
	if err := s.ensureCitationColumn(); err != nil {
		return err
	}
	return nil
}

func (s *SQLiteStore) ensureCitationColumn() error {
	_, err := s.db.Exec(`ALTER TABLE chunks ADD COLUMN citation TEXT NOT NULL DEFAULT ''`)
	if err == nil {
		return nil
	}
	if strings.Contains(strings.ToLower(err.Error()), "duplicate column name") {
		return nil
	}
	return fmt.Errorf("ensure citation column: %w", err)
}

func (s *SQLiteStore) ReplaceAll(ctx context.Context, records []Record) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM embeddings`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM chunks`); err != nil {
		return err
	}
	for _, r := range records {
		if err := insertOne(ctx, tx, r); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func insertOne(ctx context.Context, tx *sql.Tx, r Record) error {
	vecJSON, err := json.Marshal(r.Vector)
	if err != nil {
		return err
	}
	norm := vectorNorm(r.Vector)
	citation := strings.TrimSpace(r.Chunk.Citation)
	if citation == "" {
		citation = r.Chunk.ID
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO chunks(id, citation, path, text) VALUES(?,?,?,?)`, r.Chunk.ID, citation, r.Chunk.Path, r.Chunk.Text); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO embeddings(chunk_id, vector_json, norm) VALUES(?,?,?)`, r.Chunk.ID, string(vecJSON), norm); err != nil {
		return err
	}
	return nil
}

func (s *SQLiteStore) Query(ctx context.Context, query []float64, topK int) ([]types.RetrievalResult, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT c.id, c.citation, c.path, c.text, e.vector_json, e.norm
FROM chunks c
JOIN embeddings e ON e.chunk_id = c.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	qNorm := vectorNorm(query)
	if qNorm == 0 {
		return nil, nil
	}
	var results []types.RetrievalResult
	for rows.Next() {
		var id, citation, path, text, vectorJSON string
		var norm float64
		if err := rows.Scan(&id, &citation, &path, &text, &vectorJSON, &norm); err != nil {
			return nil, err
		}
		var vec []float64
		if err := json.Unmarshal([]byte(vectorJSON), &vec); err != nil {
			return nil, err
		}
		score := cosine(query, qNorm, vec, norm)
		results = append(results, types.RetrievalResult{
			Chunk:      types.Chunk{ID: id, Citation: citation, Path: path, Text: text},
			Similarity: score,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Similarity > results[j].Similarity })
	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}
	return results, nil
}

func vectorNorm(v []float64) float64 {
	var sum float64
	for _, x := range v {
		sum += x * x
	}
	return math.Sqrt(sum)
}

func cosine(a []float64, aNorm float64, b []float64, bNorm float64) float64 {
	if aNorm == 0 || bNorm == 0 || len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot float64
	for i := range a {
		dot += a[i] * b[i]
	}
	return dot / (aNorm * bNorm)
}
