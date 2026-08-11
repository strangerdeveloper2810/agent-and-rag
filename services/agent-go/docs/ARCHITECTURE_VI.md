# 🧠 J.A.R.V.I.S. Go Agent — Tài Liệu Kiến Trúc Toàn Diện

> **Mục tiêu của tài liệu này:** Giải thích TOÀN BỘ kiến trúc Go agent từ A-Z, dành cho người đang học cả Go lẫn cách build Agent. Mọi thứ được viết bằng tiếng Việt, giải thích từ dễ đến khó.

---

## Mục Lục

1. [Tổng quan: Agent là gì và J.A.R.V.I.S. hoạt động thế nào](#1-tổng-quan)
2. [Cấu trúc thư mục](#2-cấu-trúc-thư-mục)
3. [Luồng hoạt động chính (ReAct Loop)](#3-luồng-hoạt-động-chính)
4. [Từng package giải thích chi tiết](#4-chi-tiết-từng-package)
   - [4.1 Entry Points — cmd/](#41-entry-points)
   - [4.2 Provider — Kết nối LLM](#42-provider)
   - [4.3 Agent Engine — Trái tim của hệ thống](#43-agent-engine)
   - [4.4 Tools — 20 công cụ cho agent](#44-tools)
   - [4.5 Memory — Hệ thống ghi nhớ 3 tầng](#45-memory)
   - [4.6 Orchestrator — Điều phối nhiều agent](#46-orchestrator)
   - [4.7 Skills — Progressive Disclosure](#47-skills)
   - [4.8 RAG — Tìm kiếm ngữ nghĩa](#48-rag)
   - [4.9 Storage — Lưu trữ](#49-storage)
   - [4.10 Guardrails — Lớp bảo vệ](#410-guardrails)
   - [4.11 MCP — Model Context Protocol](#411-mcp)
   - [4.12 Transport/HTTP — Server](#412-transport)
   - [4.13 Các package phụ trợ](#413-phụ-trợ)
5. [Go Patterns quan trọng trong codebase](#5-go-patterns)
6. [Sơ đồ tổng thể](#6-sơ-đồ-tổng-thể)

---

## 1. Tổng quan

### Agent là gì?

```
NGƯỜI DÙNG: "Tạo cho tôi một file notes về Go"
         │
         ▼
    ┌─────────────────────────────────────────┐
    │               AGENT                      │
    │                                          │
    │  1. Suy nghĩ: "Cần tạo file .md về Go"   │
    │  2. Hành động: Gọi tool file.write        │
    │  3. Quan sát: Tool báo thành công         │
    │  4. Suy nghĩ tiếp: "Đã xong, báo lại"     │
    │  5. Trả lời: "Đã tạo file Go notes.md"    │
    └─────────────────────────────────────────┘
```

**Agent = LLM được trang bị công cụ (tools) và khả năng tự quyết định.** Thay vì chỉ trả lời từ kiến thức có sẵn, agent có thể:
- Gọi công cụ bên ngoài (tìm kiếm web, đọc/ghi file, chạy lệnh...)
- Tự suy nghĩ nhiều bước để giải quyết vấn đề phức tạp
- Ghi nhớ thông tin qua các lần trò chuyện

### J.A.R.V.I.S. dùng pattern gì?

**ReAct (Reasoning + Acting)** — đây là pattern phổ biến nhất để build agent:

```
     ┌──────────┐     ┌──────────┐     ┌──────────┐
     │  SUY NGHĨ │────▶│ HÀNH ĐỘNG │────▶│ QUAN SÁT │
     │ (Reason)  │     │  (Act)    │     │ (Observe)│
     └──────────┘     └──────────┘     └─────┬────┘
          ▲                                   │
          └───────────────────────────────────┘
                     Lặp lại cho đến khi xong
```

Trong code, vòng lặp này được hiện thực trong `internal/agent/engine.go`.

---

## 2. Cấu trúc thư mục

```
services/agent-go/
├── go.mod                          # Khai báo module + dependencies
├── go.sum                          # Checksum các dependencies
├── Dockerfile                      # Build image cho production
├── .env                            # Biến môi trường local
├── skills/                         # 25 file SKILL.md (progressive disclosure)
│   ├── code-review/SKILL.md
│   ├── deep-research/SKILL.md
│   └── ...
├── cmd/                            # Entry points
│   ├── jarvis/main.go              # CLI: chạy từ terminal
│   └── server/main.go              # HTTP server
└── internal/                       # Code chính (không export ra ngoài)
    ├── agent/                      # 🔥 TRÁI TIM: ReAct engine
    ├── provider/                   # Kết nối LLM (Claude, Gemini, Ollama)
    ├── tools/                      # 20 công cụ agent có thể dùng
    ├── memory/                     # Hệ thống ghi nhớ
    ├── orchestrator/               # Điều phối nhiều agent
    ├── skills/                     # Load skill từ disk
    ├── rag/                        # Embedding + vector search
    ├── storage/                    # SQLite + in-memory vector store
    ├── guardrails/                 # Bảo vệ (circuit breaker, input check)
    ├── mcp/                        # Model Context Protocol client
    ├── mongo/                      # MongoDB models + CRUD
    ├── proactive/                  # Cron scheduler cho scheduled tasks
    ├── personality/                # Cá tính hóa agent
    ├── eval/                       # Đánh giá chất lượng agent
    ├── metrics/                    # Đếm request, token, latency
    ├── middleware/                  # HTTP middleware (tenant)
    ├── config/                     # Load config từ env
    ├── transport/http/             # HTTP handlers (chat, health, suggestions)
    └── observability/              # Logging + tracing
```

---

## 3. Luồng hoạt động chính (ReAct Loop)

Đây là flow QUAN TRỌNG NHẤT cần hiểu. Mỗi lần user gửi tin nhắn, agent chạy qua các bước sau:

```
USER INPUT: "Tìm hiểu về Go generics rồi lưu vào file notes/go-generics.md"
│
▼
┌──────────────────────────────────────────────────────────────────┐
│ BƯỚC 1: RECALL (Gợi nhớ)                                        │
│   - Kiểm tra memory store xem có thông tin liên quan không        │
│   - VD: "user thích ghi chú ngắn gọn"                             │
└────────────────────────────┬─────────────────────────────────────┘
                             ▼
┌──────────────────────────────────────────────────────────────────┐
│ BƯỚC 2: MODEL (Gọi LLM suy nghĩ)                                 │
│   - Gửi system prompt + history + user input cho LLM              │
│   - LLM trả về: HOẶC text answer HOẶC tool calls                  │
│                                                                   │
│   LLM response: "Tôi sẽ search web trước"                         │
│   + tool_call: web.search("Go generics")                          │
└────────────────────────────┬─────────────────────────────────────┘
                             ▼
┌──────────────────────────────────────────────────────────────────┐
│ BƯỚC 3: ROUTER (Quyết định bước tiếp theo)                       │
│   - Có tool call nào chưa được thực thi không? → sang TOOLS       │
│   - Tất cả tool đã chạy xong? → quay lại MODEL                   │
│   - Đã vượt max steps? → EXTRACT → END                           │
│   - Không có tool call nào? → EXTRACT → END                      │
└────────────────────────────┬─────────────────────────────────────┘
                             ▼
┌──────────────────────────────────────────────────────────────────┐
│ BƯỚC 4: TOOLS (Thực thi công cụ)                                 │
│   - Chạy TẤT CẢ tool calls SONG SONG (parallel)                   │
│   - web.search("Go generics") → kết quả tìm kiếm                  │
│   - Ghi kết quả vào state → quay lại MODEL                        │
└────────────────────────────┬─────────────────────────────────────┘
                             ▼
                        (quay lại BƯỚC 2)
                             │
                             ▼
┌──────────────────────────────────────────────────────────────────┐
│ MODEL lần 2: "Đã có kết quả search. Giờ tôi sẽ tạo file."         │
│ + tool_call: file.write("notes/go-generics.md", "...nội dung...") │
└────────────────────────────┬─────────────────────────────────────┘
                             ▼
                        (ROUTER → TOOLS → MODEL)
                             │
                             ▼
┌──────────────────────────────────────────────────────────────────┐
│ MODEL lần 3: "Đã tạo file thành công! Nội dung bao gồm..."        │
│ (không có tool calls → kết thúc)                                  │
└────────────────────────────┬─────────────────────────────────────┘
                             ▼
┌──────────────────────────────────────────────────────────────────┐
│ BƯỚC 5: EXTRACT (Trích xuất thông tin cần nhớ)                   │
│   - Dùng regex tìm patterns như "tôi tên là...", "tôi thích..."   │
│   - Lưu vào memory store cho lần sau                              │
└────────────────────────────┬─────────────────────────────────────┘
                             ▼
┌──────────────────────────────────────────────────────────────────┐
│ END: Trả kết quả về cho user                                      │
└──────────────────────────────────────────────────────────────────┘
```

**Code tương ứng trong engine.go:**
```go
func (e *Engine) Run(ctx context.Context, in RunInput, emit EmitFunc) (Usage, error) {
    s := newState(in)        // Khởi tạo state
    node := NodeRecall       // Bắt đầu từ recall

    for {
        // Gửi event cho client (SSE streaming)
        emit(StepEvent(node))

        // Chạy node hiện tại
        next, err := e.dispatch(ctx, node, s, emit)
        if err != nil {
            return s.Usage, err
        }

        // Router quyết định node tiếp theo
        node = next
        if node == NodeEnd {
            break  // Thoát vòng lặp
        }
    }

    return s.Usage, nil
}
```

---

## 4. Chi tiết từng package

### 4.1 Entry Points

Có 2 cách chạy agent:

#### `cmd/server/main.go` — HTTP Server (Production)

```go
func main() {
    // 1. Load config từ biến môi trường (.env)
    cfg := config.Load()

    // 2. Tạo LLM provider (Gemini, Claude, hoặc auto-fallback)
    prov, _ := factory.New(cfg.Provider)

    // 3. Tạo tool registry, đăng ký ~20 tools
    reg := tools.NewRegistry()
    reg.Register(tools.NewWebSearchTool(httpClient))
    reg.Register(tools.NewFileWriteTool(allowedPaths))
    // ... đăng ký tất cả tools

    // 4. Kết nối MongoDB (cho RAG)
    mongoClient, _ := mongo.Connect(ctx, cfg.MongoURI, cfg.MongoDB)

    // 5. Tạo memory store
    memStore := memory.NewStore()

    // 6. Load skills từ disk
    skillLoader, _ := skills.NewLoader("skills/")

    // 7. Tạo 3 engine chuyên biệt
    generalEngine := agent.NewEngine(prov, reg)
    generalEngine.SetSystemPrompt(buildGeneralPrompt(memStore, skillLoader))
    generalEngine.SetMemoryNodes(
        memory.RecallNode(memStore),    // Gợi nhớ
        memory.ExtractNode(memStore),   // Trích xuất
        memory.SummarizeNode(),         // Tóm tắt
    )

    codeEngine := agent.NewEngine(prov, reg)
    codeEngine.SetSystemPrompt(buildCodePrompt(...))

    researchEngine := agent.NewEngine(prov, reg)
    researchEngine.SetSystemPrompt(buildResearchPrompt(...))

    // 8. Tạo orchestrator để route
    orch := orchestrator.New()
    orch.Register(orchestrator.AgentSpec{
        Name:            "general",
        Engine:          generalEngine,
        TriggerKeywords: nil,  // default
    })
    orch.Register(orchestrator.AgentSpec{
        Name:            "code",
        Engine:          codeEngine,
        TriggerKeywords: []string{"code", "bug", "debug", "go", "python", "refactor"},
    })
    orch.Register(orchestrator.AgentSpec{
        Name:            "research",
        Engine:          researchEngine,
        TriggerKeywords: []string{"search", "research", "tìm hiểu", "how to"},
    })

    // 9. Khởi động HTTP server
    http.Handle("/healthz", Healthz)
    http.Handle("/chat", tenantMiddleware(ChatHandler(orch)))
    http.ListenAndServe(":8080", nil)
}
```

#### `cmd/jarvis/main.go` — CLI (Development)

```bash
# One-shot: hỏi 1 câu, nhận kết quả, thoát
jarvis ask "thời tiết hôm nay thế nào"

# Interactive chat: chat qua lại như ChatGPT
jarvis chat
```

---

### 4.2 Provider — Kết nối LLM

Package `internal/provider/` là lớp trừu tượng để nói chuyện với LLM. Mục tiêu: **code agent không cần biết đang dùng Claude hay Gemini**.

#### Interface chính

```go
// provider.go - Mọi LLM provider phải implement interface này
type Provider interface {
    Generate(ctx context.Context, req GenerateRequest) (<-chan StreamChunk, error)
    Name() string
}
```

#### Các kiểu dữ liệu dùng chung (types.go)

```go
type Role string
const (
    RoleSystem    Role = "system"     // Hướng dẫn cho AI
    RoleUser      Role = "user"       // Tin nhắn từ người dùng
    RoleAssistant Role = "assistant"  // Phản hồi từ AI
    RoleTool      Role = "tool"       // Kết quả từ tool
)

type Message struct {
    Role       Role
    Content    string
    ToolCalls  []ToolCall     // Nếu AI muốn gọi tool
    ToolCallID string         // Nếu đây là kết quả của tool
}

type ToolDef struct {
    Name        string
    Description string
    Schema      map[string]any  // JSON Schema cho tham số
}

type GenerateRequest struct {
    System  string      // System prompt
    Messages []Message  // Lịch sử chat
    Tools    []ToolDef  // Các tool có sẵn
}

type StreamChunk struct {
    Kind StreamChunkKind  // Text, ToolCall, Usage, Done, Error
    Text string
    // ...
}
```

#### Factory Pattern — Tự chọn provider

```go
// factory/factory.go
func New(cfg Config) (Provider, error) {
    switch cfg.Mode {
    case "gemini":
        return gemini.New(cfg.GeminiKey, cfg.GeminiModel)
    case "anthropic":
        return anthropic.New(cfg.AnthropicKey, cfg.AnthropicModel)
    case "auto":
        // Thử Gemini trước, fallback sang Claude nếu lỗi
        providers := []provider.Provider{}
        if cfg.GeminiKey != "" {
            providers = append(providers, gemini.New(cfg.GeminiKey, cfg.GeminiModel))
        }
        if cfg.AnthropicKey != "" {
            providers = append(providers, anthropic.New(cfg.AnthropicKey, cfg.AnthropicModel))
        }
        return fallback.New(5*time.Minute, providers...)
    }
}
```

#### Các adapter cụ thể

| File | Provider | Cách hoạt động |
|------|----------|----------------|
| `gemini/gemini.go` | Google Gemini | Dùng Google GenAI SDK (`google.golang.org/genai`) |
| `gemini/cache.go` | Gemini Context Cache | Cache system prompt + tools để giảm 90% chi phí |
| `anthropic/anthropic.go` | Claude (Anthropic) | Dùng `anthropic-sdk-go` |
| `ollama/ollama.go` | Ollama (local) | Gọi HTTP REST đến Ollama server |
| `ollama/embed.go` | Ollama Embedding | Gọi `/api/embed` với model `nomic-embed-text` |
| `fallback/fallback.go` | Failover chain | Thử provider 1, nếu lỗi → provider 2, kèm cooldown |
| `factory/factory.go` | Factory | Tạo provider dựa trên config |
| `fake.go` | Test double | Trả về kết quả định sẵn, dùng khi test |

#### Cách LLM gọi tool (streaming)

```
1. Agent gửi GenerateRequest (có system prompt + messages + tool definitions)
2. Provider gọi API của LLM
3. LLM stream từng chunk về:
   - Text chunk: "Tôi sẽ tra cứu..."
   - ToolCall chunk: {name: "web.search", args: {query: "Go generics"}}
   - Usage chunk: {input_tokens: 500, output_tokens: 50}
   - Done chunk: kết thúc stream
4. Agent nhận ToolCalls → thực thi → gửi kết quả lại → LLM tiếp tục
```

---

### 4.3 Agent Engine — Trái tim

Package `internal/agent/` là phần quan trọng nhất.

#### State Machine

```go
// state.go - Các node trong state machine
type NodeID string
const (
    NodeRecall    NodeID = "recall"     // Gợi nhớ từ memory
    NodePlan      NodeID = "plan"       // (chưa dùng) Lập kế hoạch
    NodeModel     NodeID = "model"      // Gọi LLM
    NodeTools     NodeID = "tools"      // Thực thi tools
    NodeReflect   NodeID = "reflect"    // (chưa dùng) Tự đánh giá
    NodeSummarize NodeID = "summarize"  // Tóm tắt hội thoại dài
    NodeExtract   NodeID = "extract"    // Trích xuất thông tin để nhớ
    NodeInterrupt NodeID = "interrupt"  // Dừng để xin phép người dùng
    NodeEnd       NodeID = "end"        // Kết thúc
)

// Mỗi node là một hàm: nhận state, trả về node tiếp theo
type Node func(ctx context.Context, s *State, emit EmitFunc) (NodeID, error)
```

#### State — Bộ nhớ làm việc của một lần chạy

```go
// state.go
type State struct {
    Messages   []provider.Message  // Toàn bộ lịch sử hội thoại
    Scratchpad []Observation       // Kết quả các tool calls trong lượt này
    Step       int                 // Đếm số bước đã chạy
    MaxSteps   int                 // Giới hạn số bước (mặc định 10)
    Usage      Usage               // Tổng token đã dùng
    Interrupt  *Interrupt          // Nếu cần xin phép user
}
```

#### Engine — Điều phối vòng lặp

```go
// engine.go
type Engine struct {
    provider         provider.Provider    // LLM
    registry         *tools.Registry      // Các tool có sẵn
    systemPrompt     string               // System prompt
    recallFn         Node                 // Hàm gợi nhớ (có thể thay đổi)
    extractFn        Node                 // Hàm trích xuất (có thể thay đổi)
    summarizeFn      Node                 // Hàm tóm tắt (có thể thay đổi)
    circuitBreaker   *guardrails.CircuitBreaker
    maxContextTokens int                  // Giới hạn context (mặc định 100K)
    dynamicThinking  DynamicThinkingConfig
}
```

#### Router — Quyết định bước tiếp theo

Đây là hàm **thuần túy** (pure function) — không I/O, không side effect:

```go
// router.go
func route(s *State) NodeID {
    // 1. Có interrupt đang chờ? → Dừng lại
    if s.Interrupt != nil {
        return NodeInterrupt
    }

    // 2. Đã vượt max steps? → Kết thúc
    if s.Step >= s.MaxSteps {
        return NodeEnd
    }

    // 3. Lấy message cuối cùng từ assistant
    last := s.LastAssistant()
    if last == nil {
        return NodeExtract  // Không có gì → kết thúc
    }

    // 4. Đếm tool calls chưa có kết quả
    unanswered := countUnanswered(s, last)
    if unanswered > 0 {
        return NodeTools  // Còn tool chưa chạy → chạy đi
    }

    // 5. Không có tool calls nào → kết thúc
    if len(last.ToolCalls) == 0 {
        return NodeExtract
    }

    // 6. Tất cả tools đã có kết quả → quay lại LLM
    return NodeModel
}
```

#### node_model.go — Gọi LLM

```go
func nodeModel(ctx context.Context, eng modelEngine, s *State, emit EmitFunc) (NodeID, error) {
    // 1. Nếu context quá dài → cắt bớt
    if s.TotalTokens > eng.getMaxContextTokens() {
        trimmed := trimContext(s, eng.getMaxContextTokens())
        s.TrimmedTokens += trimmed
    }

    // 2. Xác định thinking level (Gemini-specific)
    thinkingLevel := ResolveThinking(eng.getDynamicThinking(), ..., s.Step)

    // 3. Gọi provider
    req := provider.GenerateRequest{
        System:   eng.getSystemPrompt(),
        Messages: s.Messages,
        Tools:    eng.getRegistry().ToolDefs(),
    }
    stream, err := eng.getProvider().Generate(ctx, req)

    // 4. Đọc stream, phân loại chunks
    var toolCalls []provider.ToolCall
    var textContent strings.Builder
    for chunk := range stream {
        switch chunk.Kind {
        case provider.KindText:
            textContent.WriteString(chunk.Text)
            emit(TextEvent(chunk.Text))  // Stream text cho client
        case provider.KindToolCall:
            toolCalls = append(toolCalls, chunk.ToolCall)
            emit(ToolStartEvent(chunk.ToolCall.Name))
        case provider.KindUsage:
            s.Usage.Add(chunk.Usage)
            emit(UsageEvent(s.Usage))
        }
    }

    // 5. Lưu message của assistant vào state
    s.Messages = append(s.Messages, provider.Message{
        Role:      provider.RoleAssistant,
        Content:   textContent.String(),
        ToolCalls: toolCalls,
    })

    s.Step++
    return route(s), nil  // Nhờ router quyết định bước tiếp
}
```

#### node_tools.go — Thực thi tools song song

```go
func nodeTools(ctx context.Context, eng toolsEngine, s *State, emit EmitFunc) (NodeID, error) {
    last := s.LastAssistant()

    // Chạy TẤT CẢ tool calls SONG SONG bằng errgroup
    results := eng.getRegistry().RunParallel(ctx, last.ToolCalls)

    // Ghi từng kết quả vào state
    for _, result := range results {
        s.AppendObservation(Observation{
            CallID: result.Call.ID,
            Name:   result.Call.Name,
            Output: result.Result.Content,
            Error:  result.Err,
        })
        emit(ToolEndEvent(result.Call.Name, result.Result.Content))
    }

    return route(s), nil  // Luôn quay lại model
}
```

#### Event system — Streaming cho client

```go
// event.go
type EventType string
const (
    EventStep      EventType = "step"       // Bắt đầu 1 node
    EventText      EventType = "text"       // Text từ LLM (stream)
    EventToolStart EventType = "tool_start" // Bắt đầu chạy tool
    EventToolEnd   EventType = "tool_end"   // Kết quả tool
    EventError     EventType = "error"      // Lỗi
    EventDone      EventType = "done"       // Hoàn thành
    EventMemory    EventType = "memory"     // Thông tin gợi nhớ
    EventUsage     EventType = "usage"      // Token usage
)

// EmitFunc là callback để gửi event cho client
type EmitFunc func(Event)
```

---

### 4.4 Tools — 20 Công cụ

Package `internal/tools/` chứa tất cả công cụ agent có thể dùng.

#### Interface Tool

```go
// tool.go
type Kind string
const (
    KindRead       Kind = "read"        // An toàn, luôn được phép
    KindWrite      Kind = "write"       // Ghi file, cần log lại
    KindDestructive Kind = "destructive" // Nguy hiểm, cần xác nhận
)

type Result struct {
    Content string  // Text để đưa vào LLM context
    Meta    any     // Metadata tùy chọn (JSON)
}

type Tool interface {
    Name() string                              // Tên tool
    Description() string                       // Mô tả cho LLM biết khi nào dùng
    Schema() map[string]any                    // JSON Schema tham số
    Kind() Kind                                // Mức độ an toàn
    Execute(ctx context.Context, args map[string]any) (Result, error)
}
```

#### Registry — Quản lý tools

```go
// registry.go
type Registry struct {
    tools map[string]Tool      // name → Tool
    order []string             // Giữ thứ tự đăng ký
}

func (r *Registry) ToolDefs() []provider.ToolDef {
    // Trả về danh sách tool definitions để gửi cho LLM
    defs := make([]provider.ToolDef, 0, len(r.order))
    for _, name := range r.order {
        t := r.tools[name]
        defs = append(defs, provider.ToolDef{
            Name:        t.Name(),
            Description: t.Description(),
            Schema:      t.Schema(),
        })
    }
    return defs
}

// Chạy nhiều tool calls SONG SONG
func (r *Registry) RunParallel(ctx context.Context, calls []ToolCall) []CallResult {
    results := make([]CallResult, len(calls))
    g, ctx := errgroup.WithContext(ctx)  // Dùng errgroup để chạy parallel

    for i, call := range calls {
        i, call := i, call  // QUAN TRỌNG: capture biến trong closure
        g.Go(func() error {
            tool := r.Get(call.Name)
            if tool == nil {
                results[i] = CallResult{Call: call, Err: NotFoundError{call.Name}}
                return nil  // Không fail cả group
            }
            result, err := tool.Execute(ctx, call.Args)
            results[i] = CallResult{Call: call, Result: result, Err: err}
            return nil  // Không fail cả group
        })
    }

    g.Wait()  // Đợi tất cả hoàn thành
    return results
}
```

#### Bảng tổng hợp 20 tools

| # | Tool | File | Mức độ | Chức năng |
|---|------|------|--------|-----------|
| 1 | `echo` | `echo.go` | Read | Trả về tham số nguyên mẫu (dùng test) |
| 2 | `file.search` | `files.go` | Read | Tìm file bằng glob pattern |
| 3 | `file.read` | `files.go` | Read | Đọc nội dung file (max 24K chars) |
| 4 | `file.write` | `file_write.go` | Write | Ghi file (max 100KB, trong thư mục cho phép) |
| 5 | `shell.exec` | `shell.go` | Destructive | Chạy lệnh shell (30s timeout, 8K output) |
| 6 | `web.search` | `web.go` | Read | Tìm kiếm web (thử Wikipedia → Google → DDG) |
| 7 | `web.fetch` | `web.go` | Read | Lấy nội dung URL (max 15K chars) |
| 8 | `git` | `git.go` | Read | Git commands đọc (log, diff, status, branch, show) |
| 9 | `calculator` | `calculator.go` | Read | Tính toán an toàn (parser tự viết, không dùng eval) |
| 10 | `datetime` | `datetime.go` | Read | Thời gian: now, convert, add, diff |
| 11 | `json` | `json.go` | Read | Format, get (dot-path), validate JSON |
| 12 | `http` | `http.go` | Write | HTTP requests (GET/POST/PUT/DELETE/PATCH) |
| 13 | `version` | `version.go` | Read | Kiểm tra phiên bản mới nhất (npm, GitHub releases) |
| 14 | `weather` | `weather.go` | Read | Thời tiết qua wttr.in (miễn phí, không cần key) |
| 15 | `translate` | `translate.go` | Read | Dịch văn bản qua libretranslate.com |
| 16 | `timer` | `timer.go` | Read | Đặt/xóa/liệt kê timer nhắc nhở (in-memory) |
| 17 | `calendar` | `calendar.go` | Read | Đọc file .ics (lịch hôm nay, thêm sự kiện) |
| 18 | `notes.search` | `notes.go` | Read | Full-text search trong markdown notes |
| 19 | `notes.create` | `notes.go` | Write | Tạo markdown note với tags |
| 20 | `memory.save` | `memory_tools.go` | Write | Lưu key-value vào memory store |
| 21 | `memory.recall` | `memory_tools.go` | Read | Tìm memory bằng keyword |
| 22 | `memory.list` | `memory_tools.go` | Read | Liệt kê tất cả memories |
| 23 | `rag.search` | `rag.go` | Read | Tìm kiếm ngữ nghĩa trong documents (vector search) |

#### Ví dụ: Cách implement một tool

```go
// calculator.go - ví dụ tool đơn giản
type CalculatorTool struct{}

func NewCalculatorTool() *CalculatorTool {
    return &CalculatorTool{}
}

func (t *CalculatorTool) Name() string { return "calculator" }

func (t *CalculatorTool) Description() string {
    return "Tính toán biểu thức toán học. Hỗ trợ +, -, *, /, %, ^, sqrt, abs, sin, cos, tan, log, ln, pi, e."
}

func (t *CalculatorTool) Schema() map[string]any {
    return map[string]any{
        "type": "object",
        "properties": map[string]any{
            "expression": map[string]any{
                "type":        "string",
                "description": "Biểu thức toán học cần tính, VD: '2 + 3 * 4'",
            },
        },
        "required": []string{"expression"},
    }
}

func (t *CalculatorTool) Kind() Kind { return KindRead }

func (t *CalculatorTool) Execute(ctx context.Context, args map[string]any) (Result, error) {
    expr, ok := args["expression"].(string)
    if !ok {
        return Result{}, fmt.Errorf("expression must be a string")
    }

    // Dùng recursive descent parser TỰ VIẾT (an toàn, không eval)
    result, err := parseAndEval(expr)
    if err != nil {
        return Result{}, err
    }

    return Result{Content: fmt.Sprintf("%g", result)}, nil
}
```

---

### 4.5 Memory — Hệ thống ghi nhớ 3 tầng

#### Các tầng memory

| Tầng | Node | Khi nào chạy | Làm gì |
|------|------|-------------|--------|
| 1. Working | State.Messages | Suốt phiên | Lịch sử hội thoại trong RAM |
| 2. Recall | `RecallNode` | Đầu phiên | Tìm facts liên quan từ store |
| 3. Extract | `ExtractNode` | Cuối phiên | Trích xuất facts mới từ hội thoại |

#### Memory Store (in-memory)

```go
// store.go
type Store struct {
    mu   sync.RWMutex
    data map[string]string      // key → value
}

func (s *Store) Set(key, value string) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.data[key] = value
}

func (s *Store) Search(query string) map[string]string {
    s.mu.RLock()
    defer s.mu.RUnlock()
    results := make(map[string]string)
    lowerQuery := strings.ToLower(query)
    for k, v := range s.data {
        if strings.Contains(strings.ToLower(k), lowerQuery) ||
           strings.Contains(strings.ToLower(v), lowerQuery) {
            results[k] = v
        }
    }
    return results
}
```

#### Extract Pattern (extract.go) — Cách agent "học" về user

Agent dùng **regex patterns** để phát hiện thông tin về user:

```go
var patterns = []struct {
    pattern *regexp.Regexp
    key     string
}{
    // Tiếng Việt
    {regexp.MustCompile(`t(ôi|ớ|ui) (tên|là) (.+?)[\.,\s]`), "user_name"},
    {regexp.MustCompile(`t(ôi|ớ) thích (.+?)[\.,\s]`), "likes"},
    {regexp.MustCompile(`t(ôi|ớ) (ở|sống) (.+?)[\.,\s]`), "location"},
    {regexp.MustCompile(`t(ôi|ớ) làm (.+?)[\.,\s]`), "job"},
    {regexp.MustCompile(`email .+? (.+?@.+?)[\.,\s]`), "email"},
    // Tiếng Anh
    {regexp.MustCompile(`my name is (.+?)[\.,\s]`), "user_name"},
    {regexp.MustCompile(`I (like|love) (.+?)[\.,\s]`), "likes"},
    {regexp.MustCompile(`I (live in|am from) (.+?)[\.,\s]`), "location"},
    // ... khoảng 15 patterns
}
```

#### Memory Flow tổng thể

```
SESSION BẮT ĐẦU
    │
    ▼
RECALL NODE: "User này là ai, thích gì?"
    │ search memory store: "user_name" → "Trinh"
    │ search memory store: "likes" → "code sạch, Go"
    │ Kết quả được inject vào system prompt
    ▼
MODEL: "Chào Trinh! Tôi biết bạn thích code sạch..."
    │ (hội thoại diễn ra)
    ▼
EXTRACT NODE: Quét toàn bộ messages
    │ Regex match: "tôi thích dark mode" → "likes" = "dark mode"
    │ Lưu vào store nếu chưa có
    ▼
SESSION KẾT THÚC
```

---

### 4.6 Orchestrator — Điều phối nhiều agent

Khi user gửi tin nhắn, orchestrator sẽ chọn agent phù hợp nhất:

```go
// orchestrator.go
type Orchestrator struct {
    agents  map[string]*AgentSpec
    order   []string
    defaultName string
}

type AgentSpec struct {
    Name            string           // "general", "code", "research"
    Description     string           // Mô tả
    Engine          *agent.Engine    // Engine chuyên biệt
    TriggerKeywords []string         // Keywords để kích hoạt
    SystemPrompt    string           // Prompt riêng cho agent này
}

func (o *Orchestrator) route(input string) *AgentSpec {
    lower := strings.ToLower(input)

    // Duyệt theo thứ tự đăng ký
    for _, name := range o.order {
        spec := o.agents[name]
        for _, kw := range spec.TriggerKeywords {
            if strings.Contains(lower, kw) {
                return spec  // Match đầu tiên
            }
        }
    }

    return o.agents[o.defaultName]  // Không match → default
}
```

**Ví dụ routing:**
- `"fix bug trong hàm login"` → chứa `"bug"` → `agent:code`
- `"tìm hiểu về Go generics"` → chứa `"tìm hiểu"` → `agent:research`
- `"chào buổi sáng"` → không match → `agent:general` (default)

**3 agent được cấu hình:**
1. **General** — Hội thoại thông thường, task văn phòng
2. **Code** — Code, debug, refactor, test
3. **Research** — Tìm kiếm, nghiên cứu, phân tích sâu

---

### 4.7 Skills — Progressive Disclosure

Skill là các file markdown (`SKILL.md`) chứa hướng dẫn cho agent. Thay vì nhồi tất cả vào system prompt (tốn token), agent chỉ biết **tên + mô tả** của skill. Khi user cần, agent mới "mở" skill ra đọc.

```
skills/
├── code-review/SKILL.md
│   ---
│   name: code-review
│   description: Review code for bugs, security, and quality
│   when_to_use: When user asks for code review
│   tools: file.read, git
│   ---
│   # Code Review Process
│   1. Read the changed files
│   2. Check for bugs...
│   ...
```

```go
// loader.go
type Loader struct {
    skills map[string]*Skill
}

func (l *Loader) ListSkills() []SkillSummary {
    // Chỉ trả về tên + mô tả (khoảng 2 dòng/skill)
    // Dùng để inject vào system prompt
}

func (l *Loader) LoadSkill(name string) *Skill {
    // Trả về toàn bộ nội dung skill khi agent cần
}

func (l *Loader) MatchSkill(userInput string) *Skill {
    // Tìm skill phù hợp bằng keyword matching
    for _, s := range l.skills {
        // So khớp tên + mô tả + when_to_use
    }
}
```

**Lợi ích:** System prompt chỉ tăng ~50 dòng thay vì hàng nghìn dòng nếu load hết skill.

---

### 4.8 RAG — Retrieval-Augmented Generation

Package `internal/rag/` giúp agent tìm kiếm trong documents đã upload.

#### Voyage AI Embedding

```go
// voyage.go
type Client struct {
    apiKey   string
    baseURL  string
    httpClient *http.Client
}

// Gọi Voyage AI API để tạo vector embedding
func (c *Client) Embed(ctx context.Context, texts []string, inputType string) ([][]float64, error) {
    // Batch tối đa 96 texts/request (respect rate limit)
    // Model: voyage-3 (1024 dimensions)
    // input_type: "query" (cho câu hỏi) hoặc "document" (cho tài liệu)
}
```

#### RAG Search Tool

```go
// tools/rag.go
func (t *RAGSearchTool) Execute(ctx context.Context, args map[string]any) (Result, error) {
    query := args["query"].(string)

    // 1. Embed câu query
    embeddings, _ := t.voyageClient.Embed(ctx, []string{query}, "query")
    queryVector := embeddings[0]

    // 2. MongoDB Atlas $vectorSearch
    pipeline := mongo.Pipeline{
        {{Key: "$vectorSearch", Value: bson.M{
            "index":         "vector_index",
            "queryVector":   queryVector,
            "path":          "embedding",
            "numCandidates": 100,
            "limit":         5,
        }}},
        // Thêm filter tenant
        {{Key: "$match", Value: bson.M{"tenantId": tenantID}}},
    }

    // 3. Trả về top 5 chunks với score
    // Kết quả: documentId, source, score, snippet (300 chars đầu)
}
```

#### Pipeline tổng thể

```
USER UPLOAD DOCUMENT (TS side)
    │
    ▼
Extract text → Chunk → Embed (Voyage AI) → Lưu MongoDB
    │
    │  (documents collection với vector_index)
    │
    ▼
USER HỎI: "Tài liệu nói gì về X?"
    │
    ▼
rag.search("X")
    │
    ▼
Embed query → $vectorSearch → Top 5 chunks → Đưa vào LLM context
```

---

### 4.9 Storage

#### SQLite Store (`storage/sqlite/`)

Lưu trữ bền vững cho conversations, messages, và memories:

```go
type Store struct {
    db *sql.DB
}

// Schema
// conversations: id, title, created_at, updated_at
// messages: id, conversation_id, role, content, tool_calls, tool_call_id, created_at
// messages_fts: FTS5 index cho full-text search messages
// memories: type, key, value, confidence, source, created_at
// memories_fts: FTS5 index cho full-text search memories
```

**Go pattern hay:** Dùng `database/sql` thuần + SQLite driver (`modernc.org/sqlite` — pure Go, không cần CGO).

#### In-Memory Vector Store (`storage/chroma/`)

```go
type VectorStore struct {
    vectors map[string][]float64     // id → vector
    metadata map[string]map[string]string  // id → metadata
}

// Cosine similarity search
func (vs *VectorStore) Search(query []float64, topK int) []SearchResult {
    // Tính cosine similarity với TẤT CẢ vectors trong store
    // Sort, trả về topK
}
```

> ⚠️ Tên package là `chroma` nhưng KHÔNG liên quan đến ChromaDB. Đây là vector store tự viết, in-memory.

---

### 4.10 Guardrails — Lớp bảo vệ

```go
// guard.go - Phân loại tool theo mức độ nguy hiểm
func CheckTool(t Tool) error {
    switch t.Kind() {
    case KindRead:
        return nil  // Luôn cho phép
    case KindWrite:
        log.Printf("WRITE: %s", t.Name())
        return nil  // Cho phép nhưng log lại
    case KindDestructive:
        return NeedConfirmationError{t.Name()}  // Cần user xác nhận
    }
}

// input.go - Chống prompt injection
func ValidateUserInput(input string) error {
    // Check patterns: "ignore previous instructions", "DAN", "system override"
    if containsPromptInjection(input) {
        return ErrPromptInjection
    }
    // Check XSS: <script>, onclick, javascript:, eval, document.cookie
    if containsXSS(input) {
        return ErrXSSInjection
    }
    // Check length
    if len(input) > 4000 {
        return ErrInputTooLong
    }
    return nil
}

// circuit_breaker.go - Phát hiện stuck loop
type CircuitBreaker struct {
    maxRepeats   int           // Mặc định 3
    recent       []recentCall  // Lịch sử gần đây
}
// Nếu tool + args giống hệt nhau được gọi 3 lần liên tiếp → ngắt
```

---

### 4.11 MCP — Model Context Protocol

MCP cho phép kết nối với external tools qua subprocess:

```go
// discovery.go
type MCPClient struct {
    cmd    *exec.Cmd        // Subprocess
    stdin  io.WriteCloser   // Gửi JSON-RPC requests
    stdout *bufio.Scanner   // Nhận JSON-RPC responses
}

// Giao tiếp qua stdin/stdout với JSON-RPC 2.0
func (c *MCPClient) Connect(command string, args ...string) error {
    c.cmd = exec.Command(command, args...)
    c.stdin, _ = c.cmd.StdinPipe()
    stdoutPipe, _ := c.cmd.StdoutPipe()
    c.stdout = bufio.NewScanner(stdoutPipe)
    c.cmd.Start()

    // Handshake: initialize → initialized
    c.sendRequest("initialize", ...)
    c.sendNotification("initialized", ...)
}

func (c *MCPClient) ListTools() ([]ToolDef, error) {
    // Gọi tools/list, parse kết quả
}

func (c *MCPClient) CallTool(name string, args map[string]any) (string, error) {
    // Gọi tools/call, trả về kết quả
}
```

**Config MCP servers qua YAML:**
```yaml
# ~/.jarvis/mcp/github.yaml
servers:
  - name: github
    command: npx
    args: ["-y", "@anthropic/mcp-server-github"]
```

---

### 4.12 Transport/HTTP

```go
// chat.go - SSE Streaming handler
func (h *ChatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // 1. Parse request
    var req ChatRequest
    json.NewDecoder(r.Body).Decode(&req)

    // 2. Validate input (guardrails)
    if err := guardrails.ValidateUserInput(req.UserMessage); err != nil {
        http.Error(w, err.Error(), 400)
        return
    }

    // 3. Set SSE headers
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")

    flusher := w.(http.Flusher)

    // 4. Tạo emit function để stream events
    emit := func(ev agent.Event) {
        data, _ := json.Marshal(ev)
        fmt.Fprintf(w, "data: %s\n\n", data)
        flusher.Flush()
    }

    // 5. Chạy agent
    input := agent.RunInput{
        UserMessage: req.UserMessage,
        History:     req.History,
        // ...
    }
    usage, err := h.runner.Run(r.Context(), input, emit)

    // 6. Gửi event done
    emit(agent.DoneEvent(usage))
}
```

---

### 4.13 Phụ trợ

#### Config (`config/`)

```go
type Config struct {
    Port            int
    AnthropicKey    string
    GeminiKey       string
    GeminiModel     string
    OllamaURL       string
    MongoDBURI      string
    MongoDBName     string
    VoyageKey       string
    SkillsDir       string
    AllowedPaths    []string
    ProviderMode    string  // "gemini", "anthropic", "ollama", "auto"
}
// Load từ environment variables (dùng joho/godotenv)
```

#### Personality (`personality/`)

```go
type Profile struct {
    Name      string
    Formality string  // Casual, Neutral, Formal
    Humor     string  // None, Dry, Playful
    Verbosity string  // Concise, Normal, Detailed
}

// Tự điều chỉnh dựa trên tương tác
func (p *PersonalityEngine) Learn(input, response string) {
    if strings.Contains(input, "ngắn gọn") {
        p.profile.Verbosity = "Concise"
    }
    if strings.Contains(input, "vui vẻ") {
        p.profile.Humor = "Playful"
    }
}
```

#### Proactive (`proactive/`)

Cron scheduler cho phép agent tự chạy task định kỳ:

```go
type ProactiveEngine struct {
    cron    *cron.Cron
    tasks   map[string]*Task
    results []TaskResult
}

// VD: Mỗi sáng 8h, agent tự tạo báo cáo
engine.AddTask("morning-brief", "0 8 * * *", "Tổng hợp task hôm nay")
```

#### Metrics (`metrics/`)

Thread-safe counter dùng `atomic.Int64`:

```go
type Metrics struct {
    requests    atomic.Int64
    toolCalls   atomic.Int64
    errors      atomic.Int64
    inputTokens atomic.Int64
    latencies   []int64  // guarded by mu
    mu          sync.Mutex
}
```

#### Tenant Middleware (`middleware/`)

```go
func TenantMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        tenantID := r.Header.Get("X-Tenant-ID")
        if tenantID == "" {
            tenantID = "default"
        }
        ctx := context.WithValue(r.Context(), tenantKey, tenantID)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

---

## 5. Go Patterns quan trọng trong codebase

Đây là những pattern Go mà em sẽ thấy xuyên suốt codebase:

### 1. Interface nhỏ (1-2 methods)

```go
// Thay vì 1 interface lớn, tách thành nhiều interface nhỏ
type Provider interface {
    Generate(ctx context.Context, req GenerateRequest) (<-chan StreamChunk, error)
    Name() string
}

type Runner interface {
    Run(ctx context.Context, in RunInput, emit EmitFunc) (Usage, error)
}
```

### 2. Functional Options (SetXxx methods)

```go
// Thay vì constructor nhiều tham số
engine := agent.NewEngine(prov, reg).
    SetSystemPrompt(prompt).
    SetMemoryNodes(recall, extract, summarize).
    SetCircuitBreaker(cb).
    SetMaxContextTokens(100000)
```

### 3. Channel cho streaming

```go
// Provider trả về channel, agent đọc từ channel
stream, err := provider.Generate(ctx, req)
for chunk := range stream {
    // Xử lý từng chunk
}
```

### 4. errgroup cho parallel execution

```go
g, ctx := errgroup.WithContext(ctx)
for i, call := range calls {
    i, call := i, call  // QUAN TRỌNG: capture loop variables
    g.Go(func() error {
        return doWork(ctx, call)
    })
}
if err := g.Wait(); err != nil {
    // Xử lý lỗi đầu tiên
}
```

### 5. sync.RWMutex cho thread-safe data

```go
type Store struct {
    mu   sync.RWMutex
    data map[string]string
}
// Đọc: RLock/RUnlock
// Ghi: Lock/Unlock
```

### 6. atomic.Int64 cho counters

```go
// Không cần mutex cho simple counters
type Metrics struct {
    requests atomic.Int64
    tokens   atomic.Int64
}
m.requests.Add(1)
```

### 7. Context propagation

```go
// ctx LUÔN là tham số đầu tiên
func (e *Engine) Run(ctx context.Context, in RunInput, emit EmitFunc) (Usage, error) {
    // Kiểm tra cancellation
    select {
    case <-ctx.Done():
        return Usage{}, ctx.Err()
    default:
    }
}
```

### 8. Struct tags cho JSON/YAML

```go
type ChatRequest struct {
    ConversationID string            `json:"conversation_id"`
    UserMessage    string            `json:"user_message"`
    Attachments    []AttachmentInput `json:"attachments,omitempty"`
}
```

### 9. Table-driven tests (trong eval/)

```go
cases := []EvalCase{
    {Name: "simple math", Input: "2+2=?", Expected: "4", Mode: Contains},
    {Name: "weather", Input: "thời tiết HN", Expected: "°C", Mode: Contains},
}
```

### 10. Pure functions tách biệt I/O

```go
// route() là pure function — không I/O, không side effect
// Rất dễ test
func route(s *State) NodeID { ... }

// dispatch() là nơi có I/O — gọi LLM, gọi tools
func (e *Engine) dispatch(ctx context.Context, node NodeID, s *State, emit EmitFunc) (NodeID, error) { ... }
```

---

## 6. Sơ đồ tổng thể

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              CMD LAYER                                       │
│  ┌──────────────────────┐  ┌──────────────────────┐                         │
│  │  cmd/jarvis (CLI)     │  │  cmd/server (HTTP)    │                         │
│  │  ask | chat | serve   │  │  /chat | /healthz     │                         │
│  └──────────┬───────────┘  └──────────┬───────────┘                         │
└─────────────┼──────────────────────────┼─────────────────────────────────────┘
              │                          │
              ▼                          ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                        TRANSPORT LAYER                                       │
│  ┌─────────────────────────────────────────────────────────────────┐        │
│  │  transport/http: SSE streaming, JSON decode, guardrails, tenant  │        │
│  └──────────────────────────────┬──────────────────────────────────┘        │
└─────────────────────────────────┼───────────────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                       ORCHESTRATOR LAYER                                     │
│  ┌─────────────────────────────────────────────────────────────────┐        │
│  │  orchestrator: keyword routing → general | code | research       │        │
│  └──────────────────────────────┬──────────────────────────────────┘        │
└─────────────────────────────────┼───────────────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                          AGENT ENGINE                                        │
│  ┌─────────────────────────────────────────────────────────────────┐        │
│  │                        ReAct Loop                                │        │
│  │                                                                  │        │
│  │  RECALL ──▶ SUMMARIZE ──▶ MODEL ──▶ ROUTER ──▶ TOOLS ──▶ MODEL  │        │
│  │     ▲                                                    │       │        │
│  │     └────────────────────────────────────────────────────┘       │        │
│  │                                                                  │        │
│  │  MODEL → EXTRACT → END                                           │        │
│  └──────────────────────────────────────────────────────────────────┘        │
└─────────────────────────────────────────────────────────────────────────────┘
              │                │                │
              ▼                ▼                ▼
    ┌─────────────┐  ┌─────────────┐  ┌─────────────┐
    │  PROVIDER    │  │   TOOLS     │  │   MEMORY    │
    │  ┌─────────┐ │  │  ┌───────┐  │  │  ┌───────┐  │
    │  │ Gemini  │ │  │  │ web   │  │  │  │ Recall │  │
    │  │ Claude  │ │  │  │ file  │  │  │  │ Extract│  │
    │  │ Ollama  │ │  │  │ shell │  │  │  │Store(in│  │
    │  │Fallback │ │  │  │  ...  │  │  │  │ memory)│  │
    │  └─────────┘ │  │  └───────┘  │  │  └───────┘  │
    └─────────────┘  └─────────────┘  └─────────────┘
              │                │
              ▼                ▼
    ┌─────────────┐  ┌─────────────┐
    │    RAG      │  │   STORAGE   │
    │  ┌───────┐  │  │  ┌───────┐  │
    │  │Voyage │  │  │  │SQLite │  │
    │  │MongoDB│  │  │  │Chroma │  │
    │  └───────┘  │  │  └───────┘  │
    └─────────────┘  └─────────────┘
```

---

## Phụ lục: Từ vựng Go cho người mới

| Thuật ngữ | Ý nghĩa |
|-----------|---------|
| `package` | Đơn vị tổ chức code. Mọi file `.go` phải khai báo package |
| `import` | Nhập package khác vào dùng |
| `func` | Khai báo hàm |
| `type X struct { ... }` | Định nghĩa kiểu dữ liệu mới (giống class không có method) |
| `func (x *X) Method()` | Method gắn với struct (giống method của class) |
| `interface { ... }` | Tập các method. Ai implement đủ method → tự động thỏa interface |
| `error` | Giá trị trả về để báo lỗi (không dùng exception) |
| `chan` | Channel — dùng để giao tiếp giữa goroutines |
| `go func()` | Chạy hàm trong goroutine (lightweight thread) |
| `defer` | Hoãn thực thi đến khi hàm kết thúc (dùng để cleanup) |
| `context.Context` | Truyền deadline, cancellation, values qua các lời gọi hàm |
| `sync.Mutex` / `sync.RWMutex` | Khóa để bảo vệ shared data |
| `atomic.Int64` | Atomic operations cho counters (nhanh hơn mutex) |
| `errgroup` | Chạy nhiều goroutines và đợi tất cả hoặc cancel khi có lỗi |
| `struct tags` | Metadata gắn với field (VD: `json:"name"`) |

---

> **Tài liệu này được tạo ngày 2026-07-30, dựa trên codebase commit `d7f00ec`.**
> Mọi thắc mắc về kiến trúc — cứ hỏi anh! 🇻🇳
