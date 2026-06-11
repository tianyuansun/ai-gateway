# AI Gateway

Unified API gateway for coding agents. Register upstream models once, expose them in three API formats (Responses / Chat Completions / Anthropic Messages), with multi-provider routing and session affinity.

## Features

- **Protocol translation**: Exposes Responses API, Chat Completions API, and Anthropic Messages API endpoints. Routes to upstream endpoints using passthrough when possible, translates when needed.
- **Multi-provider routing**: Same model from multiple providers, with priority/weighted strategies and session-level affinity.
- **Codex CLI & Claude Code compatible**: Auto-generate Codex config, native `/v1/messages` for Claude Code.

## Quick Start

```bash
# Edit config
cp config/gateway.yaml config/gateway.local.yaml
# Set your API keys
export DEEPSEEK_API_KEY=sk-...
export OPENROUTER_API_KEY=sk-or-...

# Run
go run ./cmd/gateway -config config/gateway.local.yaml

# Generate Codex config
curl "http://127.0.0.1:9000/v1/codex-config?model=deepseek-v4-pro" > ~/.codex/config.toml
```

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/responses` | POST | OpenAI Responses API (Codex CLI) |
| `/v1/chat/completions` | POST | OpenAI Chat Completions API |
| `/v1/messages` | POST | Anthropic Messages API (Claude Code) |
| `/v1/models` | GET | Model catalog |
| `/v1/codex-config` | GET | Generate Codex config.toml |
| `/health` | GET | Health check |

## Config

See [config/gateway.yaml](config/gateway.yaml) for full example.

```yaml
models:
  deepseek-v4-pro:
    aliases: ["ds-pro", "dsv4"]
    providers:
      - provider: ds-official
        priority: 1      # primary
      - provider: ds-openrouter
        priority: 2      # fallback
    routing:
      strategy: priority
      affinity: session
```
