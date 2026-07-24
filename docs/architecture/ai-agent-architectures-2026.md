# AI Agent Architectures 2026 — Modern Patterns & Mapping to Our Project

> **Audience:** FE dev learning Go backend + AI agent engineering. Designed to be interview-ready.
> **Purpose:** Understand the 2026 agent landscape, where our project fits, and WHY each architectural decision.
>
> **Sources:** [O'Reilly AI Agents Stack 2026](https://www.oreilly.com/radar/the-ai-agents-stack-2026-edition/), [MLflow Production Agents 2026](https://mlflow.org/articles/building-production-ready-ai-agents-in-2026/), [MLflow Agent Architecture Types](https://mlflow.org/articles/types-of-ai-agent-architectures-2026-developer-guide/), [Google Agent Bake-Off](https://developers.googleblog.com/build-better-ai-agents-5-developer-tips-from-the-agent-bake-off/), [Taskade 21 Agentic Design Patterns](https://www.taskade.com/blog/agentic-design-patterns), [FutureAGI LLM Agent Architectures 2026](https://futureagi.com/blog/llm-agent-architectures-core-components/)

---

## 1. Bối Cảnh 2024 → 2026: Agent Từ "Thử Nghiệm" Sang "Production"

### 1.1 Chuyển biến lớn nhất

| 2024 | 2026 |
|---|---|
| Monolithic agent 1 model làm hết | **Multi-agent decomposition** (specialized sub-agents) |
| Prompt engineering là vua | **Context engineering** — kiến trúc thông tin agent thấy mỗi lần gọi |
| Vector DB là afterthought | **Memory là first-class primitive** (3-4 tiers) |
| Hand-crafted reasoning loops | **Native reasoning models** (GPT-5, Claude Opus 4.7, Gemini 3.x tự suy luận) |
| Tool gọi qua REST API | **MCP (Model Context Protocol)** — giao thức chuẩn cho tool |
| Safety check ở output | **Guardrails ở tool execution layer** (chặn TRƯỚC khi hành động) |

### 1.2 The Production Agent Stack (O'Reilly 2026)

Mọi production agent được assemble từ 6 layer:

```
┌──────────────────────────────────────────┐
│ LAYER 6: GUARDRAILS & SAFETY             │ ← policy, HITL, tool authorization
├──────────────────────────────────────────┤
│ LAYER 5: EVAL & OBSERVABILITY            │ ← tracing, LLM-as-judge, drift detection
├──────────────────────────────────────────┤
│ LAYER 4: FRAMEWORKS & SDKS               │ ← LangGraph / CrewAI / DIY
├──────────────────────────────────────────┤
│ LAYER 3: MEMORY & KNOWLEDGE              │ ← 3-tier memory + RAG + GraphRAG
├──────────────────────────────────────────┤
│ LAYER 2: PROTOCOLS & TOOLS               │ ← MCP, A2A, tool schemas
├──────────────────────────────────────────┤
│ LAYER 1: MODELS & INFERENCE              │ ← Reasoning models, cost/latency tradeoffs
└──────────────────────────────────────────┘
```

**Bài học cho project mình:** Project của em ĐANG XÂY toàn bộ 6 layer này từ gốc — đó là lý do em hiểu sâu hơn bất kỳ ai chỉ dùng framework.

---

## 2. 5 Kiến Trúc Agent Chuẩn (MLflow 2026)

### 2.1 ReAct (Reason + Act) — SAFE DEFAULT

```
User Input → Thought → Action → Observation → Thought → Action → ... → Final Answer
```

**Pattern:** Mỗi bước: nghĩ → gọi tool → thấy kết quả → nghĩ tiếp → gọi tool hoặc trả lời.  
**Dùng khi:** Open-ended tasks (chatbot, research, support).  
**Claude Opus 4.7 đạt 87.6% SWE-bench với pure ReAct — KHÔNG CẦN external scaffolding.**

### 2.2 Plan-Execute — STRUCTURED WORKFLOWS

```
User Goal → Planner (decompose into steps) → Executor (step 1) → Executor (step 2) → ...
```

**Pattern:** Tách planning và execution. Planner (LLM mạnh) phân rã goal → các bước. Executor chạy từng bước.  
**Dùng khi:** Multi-file coding, complex research, 20+ step tasks.

### 2.3 Reflexion — SELF-CORRECTING

```
Generate → Evaluate (test/compiler/linter) → If fail: Reflect (why?) → Generate again
```

**Pattern:** Có oracle (test, compiler) check đúng/sai → nếu sai thì phân tích lỗi → sửa.  
**Dùng khi:** Code generation, math, tasks có verifiable pass/fail signal.

### 2.4 Tree-of-Thoughts — EXPLORE MULTIPLE PATHS

```
State → Generate N possible next steps → Evaluate each → Pick best → Continue
```

**Pattern:** Ở mỗi bước, sinh NHIỀU hướng đi → đánh giá → chọn hướng tốt nhất.  
**Dùng khi:** Puzzles, constraint satisfaction. **RẤT ĐẮT** (token × số nhánh).

### 2.5 Multi-Agent — SPECIALIZED SUB-AGENTS

```
Orchestrator → Triage Agent → routes to → [SQL Agent | Python Agent | Search Agent | ...]
```

**Pattern:** Microservices cho AI: mỗi sub-agent chuyên 1 việc, orchestrator điều phối.  
**Dùng khi:** Long-horizon tasks, parallel workstreams. **Pattern MẠNH NHẤT 2026.**

---

## 3. Mapping: Project Của Mình Đang Xây Gì?

### 3.1 Core Architecture: REACT + PLAN-EXECUTE HYBRID

```
Project của em:
┌─────────────────────────────────────────────────────────────┐
│ REACT LOOP (đang xây P2)                                    │
│                                                             │
│  MODEL → ROUTE → TOOLS → MODEL → ROUTE → ...               │
│                                                             │
│  + PLANNER SLOT (P8): node Plan trước MODEL                 │
│  + REFLECTION SLOT (P8): node Reflect trước final answer    │
│  + MEMORY (P7): recall trước MODEL, extract sau END         │
└─────────────────────────────────────────────────────────────┘
```

### 3.2 Bảng Mapping: Industry Pattern → Our Implementation

| Pattern (2026 Industry) | File/Component trong project | Status |
|---|---|---|
| **ReAct Loop** | `internal/agent/engine.go` + `node_model.go` + `node_tools.go` | 🔄 P2 |
| **Router** (triage/routing) | `internal/agent/router.go` — pure function `route(s) NodeID` | ✅ P2.2 |
| **Plan-Execute** | `internal/agent/node_plan.go` (P8 slot) | 📋 P8 |
| **Reflexion** | `internal/agent/node_reflect.go` (P8 slot) | 📋 P8 |
| **Tool Use (MCP-like)** | `internal/tools/tool.go` — `Tool` interface + `Registry` | ✅ P3 (base) |
| **Parallel Tool Execution** | `internal/tools/registry.go` — `RunParallel` với `errgroup` | ✅ P3 (base) |
| **3-Tier Memory** | `internal/memory/` + nodes `recall`/`summarize`/`extract` | 📋 P7 |
| **Context Engineering** | `internal/agent/context.go` (P6) — prompt assembly theo thứ tự | 📋 P6 |
| **Prompt Caching** | `provider.Options.Cache` — Anthropic cache_control, Gemini context cache | 📋 P6 |
| **HITL (Human-in-the-Loop)** | `internal/agent/state.go` — `Interrupt` + `/chat/resume` | 📋 P10 |
| **Guardrails (Tool Layer)** | `internal/tools/tool.go` — `Kind` (Read/Write/Destructive) | 📋 P10 |
| **Observability** | `internal/observability/` — OTel spans + slog | 📋 P11 |
| **Eval Harness** | `eval/` — LLM-as-judge, recall@k, faithfulness | 📋 P13 |
| **Provider Abstraction** | `internal/provider/provider.go` — `Provider` interface | ✅ P1 |
| **Multi-Provider (Pluggable)** | `internal/provider/gemini/` + `anthropic/` + `factory/` | ✅ P1 |

### 3.3 Tại Sao TỰ BUILD Thay Vì Dùng LangGraph/CrewAI?

| Lý do | Giải thích |
|---|---|
| **HỌC** | Đây là dự án học tập — hiểu agent từ gốc, không để framework che đi cơ chế |
| **Control** | Tự build = full control over state, routing, streaming (không bị framework constraints) |
| **Zero framework lock-in** | Không phụ thuộc LangGraph version upgrades, breaking changes, pricing |
| **Go performance** | Goroutine fan-out tool execution nhanh hơn JS single-thread Promise.all |
| **Portable patterns** | Pattern mình học được (state machine, router, errgroup) áp dụng được cho MỌI framework sau này |

**Khi nào nên dùng framework thay vì tự build?** Khi bạn cần production ASAP, team >5 người, hoặc cần persistence/checkpointing built-in. Nhưng SAU KHI đã tự build 1 lần, bạn sẽ dùng framework HIỆU QUẢ HƠN vì hiểu nó hoạt động thế nào bên dưới.

---

## 4. Framework Landscape 2026: Ai Đang Thống Trị?

### 4.1 Comparison Table

| | LangGraph | CrewAI | AutoGen | **Our Project** |
|---|---|---|---|---|
| **Design Model** | State graphs | Role-based crews | Chat room | **State machine + pure router** |
| **Language** | Python + TypeScript | Python | Python | **Go** |
| **Status** | Active v1.1+ | Active v1.14+ | **Maintenance mode** | **Learning/Building** |
| **Token Efficiency** | Most efficient | Moderate (1.5-2× LangGraph) | Least efficient (2-3×) | **Tự tối ưu được** |
| **Learning Curve** | Steep (graph model) | Fastest (declarative) | Moderate | **Steep but rewarding** |
| **Checkpointing** | First-class (time-travel) | Partial (v1.14+) | None | **Tự build (P10)** |
| **Cost/year (1K tasks/day)** | ~$4,400-6,600 | ~$6,600-10,200 | ~$9,100-14,600 | **Tùy model choice** |

### 4.2 AutoGen ĐÃ CHẾT (Maintenance Mode)

Từ cuối 2025, Microsoft ngừng phát triển AutoGen. Thay thế:
- **Microsoft Agent Framework (MAF)** — nếu bạn ở Azure ecosystem
- **AG2** — community fork của AutoGen
- **LangGraph** hoặc **CrewAI** — nếu framework-agnostic

**Bài học:** Đừng bet cả project vào 1 framework. Provider abstraction (như project mình có) = portable.

### 4.3 Xu Hướng: HYBRID Architectures

Research 2026 (IEEE Access) chỉ ra: **LangGraph + CrewAI hybrid đạt 96.1% success rate, giảm 76.2% token, latency nhanh hơn 14.5×**.

Cách kết hợp:
```
Triage (CrewAI role) → routes to → Specialized Agents (LangGraph graphs)
                                  → mỗi agent có state machine riêng
                                  → orchestrator điều phối qua MCP/A2A
```

**Project mình đã đi theo hướng này từ đầu:**
- **Router** = lightweight triage (không cần LLM để route!)
- **State machine engine** = specialized execution
- **Tool interface** = MCP-like standardized tool connectivity

---

## 5. Memory Architecture: Từ Vector DB Sang Multi-Tier

### 5.1 Industry Standard 2026

```
TIER 1: WORKING / IN-CONTEXT (current turn)
├── Conversation messages trong context window
├── Tool results, scratchpad
└── Lifecycle: per-request, discarded after

TIER 2: EPISODIC (cross-turn, same user/session)
├── Summary của history cũ
├── Vector search trên conversations cũ
└── Stored in: Postgres/pgvector hoặc MongoDB

TIER 3: SEMANTIC / LONG-TERM (cross-conversation, forever)
├── Facts, preferences, entities
├── Structured lookup (type+key) + Vector recall (cosine)
└── Stored in: Vector DB + structured store

TIER 4: PROCEDURAL (learned patterns) ← 2026 cutting edge
├── Reusable execution traces
├── "How to solve X" patterns
└── Framework: LEGOMem (AAMAS 2026)
```

### 5.2 Project Của Mình Map Vào Đâu?

| Tier | Our Implementation | Status |
|---|---|---|
| **Working** | `State.Messages` + `State.Scratchpad` (in-memory, per-request) | ✅ P2 |
| **Episodic** | Node `summarize` — nén history dài → 1 SystemMessage tóm tắt | 📋 P7 |
| **Semantic** | `memories` collection: `{type, key, value, confidence, embedding}` + Voyage embed + Atlas $vectorSearch | 📋 P7 |
| **Procedural** | Chưa có (có thể thêm sau — skill patterns) | 🔮 Future |

### 5.3 Context Engineering > Prompt Engineering

2026 insight: **"Bạn kiến trúc thông tin agent thấy mỗi lần gọi, không chỉ prompt nó thế nào."**

Thứ tự context assembly trong project của em (P6):
```
1. SYSTEM INSTRUCTIONS    (base rules + chống bịa)       ← [CACHE ĐƯỢC]
2. TOOL DEFINITIONS       (danh sách tool)               ← [CACHE ĐƯỢC]
3. SKILLS                 (nạp theo ngữ cảnh — P9)       ← [CACHE ĐƯỢC]
4. MEMORY RECALL          (facts liên quan)              ← [BỘ NHỚ]
5. RAG CONTEXT            (kết quả search)              ← [DỮ LIỆU THAM KHẢO]
6. HISTORY                (đã trim/summary)              ← [HỘI THOẠI]
```

**Tách DATA vs INSTRUCTION = chống prompt injection.** Document content nằm trong block `[DỮ LIỆU THAM KHẢO]` — LLM được dạy đây là data, không phải command.

---

## 6. Tool System: MCP vs Custom

### 6.1 MCP (Model Context Protocol) — Industry Standard 2026

- **97M monthly SDK downloads** — đã thắng protocol war
- Được OpenAI, Google, Microsoft adopt
- Donated cho Linux Foundation
- JSON-RPC 2.0: tools, resources, prompts

### 6.2 Tại Sao Project Mình KHÔNG Dùng MCP?

| Lý do | 
|---|---|
| **Học:** Muốn hiểu tool system hoạt động thế nào trước khi dùng protocol có sẵn |
| **Go ecosystem:** MCP Go SDK còn non (chủ yếu Python/TS) |
| **Đơn giản:** 9 tools, không cần protocol phức tạp |
| **Có thể migrate sau:** `Tool` interface của mình map thẳng sang MCP tool definition |

### 6.3 Tool Design Principles (Từ Google Agent Bake-Off)

| Principle | Our Implementation |
|---|---|
| **Single responsibility** | Mỗi tool 1 việc: `ragSearch`, `createTask`, `deleteTask` |
| **Typed params + JSON Schema** | `Tool.Schema() json.RawMessage` |
| **Structured errors** | `Result{Content, Meta}` + `CallResult.Err` — không văng raw trace |
| **Parallel invocation** | `Registry.RunParallel` với `errgroup` — fan-out tool calls |
| **Avoid tool bloat** | Chỉ 9 tools, mỗi tool có description rõ ràng |
| **Classification (Read/Write/Destructive)** | `Tool.Kind()` → quyết định guardrail (HITL cho Destructive) |

---

## 7. Guardrails: Chặn Ở Tool Layer, Không Phải Output Layer

### 7.1 Industry Lesson 2026

> *"By the time you filter the response, the agent already sent the email."*

**Pattern đúng:** Authorize TRƯỚC khi tool chạy, không filter SAU khi có output.

### 7.2 Project Implementation

```
┌──────────────────────────────────────────────────────────┐
│ INPUT GUARDRAIL                                          │
│ ├── Mark tool results as DATA (separate from instruction)│
│ └── Heuristic: detect suspicious instructions in results │
├──────────────────────────────────────────────────────────┤
│ TOOL GUARDRAIL (HITL)                                    │
│ ├── KindRead        → auto-execute                       │
│ ├── KindWrite       → execute + log                      │
│ └── KindDestructive → INTERRUPT (emit Event, persist     │
│                        State, wait for /chat/resume)      │
├──────────────────────────────────────────────────────────┤
│ OUTPUT GUARDRAIL                                         │
│ ├── If ragSearch used → REQUIRE citations                │
│ └── If retrieval empty (below minScore) → refuse to answer│
├──────────────────────────────────────────────────────────┤
│ LOOP LIMIT                                               │
│ ├── maxSteps (default 12)                                │
│ └── Token budget (per-conversation cost cap)             │
└──────────────────────────────────────────────────────────┘
```

---

## 8. Observability & Eval: Khoảng Trống Lớn Nhất 2026

### 8.1 The 37-Point Gap

89% teams có observability, nhưng chỉ **52% có evals**. Đây là nơi production quality CHẾT.

### 8.2 Eval Tiers (Industry Best Practice)

| Tier | Frequency | Method |
|---|---|---|
| **Fast checks** | Every PR | Did agent call the right tool? Did it cite sources? |
| **Nightly regression** | Daily | LLM-as-judge trên bộ câu hỏi vàng |
| **Production monitoring** | Continuous | Drift detection, cost anomaly, latency spike |

### 8.3 Our Eval Plan (P13)

```
eval/
├── questions.json         # Bộ câu hỏi vàng (có ground truth)
├── rag_eval_test.go       # Recall@k, faithfulness (LLM judge)
├── agent_eval_test.go     # Tool selection accuracy, task completion
└── README.md              # Cách chạy: go test -tags eval
```

---

## 9. Decision Tree: Khi Nào Dùng Gì?

Dựa trên [Anthropic's Building Effective Agents](https://www.anthropic.com/engineering/building-effective-agents) + 2026 updates:

```
Start here ↓

Cần autonomous decisions? 
├── NO  → Simple LLM call (không cần agent)
└── YES → Cần sequential steps?
          ├── NO  → Single agent với tools (ReAct)
          └── YES → Cần branching logic?
                    ├── NO  → Prompt chaining (deterministic pipeline)
                    └── YES → Cần parallel execution?
                              ├── NO  → ReAct loop với tools
                              └── YES → Cần multi-agent?
                                        ├── NO  → Fan-out tool execution (errgroup)
                                        └── YES → Orchestrator-workers pattern
```

**Project của em:** Đang ở ô **"ReAct loop với tools + fan-out execution"** — đúng vị trí cho hầu hết production use cases. Multi-agent (nếu cần sau này) sẽ thêm Orchestrator layer lên trên engine hiện tại.

---

## 10. Tổng Kết: Project Của Em Đang Đi Đúng Hướng Không?

### ✅ ĐIỂM MẠNH (so với industry 2026)

| Điểm mạnh | Evidence |
|---|---|
| **Provider abstraction** | Pluggable Gemini + Claude — đúng pattern "don't lock into one model" |
| **Tool classification** | Read/Write/Destructive với Kind enum — đúng pattern "guardrails at tool layer" |
| **Pure router** | `route(s) NodeID` pure function — đúng pattern "deterministic routing" |
| **Fan-out execution** | `errgroup` parallel tool calls — đúng pattern "parallelization" |
| **Memory 3-tier** | Working + Episodic + Semantic — đúng industry standard 2026 |
| **Context engineering** | Tách DATA vs INSTRUCTION, cache system+tools — đúng best practice |
| **HITL design** | Interrupt + persist State + resume — đúng pattern "human-in-the-loop" |
| **Go concurrency** | Goroutine + channel + errgroup — production-grade performance |

### ⚠️ ĐIỂM CÒN THIẾU (sẽ bổ sung)

| Thiếu | Plan |
|---|---|
| **Eval harness** | P13 — LLM-as-judge + bộ câu hỏi vàng |
| **Prompt caching** | P6 — Anthropic cache_control, Gemini context cache |
| **Procedural memory** | Future — LEGOMem pattern |
| **Multi-agent orchestration** | Future — Orchestrator-workers nếu cần |
| **MCP compliance** | Future — map Tool interface sang MCP nếu cần integrate |

### 🎯 CÂU TRẢ LỜI PHỎNG VẤN

Nếu được hỏi *"Tại sao em tự build agent engine thay vì dùng LangGraph?"*:

> "Em chọn tự build vì 2 lý do. Thứ nhất, đây là dự án học tập — em muốn hiểu agent loop, tool system, memory architecture từ gốc trước khi dùng framework. Thứ hai, engine của em follow đúng industry pattern 2026: ReAct loop với pure function router, fan-out tool execution bằng Go errgroup, 3-tier memory, guardrails ở tool layer thay vì output layer. Em có thể giải thích từng dòng code hoạt động thế nào — điều mà nếu chỉ dùng LangGraph em sẽ không làm được. Sau khi hiểu rồi, em có thể migrate sang LangGraph trong 1 tuần nếu dự án cần."

---

## 11. References

| Resource | URL |
|---|---|
| O'Reilly AI Agents Stack 2026 | https://www.oreilly.com/radar/the-ai-agents-stack-2026-edition/ |
| MLflow Production Agents 2026 | https://mlflow.org/articles/building-production-ready-ai-agents-in-2026/ |
| MLflow Agent Architecture Types | https://mlflow.org/articles/types-of-ai-agent-architectures-2026-developer-guide/ |
| Google Agent Bake-Off Tips | https://developers.googleblog.com/build-better-ai-agents-5-developer-tips-from-the-agent-bake-off/ |
| Taskade 21 Agentic Design Patterns | https://www.taskade.com/blog/agentic-design-patterns |
| FutureAGI LLM Agent Architectures | https://futureagi.com/blog/llm-agent-architectures-core-components/ |
| Anthropic Building Effective Agents | https://www.anthropic.com/engineering/building-effective-agents |
| CrewAI vs LangGraph vs AutoGen 2026 | https://futureagi.com/blog/crewai-vs-langgraph-vs-autogen-2026/ |
| IEEE Multi-Agent Framework Comparison | https://ieeexplore.ieee.org/abstract/document/11481053 |
| OWASP MCP Top 10 | https://owasp.org/www-project-mcp-top-10/ |
