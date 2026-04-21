# cheap-rag

**cheap-rag** is a deliberately simple, low-cost RAG implementation. It
answers only when local content is sufficiently similar to the
question. Otherwise, it refuses.

Not designed for scale. When scale becomes a bottleneck, replace it.

## Architecture overview

- `cmd/chatbot`: thin CLI edge for `index`, `ask`, `shell`, `inspect query`
- `internal/config`: YAML config with explicit thresholds and split generation/embedding provider settings
- `internal/fsguard`: hard boundary enforcement between `content_root` (read-only) and `runtime_root` (write-only for chatbot state)
- `internal/chunking`: deterministic chunking and indexing orchestration
- `internal/providers`: provider interfaces and implementations (`gemini`, `openai-compatible`, `xai`, `anthropic`) for embeddings + generation
- `internal/store`: SQLite-backed local vector store (inspectable, local, boring)
- `internal/retrieval`: embed query and fetch top-k by cosine similarity
- `internal/policy`: practical v1 answer validation against retrieved evidence
- `internal/chatbot`: orchestration service that applies retrieval gate + generation + validation
- `internal/httpserver`: local-only HTTP-over-unix-socket server (`/healthz`, `/ask`)
- `internal/repl`: interactive shell

## Why retrieval is the gatekeeper

The model is only allowed to answer if retrieval passes a deterministic threshold (`retrieval.min_query_similarity`).

- If top similarity is below threshold: refuse out-of-scope.
- If retrieval has no results: refuse out-of-scope.
- Generation happens only after retrieval passes.

This keeps stochastic generation behind a deterministic scope gate.

## Why validation is limited in v1

Validation in v1 is intentionally simple and inspectable:

- checks claim-token overlap against cited evidence text
- checks unsupported named entities against evidence text
- requires citations to map to retrieved chunk IDs
- applies `validation.min_evidence_coverage`

This is not a formal truth verifier. It reduces obvious unsupported claims while keeping complexity low.

## Vector store choice and tradeoffs

cheap-rag uses SQLite (`modernc.org/sqlite`) with vectors stored as JSON and cosine scoring in Go.

Pros:
- inspectable local file (`runtime_root/index.sqlite`)
- minimal moving parts
- simple migration path to ANN later

Cons:
- full scan query path is slower at larger scales
- no approximate nearest neighbour index yet

This is the intended baseline for cheap, inspectable deployments, not high-scale workloads.

## Configuration

Example: [`chatbot.example.yaml`](chatbot.example.yaml)

Required fields:
- `content_root`
- `runtime_root`
- `runtime.socket_path` (unix socket path for `serve`)
- `internal_token` (optional Bearer token for local internal auth)
- `generation_provider`
- `embedding_provider`
- `model`
- `generation_temperature` (optional, 0-2, default `0.4`)
  - Note: when `generation_provider: anthropic`, temperature must be `<= 1.0`.
- `embedding_model`
- `citation_pattern` (optional, default `{chunk_id}`)
- `responses.refusal.no_retrieval` (optional seed sentence for refusal rephrasing)
- `responses.refusal.low_similarity` (optional seed sentence for refusal rephrasing)
- `retrieval.top_k`
- `retrieval.min_query_similarity`
- `validation.min_evidence_coverage`

Backward compatibility:
- Legacy `provider` is still accepted and applied to both generation and embeddings.

Citation pattern placeholders:
- `{chunk_id}` internal stable chunk ID
- `{path}` relative content path
- `{slug}` filename without extension
- `{chunk_index}` index of chunk inside its source file

API keys:
- Gemini: `GEMINI_API_KEY`
- OpenAI-compatible: `OPENAI_API_KEY` (and optional `OPENAI_BASE_URL`)
- xAI: `XAI_API_KEY` (and optional `XAI_BASE_URL`, default `https://api.x.ai/v1`)
- Anthropic (generation): `ANTHROPIC_API_KEY` (optional `ANTHROPIC_BASE_URL`, `ANTHROPIC_VERSION`)

## Run

