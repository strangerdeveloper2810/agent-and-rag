# J.A.R.V.I.S. Multi-Agent Orchestrator — Design

> **Vấn đề:** Engine hiện tại là single-agent. Một Engine = một Provider + một Registry.  
> **Giải pháp:** Orchestrator pattern — 1 supervisor điều phối N specialized agents.  
> **Industry pattern:** Multi-Agent Supervisor (Pattern 7 trong loop engineering).

---

## 0. Single-Agent vs Multi-Agent

```
HIỆN TẠI (single-agent)                  MULTI-AGENT (tương lai)
──────────────────────────               ──────────────────────────

┌──────────────────────┐                ┌──────────────────────────┐
│  Engine              │                │  Orchestrator            │
│  ├─ Provider (1 LLM) │                │  ├─ IntentRouter          │
│  ├─ Registry (1 set) │                │  ├─ AgentPool             │
│  └─ State machine    │                │  │   ├─ GeneralAgent      │
│                      │                │  │   ├─ CodeAgent         │
│  1 agent làm mọi thứ │                │  │   ├─ ResearchAgent     │
│  Prompt chung cho     │                │  │   └─ ...              │
│  mọi loại task       │                │  └─ HandoffManager       │
└──────────────────────┘                └──────────────────────────┘

Vấn đề:                                  Giải pháp:
- Prompt quá dài (mọi tool)              - Mỗi agent có prompt + tools RIÊNG
- LLM bị nhiễu (quá nhiều tool)          - Chỉ expose tools cần thiết
- Không phân biệt task đơn giản/phức tạp - Route task đến đúng agent
- Context window lãng phí                - Context window gọn, focused
```

---

## 1. Kiến Trúc Orchestrator

```
                          USER INPUT
                              │
                              ▼
                    ┌──────────────────┐
                    │  INTENT ROUTER   │  ← "Người gác cổng"
                    │                  │
                    │  Phân loại input │
                    │  → domain        │
                    │  → complexity    │
                    │  → urgency       │
                    └────────┬─────────┘
                             │
              ┌──────────────┼──────────────┐
              ▼              ▼              ▼
        ┌──────────┐  ┌──────────┐  ┌──────────┐
        │ GENERAL  │  │  CODE    │  │ RESEARCH │
        │ AGENT    │  │  AGENT   │  │  AGENT   │
        │          │  │          │  │          │
        │ Chat     │  │ Code gen │  │ Web      │
        │ Memory   │  │ Debug    │  │ search   │
        │ Notes    │  │ Git      │  │ RAG      │
        │ Calendar │  │ Shell    │  │ Summarize│
        └──────────┘  └──────────┘  └──────────┘
              │              │              │
              └──────────────┼──────────────┘
                             │
                             ▼
                    ┌──────────────────┐
                    │ HANDOFF MANAGER  │  ← Agent A → Agent B
                    │                  │
                    │ Context transfer │
                    │ Result merge     │
                    └──────────────────┘
```

---

## 2. Code: Orchestrator Engine

### 2.1 Agent Registry

```go
// internal/orchestrator/orchestrator.go

// AgentSpec định nghĩa một specialized agent.
type AgentSpec struct {
    Name        string           // "general", "code", "research"
    Description string           // Mô tả cho intent router
    Engine      *agent.Engine    // Engine ReAct (GIỮ NGUYÊN từ P2)
    Tools       []string         // Tên tools agent này được dùng
    SystemPrompt string          // Prompt RIÊNG cho agent này
    TriggerKeywords []string     // Keyword để router chọn agent này
}

// Orchestrator quản lý nhiều agent.
type Orchestrator struct {
    agents  map[string]*AgentSpec         // name → spec
    router  *IntentRouter                 // classify → pick agent
    handoff *HandoffManager               // agent-to-agent
}
```

### 2.2 Intent Router

```go
// IntentRouter phân loại input → chọn agent phù hợp.
type IntentRouter struct {
    classifier provider.Provider  // LLM nhẹ (Gemini Flash) để classify
    agents     []*AgentSpec
}

func (r *IntentRouter) Route(ctx context.Context, input string) (*AgentSpec, error) {
    // 1. Keyword matching (nhanh, rẻ, không cần LLM)
    for _, a := range r.agents {
        for _, kw := range a.TriggerKeywords {
            if strings.Contains(strings.ToLower(input), kw) {
                return a, nil
            }
        }
    }

    // 2. Nếu không match keyword → gọi LLM nhẹ để classify
    // Prompt: "Phân loại input sau vào 1 trong các domain: [list agents].
    //          Input: {input}. Trả lời CHỈ tên domain."
    domain := r.callClassifier(ctx, input)
    if a, ok := r.agents[domain]; ok {
        return a, nil
    }

    // 3. Default → GeneralAgent
    return r.agents["general"], nil
}
```

### 2.3 Orchestrator Run

