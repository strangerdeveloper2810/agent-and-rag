# Go Agent — Architecture Deep-Dive

> **Audience:** Go backend engineers. Assumes familiarity with Go syntax; focuses on patterns, tradeoffs, and implementation details.
> **Prerequisites:** Read [`go-agent-system-design.md`](./go-agent-system-design.md) for business context and high-level design.

---

## 1. Codebase Organization

```
services/agent-go/
├── cmd/server/main.go              # Entry point: DI, wiring, graceful shutdown
├── internal/
│   ├── agent/                      # Agent Engine (P2)
│   │   ├── state.go                # State, RunInput, Observation, Interrupt, Node type
│   │   ├── event.go                # Event, EmitFunc, helper constructors
│   │   ├── router.go               # Pure function: route(s *State) NodeID
│   │   ├── router_test.go          # Table-driven tests for routing logic
│   │   ├── node_model.go           # LLM call node (P2.3)
│   │   ├── node_tools.go           # Tool execution node (P2.4)
│   │   ├── engine.go               # Run loop + dispatch (P2.5)
│   │   └── engine_test.go          # Full scenario tests with FakeProvider
│   ├── provider/                   # LLM Abstraction Layer (P1)
│   │   ├── types.go                # Normalized Message, ToolDef, StreamChunk, etc.
│   │   ├── provider.go             # Provider interface
│   │   ├── fake.go                 # FakeProvider for deterministic testing
│   │   ├── gemini/                 # Gemini adapter (genai SDK)
│   │   │   ├── gemini.go           # Client, Generate, translation functions
│   │   │   └── gemini_test.go      # Translation function tests (no network)
│   │   ├── anthropic/              # Anthropic adapter (anthropic-sdk-go)
│   │   │   ├── anthropic.go
│   │   │   └── anthropic_test.go
│   │   └── factory/                # Provider factory (env-based selection)
│   │       └── factory.go
│   ├── tools/                      # Tool System (P3)
│   │   ├── tool.go                 # Tool interface, Kind enum, Result
│   │   ├── registry.go             # Registry, RunParallel (errgroup)
│   │   ├── registry_test.go        # Concurrency test (barrier pattern)
│   │   └── echo.go                 # Echo tool (test/learning)
│   ├── rag/                        # RAG Retrieval (P5)
│   │   ├── voyage.go               # Voyage AI embedding client
│   │   └── voyage_test.go
│   ├── memory/                     # Memory 3-Tier (P7)
│   │   ├── memory.go
│   │   └── memory_test.go
│   ├── mongo/                      # MongoDB Layer (P4)
│   │   ├── mongo.go                # Client, connection, health
│   │   ├── models.go               # Go structs (schema-of-record)
│   │   ├── objectid.go             # Safe ObjectID parsing (no panic)
│   │   ├── objectid_test.go
│   │   └── tasks.go                # Task CRUD operations
│   ├── config/                     # Configuration (P0)
│   │   └── config.go               # env → Config struct, validation
│   ├── guardrails/                 # Safety (P10)
│   ├── skills/                     # Progressive Disclosure (P9)
│   ├── transport/http/             # HTTP Transport
│   │   ├── health.go               # GET /healthz, GET /readyz
│   │   ├── health_test.go
│   │   └── chat.go                 # POST /chat (SSE streaming) — P2.6
│   └── observability/              # OTel + slog (P11)
│       ├── observability.go
│       └── observability_test.go
├── skills/                         # SKILL.md files (data)
├── eval/                           # Eval harness (P13)
├── go.mod
├── go.sum
├── Dockerfile                      # Multi-stage (distroless)
├── package.json                    # Turbo shim
└── README.md
```

### 1.1 Package Design Principles

| Principle | Manifestation |
|---|---|
| **No circular imports** | `provider` defines interface; `gemini`/`anthropic` implement it; `factory` imports all three |
| **Internal packages** | Everything under `internal/` — not importable by other modules |
| **Interfaces at call site** | `Provider` defined in `provider` package where it's used, not where it's implemented |
| **Test files alongside source** | `*_test.go` in same package (white-box for unexported functions) |
| **Fakes, not mocks** | `FakeProvider` is a real implementation with deterministic behavior; no mocking framework |

