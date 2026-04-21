# Architecture

cheap-rag keeps core logic explicit and local.

## Package breakdown

- `cmd/chatbot`: thin CLI edge for `index`, `ask`, `shell`, `inspect query`, `serve`
- `internal/config`: YAML config with explicit thresholds and split generation/embedding provider settings
- `internal/fsguard`: hard boundary enforcement between `content_root` (read-only) and `runtime_root` (write-only for chatbot state)
- `internal/chunking`: deterministic chunking and indexing orchestration
- `internal/providers`: provider interfaces and implementations (`gemini`, `openai-compatible`, `xai`, `anthropic`) for embeddings + generation
- `internal/store`: SQLite-backed local vector store (inspectable, local, boring)
- `internal/retrieval`: embed query and fetch top-k by cosine similarity
- `internal/policy`: practical answer validation against retrieved evidence
- `internal/chatbot`: orchestration service that applies retrieval gate + generation + validation
- `internal/httpserver`: local-only HTTP-over-unix-socket server (`/healthz`, `/ask`)
- `internal/repl`: interactive shell
- `examples/unix_client`: minimal HTTP-over-unix-socket Go client

## Orchestration flow

Pipeline:

`embed -> retrieve -> gate -> generate -> validate`

1. Embed: the question is embedded with the configured embeddings provider/model.
2. Retrieve: top-k chunks are retrieved from local indexed content.
3. Gate: if retrieval is empty or similarity is below threshold, answer is refused.
4. Generate: model is asked to answer using retrieved evidence only.
5. Validate: cheap checks run against retrieved evidence before answer is accepted.

## Runtime boundaries

- Reads are constrained to `content_root`.
- Writes are constrained to `runtime_root`.
- Index state lives under `runtime_root/index.sqlite`.
- HTTP serving is local-only via Unix socket (`runtime.socket_path`).
