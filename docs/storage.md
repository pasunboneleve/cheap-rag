# Storage

cheap-rag uses SQLite (`modernc.org/sqlite`) with vectors stored as JSON and cosine scoring in Go.

## Design

- storage file: `runtime_root/index.sqlite`
- chunks table: chunk metadata and text
- embeddings table: serialized embedding vectors + norm
- retrieval: full-scan cosine similarity, then top-k selection

## Indexing strategy

- content is chunked deterministically
- each chunk is embedded and written to local SQLite
- index replacement is explicit (reindex writes fresh chunk/embedding state)
- query-time retrieval embeds the question and compares against local vectors

## Tradeoffs

Pros:
- inspectable local file (`runtime_root/index.sqlite`)
- minimal moving parts
- simple migration path to ANN later

Cons:
- full scan query path is slower at larger scales
- no approximate nearest neighbour index yet

This is the intended baseline for cheap, inspectable deployments, not high-scale workloads.
