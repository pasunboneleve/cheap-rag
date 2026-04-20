package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/dmvianna/cheap-rag/internal/llm"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
	version    string
}

func NewClient() *Client {
	base := os.Getenv("ANTHROPIC_BASE_URL")
	if base == "" {
		base = "https://api.anthropic.com/v1"
	}
	version := os.Getenv("ANTHROPIC_VERSION")
	if version == "" {
		version = "2023-06-01"
	}
	return &Client{
		httpClient: &http.Client{Timeout: 45 * time.Second},
		baseURL:    strings.TrimRight(base, "/"),
		apiKey:     os.Getenv("ANTHROPIC_API_KEY"),
		version:    version,
	}
}

func (c *Client) Name() string { return "anthropic" }

func (c *Client) Generate(ctx context.Context, req llm.GenerationRequest) (llm.GenerationResponse, error) {
	if c.apiKey == "" {
		return llm.GenerationResponse{}, errors.New("ANTHROPIC_API_KEY is required")
	}
	prompt := buildPrompt(req)
	body := map[string]any{
		"model":       req.Model,
		"max_tokens":  700,
		"temperature": 0.0,
		"system":      req.SystemPolicy,
		"messages": []map[string]any{{
			"role":    "user",
			"content": prompt,
		}},
	}
	var resp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := c.postJSON(ctx, c.baseURL+"/messages", body, &resp); err != nil {
		return llm.GenerationResponse{}, err
	}
	if strings.TrimSpace(resp.Error.Message) != "" {
		return llm.GenerationResponse{}, fmt.Errorf("generation API error (%s): %s", resp.Error.Type, resp.Error.Message)
	}
	if len(resp.Content) == 0 {
		return llm.GenerationResponse{}, errors.New("generation response has no content")
	}
	var rawParts []string
	for _, part := range resp.Content {
		if part.Type != "text" {
			continue
		}
		rawParts = append(rawParts, part.Text)
	}
	raw := strings.TrimSpace(strings.Join(rawParts, "\n"))
	if raw == "" {
		return llm.GenerationResponse{}, errors.New("generation response has no text content")
	}
	parsed, err := parseJSONOutput(raw)
	if err != nil {
		return llm.GenerationResponse{}, err
	}
	parsed.RawText = raw
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
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", c.version)
	httpReq.Header.Set("content-type", "application/json")
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

func buildPrompt(req llm.GenerationRequest) string {
	var sb strings.Builder
	sb.WriteString("Question:\n")
	sb.WriteString(req.Question)
	sb.WriteString("\n\nEvidence chunks:\n")
	for _, c := range req.Evidence {
		sb.WriteString("- ID: ")
		sb.WriteString(c.ID)
		sb.WriteString(" | Cite as: ")
		if strings.TrimSpace(c.Citation) != "" {
			sb.WriteString(c.Citation)
		} else {
			sb.WriteString(c.ID)
		}
		sb.WriteString(" | Path: ")
		sb.WriteString(c.Path)
		sb.WriteString("\n")
		sb.WriteString(c.Text)
		sb.WriteString("\n\n")
	}
	sb.WriteString(`Output JSON only: {"answer":"...","citations":["citation_value"]}`)
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
