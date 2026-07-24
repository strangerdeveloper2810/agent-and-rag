# JARVIS Development Guide

Huong dan phat trien cho developer muon dong gop vao JARVIS codebase. Bao gom cau truc du an, quy trinh test, cach them provider/agent moi, va quy uoc code.

Development guide for contributing to the JARVIS codebase. Covers project structure, testing workflow, adding new providers/agents, and code conventions.

---

## Prerequisites

| Tool | Version | Why |
|------|---------|-----|
| **Go** | 1.25+ | Agent runtime (goroutines, channels, errgroup) |
| **Node.js** | 22+ | Gateway (Fastify) + Frontend (React/Vite) |
| **pnpm** | 9+ | Monorepo package manager |
| **MongoDB** | 7+ | Database (Atlas or local) |
| **Docker** | 26+ | (Optional) Containerized deployment |

### API Keys (for full functionality)

- **Gemini API key** — [aistudio.google.com](https://aistudio.google.com) (free)
- **Anthropic API key** — [console.anthropic.com](https://console.anthropic.com)
- **Voyage AI key** — [voyageai.com](https://voyageai.com) (for RAG/embeddings)

---

## Project Structure (Deep Dive)

```
ai-agent-tut/
│
├── apps/
│   ├── api/                          # TypeScript Fastify gateway
│   │   └── src/
│   │       ├── agent/                # LangGraph agent (legacy — being replaced by Go)
│   │       │   ├── graph.ts          # StateGraph definition
│   │       │   ├── lc-tools.ts       # LangChain tools
│   │       │   └── graph-runner.ts   # Agent execution loop
│   │       ├── modules/              # Feature modules (modular monolith)
│   │       │   ├── chat/             # Chat routes + controllers
│   │       │   ├── conversations/    # CRUD for conversation history
│   │       │   ├── documents/        # Upload, chunk, embed, search
│   │       │   └── tasks/            # Task CRUD API
│   │       ├── middleware/           # Error handler, auth, rate-limit
│   │       └── lib/                  # MongoDB client, LLM clients, errors
│   │
│   └── web/                          # React SPA
│       └── src/
│           ├── components/           # Chat, DocumentList, TaskBoard
│           ├── hooks/                # useSSE, useChat, useDocuments
│           └── lib/                  # API client (singleton), types
│
├── services/
│   └── agent-go/                     # Go agent runtime (JARVIS)
│       ├── cmd/
│       │   ├── server/main.go        # HTTP server + wiring
│       │   └── jarvis/main.go        # CLI (serve, ask, chat)
│       ├── internal/
│       │   ├── agent/                # Core engine
│       │   │   ├── state.go          # State, RunInput, Node type, Runner interface
│       │   │   ├── event.go          # Event type, SSE helper constructors
│       │   │   ├── router.go         # Pure function: route(s) NodeID
│       │   │   ├── engine.go         # Run loop + dispatch
│       │   │   ├── context.go        # BuildSystemPrompt()
│       │   │   ├── node_model.go     # LLM call node
│       │   │   └── node_tools.go     # Tool execution node (fan-out)
│       │   ├── provider/             # LLM abstraction layer
│       │   │   ├── provider.go       # Provider interface
│       │   │   ├── types.go          # Message, ToolDef, StreamChunk, etc.
│       │   │   ├── fake.go           # FakeProvider (deterministic testing)
│       │   │   ├── gemini/           # Gemini adapter
│       │   │   ├── anthropic/        # Claude adapter
│       │   │   ├── ollama/           # Local Ollama adapter
│       │   │   └── factory/          # Provider selection by env
│       │   ├── tools/                # Tool system
│       │   │   ├── tool.go           # Tool interface + Kind enum
│       │   │   ├── registry.go       # Registry + RunParallel (errgroup)
│       │   │   ├── echo.go           # Echo tool (test/learning)
│       │   │   ├── files.go          # File search + read tools
│       │   │   └── web.go            # Web search + fetch tools
│       │   ├── memory/               # 3-tier memory
│       │   │   ├── store.go          # In-memory key-value store
│       │   │   ├── memory.go         # Types: Item, MemoryType, MergeMemories
│       │   │   ├── recall.go         # RecallNode: search memory by user query
│       │   │   ├── extract.go        # ExtractNode: pattern-based memory extraction
│       │   │   └── summarize.go      # SummarizeNode: condense long history
│       │   ├── orchestrator/         # Multi-agent orchestration
│       │   │   └── orchestrator.go   # Orchestrator, AgentSpec, routing, delegation
│       │   ├── guardrails/           # Safety checks
│       │   │   ├── guard.go          # CheckTool (Read/Write/Destructive)
│       │   │   └── circuit_breaker.go # Stuck loop detection
│       │   ├── mongo/                # MongoDB driver layer
│       │   ├── rag/                  # Voyage AI embedding client
│       │   ├── storage/              # Additional storage backends
│       │   │   ├── chroma/           # Chroma vector DB (optional)
│       │   │   └── sqlite/           # SQLite for local dev
│       │   ├── config/               # Env-based configuration
│       │   ├── transport/http/       # HTTP handlers
│       │   │   ├── chat.go           # POST /chat (SSE streaming)
│       │   │   └── health.go         # GET /healthz
│       │   └── observability/        # slog + OpenTelemetry
│       ├── skills/                   # SKILL.md files (progressive disclosure)
│       ├── eval/                     # Agent evaluation harness
│       ├── go.mod
│       ├── go.sum
│       └── Dockerfile                # Multi-stage distroless
│
├── docs/                             # Project documentation
│   ├── architecture/                 # Architecture deep-dives
│   ├── plans/                        # Design + implementation plans
│   └── go-patterns/                  # Go production patterns catalog
│
└── package.json                      # pnpm workspace root
```

---

## How to Run Tests

### Go Tests (agent runtime)

```bash
cd services/agent-go

# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run specific package
go test -v ./internal/agent/...
go test -v ./internal/tools/...
go test -v ./internal/memory/...

# Run with race detection
go test -race ./...

# Run with coverage
go test -cover ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Current Test Coverage

| Package | Tests | Pattern |
|---------|:-----:|---------|
| `internal/agent` | `router_test.go`, `node_model_test.go` | Table-driven routing, LLM call scenarios |
| `internal/tools` | `registry_test.go`, `files_test.go`, `web_test.go` | Tool execution, concurrency, error cases |
| `internal/provider` | `gemini_test.go`, `anthropic_test.go` | Translation functions (no network) |
| `internal/memory` | `memory_test.go` | Merge, validate, store operations |
| `internal/guardrails` | `circuit_breaker_test.go` | Stuck loop detection |
| `internal/orchestrator` | `orchestrator_test.go` | Keyword routing, delegation |
| `internal/transport/http` | `health_test.go` | Health check endpoint |
| `internal/mongo` | `objectid_test.go` | Safe ObjectID parsing |
| `internal/observability` | `observability_test.go` | Logger + tracer setup |

### TypeScript Tests (gateway + frontend)

```bash
# From monorepo root
pnpm test                    # Run all tests (Vitest)
pnpm --filter @app/api test  # API tests only
pnpm --filter @app/web test  # Frontend tests only
pnpm typecheck               # TypeScript type checking (all apps)
```

---

## How to Add a New Provider

Adding support for a new LLM provider (e.g., OpenAI, Groq, DeepSeek) requires implementing the `Provider` interface.

### Step 1: Create the adapter package

```bash
mkdir -p services/agent-go/internal/provider/openai
```

### Step 2: Implement the Provider interface

```go
// internal/provider/openai/openai.go
package openai

import (
    "context"
    "github.com/ai-agent-tut/agent-go/internal/provider"
)

type Client struct {
    apiKey string
    model  string
    // Add HTTP client, etc.
}

func New(apiKey, model string) (*Client, error) {
    if apiKey == "" {
        return nil, errors.New("OPENAI_API_KEY is required")
    }
    return &Client{apiKey: apiKey, model: model}, nil
}

func (c *Client) Name() string { return "openai" }

func (c *Client) Generate(ctx context.Context, req provider.GenerateRequest) (<-chan provider.StreamChunk, error) {
    ch := make(chan provider.StreamChunk, 64)

    go func() {
        defer close(ch)

        // 1. Translate provider.ToolDef -> OpenAI function format
        // 2. Translate provider.Message -> OpenAI message format
        // 3. Call OpenAI streaming API
        // 4. Stream back via ch as provider.StreamChunk:
        //    - Text delta -> ChunkText
        //    - Function call -> ChunkToolCall
        //    - Usage stats -> ChunkUsage
        //    - Done signal -> ChunkDone
        //    - Error -> ChunkError

        // Respect ctx for cancellation
        select {
        case <-ctx.Done():
            return
        default:
        }
    }()

    return ch, nil
}
```

### Step 3: Register in the factory

```go
// internal/provider/factory/factory.go
func New(cfg config.Config) (provider.Provider, error) {
    switch cfg.Provider {
    case "gemini":
        // ... existing ...
    case "anthropic":
        // ... existing ...
    case "openai":                                 // <-- NEW
        if cfg.OpenAIKey == "" {
            return nil, errors.New("OPENAI_API_KEY required for provider=openai")
        }
        return openai.New(cfg.OpenAIKey, cfg.OpenAIModel)
    default:
        return nil, errors.New("unknown LLM_PROVIDER: " + cfg.Provider)
    }
}
```

### Step 4: Add config fields

```go
// internal/config/config.go
type Config struct {
    // ... existing ...
    OpenAIKey   string  // <-- NEW
    OpenAIModel string  // <-- NEW
}

func Load() (Config, error) {
    c := Config{
        // ... existing ...
        OpenAIKey:   os.Getenv("OPENAI_API_KEY"),       // <-- NEW
        OpenAIModel: envOr("OPENAI_MODEL", "gpt-4o"),    // <-- NEW
    }
    // ...
}
```

### Step 5: Write translation tests

```go
// internal/provider/openai/openai_test.go
package openai

import (
    "testing"
    "github.com/ai-agent-tut/agent-go/internal/provider"
)

func TestTranslateToolDef_ToOpenAI(t *testing.T) {
    // Test that a provider.ToolDef translates correctly
    // to OpenAI's function-calling format.
    // No network calls — pure function tests.
}

func TestTranslateStreamChunk_FromOpenAI(t *testing.T) {
    // Test that OpenAI's streaming delta format
    // translates correctly to provider.StreamChunk.
}
```

### Key Principle: Provider-Agnostic Types

The engine NEVER knows which provider is being used. All provider-specific formats are translated into the normalized types in `internal/provider/types.go`:

```
Gemini Content/Part  ←→  provider.Message/ToolCall/ToolDef
Anthropic Message    ←→  provider.Message/ToolCall/ToolDef
OpenAI Message       ←→  provider.Message/ToolCall/ToolDef
```

The translation happens entirely inside each adapter package — the engine only sees the normalized types.

---

## How to Add a New Agent to the Orchestrator

### Step 1: Create the engine

```go
// In cmd/server/main.go or cmd/jarvis/main.go setup()

// 1. Create a dedicated tool registry (if needed)
researchRegistry := tools.NewRegistry()
researchRegistry.Register(tools.NewWebSearchTool(nil))
researchRegistry.Register(tools.NewWebFetchTool(nil))

// 2. Create the engine
researchEngine := agent.NewEngine(prov, researchRegistry)
researchEngine.SetMemoryNodes(
    memory.RecallNode(store),
    memory.ExtractNode(store),
    memory.SummarizeNode(),
)
```

### Step 2: Define the AgentSpec

```go
// 3. Register with orchestrator
orch.Register(&orchestrator.AgentSpec{
    Name:        "research",
    Description: "Web research specialist — searches and synthesizes information",
    Engine:      researchEngine,
    TriggerKeywords: []string{
        "research", "google", "look up", "what is", "who is",
        "find information", "search the web", "tell me about",
    },
    SystemPrompt: agent.BuildSystemPrompt(nil),
})
```

### Step 3: Test keyword routing

```go
// internal/orchestrator/orchestrator_test.go
func TestOrchestrator_ResearchRouting(t *testing.T) {
    orch := orchestrator.New()
    orch.Register(&orchestrator.AgentSpec{
        Name:            "research",
        TriggerKeywords: []string{"research", "search the web"},
        Engine:          agent.NewEngine(fakeProvider, tools.NewRegistry()),
    })
    orch.Register(&orchestrator.AgentSpec{
        Name:            "general",
        TriggerKeywords: []string{},
        Engine:          agent.NewEngine(fakeProvider, tools.NewRegistry()),
    })
    orch.SetDefault("general")

    tests := []struct {
        input    string
        wantAgent string
    }{
        {"help me research Go generics", "research"},
        {"search the web for AI news", "research"},
        {"hello how are you", "general"},
        {"what time is it", "general"},
    }

    for _, tt := range tests {
        t.Run(tt.input, func(t *testing.T) {
            // Test that orchestrator routes to the correct agent
            // orch.Run() would select the right engine
        })
    }
}
```

### Keyword Routing Rules

- **Order matters** — agents registered first have higher priority
- **Case-insensitive** — `"Go"` matches `"go"`, `"GO"`
- **Substring match** — `"debug"` matches `"help me debug this"`
- **First match wins** — if multiple agents match, the first one registered wins
- **Empty triggers = default only** — agent with empty `TriggerKeywords` can only be reached as the default

### Agent-to-Agent Delegation

```go
// Agent "general" delegates to agent "code" for a programming task
result, err := orch.Delegate(ctx, orchestrator.HandoffRequest{
    From:    "general",
    To:      "code",
    Context: "The user is asking about a Go concurrency bug",
    Task:    "Why does this goroutine leak? go func() { ch <- val }()",
})
// result.Result contains the code agent's response
// result.Usage contains token usage from the delegated call
```

---

## Code Style

### Go

```bash
# Format code
gofmt -w ./internal/... ./cmd/...

# Vet for common mistakes
go vet ./...

# Lint (optional, install golangci-lint)
golangci-lint run ./...
```

**Conventions:**

```go
// 1. Package comment at top of file
// Package agent chứa "engine" tự dựng: state machine chạy vòng
// recall->plan->model->route->tools->reflect->summarize->extract.

// 2. Exported types/functions have bilingual comments
type Engine struct {
    // prov is the LLM provider injected via constructor
    prov provider.Provider
}

// 3. Constructor always named NewXxx
func NewEngine(prov provider.Provider, registry *tools.Registry) *Engine { ... }

// 4. Interfaces are SMALL (1-3 methods)
type Provider interface {
    Generate(ctx context.Context, req GenerateRequest) (<-chan StreamChunk, error)
    Name() string
}

// 5. Errors are wrapped with context
return NodeEnd, fmt.Errorf("model: generate: %w", err)

// 6. Table-driven tests
tests := []struct {
    name  string
    input State
    want  NodeID
}{ ... }
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        got := route(&tt.input)
        if got != tt.want {
            t.Errorf("route() = %q, want %q", got, tt.want)
        }
    })
}

