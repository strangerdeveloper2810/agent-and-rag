# AI Agent System — System Design Document

> **Audience:** Backend engineers (mid-level+) evaluating architecture decisions, system boundaries, and production readiness.
> **Status:** Phase 2/14 — Agent Engine core implementation.
> **Companion docs:** [`go-agent-architecture-deep-dive.md`](./go-agent-architecture-deep-dive.md) (technical deep-dive), [`go-production-patterns.md`](../go-patterns/go-production-patterns.md) (Go patterns catalog).

---

## 1. Business Context

### 1.1 What Problem Does This Solve?

Users interact with an AI chatbot that can:
- **Search documents** via RAG (Retrieval-Augmented Generation) — semantic search over uploaded PDFs/text
- **Manage tasks** — create, list, update, delete tasks during conversation
- **Remember context** — persistent memory across conversations (preferences, facts, entities)
- **Use tools** — the agent autonomously decides which tools to invoke based on user intent

Example conversation flow:
```
User: "Tìm tài liệu về chính sách nghỉ phép"
Agent: [ragSearch("nghỉ phép")] → "Có 3 tài liệu liên quan..."
User: "Tạo task xin nghỉ phép tuần sau"
Agent: [createTask("xin nghỉ phép", due="next week")] → "Đã tạo task #42"
```

### 1.2 Business Constraints

| Constraint | Impact |
|---|---|
| **Cost-sensitive** | Gemini 3.1 Flash Lite default; prompt caching; token budget per conversation |
| **Multi-tenant** | Each user's conversations/documents isolated (MongoDB per-user filtering) |
| **Real-time** | SSE streaming — users see tokens as they're generated, not after completion |
| **Safety** | Destructive operations (deleteTask) require human-in-the-loop (HITL) confirmation |
| **Observability** | Must track token usage, cost, latency per conversation for cost attribution |

### 1.3 Scale Expectations

| Metric | Target |
|---|---|
| Concurrent chat sessions | 100-1,000 (MVP → production) |
| Documents in vector store | 10,000+ chunks |
| Response latency (first token) | <2s for cached context, <5s cold start |
| Agent loop steps | 1-12 per user message |
| Memory recall latency | <200ms (vector search) |

---

## 2. System Architecture

### 2.1 High-Level Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                        CLIENT LAYER                              │
│  ┌─────────────┐                                                │
│  │ React (SPA)  │  TypeScript, Vite 8, React 19.2               │
│  │ apps/web     │  Lazy routes, code-splitting, Suspense         │
│  └──────┬──────┘                                                │
└─────────┼────────────────────────────────────────────────────────┘
          │ SSE (text/event streaming) + REST
          ▼
