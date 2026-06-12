# ADR 0003: Multi-Format Model Catalog

## Status
Accepted

## Context

The gateway's `GET /v1/models` endpoint currently returns only the OpenAI model list format (`{object: "list", data: [...]}`). Anthropic clients (Claude Code, etc.) call `GET /v1/models` expecting the Anthropic format (`{data: [...], has_more, first_id, last_id}`) with nested capability structures (`{thinking: {supported: true, types: {...}}}`) rather than the flat OpenAI capabilities (`SupportsTools: true`). The two formats are structurally incompatible.

## Decision

**Single endpoint `/v1/models` returns one of two formats based on header detection.**

### Format detection (OR logic, first match wins)

| Priority | Condition | Format |
|----------|-----------|--------|
| 1 | `Accept: application/x-anthropic-json` | Anthropic |
| 2 | `x-api-key` + `anthropic-version` headers present | Anthropic |
| 3 | `Authorization: Bearer` header | OpenAI |
| 4 | None of the above | OpenAI |

### Capability mapping

The gateway's config model capabilities (flat) are mapped to the Anthropic nested capability format. Fields without a direct config equivalent default to `{"supported": false}`.

Key mappings:
- `SupportsTools` → `tools.supported` (Anthropic doesn't have a top-level `tools`, but maps to tool-use content blocks)
- `ContextWindow` → `max_input_tokens`
- `MaxOutputTokens` → `max_tokens`
- `SupportsVision` → `image_input.supported`
- `SupportsReasoning` → `thinking.supported`

### Config unchanged

`gateway.yaml` model capabilities keep their current flat structure. No configuration changes needed.

### Pagination stubs

The Anthropic format requires `has_more`, `first_id`, `last_id`. Since the gateway serves a static model list (no pagination), these return: `has_more: false`, `first_id: ""` and `last_id: ""` for empty lists, or the first/last model ID for non-empty lists.

## Alternatives Considered

### Separate endpoint (`/v1beta/models`)
Rejected: Anthropic clients hardcode `GET /v1/models`; a separate path would require client-side config changes.

### Extend config with full Anthropic capabilities
Rejected: unnecessary complexity. The mapping layer covers all practical cases without config bloat.

### Pull models from upstream
Rejected: adds latency, auth complexity, and cache management for a static list that gateway operators configure anyway.

## Consequences
- `ModelsHandler` gains format detection logic and a `toAnthropicFormat()` method.
- No config schema changes.
- No new endpoints.
- Anthropic-format clients can discover gateway models without client-side workarounds.