---

## 2. Concurrency Model

### 2.1 Goroutine Lifecycle

Every goroutine in this system must have a clear lifecycle:

```
┌──────────────────────────────────────────────────────────┐
│ HTTP Handler (1 goroutine per request)                   │
│   │                                                      │
│   ├─► Engine.Run (main goroutine)                        │
│   │     │                                                │
│   │     ├─► nodeModel ─► provider.Generate               │
│   │     │     │                                          │
│   │     │     └─► goroutine: stream chunks to channel    │
│   │     │         (dies when: all chunks sent OR ctx.Done│
│   │     │                                                │
│   │     ├─► nodeTools ─► registry.RunParallel            │
│   │     │     │                                          │
│   │     │     └─► errgroup: N goroutines                 │
│   │     │         (dies when: all tools complete)        │
│   │     │                                                │
│   │     └─► emit ─► SSE writer                           │
│   │           (same goroutine as handler)                │
│   │                                                      │
│   └─► ctx.Done() → ALL goroutines receive cancel signal  │
└──────────────────────────────────────────────────────────┘
```

### 2.2 Context Propagation

```go
// Rule: EVERY function that blocks MUST accept context.Context as first parameter.
// NO EXCEPTIONS.

// ✅ Correct:
func (e *Engine) Run(ctx context.Context, in RunInput, emit EmitFunc) error
func (r *Registry) RunParallel(ctx context.Context, calls []provider.ToolCall) []CallResult
func (c *Client) Generate(ctx context.Context, req GenerateRequest) (<-chan StreamChunk, error)

// ❌ Wrong:
func (e *Engine) Run(in RunInput, emit EmitFunc) error  // No ctx = can't cancel
```

**Context chain:**
```
http.Request.Context()          (auto-cancelled on client disconnect)
    └─► Engine.Run(ctx, ...)    (passes to every node)
            ├─► nodeModel(ctx, ...)
            │       └─► provider.Generate(ctx, ...)
            │               └─► goroutine: select { case ch <- chunk: case <-ctx.Done(): return }
            └─► nodeTools(ctx, ...)
                    └─► registry.RunParallel(ctx, ...)
                            └─► errgroup: ctx passed to each Execute(ctx, ...)
```

### 2.3 Channel Patterns

#### Pattern 1: Streaming with cancellation

```go
// Provider: write-only channel, respects ctx
func streamResults(ctx context.Context, chunks []StreamChunk) <-chan StreamChunk {
    ch := make(chan StreamChunk, len(chunks))  // buffered = no goroutine leak if consumer exits early
    go func() {
        defer close(ch)  // ALWAYS close — signals consumer to stop ranging
        for _, c := range chunks {
            select {
            case ch <- c:       // send succeeded
            case <-ctx.Done():  // consumer cancelled
                return
            }
        }
    }()
    return ch
}

// Consumer:
for chunk := range stream {  // auto-exits when channel is closed
    process(chunk)
}
```

#### Pattern 2: Fan-out with ordered results

```go
// Pre-allocate slice by index = order preservation without mutex
results := make([]CallResult, len(calls))
for i, call := range calls {
    i, call := i, call  // pin variables (critical pre-Go-1.22)
    g.Go(func() error {
        results[i] = execute(call)  // unique index = no data race
        return nil
    })
}
```

### 2.4 sync/errgroup Cheat Sheet

| Tool | When to Use |
|---|---|
| `sync.WaitGroup` | Fire-and-forget N goroutines, no error propagation |
| `errgroup.Group` | N goroutines, FIRST error cancels all others |
| `errgroup.WithContext` | N goroutines + derived context auto-cancelled on first error |
| Manual `chan error` | Custom error aggregation (e.g., collect all errors, not just first) |

**Our choice for tool execution:** `errgroup.Group` (not `WithContext`). Why?
- Tool errors don't cancel sibling tools (1 tool failing ≠ all tools should stop)
- We set `return nil` in each goroutine → `g.Wait()` never returns error
- Errors are stored in `CallResult.Err` for the LLM to handle

---

## 3. Error Handling Strategy

### 3.1 Error Categories

