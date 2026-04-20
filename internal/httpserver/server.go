package httpserver

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dmvianna/cheap-rag/internal/types"
)

type Asker interface {
	Ask(ctx context.Context, question string) (types.AskOutcome, error)
}

type Server struct {
	asker Asker
	token string
	log   *log.Logger
}

type askRequest struct {
	Question string `json:"question"`
}

type askResponse struct {
	Answer    string              `json:"answer"`
	Refusal   *string             `json:"refusal"`
	Retrieval []retrievalResponse `json:"retrieval"`
}

type retrievalResponse struct {
	ChunkID    string  `json:"chunk_id"`
	Similarity float64 `json:"similarity"`
	Path       string  `json:"path"`
	Citation   string  `json:"citation"`
}

func New(asker Asker, token string, logger *log.Logger) *Server {
	return &Server{
		asker: asker,
		token: strings.TrimSpace(token),
		log:   logger,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/ask", s.handleAsk)
	return s.requestLogMiddleware(mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) handleAsk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.authorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	var req askRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
		return
	}
	question := strings.TrimSpace(req.Question)
	if question == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "question is required"})
		return
	}
	out, err := s.asker.Ask(r.Context(), question)
	if err != nil {
		s.log.Printf("ask error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	resp := askResponse{
		Answer:    out.Answer,
		Refusal:   nil,
		Retrieval: toRetrievalResponse(out.Retrieved),
	}
	if out.Refused {
		resp.Answer = ""
		ref := out.RefusalReason
		resp.Refusal = &ref
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) authorized(r *http.Request) bool {
	if s.token == "" {
		return true
	}
	h := strings.TrimSpace(r.Header.Get("Authorization"))
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) {
		return false
	}
	got := strings.TrimSpace(strings.TrimPrefix(h, prefix))
	return subtle.ConstantTimeCompare([]byte(got), []byte(s.token)) == 1
}

func (s *Server) requestLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.log.Printf("request %s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func toRetrievalResponse(in []types.RetrievalResult) []retrievalResponse {
	out := make([]retrievalResponse, 0, len(in))
	for _, r := range in {
		cite := strings.TrimSpace(r.Chunk.Citation)
		if cite == "" {
			cite = r.Chunk.ID
		}
		out = append(out, retrievalResponse{
			ChunkID:    r.Chunk.ID,
			Similarity: r.Similarity,
			Path:       r.Chunk.Path,
			Citation:   cite,
		})
	}
	return out
}

func ListenUnixSocket(socketPath string) (net.Listener, func() error, error) {
	if strings.TrimSpace(socketPath) == "" {
		return nil, nil, errors.New("runtime.socket_path is required")
	}
	if err := os.MkdirAll(filepath.Dir(socketPath), 0o755); err != nil {
		return nil, nil, fmt.Errorf("create socket dir: %w", err)
	}
	if err := os.Remove(socketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, nil, fmt.Errorf("remove existing socket: %w", err)
	}
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, nil, fmt.Errorf("listen unix socket: %w", err)
	}
	if err := os.Chmod(socketPath, 0o660); err != nil {
		_ = ln.Close()
		return nil, nil, fmt.Errorf("chmod socket: %w", err)
	}
	var once sync.Once
	cleanup := func() error {
		var cleanupErr error
		once.Do(func() {
			if err := ln.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				cleanupErr = err
			}
			if err := os.Remove(socketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
				if cleanupErr == nil {
					cleanupErr = err
				}
			}
		})
		return cleanupErr
	}
	return ln, cleanup, nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
