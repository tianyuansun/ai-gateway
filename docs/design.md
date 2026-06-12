# AI Gateway 设计方案

## 目标

统一 API 网关，解决三个问题：

1. **协议透明**：注册上游模型（Chat API / Anthropic API），对外暴露 Responses / Chat / Anthropic 三种 API 格式
2. **多供应商路由**：同模型多供应商，会话级亲和，可配路由策略
3. **Coding Agent 兼容**：Codex CLI、Claude Code 等开箱即用

---

## 总体架构

```
Codex CLI ──► POST /v1/responses ──┐
Claude Code ──► POST /v1/messages ──┤
Chat Agent ──► POST /v1/chat/... ───┤
                                    ▼
                         ┌──────────────────────┐
                         │    Ingress Layer      │  协议识别 + 请求解析
                         │  3 种 API 端点        │
                         └──────────┬───────────┘
                                    ▼
                         ┌──────────────────────┐
                         │    Router             │  模型名匹配 + 供应商选择
                         │  Model Registry       │  两层路由
                         └──────────┬───────────┘
                                    ▼
                         ┌──────────────────────┐
                         │  Protocol Translator  │  2 核心 + 2 降级翻译路径
                         │  优先直通，不能才翻译   │
                         └──────────┬───────────┘
                                    ▼
                         ┌──────────────────────┐
                         │    Egress Layer       │  上游 HTTP 调用 + 重试/降级
                         │  健康探活              │
                         └──────────────────────┘
```

---

## 1. 核心简化：利用厂商双端点

大多数厂商（DeepSeek、Kimi、GLM）同时提供 Chat 和 Anthropic 端点：

```yaml
providers:
  ds-official:
    endpoints:
      chat:      "https://api.deepseek.com/v1"
      anthropic: "https://api.deepseek.com/anthropic"
    api_key_env: "DEEPSEEK_API_KEY"
```

### 翻译矩阵：能直通就直通，不能才翻译

```
                    上游端点可用格式
                  chat-only   anth-only   chat+anth
暴露 API           │           │           │
──────────────────┼───────────┼───────────┼──────────────
Responses (Codex) │ 必须翻译   │ 必须翻译   │ 必须翻译 ← 无厂商原生支持
Chat Completions  │ 直通 ✅    │ 必须翻译   │ 直通 (优选 chat)
Anthropic Messages│ 必须翻译   │ 直通 ✅    │ 直通 (优选 anth)
```

只有涉及 Responses API 的路径必须翻译。Anthropic ↔ Chat 互译仅在端点缺失时触发。

### 翻译路径：6 条降为 2 核心 + 2 降级

```
核心（每次都要）：
  ResToChat   ← Codex → Chat 端点
  ResToAnth   ← Codex → Anthropic 端点（工具调用保真度更高）

降级（仅上游端点缺失时触发）：
  AnthToChat  ← Claude Code → 纯 Chat 端点厂商
  ChatToAnth  ← Chat 客户端 → 纯 Anthropic 端点厂商
```

### 端点选择逻辑

```go
func (g *Gateway) resolvePath(exposedAPI APIFormat, p *Provider) (Translator, string) {
    switch exposedAPI {
    case "responses":
        // 优先 Anthropic 端点（工具调用保真度 > Chat），没有则用 Chat
        if p.Endpoints.Anthropic != "" {
            return g.tr["ResToAnth"], p.Endpoints.Anthropic
        }
        return g.tr["ResToChat"], p.Endpoints.Chat

    case "anthropic":
        if p.Endpoints.Anthropic != "" {
            return &Passthrough{}, p.Endpoints.Anthropic
        }
        return g.tr["AnthToChat"], p.Endpoints.Chat

    case "chat":
        if p.Endpoints.Chat != "" {
            return &Passthrough{}, p.Endpoints.Chat
        }
        return g.tr["ChatToAnth"], p.Endpoints.Anthropic
    }
}
```

---