| Category | Handling | Example |
|---|---|---|
| **Fatal** | Return error, abort loop, emit `{type:"error"}` | Provider API key invalid |
| **Recoverable** | Feed error to LLM as observation, continue loop | Tool execution timeout |
| **Expected** | Graceful handling (no error event) | RagSearch returning 0 results |
| **Cancellation** | `ctx.Err()` → clean up, return early | Client disconnects |

### 3.2 Error Wrapping Convention

```go
// ✅ Use %w to wrap — preserves error chain for errors.Is/errors.As
return fmt.Errorf("gemini: generate stream: %w", err)

// ✅ Define sentinel errors for caller checks
var ErrDocumentNotFound = errors.New("document not found")

// ✅ Custom error types for structured context
type NotFoundError struct {
    Name string
}
func (e *NotFoundError) Error() string {
    return "tools: tool not found: " + e.Name
}

// ❌ Don't use %v (breaks error chain)
return fmt.Errorf("gemini: generate stream: %v", err)

// ❌ Don't panic for expected errors
if err != nil {
    panic(err)  // NEVER in production code
}
```

### 3.3 ObjectID Validation (Defense at Boundary)

```go
// ❌ WRONG — panics if invalid:
objID, _ := primitive.ObjectIDFromHex(id)

// ✅ CORRECT — returns error:
func ToObjectID(id string) (primitive.ObjectID, error) {
    objID, err := primitive.ObjectIDFromHex(id)
    if err != nil {
        return primitive.NilObjectID, fmt.Errorf("invalid ObjectID %q: %w", id, err)
    }
    return objID, nil
}
```

---

## 4. Testing Strategy

### 4.1 Test Pyramid

```
           ┌─────────┐
           │   E2E   │  P14: Happy path (docker-compose + real LLM)
           │  (P14)  │
           └─────────┘
        ┌───────────────┐
        │  Integration   │  P4/P5: Real MongoDB + Voyage (optional testcontainers)
        │  (P4/P5 opt)   │
        └───────────────┘
    ┌───────────────────────┐
    │    Engine Tests        │  P2: FakeProvider + Echo tool (no network)
    │    (Scenario)          │
    └───────────────────────┘
┌───────────────────────────────┐
│     Unit Tests (Pure Functions)│  Router, translation, validation (0 I/O)
└───────────────────────────────┘
```

### 4.2 Testing Pure Functions (No I/O)

```go
// Router = pure function → test with table-driven pattern
func TestRoute(t *testing.T) {
    tests := []struct {
        name string
        s    *State
        want NodeID
    }{
        {"tool calls → tools", &State{...}, NodeTools},
        {"final answer → end", &State{...}, NodeEnd},
        {"maxSteps → end",   &State{Step: 12, MaxSteps: 12}, NodeEnd},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            if got := route(tt.s); got != tt.want {
                t.Errorf("route() = %q, want %q", got, tt.want)
            }
        })
    }
}
```

### 4.3 Testing I/O Functions (Fake, Don't Mock)

```go
// Instead of mocking provider.Generate, use a deterministic fake:
fake := provider.NewFake(
    provider.StreamChunk{Kind: provider.ChunkText, Text: "Hello"},
    provider.StreamChunk{Kind: provider.ChunkDone},
)
// fake.Generate() returns a real channel with these exact chunks
// No mock framework, no expectation verification — just data

// Pros:
// - Readable: see the EXACT data the test uses
// - No magic: no mock.Expect(), no Verify()
// - Reusable: same FakeProvider used in router, engine, and SSE tests
```

### 4.4 Testing Concurrency (Barrier Pattern)

```go
// barrierTool proves RunParallel is actually concurrent:
// Each goroutine calls wg.Done(), then waits for ALL goroutines.
// If sequential: second goroutine never starts → deadlock → test timeout.
// If parallel: all goroutines start → all call Done → all pass barrier.
type barrierTool struct {
    wg *sync.WaitGroup
}
func (b barrierTool) Execute(ctx context.Context, args json.RawMessage) (Result, error) {
    b.wg.Done()     // "I'm here"
    b.wg.Wait()     // Wait for everyone else
    return Result{Content: "ok"}, nil
}
```

---

## 5. Performance Considerations

### 5.1 Memory Allocations

