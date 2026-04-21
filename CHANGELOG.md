# Changelog

All notable changes to `cheap-rag` will be recorded in this file.

## [Unreleased]

## [0.2.0] - 2026-04-21

### Added
- Repository automation baseline:
  - Linux and macOS CI workflows
  - Linux and macOS release workflows plus reusable release workflow
  - release notes extraction script
  - release schedule documentation
  - repository hooks integration via `.envrc` and `hooks/`
- Expanded documentation set under `docs/`:
  - architecture
  - config
  - API
  - storage
  - security
  - validation
- README CI badges for Linux and macOS workflows.

### Changed
- Refactored the top-level README into a high-signal quick overview, with
  detailed reference content moved to `docs/`.

### Fixed
- Hardened unix-socket listener test portability on macOS by shortening the
  test socket path in `TestListenUnixSocketCleansUp`.

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