```bash
go run ./cmd/chatbot index --config ./chatbot.example.yaml
go run ./cmd/chatbot shell --config ./chatbot.example.yaml
go run ./cmd/chatbot serve --config ./chatbot.example.yaml
go run ./cmd/chatbot ask --config ./chatbot.example.yaml "what is cheap to change?"
go run ./cmd/chatbot inspect query --config ./chatbot.example.yaml "ci cd"
```

## Unix socket API

When running `serve`, cheap-rag listens on `runtime.socket_path` using HTTP over a Unix domain socket only.

Endpoints:
- `GET /healthz` -> `200 OK`
- `POST /ask`

Request body:

```json
{
  "question": "how can I build software that is cheap to change?"
}
```

Response body:

```json
{
  "outcome": "answer",
  "content": "Use short feedback loops and explicit boundaries...",
  "reason": null,
  "query_similarity": 0.73,
  "provider_statuses": {"embedding": 200, "generation": 200},
  "retrieval": [
    {"chunk_id":"chunk_1","similarity":0.73,"path":"post.md","citation":"my-post-slug"}
  ]
}
```

Refusal example:

```json
{
  "outcome": "refusal",
  "content": "Sorry, I don't know how to answer this.",
  "reason": "out-of-scope",
  "query_similarity": 0.18,
  "retrieval": []
}
```

`reason` is `null` on successful answers. Refusal reasons include `out-of-scope`, `provider-timeout`, and `provider-error`.

`provider_statuses` carries provider HTTP responses when available, including success (for example `{"embedding":200,"generation":200}`) and failure diagnostics (for example `{"embedding":401}` or `{"generation":504}`).

## Security model

- No TCP listener is opened.
- Server binds only to the configured Unix socket path.
- Socket file is recreated on startup and chmod'd to `0660`.
- If `internal_token` is set, requests must include `Authorization: Bearer <token>`.

You can also override config fields with flags:

```bash
go run ./cmd/chatbot shell \
  --content ./content \
  --runtime ./.chatbot \
  --generation-provider gemini \
  --embedding-provider gemini \
  --model gemini-2.0-flash \
  --generation-temperature 0.5 \
  --embedding-model gemini-embedding-001
```

xAI example:

```bash
export XAI_API_KEY=...
go run ./cmd/chatbot shell \
  --content ./content \
  --runtime ./.chatbot \
  --generation-provider xai \
  --embedding-provider gemini \
  --model grok-4-0709 \
  --embedding-model gemini-embedding-001
```

Anthropic generation + OpenAI embeddings example:

```bash
export ANTHROPIC_API_KEY=...
export OPENAI_API_KEY=...
go run ./cmd/chatbot shell \
  --content ./content \
  --runtime ./.chatbot \
  --generation-provider anthropic \
  --embedding-provider openai-compatible \
  --model claude-sonnet-4-5 \
  --embedding-model text-embedding-3-small
```

Blog slug citation example:

```yaml
citation_pattern: "{slug}"
```

## Guardrail behaviour examples

Sample refusal message:

```text
Sorry, but I could not relate your question to the content I have.
```

Sample grounded answer shape:

```json
{
  "answer": "Cheap change is mainly about reducing coupling and using explicit interfaces so changes stay local.",
  "citations": ["chunk_123abc"]
}
```

## Path and filesystem safety

- content is read only from `content_root`
- runtime files are written only under `runtime_root`
- runtime root cannot be nested under content root
- path traversal attempts are rejected
- symlink escapes outside roots are rejected

## Go client example (unix socket)

```go
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

func main() {
	socketPath := "/tmp/cheap-rag.sock"
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
			},
		},
		Timeout: 15 * time.Second,
	}
	body := []byte(`{"question":"what is cheap to change?"}`)
	req, _ := http.NewRequest(http.MethodPost, "http://unix/ask", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// req.Header.Set("Authorization", "Bearer <internal_token>")
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	fmt.Println(string(b))
}
```

## Current scope and stubs

Implemented:
- CLI scaffold with index/ask/shell/inspect
- provider abstractions and two concrete providers
- retrieval threshold refusal
- deterministic post-generation validation
- SQLite local vector store
- tests for path boundaries, out-of-scope refusal, retrieval threshold refusal, validation refusal

Still minimal in v1:
- no ANN index
- no incremental indexing (current index replace is full rebuild)
- simplistic claim/entity heuristics for validation
- no structured logging pipeline yet