┌─────────────────────────────────────────────────────────────────┐
│                      GATEWAY LAYER (TypeScript)                  │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ apps/api — Fastify 5                                       ││
│  │                                                             ││
│  │ Responsibilities:                                           ││
│  │  • Edge: CORS, rate-limit, Zod validation, auth             ││
│  │  • File: PDF/text extraction (pdf-parse), chunking          ││
│  │  • Embed: Voyage AI embedding (1024-dim) → write documents  ││
│  │  • CRUD: conversations, messages (MongoDB)                  ││
│  │  • Proxy: SSE /chat → agent-go (BFF pattern)                ││
│  │  • Type sharing: generate TypeScript types for frontend     ││
│  └──────────────────────────┬──────────────────────────────────┘│
└─────────────────────────────┼────────────────────────────────────┘
                              │ HTTP + SSE (internal, localhost)
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      AGENT RUNTIME (Go)                          │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ services/agent-go — Go 1.24+                               ││
│  │                                                             ││
│  │ ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────┐ ││
│  │ │ Provider │  │  Agent   │  │  Tools   │  │   Memory     │ ││
│  │ │ Layer    │  │  Engine  │  │  System  │  │   3-Tier     │ ││
│  │ │          │  │          │  │          │  │              │ ││
│  │ │ Gemini   │  │ State    │  │ Registry │  │ Working      │ ││
│  │ │ Claude   │  │ Machine  │  │ Fan-out  │  │ Episodic     │ ││
│  │ │ (plugs)  │  │ Loop     │  │ Guards   │  │ Semantic     │ ││
│  │ └──────────┘  └──────────┘  └──────────┘  └──────────────┘ ││
│  │                                                             ││
│  │ ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────┐ ││
│  │ │   RAG    │  │ Context  │  │ Guardrails│  │ Observability│ ││
│  │ │ Retrieval│  │ Engine   │  │  + HITL  │  │   (OTel)    │ ││
│  │ └──────────┘  └──────────┘  └──────────┘  └──────────────┘ ││
│  └─────────────────────────────────────────────────────────────┘│
│                                                                  │
│  Transport: net/http (Go 1.22+ enhanced routing)                 │
│  Streaming: SSE via http.Flusher                                 │
│  Concurrency: goroutine + channel + errgroup                     │
└──────────────────────────────┬───────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│                       DATA LAYER                                 │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ MongoDB Atlas (shared cluster)                              ││
│  │                                                             ││
│  │ Collections:                                                ││
│  │  conversations (owned by TS gateway)                        ││
│  │  messages      (owned by TS gateway)                        ││
│  │  documents     (TS writes; Go reads for RAG)                ││
│  │  tasks         (Go CRUD via tool calls)                     ││
│  │  memories      (Go CRUD — semantic + structured)            ││
│  │                                                             ││
│  │ Atlas Search: $vectorSearch on documents.embedding (1024d)  ││
│  │               $vectorSearch on memories.embedding (1024d)   ││
│  └─────────────────────────────────────────────────────────────┘│
│                                                                  │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ Voyage AI (Embedding API)                                   ││
│  │  • Model: voyage-3 (1024 dimensions)                        ││
│  │  • Batch: 128 chunks/request (API limit: 128)               ││
│  │  • Retry: exponential backoff on 429                       ││
│  │  • Clients: TS (ingest) + Go (RAG retrieval)                ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
```

### 2.2 Why Polyglot? (TypeScript + Go)

This is the most controversial decision. Here's the rationale:

| Factor | TypeScript (Node.js) | Go |
|---|---|---|
| **PDF/text extraction** | ✅ Rich ecosystem (pdf-parse, mammoth) | ❌ Poor PDF support |
| **File upload/streaming** | ✅ Busboy, multipart native | ❌ Verbose |
| **Edge middleware** | ✅ Fastify plugin ecosystem | ✅ Can do, but fewer plugins |
| **Concurrent tool calls** | ❌ Promise.all (single-threaded) | ✅ goroutine (true parallelism) |
| **LLM streaming** | ✅ LangChain/LangGraph | ✅ Lightweight, custom loop |
| **Memory efficiency** | ❌ High per-connection memory | ✅ Low goroutine overhead (2KB stack) |
| **Type safety** | ✅ TypeScript generics | ✅ Go generics (Go 1.18+) |
| **Learning value** | — | ✅ Understand agent from scratch |

**Tradeoff accepted:** Schema duplication between Go structs and Zod validators. We manage this with `// sync-schema` comments and a shared schema-of-record per collection.

### 2.3 Communication Protocol: HTTP + SSE

Why not gRPC/WebSocket/message queue?

| Option | Pros | Cons | Verdict |
|---|---|---|---|
| **HTTP + SSE** | Simple, browsers native, stateless Go | Less efficient than binary | ✅ Chosen |
| gRPC | Fast binary, typed contracts | Browser needs gRPC-web proxy, overkill for internal | ❌ |
| WebSocket | Bidirectional | Stateful, harder load balancing, not needed (client→server via POST) | ❌ |
| Message queue | Decoupled, retry | Added infra, latency, overengineered for single-service agent | ❌ |

**Key design principle:** Go agent is **stateless about conversations**. History is transmitted in every request. This means:
- No sticky sessions needed
- Any agent-go instance can handle any request
- Resume (HITL) via persisted State in MongoDB

---

## 3. Agent Engine Design

### 3.1 Why Self-Built Engine (Not LangGraph)?