## 2. 模型选择：两层路由

### 第一层：模型选择（客户端决定）

客户端在请求体中指定 `model` 名，网关做匹配。支持别名：

```yaml
models:
  deepseek-v4-pro:
    aliases: ["ds-pro", "dsv4", "deepseek"]
    ...
```

```
Codex CLI:    {"model": "ds-pro", ...}
Claude Code:  {"model": "deepseek-v4-pro", ...}
                        │
                        ▼
              网关解析 model 名 → 查 aliases → 找到模型配置
```

### 第二层：供应商选择（网关决定）

```yaml
models:
  deepseek-v4-pro:
    routing:
      strategy: priority
      affinity: session
    providers:
      - provider: ds-official
        priority: 1
      - provider: ds-openrouter
        priority: 2
```

**路由策略**：

| 策略 | 行为 | 适用场景 |
|------|------|---------|
| `priority` | 主备模式，高优先级优先，不可用时降级 | 官方 API + OpenRouter 备路 |
| `weighted` | 按权重分流 | 多个代理均摊负载 |
| `random` | 随机 | 测试用 |

Priority 覆盖 90% 场景。

### 路由决策流

```
请求 model: "deepseek-v4-pro"
         │
         ▼
  ┌─────────────────────────────┐
  │ 已绑定 session → provider？  │
  │  是 → 直接用绑定             │
  │  否 → 执行路由策略           │
  └──────────┬──────────────────┘
             ▼
  ┌─────────────────────────────┐
  │ 过滤 unhealthy provider     │
  │ 应用路由策略                 │
  │ 创建 session 绑定            │
  └─────────────────────────────┘
```

---

## 3. 会话亲和

### 会话标识提取（auto 模式）

```
1. Responses API → 提取 previous_response_id
   新会话（无 previous_response_id）→ 用响应的 response.id 作为绑定 key

2. Chat Completions → 检查 X-Session-Id 请求头
   无则生成，在响应头返回 X-Session-Id

3. Anthropic Messages → 同理 X-Session-Id
```

### Codex 天然适合

```
Codex 首次请求：POST /v1/responses (无 previous_response_id)
  网关：创建 session → 路由 → 绑定 provider
  响应：{"id": "gw_resp_001", ...}

Codex 后续：POST /v1/responses (previous_response_id: "gw_resp_001")
  网关：找到 session → 直接路由到绑定的 provider
```

### 会话存储

```go
type Session struct {
    ID         string             // 会话 ID
    ModelName  string             // 模型名
    ProviderID string             // 绑定的供应商
    Messages   []Message          // 完整消息历史（用于 Chat API 重建）
    ReasoningRecords []Reasoning  // 跨轮推理内容缓存
    CreatedAt  time.Time
    LastAccess time.Time
    TTL        time.Duration
}

type SessionStore interface {
    Get(sessionID string) (*Session, error)
    Set(sessionID string, s *Session) error
    Delete(sessionID string) error
    Prune()
}
```

默认内存 LRU + TTL，多实例部署时换 Redis。

---

## 4. 协议翻译

### 4.1 通用架构

```go
type Translator interface {
    // 请求转换：暴露 API 格式 → Provider 原生格式
    TranslateRequest(ctx context.Context, req *Request, s *Session) (*UpstreamRequest, error)

    // 流式响应转换：Provider SSE 流 → 暴露 API SSE 流
    TranslateStream(ctx context.Context, upstream io.Reader, req *Request, s *Session) <-chan SSEEvent

    // 非流式响应转换
    TranslateResponse(ctx context.Context, upstream *http.Response, req *Request, s *Session) (*Response, error)

    // 更新会话状态
    UpdateSession(s *Session, req *Request, resp *Response)
}
```

### 4.2 核心路径 A：Responses API → Chat Completions

Codex CLI → DeepSeek / GLM / Kimi 走此路径。

#### 请求翻译

