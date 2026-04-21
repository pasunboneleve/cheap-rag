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
	"regexp"
	"strconv"
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
	Outcome      string              `json:"outcome"`
	Content      string              `json:"content"`
	Reason       string              `json:"reason"`
	ProviderCode *int                `json:"provider_status,omitempty"`
	Similarity   *float64            `json:"query_similarity,omitempty"`
	Retrieval    []retrievalResponse `json:"retrieval"`
	Statuses     map[string]any      `json:"statuses,omitempty"`
}

type retrievalResponse struct {
	ChunkID    string  `json:"chunk_id"`
	Similarity float64 `json:"similarity"`
	Path       string  `json:"path"`
	Citation   string  `json:"citation"`
}

var apiStatusRe = regexp.MustCompile(`api status\s+(\d+)`)

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
		reason, providerStatus := classifyAskError(err)
		resp := askResponse{
			Outcome:      "refusal",
			Content:      "Sorry, I don't know how to answer this.",
			Reason:       reason,
			ProviderCode: providerStatus,
			Retrieval:    []retrievalResponse{},
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}
	querySimilarity := topSimilarity(out.Retrieved)
	resp := askResponse{
		Outcome:    "answer",
		Content:    out.Answer,
		Reason:     "none",
		Similarity: querySimilarity,
		Retrieval:  toRetrievalResponse(out.Retrieved),
		Statuses: map[string]any{
			"validation_ok": out.Validation.Valid,
		},
	}
	if out.Refused {
		resp.Outcome = "refusal"
		resp.Content = out.RefusalReason
		resp.Reason = "out-of-scope"
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

func topSimilarity(in []types.RetrievalResult) *float64 {
	if len(in) == 0 {
		return nil
	}
	score := in[0].Similarity
	return &score
}

func classifyAskError(err error) (string, *int) {
	msg := strings.ToLower(err.Error())
	status := providerStatusCode(err)
	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(msg, "deadline exceeded") || strings.Contains(msg, "timeout") {
		return "provider-timeout", status
	}
	if status != nil && (*status == http.StatusRequestTimeout || *status == http.StatusGatewayTimeout) {
		return "provider-timeout", status
	}
	return "provider-error", status
}

func providerStatusCode(err error) *int {
	m := apiStatusRe.FindStringSubmatch(err.Error())
	if len(m) != 2 {
		return nil
	}
	n, convErr := strconv.Atoi(m[1])
	if convErr != nil {
		return nil
	}
	return &n
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