LangGraph (TypeScript/Python) provides:
- `StateGraph` with conditional edges
- Checkpointer (persist state)
- `streamEvents()` for streaming

We replace each:

| LangGraph Feature | Our Go Replacement | Learning Value |
|---|---|---|
| `StateGraph` | `map[NodeID]Node` + `route(s) NodeID` pure function | State machines, graph traversal |
| `Checkpointer` | `State` struct + optional Mongo persistence | Serialization, resume patterns |
| `streamEvents` | `chan Event` + SSE writer | Channels, streaming I/O |
| `ToolNode` | `tools.Registry` + `errgroup` fan-out | Concurrency, parallel execution |

### 3.2 State Machine Design

```
                    ┌──────────────────────────────┐
                    │         START                 │
                    └─────────────┬────────────────┘
                                  │
                                  ▼
                         ┌────────────┐
                         │   RECALL   │  ← Structured + vector memory lookup
                         └─────┬──────┘
                               │
                               ▼
                         ┌────────────┐
                         │   PLAN     │  ← (Optional, P8) Task decomposition
                         └─────┬──────┘
                               │
                               ▼
              ┌────────────────► MODEL ◄─────────────────────┐
              │                (LLM call)                    │
              │                 │     │                      │
              │          text   │     │ tool_calls           │
              │                 ▼     ▼                      │
              │           ┌──────────────┐                   │
              │           │    ROUTE     │  Pure function:   │
              │           │              │  route(s) NodeID  │
              │           └──┬───┬───┬───┘                   │
              │              │   │   │                        │
              │    final ┌───┘   │   └───┐ need_reflect      │
              │          │       │       │ (P8)               │
              │          ▼       ▼       ▼                    │
              │   ┌──────────┐ ┌──────────────┐              │
              │   │SUMMARIZE │ │    TOOLS     │              │
              │   │(P7)      │ │              │              │
              │   └────┬─────┘ │ Fan-out      │              │
              │        │       │ errgroup     │              │
              │        ▼       │ Execute all  │              │
              │   ┌──────────┐ │ tool calls   │              │
              │   │ EXTRACT  │ │ in parallel  │              │
              │   │(P7)      │ └──────┬───────┘              │
              │   └────┬─────┘        │                       │
              │        │              │ results               │
              │        ▼              └───────────────────────┘
              │   ┌──────────┐
              │   │   END    │
              │   └──────────┘
              │
              └──── (reflect → back to MODEL, P8)
```

**Loop invariants:**
1. Every iteration increments `state.Step`
2. `state.Step >= maxSteps` → force END (safety liveness)
3. `ctx.Err() != nil` → abort immediately (cancellation)
4. Tool errors are fed back to MODEL as observations (never crash the loop)

### 3.3 Streaming Event Contract

All events follow this schema (SSE `data: <json>\n\n`):

```typescript
type AgentEvent =
  | { type: "step";         node: string }                          // Entering a node
  | { type: "text";         text: string }                          // LLM token
  | { type: "tool_start";   name: string }                          // Tool execution begins
  | { type: "tool_end";     name: string; ok?: boolean }             // Tool execution ends
  | { type: "citation";     text: string }                          // JSON: [{documentId, source, score}]
  | { type: "memory";       op: "recall" | "save"; count: number }  // Memory operation
  | { type: "interrupt";    name: string; message: string }         // HITL pause
  | { type: "error";        message: string }                       // Recoverable error
  | { type: "done";         usage: { inputTokens: number; outputTokens: number } }
```

**Why SSE and not WebSocket?**
- SSE is unidirectional (server→client), which matches our use case
- Browser-native `EventSource` API (auto-reconnect, no library)
- HTTP/2 multiplexing with existing infra
- POST for client→server (user message) is simpler than WS message framing

---

## 4. Provider Layer (Pluggable LLM)

### 4.1 Interface Design

```go
type Provider interface {
    Generate(ctx context.Context, req GenerateRequest) (<-chan StreamChunk, error)
    Name() string
}
```

