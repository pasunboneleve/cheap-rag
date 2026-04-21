# API

cheap-rag serves HTTP over a Unix domain socket only.

`runtime.socket_path` config controls where the socket is bound.

## Endpoints

- `GET /healthz` -> `200 OK`
- `POST /ask`

## /ask request

```json
{
  "question": "how can I build software that is cheap to change?"
}
```

## /ask response schema

```json
{
  "outcome": "answer | refusal",
  "content": "string",
  "reason": "out-of-scope | provider-timeout | provider-error | null",
  "query_similarity": 0.73,
  "provider_statuses": {"embedding": 200, "generation": 200},
  "retrieval": [
    {"chunk_id":"chunk_1","similarity":0.73,"path":"post.md","citation":"my-post-slug"}
  ]
}
```

Notes:

- `reason` is `null` on successful answers.
- `provider_statuses` carries provider HTTP responses when available, including success (for example `{"embedding":200,"generation":200}`) and failure diagnostics (for example `{"embedding":401}` or `{"generation":504}`).
- refusal reasons include `out-of-scope`, `provider-timeout`, and `provider-error`.

## Concurrency and retry behaviour

- `/ask` is bounded by `server.max_inflight_requests` to avoid unbounded concurrent work.
- `/ask` request bodies are capped by `server.max_request_body_bytes`.
- Provider HTTP calls retry once on transient failures (`429`, `5xx`, or clear transport timeouts), with a short jittered backoff.
- Provider HTTP calls do not retry `400`, `401`, or `403`.

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