```
Responses API 请求                     Chat Completions 请求
══════════════════                     ══════════════════════
input: [                               messages: [
  {type:"message", role:"user",           {role:"user", content:"..."},
   content:[{type:"input_text",           {role:"assistant", content:null,
            text:"..."}]},                  tool_calls:[...]},
  {type:"message", role:"assistant",      {role:"tool", tool_call_id:"x",
   content:[...]},                          content:"..."}
  {type:"function_call_output",          ]
   call_id:"x", output:"..."}            tools: [{type:"function",...}]
]                                        reasoning_effort: "high"
tools: [{type:"function",...}]
previous_response_id: "resp_abc"
reasoning: {effort:"high"}
```

核心挑战：`previous_response_id` → 完整 `messages` 数组重建。网关必须在 Session 中维护消息历史。

#### 流式 SSE 翻译

```
Chat Completions SSE                    Responses API SSE
═════════════════════                   ════════════════════
data: {"choices":[{"delta":             event: response.output_text.delta
  {"content":"Hello"}}]}                data: {"delta":"Hello"}

data: {"choices":[{"delta":             event: response.output_item.added
  {"tool_calls":[{"id":"x",             data: {"type":"function_call",
    "function":{"name":"read",            "id":"x","name":"read"}
    "arguments":""}}]}]}
                                        event: response.function_call_
data: {"choices":[{"delta":               arguments.delta
  {"tool_calls":[{"function":           data: {"delta":"{\"path\":\"/etc/"}
    {"arguments":"{\"path\":\"/"}}]}]}

                                        event: response.function_call_
                                          arguments.done
                                        data: {"arguments":"..."}

data: {"choices":[{"delta":             event: response.output_item.added
  {"reasoning_content":"思考..."}}]}      data: {"type":"reasoning_text"}
                                        event: response.reasoning_text.delta
                                        data: {"delta":"思考..."}

data: [DONE]                            event: response.completed
                                        (连接关闭)
```

### 4.3 核心路径 B：Responses API → Anthropic Messages

Codex CLI → DeepSeek Anthropic 端点 / Claude 走此路径。

```
Responses API                           Anthropic Messages
═════════════                           ══════════════════
input: [                                messages: [
  {type:"message", role:"user",            {role:"user",
   content:[{type:"input_text",              content:[{type:"text","..."}]}
            text:"..."}]},                {role:"assistant",
  {type:"message", role:"assistant",        content:[{type:"tool_use",
   content:[...]},                             id:"x",name:"read",
  {type:"function_call_output",               input:{...}}]}
   call_id:"x", output:"..."}             {role:"user",
]                                            content:[{type:"tool_result",
tools: [{type:"function",...}]                 tool_use_id:"x",
instructions: "..." 或 首个 system msg →       content:"..."}]}
system: "..."                              ]
reasoning: {effort:"high"}              tools: [{name:"read",...}]
                                        system: "..."
                                        thinking: {type:"enabled",
                                                   budget_tokens: 16000}
```

流式翻译（Anthropic 分层事件结构）：

```
Anthropic SSE                           Responses API SSE
══════════════                          ════════════════════
event: message_start                    (无对应，内部状态)
event: content_block_start              event: response.output_item.added
  content_block: {type:"tool_use",        data: {type:"function_call",
    id:"x", name:"read"}                    id:"x", name:"read"}
event: content_block_delta              event: response.function_call_
  delta: {type:"input_json_delta",        arguments.delta
    partial_json:"{\"path\":"}          data: {"delta":"{\"path\":"}

event: content_block_start              event: response.output_item.added
  content_block: {type:"text",            data: {type:"reasoning_text"}
    text:"思考..."}
event: content_block_delta              event: response.reasoning_text.delta
  delta: {type:"thinking_delta",         data: {"delta":"..."}
    thinking:"..."}

event: content_block_start              event: response.output_text.delta
  content_block: {type:"text",           data: {"delta":"Hello"}
    text:"Hello"}

event: message_delta                    event: response.completed
  delta: {stop_reason:"end_turn"}       data: {response:{id:"gw_xyz",...}}
event: message_stop
```