**Why read-only channel (`<-chan StreamChunk`)?**
- Ensures only the Provider writes to the channel
- Prevents the consumer (engine) from accidentally sending data back
- Compile-time guarantee of data flow direction

### 4.2 Normalized Types

All provider-specific types are translated to/from our normalized types:

```
Gemini Content/Part  ←→  provider.Message/ToolCall/ToolDef
Anthropic Message/ContentBlock  ←→  provider.Message/ToolCall/ToolDef
```

**Why normalization?**
1. **Testability:** Test translation functions without network (pure functions)
2. **Provider swapping:** Change provider via env var, zero engine changes
3. **Mocking:** `FakeProvider` implements the same interface for deterministic testing

### 4.3 Cost Optimization

| Technique | Implementation | Impact |
|---|---|---|
| **Default model** | Gemini 3.1 Flash Lite (cheapest) | ~$0.075/1M input tokens |
| **Thinking OFF by default** | Gemini 3.x thinking adds 30-60s latency | 50-80% latency reduction |
| **Prompt caching** | Anthropic `cache_control`, Gemini context cache | 90% cost reduction on cached tokens |
| **Token budget** | Hard limit per conversation turn | Prevents runaway costs |

---

## 5. Tool System

### 5.1 Tool Classification

| Kind | Examples | Guardrail |
|---|---|---|
| **Read** | `ragSearch`, `listDocuments`, `readDocument`, `listTasks`, `recallMemory` | None (safe) |
| **Write** | `createTask`, `updateTask`, `saveMemory` | Validate, log |
| **Destructive** | `deleteTask` | **HITL interrupt** — user must confirm |

### 5.2 Parallel Execution

When the LLM returns multiple tool calls in one response (e.g., "search docs AND list tasks"), we execute them concurrently:

```go
// errgroup: all succeed or first error (configurable)
// Results preserved in original order (pre-allocated slice by index)
func (r *Registry) RunParallel(ctx context.Context, calls []provider.ToolCall) []CallResult {
    results := make([]CallResult, len(calls))  // pre-allocate = order preservation
    var g errgroup.Group
    for i, call := range calls {
        i, call := i, call  // pin loop variables (Go 1.22+ auto-pins, but explicit for clarity)
        results[i].Call = call
        g.Go(func() error {
            res, err := r.runOne(ctx, call)
            results[i].Result = res
            results[i].Err = err
            return nil  // tool errors don't cancel siblings
        })
    }
    g.Wait()
    return results
}
```

**Why not `sync.WaitGroup` + mutex?**
- `errgroup` = `WaitGroup` + error propagation (cleaner API)
- No mutex needed: each goroutine writes to a unique index (data-race free by construction)

### 5.3 Tool Registry

```go
type Tool interface {
    Name() string
    Description() string
    Schema() json.RawMessage
    Kind() Kind
    Execute(ctx context.Context, args json.RawMessage) (Result, error)
}
```

**Why `json.RawMessage` for Schema and Args?**
- **Deferred parsing:** Schema passed to LLM without transformation
- **Lazy validation:** Args validated inside `Execute`, not in engine
- **Zero-allocation passthrough:** `json.RawMessage` is a `[]byte` alias — no unmarshal cost

---

## 6. Memory Architecture (3-Tier)

### 6.1 Tier Design

```
┌─────────────────────────────────────────────────────────────┐
│ Tier 1: WORKING MEMORY (in-memory, 1 conversation turn)     │
│ ─────────────────────────────────────────────────────────── │
│ State.Messages + State.Scratchpad                           │
│ Lifecycle: created per chat request, discarded after        │
│ Purpose: LLM sees full conversation context for this turn   │
│ Size: ~10-50 messages                                       │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼ (summarize node, P7)
┌─────────────────────────────────────────────────────────────┐
│ Tier 2: EPISODIC MEMORY (in-memory, cross-turn)             │
│ ─────────────────────────────────────────────────────────── │
│ Summary of old history (1 SystemMessage)                    │
│ Purpose: Keep context window small while retaining history  │
│ Trigger: when message count > threshold (e.g., 20)          │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼ (extract node, P7)
┌─────────────────────────────────────────────────────────────┐
│ Tier 3: SEMANTIC MEMORY (persisted, cross-conversation)     │
│ ─────────────────────────────────────────────────────────── │
│ MongoDB collection `memories`                               │
│ Schema: { type, key, value, confidence, embedding[], ... }  │
│ Purpose: Remember user preferences, facts, entities forever │
│ Recall: structured (type+key) + vector (cosine similarity)  │
│ Write: LLM extracts facts → upsert (dedup by type+key)     │
└─────────────────────────────────────────────────────────────┘
```

