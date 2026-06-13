# ADR 0004: SQLite Session Store for Reasoning Content Durability

## Status
Accepted

## Context

The gateway's `MemoryStore` session backend loses all state on restart/crash/upgrade. The session holds three categories of data: message history, provider binding, and reasoning records.

Message history and provider binding are reconstructable: coding agents send full input history on every request, and the provider can be re-selected. But **reasoning records are not** — Codex CLI does not use `previous_response_id`, and neither Codex CLI nor Claude Code send thinking/reasoning content back in their request payloads. The gateway's session is the sole persistence point for reasoning content across multi-turn conversations with DeepSeek R1/V4 and other reasoning models.

## Decision

Add a `SQLiteStore` backend implementing the existing `session.Store` interface. The `session.Store` interface (`Get`, `Set`, `Delete`, `Prune`) is unchanged.

### Storage design
- Single SQLite file (`gateway-sessions.db` by default, path configurable).
- Schema: `id TEXT PRIMARY KEY, data JSON, created_at INTEGER, last_access INTEGER`.
- `Prune()` is called periodically (every 60s) to DELETE rows where `last_access < now - TTL`.
- TTL is enforced at read time as well — expired sessions return nil.

### Why SQLite
- **Zero ops**: single file, no daemon, no network. Bundled in the gateway binary via Go's `database/sql` + `modernc.org/sqlite` (pure Go, no CGO).
- **Durable**: survives restarts, crashes, and upgrades.
- **Sufficient**: single-node deployment (the current model); gateway is not distributed yet.
- **Simple migration**: `session.Store` interface unchanged — flip config from `backend: memory` to `backend: sqlite`.

### Why not Redis
Redis is the right answer for distributed deployments, but adds an external dependency. If/when scaling to multiple gateway instances, a `RedisStore` can be added behind the same `session.Store` interface without changing any callers.

## Alternatives considered

### Keep only MemoryStore
Rejected: reasoning content loss on restart is user-visible and degrades multi-turn quality with reasoning models.

### Redis now
Rejected: adds operational complexity for a single-node deployment. The `Store` interface supports adding Redis later with zero caller changes.

### JSON files per session
Rejected: no atomicity, no query support for prune, poor concurrent access.

### BoltDB
Rejected: SQLite has better tooling (`sqlite3` CLI for debugging), richer query support for TTL cleanup, and `modernc.org/sqlite` removes CGO dependency.

## Consequences
- Gateway gains a dependency on `modernc.org/sqlite` (pure Go, ~2MB binary increase).
- Config gains `server.session.backend: sqlite` and `server.session.sqlite_path` fields.
- `MemoryStore` remains the default; `SQLiteStore` is opt-in.
- No caller changes — `session.Store` interface is stable.
