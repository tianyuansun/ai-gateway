# ADR 0006: Session Store is Stateless — No Message History or Reasoning Records

## Status
Accepted

## Context

The gateway's `Session` struct currently defines three categories of cross-turn state:
- `Messages` — conversation history (rebuilt from the agent's request body each turn)
- `ReasoningRecords` — reasoning/thinking content extracted from upstream responses
- `ProviderID` / `ModelName` — provider affinity binding

The original design assumed that some clients might send only incremental input per turn (using `previous_response_id` semantics), requiring the gateway to maintain and rebuild full conversation history on their behalf.

### Research findings

A comprehensive survey of all major coding agents found:

| Agent | API Format | Sends Full History? | Uses Server-Side State? |
|-------|-----------|---------------------|------------------------|
| Codex CLI | Responses | Yes | No (deliberately avoids `previous_response_id`) |
| Claude Code | Anthropic Messages | Yes | No |
| Gemini CLI | Gemini `generateContent` | Yes | No |
| GitHub Copilot | Chat / Responses-like | Yes | No (SDK session is lifecycle only) |
| Pi | Multi-provider | Yes | No |
| Aider | Chat Completions | Yes | No |
| Cursor | Chat + Responses hybrid | Yes | No |
| Continue.dev | Chat Completions | Yes | No |
| Cline / Roo Code | Multi-provider | Yes | No |

**No production coding agent relies on server-side conversation state.** Every agent sends the full conversation history (all messages, tool calls, tool results, and reasoning items) in each API request body. This is a deliberate design choice for reliability, provider flexibility, ZDR compliance, and debuggability.

Additionally, reasoning content does not need to be stored separately — Codex CLI includes `ResponseItem::Reasoning` items in the `input` array of subsequent requests. Reasoning flows through the request body, not through a separate session mechanism.

### Dead code in the current gateway

The production code path (`handleProxy` → `TranslateStream` / `handleNonStream`) never calls `TranslateResponse` or `UpdateSession`. As a result:

- `s.Messages` is never populated — the `rebuildMessages` session path (`if s != nil && len(s.Messages) > 0`) is always false
- `s.ReasoningRecords` is never populated — the `sessionMessages` reasoning injection path is unreachable
- The only live session fields are `ProviderID` and `ModelName` (set in `handleProxy` after provider selection)

## Decision

**Remove `Messages` and `ReasoningRecords` from the `Session` struct.** The session store will only persist `ID`, `ProviderID`, `ModelName`, and lifecycle fields (`CreatedAt`, `LastAccess`, `TTL`).

### What stays

- `session.Store` interface and backends (`MemoryStore`, `SQLiteStore`)
- Provider affinity via `ProviderID` — the primary purpose of the session store
- Session ID extraction from `X-Session-Id` header and `previous_response_id` body field

### What is removed

- `Session.Messages` field and the `Message` struct
- `Session.ReasoningRecords` field and the `Reasoning` struct
- `Translator.TranslateResponse` method from the interface (only called in tests)
- `Translator.UpdateSession` method from the interface (only called in tests)
- `appendToSession` / `sessionMessages` / `sessionToAnthropicMessagesWithReasoning` in translators
- `extractReasoningContent` (dead code path, no longer reachable)

### What is deferred (separate work)

- **Reasoning forwarding in streaming translation**: The `TranslateStream` methods in `ResToChat` and `ResToAnth` do not emit reasoning/thinking events. This is a separate bug — Codex CLI never receives reasoning content through the gateway's streaming path, which breaks multi-turn conversations with models like DeepSeek V4 Pro that require reasoning to be sent back after tool calls. Fixing this requires changes to `TranslateStream` (not session store), to emit `response.reasoning_text.delta` events from upstream `reasoning_content` SSE chunks.

## Alternatives Considered

### Keep full session store, fix the dead code wiring

Rejected: would require adding `TranslateResponse` + `UpdateSession` calls to the streaming production path, adding complexity to maintain state that every client already provides in full. The session store would become a cache of data that is already in the request body, introducing consistency risks (stale session vs fresh request) with no benefit.

### Remove session store entirely

Rejected: provider stickiness is essential for multi-turn conversations — without it, each turn could route to a different provider, breaking the session model that coding agents expect.

## Consequences

- **Positive**: Smaller session footprint in memory/SQLite — only ~3 strings per session instead of arbitrary-length message arrays
- **Positive**: Cleaner translator interface — `TranslateRequest` + `TranslateStream` are the only two methods; no `TranslateResponse` or `UpdateSession`
- **Positive**: No risk of session-vs-request consistency bugs — the request body is always the single source of truth
- **Neutral**: SQLite session store continues to work for provider affinity, just with less data per row
- **Neutral**: `rebuildMessages` / `buildMessages` simplify to always use the request body path, removing the session branch
- **Negative / deferred**: Reasoning forwarding in streaming translation remains broken — this is a separate bug to be fixed in `TranslateStream`, documented in a follow-up issue