### 6.2 Why Hybrid Recall?

Structured lookup is fast and exact ("what's the user's preferred language?"), but vector search finds semantically similar facts ("any preferences about communication style?"). We merge and deduplicate both results.

---

## 7. Safety & Guardrails

### 7.1 Defense in Depth

| Layer | Mechanism | What It Prevents |
|---|---|---|
| **Input** | Mark tool results as DATA (separate from instructions) | Prompt injection via documents |
| **Output** | Require citations when `ragSearch` was used | Hallucination without sources |
| **Tool** | HITL interrupt for `KindDestructive` | Accidental deletion |
| **Loop** | `maxSteps` + token budget | Infinite loops, runaway costs |
| **Network** | `context.Context` cancellation | Leaked goroutines on client disconnect |

### 7.2 HITL (Human-in-the-Loop) Flow

```
┌──────────────────────────────────────────────────────────┐
│ 1. LLM calls deleteTask                                  │
│ 2. Tool guardrail: Kind=Destructive → don't execute      │
│ 3. Engine emits Event{type:"interrupt", name:"deleteTask"}│
│ 4. Engine persists State to MongoDB (runId)              │
│ 5. Engine returns END (loop stops)                       │
│ 6. UI shows confirmation dialog to user                  │
│ 7. User clicks "Approve" → POST /chat/resume {runId,     │
│    decision: "approve"}                                  │
│ 8. Engine loads State from MongoDB, executes deleteTask, │
│    continues loop                                        │
└──────────────────────────────────────────────────────────┘
```

---

## 8. Roadmap

| Phase | Component | Status |
|---|---|---|
| P0 | Scaffold + CI + Docker | ✅ Complete |
| P1 | Provider (Gemini + Claude + Fake) | ✅ Complete |
| **P2** | **Agent Engine (model, route, tools, loop)** | **🔄 In Progress** |
| P3 | Tool system complete (9 tools) | Pending |
| P4 | MongoDB (Go driver) + task tools | Pending |
| P5 | RAG retrieval (Voyage + Atlas $vectorSearch) | Pending |
| P6 | Context engineering + prompt caching | Pending |
| P7 | Memory 3-tier | Pending |
| P8 | Planner + reflection nodes | Pending |
| P9 | Skills (progressive disclosure) | Pending |
| P10 | Guardrails + HITL | Pending |
| P11 | Observability (OpenTelemetry) | Pending |
| P12 | Gateway integration (TS ↔ Go) | Pending |
| P13 | Eval harness | Pending |
| P14 | Polish + docs + e2e | Pending |

---

## 9. Key Technology Choices Summary

| Decision | Choice | Why |
|---|---|---|
| Language (agent) | Go 1.24+ | Concurrency, memory, learning |
| Language (gateway) | TypeScript (Fastify) | PDF, file upload, ecosystem |
| LLM | Gemini 3.1 Flash Lite (default) + Claude (optional) | Cost optimization |
| Embedding | Voyage AI voyage-3 (1024d) | Quality/cost balance |
| Database | MongoDB Atlas (shared) | Vector search built-in, no separate vector DB |
| Transport | HTTP + SSE | Simple, browser-native, stateless agent |
| Agent framework | Self-built (no LangGraph) | Learning, control, zero dependency |
| Observability | OpenTelemetry + slog | Standard, vendor-neutral |
| CI | GitHub Actions (path-filtered) | Simple, free for public repos |
| Container | Multi-stage Docker (distroless) | Small image (~15MB), secure |