```go
func (o *Orchestrator) Run(ctx context.Context, in agent.RunInput, emit agent.EmitFunc) (provider.Usage, error) {
    // 1. Route: chọn agent
    spec, err := o.router.Route(ctx, in.UserMessage)
    if err != nil {
        return provider.Usage{}, err
    }

    // 2. Modulate system prompt cho agent được chọn
    // KHÔNG thay đổi engine — chỉ thay đổi input
    enrichedInput := in
    enrichedInput.SystemPrompt = spec.SystemPrompt

    // 3. Run agent engine (GIỮ NGUYÊN ENGINE từ P2!)
    emit(agent.Event{Type: "agent", Node: spec.Name})  // báo client biết agent nào đang chạy
    return spec.Engine.Run(ctx, enrichedInput, emit)
}
```

---

## 3. Handoff Protocol (Agent → Agent)

### 3.1 Khi Nào Cần Handoff?

```
User: "Tìm bug trong code này và giải thích nguyên nhân"

GeneralAgent tiếp nhận → phân tích:
  "Đây là task code analysis → handoff sang CodeAgent"
  → context transfer: "User muốn tìm bug trong đoạn code sau:..."

CodeAgent nhận context → thực thi → trả kết quả về GeneralAgent
  → GeneralAgent tổng hợp → trả lời user
```

### 3.2 Handoff Manager

```go
type HandoffRequest struct {
    From    string          // agent name gửi
    To      string          // agent name nhận
    Context string          // context tóm tắt để agent nhận hiểu
    Task    string          // task cụ thể
}

type HandoffResult struct {
    Agent   string          // agent đã xử lý
    Result  string          // kết quả
    Usage   provider.Usage  // token usage
}

type HandoffManager struct {
    orchestrator *Orchestrator
}

func (h *HandoffManager) Delegate(ctx context.Context, req HandoffRequest) (*HandoffResult, error) {
    toAgent := h.orchestrator.agents[req.To]
    if toAgent == nil {
        return nil, fmt.Errorf("agent %q not found", req.To)
    }

    // Tạo input mới cho agent nhận
    input := agent.RunInput{
        UserMessage:  req.Task,
        History:      []provider.Message{
            {Role: provider.RoleSystem, Content: req.Context},
        },
        MaxSteps:     8,
    }

    // Chạy agent nhận (có thể là blocking hoặc goroutine nếu async)
    emit := func(e agent.Event) {} // suppress events from sub-agent (or aggregate)
    usage, err := toAgent.Engine.Run(ctx, input, emit)
    if err != nil {
        return nil, err
    }

    return &HandoffResult{
        Agent:  req.To,
        Result: "...", // extract from state
        Usage:  usage,
    }, nil
}
```

---

## 4. So Sánh Trước/Sau

| | Single-Agent (Hiện tại) | Multi-Agent Orchestrator |
|---|---|---|
| **Số Engine** | 1 | N (mỗi domain 1 engine) |
| **Prompt** | 1 prompt chung cho mọi task | Mỗi agent prompt riêng, tối ưu cho domain |
| **Tools** | Tất cả tools expose cho 1 LLM | Mỗi agent chỉ thấy tools liên quan |
| **Routing** | Không có — engine chạy thẳng | IntentRouter: keyword → LLM classify |
| **Handoff** | Không có | Agent A có thể delegate sang Agent B |
| **Context window** | Đầy vì quá nhiều tool + prompt | Gọn vì chỉ có domain-specific context |
| **Cost** | Cao (prompt dài) | Thấp hơn (prompt ngắn, đúng trọng tâm) |
| **Độ phức tạp** | Đơn giản | Trung bình |
| **Engine core** | ReAct loop | **GIỮ NGUYÊN** — mỗi sub-agent vẫn là ReAct loop |

---

## 5. Khi Nào Nâng Cấp?

```
HIỆN TẠI: Single-agent đủ dùng cho:
  ✅ Chat thông thường
  ✅ Tool calling (ít hơn 10 tools)
  ✅ Memory recall
  ✅ Context retrieval

NÂNG CẤP LÊN MULTI-AGENT KHI:
  □ Có >15 tools → LLM bị nhiễu, gọi sai tool
  □ Prompt >2000 tokens → context window lãng phí
  □ Có domain RÕ RÀNG (code, research, personal)
  □ Muốn dùng LLM KHÁC NHAU cho từng domain
    (Gemini Flash cho general, Claude Opus cho code)
  □ Cần handoff giữa các agent
```

---

## 6. Implementation Path

```
BƯỚC 1 (hiện tại): Single Engine hoàn chỉnh
  □ Đủ tools, memory, context
  □ Chạy ổn định 1-2 tuần

BƯỚC 2: Thêm Intent Router (đơn giản nhất)
  □ Keyword-based routing (không cần LLM)
  □ 2 agent: General + Code
  □ Mỗi agent = 1 Engine với prompt + tools riêng

BƯỚC 3: Thêm Handoff
  □ Agent có thể gọi agent khác như 1 tool
  □ Context transfer khi handoff

BƯỚC 4: LLM-based routing
  □ Dùng LLM nhẹ để classify input phức tạp
  □ Multi-provider: mỗi agent dùng LLM khác nhau
```