### 4.4 降级路径

**Anthropic → Chat**（Claude Code → 纯 Chat 端点）：

```
Anthropic Messages                     Chat Completions
════════════════                        ═════════════════
system: "You are..."                   messages: [
messages: [                              {role:"system", content:"You are..."},
  {role:"user",                          {role:"user", content:"写个快排"},
    content:[{type:"text","写个快排"}]},  {role:"assistant", content:null,
  {role:"assistant",                       tool_calls:[
    content:[{type:"tool_use",              {id:"x",function:{name:"read",
      id:"x",name:"read",                     arguments:"..."}}]},
      input:{...}}]},                     {role:"tool", tool_call_id:"x",
  {role:"user",                             content:"..."}
    content:[{type:"tool_result",       ]
      tool_use_id:"x",
      content:"..."}]}
]
tools: [{name:"read",...}]
```

关键映射：

| Anthropic | OpenAI Chat |
|-----------|-------------|
| `tool_use` content block | `tool_calls[]` in assistant message |
| `tool_result` user block | `role:"tool"` message |
| `thinking` block | `reasoning_content`（DeepSeek 特定）|
| `stop_reason: "tool_use"` | `finish_reason: "tool_calls"` |
| `stop_reason: "end_turn"` | `finish_reason: "stop"` |
| `stop_reason: "max_tokens"` | `finish_reason: "length"` |

**Chat → Anthropic**（Chat 客户端 → 纯 Anthropic 端点）：

```
Chat Completions                        Anthropic Messages
════════════════                        ══════════════════
messages: [                             messages: [
  {role:"system", content:"..."},          {role:"user",
  {role:"user", content:"..."},              content:[{type:"text","..."}]}
  {role:"assistant",                      {role:"assistant",
    tool_calls:[                             content:[{type:"tool_use",
    {id:"x",function:{name:"read",              id:"x",name:"read",
      arguments:"..."}}]},                       input:{...}}]}
  {role:"tool", tool_call_id:"x",         {role:"user",
    content:"..."}                           content:[{type:"tool_result",
]                                                tool_use_id:"x",
tools: [{type:"function",...}]                    content:"..."}]}
                                          ]
                                         tools: [{name:"read",...}]
                                         system: "..." (从 system message 提取)
```

---

## 5. Coding Agent 兼容

### 5.1 Codex CLI

#### 所需端点

```
POST /v1/responses          ← 核心 agent loop
GET  /v1/models             ← 模型列表
POST /v1/responses/compact  ← 长会话压缩
```

#### 自动生成配置

`GET /v1/codex-config?model=deepseek-v4-pro` 返回：

```toml
model = "deepseek-v4-pro"
model_provider = "ai-gateway"

[model_providers.ai-gateway]
name = "AI Gateway"
base_url = "http://127.0.0.1:9000/v1"
wire_api = "responses"
env_key = "GATEWAY_API_KEY"

[model_properties."deepseek-v4-pro"]
context_window = 262144
max_context_window = 1048576
supports_parallel_tool_calls = true
supports_reasoning_summaries = false
input_modalities = ["text"]
```

一键配置：

```bash
curl -s http://127.0.0.1:9000/v1/codex-config?model=deepseek-v4-pro > ~/.codex/config.toml
```

#### 会话粘滞

Codex 的 `previous_response_id` 天然适合做 session key，无需额外 header。

### 5.2 Claude Code

#### 所需端点

```
POST /v1/messages           ← 核心端点
GET  /v1/models             ← 模型发现（可选）
```

#### 配置方式

```bash
claude --model deepseek-v4-pro \
       --api-url http://127.0.0.1:9000 \
       --api-key sk-gateway
```

或写入 `~/.claude/settings.json`：

```json
{
  "apiUrl": "http://127.0.0.1:9000",
  "apiKey": "sk-gateway",
  "model": "deepseek-v4-pro"
}
```

