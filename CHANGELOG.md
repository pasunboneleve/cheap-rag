# Changelog

All notable changes to `cheap-rag` will be recorded in this file.

## [Unreleased]

## [0.1.0] - 2026-04-21

### Added
- Standalone cheap-rag CLI with `index`, `ask`, `shell`, `inspect query`, and
  unix-socket `serve` commands.
- Split provider abstraction for generation and embeddings, with support for
  Gemini, OpenAI-compatible, xAI generation, and Anthropic generation.
- Retrieval-gated answering with refusal paths and inspectable retrieval output.
- Unix-socket HTTP API (`/ask`, `/healthz`) with optional internal Bearer token
  auth and provider status reporting.
- Concurrency hardening for `/ask` including in-flight request caps and request
  body size limits.
- Focused retry handling for transient provider failures (`429`, `5xx`,
  timeout-like transport errors).
- Local SQLite-backed index and runtime filesystem boundary enforcement.

### Changed
- Module path moved to `github.com/pasunboneleve/cheap-rag`.
