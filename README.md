# cheap-rag
[![Linux CI](https://github.com/pasunboneleve/cheap-rag/actions/workflows/linux-ci.yml/badge.svg)](https://github.com/pasunboneleve/cheap-rag/actions/workflows/linux-ci.yml)
[![macOS CI](https://github.com/pasunboneleve/cheap-rag/actions/workflows/macos-ci.yml/badge.svg)](https://github.com/pasunboneleve/cheap-rag/actions/workflows/macos-ci.yml)

**cheap-rag** is a deliberately simple, low-cost RAG implementation. It
answers only when local content is sufficiently similar to the
question. Otherwise, it refuses.

This is the intended shape for this project.
Not designed for scale. When scale becomes a bottleneck, replace it.

## Why this exists

This project keeps deterministic guardrails in front of a stochastic model.
Retrieval decides whether the model is allowed to speak.

If local evidence is out of scope, cheap-rag refuses.
If local evidence is in scope, cheap-rag answers from that evidence.

## Architecture

Pipeline:

`embed -> retrieve -> gate -> generate -> validate`

- embed: turn the question into a vector
- retrieve: fetch top-k local chunks by similarity
- gate: refuse if similarity/evidence is insufficient
- generate: answer only from retrieved chunks
- validate: lightweight checks for evidence coverage/support

## Quick start

```bash
go run ./cmd/chatbot index --config ./chatbot.example.yaml
go run ./cmd/chatbot shell --config ./chatbot.example.yaml
go run ./cmd/chatbot serve --config ./chatbot.example.yaml
go run ./cmd/chatbot ask --config ./chatbot.example.yaml "what is cheap to change?"
go run ./cmd/chatbot inspect query --config ./chatbot.example.yaml "ci cd"
```

## API example

When running `serve`, cheap-rag listens on `runtime.socket_path` using HTTP over a Unix domain socket only.

Success:

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

Refusal:

```json
{
  "outcome": "refusal",
  "content": "Sorry, I don't know how to answer this.",
  "reason": "out-of-scope",
  "query_similarity": 0.18,
  "retrieval": []
}
```

## Key ideas

- retrieval is the gatekeeper: generation happens only after retrieval passes scope checks
- validation is intentionally limited: useful heuristics, not a formal truth system
- local + inspectable beats scalable-by-default for this project

## Documentation

- [Architecture](docs/architecture.md)
- [Configuration](docs/config.md)
- [API](docs/api.md)
- [Storage](docs/storage.md)
- [Security](docs/security.md)
- [Validation](docs/validation.md)
- [Release schedule](docs/release-schedule.md)

## Security model

- no TCP listener is opened
- server binds only to the configured Unix socket path
- socket file is recreated on startup and chmod'd to `0660`
- if `internal_token` is set, requests must include `Authorization: Bearer <token>`

See [docs/security.md](docs/security.md) for full details.
