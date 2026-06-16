# ADR 0005: Local Compact via `instructions` Forwarding in ResToChat

## Status
Accepted

## Context

The gateway must support Codex CLI compaction to prevent context window exhaustion during long coding sessions. Codex CLI implements three compact strategies, but only one applies when the gateway is in use:

| Strategy | Endpoint | Streaming | When Used |
|----------|----------|-----------|-----------|
| Local Compact | `POST /v1/responses` | SSE stream | Provider is NOT OpenAI/Azure |
| Remote Compact v1 | `POST /v1/responses/compact` | Unary | Provider IS OpenAI/Azure, feature flag disabled |
| Remote Compact v2 | `POST /v1/responses` | SSE stream | Provider IS OpenAI/Azure, feature flag enabled |

The strategy is selected in `codex-rs/core/src/session/turn.rs:905`:

```rust
if should_use_remote_compact_task(turn_context.provider.info()) {
    if turn_context.features.enabled(Feature::RemoteCompactionV2) {
        run_remote_compact_v2()  // → /v1/responses (streaming, CompactionTrigger+Compaction)
    } else {
        run_remote_compact_v1()  // → /v1/responses/compact (unary)
    }
} else {
    run_local_compact()          // → /v1/responses (streaming, instructions=SUMMARIZATION_PROMPT)
}
```

`supports_remote_compaction()` (`codex-rs/model-provider-info/src/lib.rs:399`) returns true only for OpenAI and Azure providers. When Codex CLI is pointed at the gateway via `/v1/codex-config`, the provider is not OpenAI or Azure — so Codex CLI uses **local compact** via `POST /v1/responses`.

### How local compact works

1. Codex CLI synthesizes a summarization prompt (`SUMMARIZATION_PROMPT`) and places it in the `instructions` field of a standard Responses API request
2. Sends the full conversation history as `input` items
3. Streams the response from `/v1/responses`
4. Extracts the assistant's text from the stream, prepends `SUMMARY_PREFIX`, and builds compacted history client-side

The core function is `drain_to_completed()` in `codex-rs/core/src/compact.rs:608`.

### The gap

The gateway already routes `POST /v1/responses` through `handleProxy` → translator pipeline. For Chat-only providers (e.g., DeepSeek via OpenRouter), the `ResToChat` translator converts Responses API requests to Chat Completions. But `ResToChat.TranslateRequest()` (`pkg/translator/res_to_chat.go:18`) builds the Chat messages array from `body.Input.Items` and `body.Tools`, while **never reading `body.Instructions`**. The `instructions` field — a standard Responses API field already parsed by the schema (`pkg/schema/responses/responses.go:18-19`) — is silently dropped.

This means the summarization prompt never reaches the upstream Chat model. The model receives conversation history but no instruction to summarize it, producing a generic response instead of a structured handoff summary.

## Decision

**Fix `ResToChat.TranslateRequest()` to forward `body.Instructions` as a system message**, rather than creating a new endpoint, format, or translator.

### Why this approach

- The existing `handleProxy` pipeline already handles model resolution, provider selection with session affinity, buffered logging, request IDs, and error handling. Local compact requests go through this pipeline naturally — they are standard Responses API requests arriving at `POST /v1/responses`.
- The only missing piece is the `instructions` field. Adding it as a system message in the Chat messages array is a one-line change in two code paths.
- No new API format (`FormatCompact`), translator (`CompactToChatTranslator`), or shared function extraction (`ConvertInputItems`) is needed. The existing `ResToChat` translator already converts input items, tools, and reasoning correctly. Only `instructions` is missing.

### Implementation

In both `TranslateRequest` and `rebuildMessages`:

```go
if body.Instructions != nil && *body.Instructions != "" {
    msgs = append([]chat.ChatCompletionMessage{{
        Role:    "system",
        Content: &chat.ChatCompletionMessageContent{String: body.Instructions},
    }}, msgs...)
}
```

- **`sessionMessages` path**: When a session with prior messages exists, prepend `instructions` as system message before the session messages.
- **Item-by-item `rebuildMessages` path**: When no session exists, prepend `instructions` as system message before the converted input items.

`TranslateStream` and `TranslateResponse` need no changes — local compact is streaming, and the existing SSE translation wraps Chat delta events into Responses SSE events correctly.

### What stays the same

- `ServeCompact` (`pkg/ingress/gateway.go:101`) remains as-is for now — it handles remote compact v1 for providers with native Responses endpoints.
- Session affinity already works: `handleProxy` extracts `X-Session-Id` and passes it to `ProviderSelector.Select()`.
- Buffered logging and request IDs already work: all `handleProxy` paths get them.

## Alternatives Considered

### New `FormatCompact` + `CompactToChatTranslator` + `/v1/responses/compact` pipeline (original proposal)

Rejected for the immediate fix: local compact does not use `/v1/responses/compact` — it uses `/v1/responses`. Creating a new format and translator for an endpoint Codex CLI doesn't call would add complexity without solving the actual problem. The compact endpoint pipeline may be useful later for remote compact v1 fallback, but it is not required for local compact support.

### Duplicate summarization logic in a standalone handler

Rejected: duplicates model resolution, provider selection, HTTP calling, session affinity, and error handling already present in `handleProxy`.

### Anthropic Messages path for compact

Rejected: Codex CLI has no Anthropic compact concept. `instructions` forwarding in `ResToAnth` is a separate concern.

## Consequences

- **Positive**: Local compact works for all Chat-only providers through the existing pipeline. No new API surface, format, or translator. Codex CLI users on Chat-only providers can run long sessions without context window exhaustion.
- **Positive**: `instructions` forwarding benefits all `/v1/responses` requests, not just compact. The `instructions` field is a standard Responses API parameter for developer-level instructions.
- **Neutral**: Remote compact v1 and v2 remain unsupported through the gateway (only passthrough for providers with native Responses endpoints). This is acceptable because Codex CLI does not use remote compact against the gateway.
- **Neutral**: `ResToChat.rebuildMessages` is not refactored to a shared `ConvertInputItems` — the existing inline conversion works correctly and extraction is deferred until needed by a second consumer.