#### 会话粘滞

Claude Code 无 `previous_response_id` 机制。网关用 `X-Session-Id` 响应头传递 session ID，Claude Code 后续请求回传。若不回传，降级为用户 IP + 时间窗口的弱亲和。

---

## 6. 完整配置示例

```yaml
# gateway.yaml
server:
  listen: "127.0.0.1:9000"
  session:
    ttl_seconds: 3600
    backend: memory
    key_source: auto

providers:
  ds-official:
    endpoints:
      chat: "https://api.deepseek.com/v1"
      anthropic: "https://api.deepseek.com/anthropic"
    api_key_env: "DEEPSEEK_API_KEY"

  ds-openrouter:
    endpoints:
      chat: "https://openrouter.ai/api/v1"
    api_key_env: "OPENROUTER_API_KEY"

  anthropic-direct:
    endpoints:
      anthropic: "https://api.anthropic.com/v1"
    api_key_env: "ANTHROPIC_API_KEY"

models:
  deepseek-v4-pro:
    aliases: ["ds-pro", "dsv4", "deepseek"]
    display_name: "DeepSeek V4 Pro"
    capabilities:
      context_window: 262144
      max_output_tokens: 32768
      supports_tools: true
      supports_parallel_tool_calls: true
      supports_vision: false
      supports_reasoning: true
      input_modalities: ["text"]
    routing:
      strategy: priority
      affinity: session
    providers:
      - provider: ds-official
        priority: 1
      - provider: ds-openrouter
        priority: 2

  deepseek-v4-flash:
    aliases: ["ds-flash"]
    display_name: "DeepSeek V4 Flash"
    capabilities:
      context_window: 262144
      max_output_tokens: 16384
      supports_tools: true
      supports_parallel_tool_calls: true
      supports_vision: false
      supports_reasoning: false
    routing:
      strategy: priority
    providers:
      - provider: ds-official
        priority: 1

  claude-opus-4-7:
    display_name: "Claude Opus 4.7"
    capabilities:
      context_window: 200000
      max_output_tokens: 32768
      supports_tools: true
      supports_vision: true
      supports_reasoning: true
    providers:
      - provider: anthropic-direct
        priority: 1
```

---

## 7. 模型目录 API

### `GET /v1/models`

```json
{
  "object": "list",
  "data": [
    {
      "id": "deepseek-v4-pro",
      "object": "model",
      "display_name": "DeepSeek V4 Pro",
      "aliases": ["ds-pro", "dsv4", "deepseek"],
      "capabilities": {
        "context_window": 262144,
        "max_output_tokens": 32768,
        "supports_tools": true,
        "supports_vision": false,
        "supports_reasoning": true
      },
      "providers": [
        {"id": "ds-official", "status": "healthy"},
        {"id": "ds-openrouter", "status": "healthy"}
      ]
    }
  ]
}
```

### `GET /health`

```json
{
  "status": "ok",
  "providers": {
    "ds-official": "healthy",
    "ds-openrouter": "degraded"
  }
}
```

网关后台每 30s 对每个 provider 发轻量 probe 请求，更新健康状态。

---

## 8. 项目结构

```
ai-gateway/
├── cmd/gateway/main.go
├── config/
│   ├── config.go              # YAML 配置解析
│   └── gateway.yaml           # 默认配置
├── ingress/
│   ├── responses.go            # POST /v1/responses
│   ├── chat.go                 # POST /v1/chat/completions
│   ├── messages.go             # POST /v1/messages
│   └── models.go               # GET /v1/models + codex-config
├── router/
│   ├── model.go                # 模型名匹配（别名解析）
│   └── provider.go             # 供应商选择（priority/weighted + affinity）
├── translator/
│   ├── translator.go           # Translator interface
│   ├── res_to_chat.go          # Codex → Chat 端点（核心）
│   ├── res_to_anth.go          # Codex → Anthropic 端点（核心）
│   ├── anth_to_chat.go         # Anthropic → Chat（降级）
│   ├── chat_to_anth.go         # Chat → Anthropic（降级）
│   ├── passthrough.go          # 同协议直通
│   └── shared/
│       ├── sse.go              # SSE 解析/发射
│       └── accumulator.go      # 工具 delta 累加 + 消息重建
├── session/
│   ├── store.go                # SessionStore interface
│   └── memory.go               # 内存 LRU + TTL 实现
└── provider/
    ├── client.go                # 上游 HTTP 调用
    └── health.go                # 后台健康探活
```

