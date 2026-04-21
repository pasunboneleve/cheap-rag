# Security

cheap-rag is designed for local-only serving with explicit filesystem boundaries.

## Transport and socket

- no TCP listener is opened
- server binds only to the configured Unix socket path
- socket file is recreated on startup and chmod'd to `0660`
- listener is closed and socket file removed on shutdown

## Internal auth

If `internal_token` is set, requests must include:

`Authorization: Bearer <token>`

Otherwise requests are rejected with `401`.

## Filesystem boundaries

- content is read only from `content_root`
- runtime files are written only under `runtime_root`
- runtime root cannot be nested under content root
- path traversal attempts are rejected
- symlink escapes outside roots are rejected

## Error handling boundary

HTTP responses are machine-readable and avoid exposing internal stack traces.
Detailed failure context is logged server-side.
