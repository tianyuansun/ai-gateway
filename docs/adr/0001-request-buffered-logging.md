# ADR 0001: Per-Request Buffered Logging with Trigger Flush

## Status
Accepted

## Context

The gateway currently uses ad-hoc `log.Printf` calls that provide no structured context, no request tracing, and no ability to gate verbosity. When a request fails or is slow, operators need a full trace of every decision the gateway made for that specific request — model resolution, provider selection, translator choice, upstream timing, etc.

We evaluated two paths: (a) always-on structured logging to a central sink, and (b) per-request buffered logging that is only emitted when a trigger fires. Option (a) produces too much noise for healthy traffic; option (b) gives targeted traces only when needed.

## Decision

Implement per-request buffered logging using Go's `log/slog` (standard library, available since Go 1.21).

### Architecture
- A custom `slog.Handler` wraps a per-request ring buffer.
- The buffer is carried in `context.Context`; all handler functions write log records via `slog.DebugContext(ctx, msg, attrs...)`.
- At request end, a post-handler check evaluates *trigger conditions*. If any fires, the buffer is flushed to `os.Stderr` (configurable `io.Writer`). Otherwise, the buffer is discarded.

### Trigger conditions
| Condition | Threshold |
|-----------|-----------|
| Upstream error | status >= 500 or client call returns error |
| Latency | total request latency > configurable threshold (default 5s) |
| Explicit debug | `X-Debug: true` request header |
| Client cancel | `ctx.Err() != nil` |

### Log levels
- Global default via `gateway.yaml` (`server.log_level`), runtime override via `PUT /admin/log-level`.
- Request-level override: `X-Debug: true` forces `debug` for that request.
- Standard slog levels: `debug`, `info`, `warn`, `error`.

### Log format
JSON Lines — one record per line, all sharing a `request_id`:

```json
{"time":"...","level":"DEBUG","request_id":"gw-abc123","msg":"provider selected","strategy":"priority","provider":"ds-official"}
{"time":"...","level":"ERROR","request_id":"gw-abc123","msg":"upstream error","status":502,"latency_ms":5230}
```

### Dynamic level change
`PUT /admin/log-level` with `{"level": "debug"}` body updates the global level in-memory. Config file reload (future `SIGHUP`) also updates it.

## Alternatives considered

### zap
`go.uber.org/zap` is faster at formatting but requires wrapper libraries for context propagation. `slog` natively integrates context via `slog.DebugContext(ctx, ...)`, which is the critical path for our design. The per-request buffer overhead dominates any formatting speed difference.

### Always-on streaming
Sending all logs to a central sink would require an external dependency (Loki, ELK) and produces too much data for healthy traffic. Triggered flush gives targeted traces without infrastructure overhead.

### Structured text (non-JSON)
Human-readable logs are easier to scan but harder to query with `jq` and log aggregation tools. JSON Lines is the pragmatic middle ground.

## Consequences
- All handler functions gain `slog` calls at key decision points.
- A new `pkg/logging` package implements the custom handler.
- Admin API gains one new endpoint (`/admin/log-level`).
- No change to existing interfaces or data structures.