// 7. Context always first parameter
func (e *Engine) Run(ctx context.Context, in RunInput, emit EmitFunc) (provider.Usage, error)

// 8. Return channels are receive-only for safety
Generate(ctx context.Context, req GenerateRequest) (<-chan StreamChunk, error)

// 9. No init() functions — explicit initialization only
// 10. No global state — everything is dependency-injected
```

### TypeScript

```bash
# Lint
pnpm --filter @app/api lint
pnpm --filter @app/web lint

# Format
pnpm --filter @app/api format
```

**Conventions:**
- Zod for runtime validation of all external inputs
- Singleton HTTP client via `lib/http.ts`
- Module pattern: `controllers/` + `services/` + `repositories/` + `routes.ts`
- Async error handling via Fastify error handler middleware
- No `any` — use `unknown` and narrow types

---

## Commit Conventions

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

**Types:**

| Type | When to use |
|------|------------|
| `feat` | New feature (e.g., `feat(agent): add planner node for task decomposition`) |
| `fix` | Bug fix (e.g., `fix(tools): handle empty pattern in file.search`) |
| `docs` | Documentation (e.g., `docs: add API reference for SSE events`) |
| `refactor` | Code change that neither fixes nor adds features |
| `test` | Adding or updating tests |
| `chore` | Build, CI, dependencies (e.g., `chore: update go.mod dependencies`) |
| `perf` | Performance improvement |

**Scopes (suggested):**

| Scope | Area |
|-------|------|
| `agent` | Agent engine (state, nodes, router) |
| `provider` | LLM provider adapters |
| `tools` | Tool implementations and registry |
| `memory` | Memory system (store, recall, extract, summarize) |
| `orchestrator` | Multi-agent orchestration |
| `transport` | HTTP handlers (SSE, health) |
| `guardrails` | Safety checks + circuit breaker |
| `mongo` | MongoDB operations |
| `rag` | RAG retrieval |
| `config` | Environment configuration |
| `api` | TypeScript API gateway |
| `web` | React frontend |

**Examples:**
```
feat(tools): add calculator.eval tool for math expressions
fix(provider): handle Gemini streaming error on empty response
docs: add deployment guide with Docker Compose + Ollama
refactor(agent): extract router to pure function for testability
test(memory): add concurrency test for in-memory store
```

---

## Pull Request Checklist

Before opening a PR:

- [ ] Code compiles: `go build ./...` and `pnpm typecheck`
- [ ] Tests pass: `go test ./...` and `pnpm test`
- [ ] New code has tests (table-driven for Go, Vitest for TS)
- [ ] Documentation updated (these docs, README, or inline comments)
- [ ] No commented-out code or debugging prints
- [ ] Error messages are descriptive and bilingual (VN + EN)
- [ ] Commit messages follow conventional commit format
- [ ] Branch is rebased on latest `master`
