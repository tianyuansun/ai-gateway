# ADR 0002: Spec-Generated Schema Types (Not SDKs)

## Status
Accepted

## Context

The gateway defines ~2000 lines of hand-rolled Go structs for three API formats: Anthropic Messages, OpenAI Chat Completions, and OpenAI Responses API. These types are incomplete (only fields used by existing translators), un-annotated, and disconnected from the upstream specs they represent. 

We considered replacing them with the official Go SDKs: `github.com/openai/openai-go` and `github.com/anthropics/anthropic-sdk-go`. These provide complete, well-documented types maintained by OpenAI and Anthropic respectively.

We surveyed three comparable Go gateway projects — [moon-bridge](https://github.com/ZhiYi-R/moon-bridge), [ccx](https://github.com/BenedictKing/ccx), and [codex-shim](https://github.com/0xSero/codex-shim, Python) — all of which use hand-rolled types. None import the official SDKs.

## Decision

**Do not use the official SDKs.** Their core value proposition is the HTTP client + auth + retry layer, which the gateway already implements independently. The type definitions alone don't justify the dependency weight, and the types from `openai-go` and `anthropic-sdk-go` are not interoperable — translators would still need to map between them.

Instead, **generate Go structs directly from OpenAPI/JSON Schema specs** published by OpenAI and Anthropic. Use a code generation tool (e.g., `oapi-codegen`) to produce three independent type packages sourced from the upstream specs:

- `pkg/schema/anthropic/` — from Anthropic Messages API spec
- `pkg/schema/chat/` — from OpenAI Chat Completions API spec
- `pkg/schema/responses/` — from OpenAI Responses API spec

Translators operate directly between these three type sets, with no intermediate canonical representation.

## Alternatives Considered

### Official Go SDKs
Rejected: HTTP client and auth are unused; two SDKs would add significant dependency weight; types remain non-interoperable between formats; no project in our survey uses this approach.

### Hand-write all types (status quo)
Rejected: maintenance burden grows as fields are added piecemeal; types never fully match upstream specs; no source of truth beyond "what the current translators need."

### Unified canonical intermediate type
Rejected: adds an extra abstraction layer that must be kept in sync with three upstream specs; translators already exist to perform format-to-format mapping.

## Consequences
- A code generation step is added to the build toolchain (or a one-time generation committed to the repo).
- Generated types may require post-processing for idiomatic Go (e.g., `oneOf` unions).
- When upstream APIs add fields, regeneration pulls them in automatically — the delta becomes a diff review rather than manual struct editing.
- Translator code is rewritten to use the generated types, replacing the current hand-rolled structs.
