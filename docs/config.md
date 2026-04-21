# Configuration

Example config: [`chatbot.example.yaml`](../chatbot.example.yaml)

## Required fields

- `content_root`
- `runtime_root`
- `runtime.socket_path` (unix socket path for `serve`)
- `server.max_inflight_requests` (max concurrent `/ask` requests; saturation returns `503`)
- `server.max_request_body_bytes` (max `/ask` request payload size; oversized body returns `413`)
- `internal_token` (optional Bearer token for local internal auth)
- `generation_provider`
- `embedding_provider`
- `model`
- `embedding_model`
- `retrieval.top_k`
- `retrieval.min_query_similarity`
- `validation.min_evidence_coverage`

## Optional fields

- `generation_temperature` (0-2, default `0.4`)
  - when `generation_provider: anthropic`, temperature must be `<= 1.0`
- `citation_pattern` (default `{chunk_id}`)
- `responses.refusal.no_retrieval`
- `responses.refusal.low_similarity`

## Backward compatibility

Legacy `provider` is still accepted and applied to both generation and embeddings.

## Provider settings

- `generation_provider` picks the chat model backend.
- `embedding_provider` picks the embedding backend.
- `model` is the generation model name.
- `embedding_model` is the embedding model name.

## Environment variables

- Gemini: `GEMINI_API_KEY`
- OpenAI-compatible: `OPENAI_API_KEY` (optional `OPENAI_BASE_URL`)
- xAI: `XAI_API_KEY` (optional `XAI_BASE_URL`, default `https://api.x.ai/v1`)
- Anthropic (generation): `ANTHROPIC_API_KEY` (optional `ANTHROPIC_BASE_URL`, `ANTHROPIC_VERSION`)

## Citation patterns

Placeholders:

- `{chunk_id}` internal stable chunk ID
- `{path}` relative content path
- `{slug}` filename without extension
- `{chunk_index}` index of chunk inside its source file

Example:

```yaml
citation_pattern: "{slug}"
```

## CLI overrides

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

xAI generation + Gemini embeddings:

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

Anthropic generation + OpenAI embeddings:

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