| Pattern | Allocations | Better Alternative |
|---|---|---|
| `string += chunk` in loop | O(n²) strings allocated | `strings.Builder` |
| `[]Message` append one-by-one | Multiple slice grows | Pre-allocate: `make([]Message, 0, expectedSize)` |
| `json.Unmarshal` then `json.Marshal` | Double serialization | Use `json.RawMessage` for passthrough |

### 5.2 Channel Buffer Sizing

```go
// Rule: buffer = worst-case items producer can send before consumer reads
ch := make(chan StreamChunk, 16)  // LLM chunks: moderate volume, frequent reads
ch := make(chan Event, 64)        // Engine events: higher volume, SSE writer may lag

// Buffered > unbuffered when: producer and consumer have different speeds
// Unbuffered > buffered when: you want back-pressure (slow consumer → slow producer)
```

### 5.3 Goroutine Leak Prevention

```go
// ❌ LEAK: goroutine blocks forever on send if consumer stops reading
go func() {
    ch <- result  // blocks forever if no reader
}()

// ✅ SAFE: select with ctx.Done()
go func() {
    select {
    case ch <- result:
    case <-ctx.Done():
        return
    }
}()
```

---

## 6. Why Each Tech Choice (Interview Talking Points)

### 6.1 `net/http` over Gin/Chi/Fiber

Go 1.22 introduced enhanced routing (`mux.HandleFunc("GET /healthz", handler)`). For our scale (internal service, <1000 req/s), the standard library is sufficient. No framework dependency = fewer CVEs, faster build, simpler debugging.

**When would we switch?** At >10K req/s with complex middleware chains (rate limiting, auth, tracing) → consider `chi` (lightweight) or `fiber` (performance). Not needed now.

### 6.2 MongoDB over PostgreSQL

Vector search is the killer feature. Atlas `$vectorSearch` on `documents.embedding` and `memories.embedding` avoids deploying a separate vector database (Pinecone, Weaviate, Qdrant). For the relational parts (conversations, messages), MongoDB's document model maps naturally to our JSON-shaped data.

**Tradeoff:** No JOINs → denormalize conversation title into messages. Acceptable for our scale.

### 6.3 Go stdlib `testing` over Testify

Zero external test dependencies. The `testing` package + `t.Run` subtests + `t.Errorf` is sufficient. Testify adds `assert.Equal` but also adds a dependency that can break. We're learning Go — learn the standard library first.

### 6.4 JSON Schema as `json.RawMessage` over `map[string]any`

```go
// RawMessage: zero-alloc passthrough
Schema() json.RawMessage  // just []byte — no unmarshal

// map[string]any: allocates on every call
Schema() map[string]any  // requires json.Unmarshal + memory for map
```

For tool definitions passed to LLM (which are just forwarded as JSON), `RawMessage` avoids unnecessary parsing.

---

## 7. Dependency Graph

```
cmd/server/main.go
  ├── internal/config          (env → Config struct)
  ├── internal/transport/http  (POST /chat SSE, health)
  │     └── internal/agent     (Engine)
  ├── internal/provider/factory
  │     ├── internal/provider/gemini
  │     └── internal/provider/anthropic
  ├── internal/mongo           (MongoDB client)
  └── internal/observability   (OTel + slog)

internal/agent
  ├── internal/provider        (Provider interface + types)
  └── internal/tools           (Tool interface + Registry)

internal/tools
  └── internal/provider        (ToolDef for LLM)

internal/rag
  └── internal/mongo           (read documents collection)
```

**No cycles.** Every import goes from concrete → abstract (provider interface, Tool interface).

---

## 8. Further Reading (for Mid-Level Go Interview Prep)

| Topic | Why It Matters |
|---|---|
| **Go Memory Model** | Understanding happens-before for goroutine communication |
| **Escape Analysis** | Know when heap vs stack allocation happens (`go build -gcflags="-m"`) |
| **pprof** | CPU, memory, goroutine profiling (`net/http/pprof`) |
| **Race Detector** | Always run `go test -race` (finds data races goroutine doesn't show) |
| **Scheduler** | GMP model: Goroutines, Machine threads, Processors |
| **GC Tuning** | `GOGC`, `GOMEMLIMIT` for container environments |
