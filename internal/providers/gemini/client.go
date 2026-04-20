package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/dmvianna/chatbot-prototype/internal/llm"
)

type Client struct {
	httpClient *http.Client
	apiKey     string
	baseURL    string
}

func NewClient() *Client {
	base := os.Getenv("GEMINI_BASE_URL")
	if base == "" {
		base = "https://generativelanguage.googleapis.com/v1beta"
	}
	return &Client{
		httpClient: &http.Client{Timeout: 45 * time.Second},
		apiKey:     os.Getenv("GEMINI_API_KEY"),
		baseURL:    strings.TrimRight(base, "/"),
	}
}

func (c *Client) Name() string { return "gemini" }

func (c *Client) Embed(ctx context.Context, texts []string, model string) ([][]float64, error) {
	if c.apiKey == "" {
		return nil, errors.New("GEMINI_API_KEY is required")
	}
	out := make([][]float64, 0, len(texts))
	for _, t := range texts {
		body := map[string]any{
			"model": "models/" + model,
			"content": map[string]any{
				"parts": []map[string]string{{"text": t}},
			},
		}
		u := fmt.Sprintf("%s/models/%s:embedContent?key=%s", c.baseURL, model, url.QueryEscape(c.apiKey))
		var resp struct {
			Embedding struct {
				Values []float64 `json:"values"`
			} `json:"embedding"`
			Error any `json:"error"`
		}
		if err := c.postJSON(ctx, u, body, &resp); err != nil {
			return nil, err
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("gemini embeddings API error: %v", resp.Error)
		}
		out = append(out, resp.Embedding.Values)
	}
	return out, nil
}

func (c *Client) Generate(ctx context.Context, req llm.GenerationRequest) (llm.GenerationResponse, error) {
	if c.apiKey == "" {
		return llm.GenerationResponse{}, errors.New("GEMINI_API_KEY is required")
	}
	prompt := buildPrompt(req)
	body := map[string]any{
		"contents": []map[string]any{{
			"parts": []map[string]string{{"text": prompt}},
		}},
		"generationConfig": map[string]any{
			"temperature": 0.1,
		},
		"systemInstruction": map[string]any{
			"parts": []map[string]string{{"text": req.SystemPolicy}},
		},
	}
	u := fmt.Sprintf("%s/models/%s:generateContent?key=%s", c.baseURL, req.Model, url.QueryEscape(c.apiKey))
	var resp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		Error any `json:"error"`
	}
	if err := c.postJSON(ctx, u, body, &resp); err != nil {
		return llm.GenerationResponse{}, err
	}
	if resp.Error != nil {
		return llm.GenerationResponse{}, fmt.Errorf("gemini generation API error: %v", resp.Error)
	}
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return llm.GenerationResponse{}, errors.New("gemini response has no text candidate")
	}
	raw := strings.TrimSpace(resp.Candidates[0].Content.Parts[0].Text)
	parsed, err := parseJSONOutput(raw)
	if err != nil {
		return llm.GenerationResponse{}, err
	}
	parsed.RawText = raw
	return parsed, nil
}

func buildPrompt(req llm.GenerationRequest) string {
	var sb strings.Builder
	sb.WriteString("Question:\n")
	sb.WriteString(req.Question)
	sb.WriteString("\n\nEvidence chunks:\n")
	for _, c := range req.Evidence {
		sb.WriteString("- ID: ")
		sb.WriteString(c.ID)
		sb.WriteString(" | Path: ")
		sb.WriteString(c.Path)
		sb.WriteString("\n")
		sb.WriteString(c.Text)
		sb.WriteString("\n\n")
	}
	sb.WriteString(`Output JSON only: {"answer":"...","citations":["chunk_id"]}`)
	return sb.String()
}

func parseJSONOutput(raw string) (llm.GenerationResponse, error) {
	re := regexp.MustCompile(`\{[\s\S]*\}`)
	jsonPart := re.FindString(raw)
	if jsonPart == "" {
		return llm.GenerationResponse{}, fmt.Errorf("model output missing JSON object")
	}
	var parsed llm.GenerationResponse
	if err := json.Unmarshal([]byte(jsonPart), &parsed); err != nil {
		return llm.GenerationResponse{}, fmt.Errorf("parse model JSON output: %w", err)
	}
	parsed.Answer = strings.TrimSpace(parsed.Answer)
	if parsed.Answer == "" {
		return llm.GenerationResponse{}, errors.New("empty answer")
	}
	return parsed, nil
}

func (c *Client) postJSON(ctx context.Context, endpoint string, body any, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal body: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	res, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if res.StatusCode >= 300 {
		return fmt.Errorf("api status %d: %s", res.StatusCode, strings.TrimSpace(string(b)))
	}
	if err := json.Unmarshal(b, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
