package xai

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
	"github.com/dmvianna/cheap-rag/internal/netretry"
	"github.com/dmvianna/cheap-rag/internal/providerdiag"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
}

func NewClient() *Client {
	base := os.Getenv("XAI_BASE_URL")
	if base == "" {
		base = "https://api.x.ai/v1"
	}
	return &Client{
		httpClient: &http.Client{Timeout: 45 * time.Second},
		baseURL:    strings.TrimRight(base, "/"),
		apiKey:     os.Getenv("XAI_API_KEY"),
	}
}

func (c *Client) Name() string { return "xai" }

func (c *Client) Embed(ctx context.Context, texts []string, model string) ([][]float64, error) {
	if c.apiKey == "" {
		return nil, errors.New("XAI_API_KEY is required")
	}
	reqBody := map[string]any{"model": model, "input": texts}
	var res struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
		Error any `json:"error"`
	}
	if err := c.postJSON(ctx, c.baseURL+"/embeddings", reqBody, &res); err != nil {
		return nil, err
	}
	if res.Error != nil {
		return nil, fmt.Errorf("embeddings API error: %v", res.Error)
	}
	out := make([][]float64, 0, len(res.Data))
	for _, row := range res.Data {
		out = append(out, row.Embedding)
	}
	return out, nil
}

func (c *Client) Generate(ctx context.Context, req llm.GenerationRequest) (llm.GenerationResponse, error) {
	if c.apiKey == "" {
		return llm.GenerationResponse{}, errors.New("XAI_API_KEY is required")
	}
	prompt := buildPrompt(req)
	body := map[string]any{
		"model": req.Model,
		"messages": []map[string]string{
			{"role": "system", "content": req.SystemPolicy},
			{"role": "user", "content": prompt},
		},
		"temperature": req.Temperature,
	}
	var resp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error any `json:"error"`
	}
	if err := c.postJSON(ctx, c.baseURL+"/chat/completions", body, &resp); err != nil {
		return llm.GenerationResponse{}, err
	}
	if resp.Error != nil {
		return llm.GenerationResponse{}, fmt.Errorf("generation API error: %v", resp.Error)
	}
	if len(resp.Choices) == 0 {
		return llm.GenerationResponse{}, errors.New("generation response has no choices")
	}
	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	parsed, err := parseJSONOutput(content)
	if err != nil {
		return llm.GenerationResponse{}, err
	}
	parsed.RawText = content
	return parsed, nil
}

func (c *Client) postJSON(ctx context.Context, url string, body any, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal body: %w", err)
	}
	for attempt := 0; attempt < netretry.MaxAttempts(); attempt++ {
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			return fmt.Errorf("build request: %w", err)
		}
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
		httpReq.Header.Set("Content-Type", "application/json")
		res, err := c.httpClient.Do(httpReq)
		if err != nil {
			if attempt == 0 && netretry.ShouldRetryTransport(err) && ctx.Err() == nil {
				if sleepErr := netretry.SleepWithContext(ctx, netretry.Backoff(attempt, nil)); sleepErr == nil {
					continue
				}
			}
			return fmt.Errorf("http request: %w", err)
		}
		providerdiag.RecordStatus(ctx, res.StatusCode)
		b, readErr := io.ReadAll(io.LimitReader(res.Body, 2<<20))
		_ = res.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read response: %w", readErr)
		}
		if res.StatusCode >= 300 {
			if attempt == 0 && netretry.ShouldRetryStatus(res.StatusCode) && ctx.Err() == nil {
				if sleepErr := netretry.SleepWithContext(ctx, netretry.Backoff(attempt, nil)); sleepErr == nil {
					continue
				}
			}
			return fmt.Errorf("api status %d: %s", res.StatusCode, strings.TrimSpace(string(b)))
		}
		if err := json.Unmarshal(b, out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
		return nil
	}
	return errors.New("request failed after retry")
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