---

## 9. 技术选型

| 选择 | 理由 |
|------|------|
| **Go** | 单二进制部署，goroutine 天然适配流式/高并发，与 Moon Bridge 同生态 |
| **YAML 配置** | 可读性 > JSON，Coding Agent 生态通用 |
| **内存会话 + 可选 Redis** | 默认零依赖，多实例时加 Redis |
| **SSE 翻译用状态机** | 翻译路径本质是有限状态转换，比正则/字符串替换可靠 |
| **Provider 用 interface 抽象** | 新增供应商零侵入 Translator |

---

## 10. 与现有方案对比

| 维度 | codex-shim | Moon Bridge | codex-relay | mimo2codex | ai-gateway |
|------|-----------|-------------|-------------|------------|------------|
| 暴露 API | Responses | Responses + Anthropic(partial) | Responses | Responses | **Responses + Chat + Anthropic** |
| 翻译路径 | 3（单入多出）| 3 | 1 | 1 | **2 核心 + 2 降级** |
| 厂商端点利用 | 手动选 provider type | 手动选 mode | Chat only | Chat only | **自动选最优端点** |
| 多供应商同模型 | ❌ 重复条目 | ❌ | ❌ | ❌ | ✅ 原生路由配置 |
| 会话粘滞 | ❌ | ❌ | ❌ | ❌ | ✅ previous_response_id |
| Claude Code 兼容 | ❌ | 部分 | ❌ | ❌ | ✅ |
| Codex 配置生成 | ✅ | ✅ | ✅ | ✅ | ✅ + 一键端点 |
| 语言 | Python | Go | Rust | TypeScript | **Go** |
| 代码规模 | ~2000 行 | ~5000 行 | — | — | **~3000 行（估算）** |

---

## 11. 实现路线

### Phase 1：核心翻译 + 单供应商
- Translator interface + ResToChat / ResToAnth
- 内存会话存储 + 消息历史维护
- 单供应商，无路由
- Codex CLI 端到端验证

### Phase 2：多供应商 + 路由
- Router 引擎（priority 主备）
- 会话亲和
- Provider 健康探活
- 降级重试

### Phase 3：全协议 + Claude Code
- AnthToChat / ChatToAnth 降级翻译
- `/v1/codex-config` 端点
- Claude Code 兼容验证

### Phase 4：生产就绪
- Redis 会话后端
- 用量追踪 + 成本统计
- Admin API（热重载、Provider 开关）
- Docker 镜像

---

## 12. 关键风险

1. **流式翻译正确性**：SSE 事件顺序和完整性是 agent loop 生命线。一个事件丢失或乱序导致工具调用失败。需 extensive 集成测试。
2. **推理内容跨轮保留**：DeepSeek R1/V4 的 `reasoning_content` 需在后续请求中回传，否则模型行为退化。Translator 必须在 Session 中缓存。
3. **Claude Code 兼容深度**：Claude Code 用 Anthropic Messages API 有大量扩展字段（`extended_thinking`、`server_tools` 等），DeepSeek 的 Anthropic 端点不一定完全兼容，需实际测试。
4. **厂商 Anthropic 端点保真度**：DeepSeek 的 `/anthropic` 端点与原生 Anthropic API 可能存在细微差异，`tool_use` / `thinking` 格式需要逐个验证。
