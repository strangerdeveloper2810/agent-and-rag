# Loop Engineering — "Trái Tim" Của AI Agent

> **Audience:** FE dev learning Go backend. Đây là doc QUAN TRỌNG NHẤT để hiểu agent hoạt động thế nào.
> **Quote 2026:** *"I don't prompt Claude anymore. I have loops running that prompt Claude and figuring out what to do. My job is to write loops."* — Boris Cherny, Claude Code lead at Anthropic.
>
> **Sources:** [Data Science Dojo: 10 Loop Engineering Patterns](https://datasciencedojo.com/blog/loop-engineering-design-patterns/), [FutureAGI: Agent Loop Guide](https://futureagi.com/glossary/agent-loop/), [Data Science Dojo: Agentic Loops Guide](https://datasciencedojo.com/blog/agentic-loops-explained-from-react-to-loop-engineering-2026-guide/), [FutureAGI: ReAct Agent Loop](https://futureagi.com/blog/loop-engineering/react-agent-loop/), [Steve Kinney: Anatomy of an Agent Loop](https://stevekinney.com/writing/agent-loops), [Orkes: Building Durable Loops](https://orkes.io/blog/building-durable-loops-with-conductor-part-1)

---

## 0. Loop Engineering Là Gì?

### Không phải prompt engineering — là CONTROL SYSTEM ENGINEERING

```
2023: "Hãy viết prompt thật hay"              ← Prompt Engineering
2024: "Hãy dùng framework có sẵn"             ← Framework Era
2025: "Hãy thiết kế VÒNG LẶP cho agent"       ← Loop Engineering
2026: "Hãy build factory chạy nhiều loop"     ← Harness Engineering
```

**Loop engineering = thiết kế MÔI TRƯỜNG THỰC THI** cho agent:
- Khi nào bắt đầu? (trigger)
- Điều kiện dừng là gì? (termination)
- Làm sao biết đang tiến bộ? (progress feedback)
- Làm sao tránh chạy vô hạn? (safety bounds)
- Làm sao phục hồi khi lỗi? (error recovery)

---

## 1. 10 Loop Engineering Patterns (2026 Industry Standard)

### TỔNG QUAN 3 TIER

```
┌──────────────────────────────────────────────────────────────────┐
│ TIER 3: PRODUCTION HARDENING (nơi hầu hết thất bại)              │
│ 8. Circuit Breaker    9. Heartbeat Loop    10. Bounded Execution │
├──────────────────────────────────────────────────────────────────┤
│ TIER 2: PRACTITIONER                                              │
│ 5. Ralph Loop    6. Evaluator-Optimizer    7. Multi-Agent Sup.   │
├──────────────────────────────────────────────────────────────────┤
│ TIER 1: FOUNDATIONAL (bắt đầu từ đây)                             │
│ 1. ReAct Loop    2. Reflection    3. Tool Use    4. Prompt Chain │
└──────────────────────────────────────────────────────────────────┘
```

---

### PATTERN 1: ReAct Loop (Reason + Act)

**Đây là pattern CƠ BẢN NHẤT — mọi agent đều bắt đầu từ đây.**

```
┌─────────────────────────────────────────────────────────┐
│  while (!done) {                                        │
│    thought = LLM.think(state)     // REASON             │
│    if (thought.hasToolCalls()) {                        │
│      results = executeTools(thought.toolCalls) // ACT   │
│      state.add(results)           // OBSERVE            │
│    } else {                                             │
│      return thought.text          // FINAL ANSWER       │
│    }                                                    │
│  }                                                      │
└─────────────────────────────────────────────────────────┘
```

**Canonical pseudocode (mọi framework dùng):**
```javascript
while (!done) {
  const response = await callLLM(messages);
  if (response.toolCalls.length > 0) {
    const results = await executeTools(response.toolCalls);
    messages.push(...results);
  } else {
    done = true;
    return response;
  }
}
```

**Cơ chế KEY:** `tool_calls` = tín hiệu "TÔI CẦN THÊM THÔNG TIN". `text-only` = tín hiệu "TÔI CÓ ĐỦ ĐỂ TRẢ LỜI". KHÔNG cần explicit `stop` tool — model tự biết khi nào dừng.

**Mapping vào project của em (P2.5):**
```go
// File: internal/agent/engine.go
func (e *Engine) Run(ctx context.Context, in RunInput, emit EmitFunc) error {
    s := newState(in)
    node := NodeModel
    for {                                    // ← ĐÂY LÀ REACT LOOP
        if ctx.Err() != nil { return ctx.Err() }
        next, err := e.dispatch(ctx, node, s, emit)
        if err != nil { return err }
        if next == NodeEnd { break }         // ← NATURAL TERMINATION
        node = next
    }
    return nil
}
```

---

### PATTERN 2: Reflection Loop (Tự Phê Bình)

```
Generate → Self-Critique → Find Gaps → Improve → Generate → ...
```

**Khác với ReAct:** Agent tự review output CỦA CHÍNH MÌNH trước khi trả về. Không cần external validator.

**Mapping vào project em (P8):**
```go
// File: internal/agent/node_reflect.go (P8)
func nodeReflect(ctx context.Context, eng modelEngine, s *State, emit EmitFunc) (NodeID, error) {
    // Gửi output hiện tại cho LLM với prompt: "Kiểm tra câu trả lời này:
    // - Có thiếu thông tin không?
    // - Có dẫn nguồn đầy đủ không?
    // - Có logic nào sai không?"
    // Nếu OK → END. Nếu có vấn đề → quay lại MODEL để sửa.
}
```

---

### PATTERN 3: Tool Use Loop

```
LLM → Tool Call → Execute → Result → LLM → Tool Call → ... → Final
```

Agent gọi external tools trong loop. **Đây là pattern SẢN XUẤT NHIỀU NHẤT.**

**Mapping vào project em:**
```go
// File: internal/tools/registry.go (đã có)
func (r *Registry) RunParallel(ctx context.Context, calls []provider.ToolCall) []CallResult {
    // Fan-out: N tool calls chạy SONG SONG bằng errgroup
    results := make([]CallResult, len(calls))
    var g errgroup.Group
    for i, call := range calls {
        i, call := i, call
        g.Go(func() error {
            results[i] = r.runOne(ctx, call)
            return nil
        })
    }
    g.Wait()
    return results
}
```

---

### PATTERN 4: Prompt Chaining (Deterministic Pipeline)

```
Output A → Input B → Output B → Input C → Output C
```

Code điều khiển flow, KHÔNG PHẢI agent. Predictable nhất, ít autonomy nhất.

**Mapping vào project em:** Context assembly (P6):
```go
// 1. Recall memory → 2. Build prompt → 3. Call LLM → 4. Extract facts
// Mỗi bước là deterministic, không có branching
```

---

### PATTERN 5: Ralph Loop (Continuous Until Valid)

```
while (!externalValidator.pass()) {
    resetContext()          // ← KEY: fresh context mỗi lần lặp
    generate()
    externalValidator.check()  // compiler, linter, test suite
}
```

**Đặt tên theo Geoffrey Huntley (2025):** bash one-liner chạy agent đến khi test xanh.  
**KEY INSIGHT:** Mỗi iteration RESET CONTEXT — state nằm trên filesystem, không trong context window.

**Dùng khi:** Code generation có test suite, tasks có verifiable pass/fail signal.

---

### PATTERN 6: Evaluator-Optimizer Loop

```
Generator Agent → Output → Evaluator Agent (ĐỘC LẬP) → Structured Feedback → Generator sửa → ...
```

**Khác Reflection ở chỗ:** Evaluator là agent RIÊNG BIỆT (có thể dùng model khác). Generator không thể "tự dối mình" vì critic là độc lập.

**Mapping vào project em (P13 eval harness):**
```go
// LLM-as-judge: 1 model sinh, 1 model KHÁC đánh giá
func evaluateFaithfulness(answer, context string) Score {
    judgePrompt := "So sánh câu trả lời với context. Có bịa không?"
    return callJudge(judgePrompt)
}
```

---

### PATTERN 7: Multi-Agent Supervisor Loop

```
Supervisor
├── Researcher Agent (internal ReAct loop)
├── Coder Agent (Ralph loop với compiler check)
└── QA Agent (Evaluator-Optimizer loop)
```

Mỗi worker agent có loop RIÊNG. Supervisor điều phối, KHÔNG làm việc.

**Mapping vào project em (future — sau P14):**
```go
type Orchestrator struct {
    agents map[string]*Engine  // mỗi agent là 1 engine riêng
}
func (o *Orchestrator) Delegate(task Task) Result {
    agent := o.selectAgent(task)  // route dựa trên task type
    return agent.Run(ctx, task)
}
```

---

### PATTERN 8: Circuit Breaker ⚡ PRODUCTION CRITICAL

```
Monitor progress → N iterations không tiến bộ → TRIP → Terminate + Alert
```

**Đây là pattern SỐNG CÒN cho production.** Không có nó = agent cháy tiền vô hạn.

```
Same file state × 3 cycles?  → TRIP
Same error message × 3?      → TRIP
No measurable progress?      → TRIP
```

**Mapping vào project em (P10 guardrails):**
```go
// File: internal/guardrails/circuit_breaker.go (P10)
type CircuitBreaker struct {
    lastActions []string  // ring buffer của N action gần nhất
    maxRepeats  int       // default: 3
}

func (cb *CircuitBreaker) Check(action string) error {
    cb.lastActions = append(cb.lastActions, action)
    if len(cb.lastActions) > cb.maxRepeats {
        cb.lastActions = cb.lastActions[1:]
    }
    if allEqual(cb.lastActions) {
        return ErrStuck{same: action, count: cb.maxRepeats}
    }
    return nil
}
```

---

### PATTERN 9: Heartbeat Loop

```
Schedule/Event → Wake → Check condition → Act if needed → Sleep → ...
                 ↑_____________________________________________|
```

Agent chạy theo lịch hoặc event trigger, không phải theo request.

**Dùng khi:** Monitoring, scheduled tasks, background jobs.

---

### PATTERN 10: Bounded Execution + Context Engineering

**2 mặt của cùng 1 đồng xu:**

#### A) Bounded Execution — Hard Caps

| Cap | Default | Ngăn gì |
|---|---|---|
| `maxSteps` | 12 | Runaway loop |
| `tokenBudget` | 100K input + 10K output | Cost explosion |
| `wallTime` | 60s per request | Hanging request |
| `costCap` | $0.50 per trace | Billing shock |

#### B) Context Engineering — Quản Lý Context Window

> **Thống kê 2026:** Tool responses chiếm **67.6% tokens** trong production agent. System prompt chỉ 3.4%.

4 chiến lược:

| Strategy | Cách làm | Ví dụ |
|---|---|---|
| **Write context** | Lưu ra ngoài window | Scratchpad file, todo.md, progress notes |
| **Select context** | Kéo vào khi cần | Grep file, glob search, RAG retrieval |
| **Compress context** | Nén cũ thành summary | Tool output >5K chars → summarize |
| **Isolate context** | Sub-agent với window riêng | 10K token subtask → 1K summary về main loop |

**Mapping vào project em (P6):**
```go
// File: internal/agent/context.go (P6)
func assemblePrompt(s *State, mems []Memory, docs []Document) string {
    // Thứ tự CỐ ĐỊNH:
    // 1. SYSTEM (cố định — cache được)
    // 2. TOOLS  (cố định — cache được)
    // 3. SKILLS (cố định — cache được)
    // 4. [BỘ NHỚ] — memory recall
    // 5. [DỮ LIỆU THAM KHẢO] — RAG results ← TÁCH DATA vs INSTRUCTION
    // 6. [HỘI THOẠI] — history đã trim
    //
    // Nếu vượt token budget → trim history, giữ K message gần nhất + summary (P7)
}
```

---

## 2. Loop Engineering Trong Project Của Em: TỪNG DÒNG CODE

### 2.1 Kiến Trúc Loop Hiện Tại (P2)

```
                    ┌─────────────────────────────┐
                    │  engine.Run(ctx, input, emit) │
                    └─────────────┬───────────────┘
                                  │
                    ┌─────────────▼───────────────┐
                    │  node := NodeModel           │
                    │  for {                       │
                    │    ctx.Err()? → return       │  ← CANCEL CHECK
                    │    next, err := dispatch()   │
                    │    err != nil? → return      │  ← ERROR HANDLING
                    │    next == END? → break      │  ← NATURAL TERMINATION
                    │    node = next               │
                    │  }                           │
                    │  emit(DoneEvent)             │
                    └─────────────────────────────┘
```

### 2.2 4 Lớp Termination (Đầy Đủ Theo Industry Standard)

| Layer | Cơ chế | Code trong project |
|---|---|---|
| **L1: Explicit** | `len(toolCalls) == 0` → END | `router.go`: assistant không có tool_calls → `NodeEnd` |
| **L2: Resource Caps** | `maxSteps >= 12` → END | `router.go`: `s.Step >= s.MaxSteps` → `NodeEnd` |
| **L3: Behavioral** | Circuit breaker (P10) | Phát hiện same action × 3 → trip |
| **L4: Evaluator** | Reflection node (P8) | Model khác kiểm tra "task đã hoàn thành chưa?" |

### 2.3 So Sánh Implementation: Go vs JavaScript

| Thành phần | JavaScript (Node.js) | Go (project của em) |
|---|---|---|
| **Loop** | `while (true)` hoặc LangGraph graph | `for {}` với explicit dispatch |
| **Concurrent tools** | `Promise.all(tools.map(fn))` | `errgroup` + goroutine, pre-allocated slice by index |
| **Cancellation** | `AbortController.signal` | `context.Context` — truyền qua MỌI hàm |
| **Streaming** | `for await (chunk of stream)` | `for chunk := range channel` — same mental model |
| **Error handling** | `try/catch` — lỗi = exception | `if err != nil` — lỗi = return value |
| **Tool errors** | Throw exception → loop crash | Return error string → LLM sees & self-corrects |
| **State snapshot** | `const state = {...prev}` spread | `RWMutex.RLock()` snapshot struct fields |

---

## 3. Production Hardening Checklist

Đây là checklist cho production agent loop — **em sẽ có thể check TỪNG MỤC sau P10:**

### 3.1 Loop Safety

- [ ] **Max iterations cap** — không loop quá 12 steps (P2: `router.go`)
- [ ] **Context cancellation** — `ctx.Done()` check MỌI vòng lặp (P2: `engine.go`)
- [ ] **Per-tool timeout** — mỗi tool có `context.WithTimeout` riêng (P3)
- [ ] **Token budget** — cắt khi vượt ngưỡng (P6)
- [ ] **Circuit breaker** — phát hiện stuck loop (P10)
- [ ] **Cost cap per trace** — $0.50 max (P11)

### 3.2 Error Recovery

- [ ] **Tool errors → observations** — lỗi tool thành text cho LLM đọc (P2: `AppendObservation`)
- [ ] **Panic recovery** — `defer recover()` trong mỗi goroutine (P2.4)
- [ ] **Retry with backoff** — LLM 429/5xx → exponential backoff (P1: trong provider)
- [ ] **Idempotent tool execution** — retry không double side-effect (P4)

### 3.3 Context Management

- [ ] **Context assembly order** — system → tools → memory → data → history (P6)
- [ ] **DATA vs INSTRUCTION separation** — chống prompt injection (P6)
- [ ] **Prompt caching** — system+tools đánh dấu cache (P6: 10x rẻ hơn)
- [ ] **History trimming** — giữ K message gần nhất + summary (P7)

### 3.4 Observability

- [ ] **Step count per trace** — histogram (P11)
- [ ] **Token usage per step** — cost attribution (P11)
- [ ] **Tool latency per call** — slow tool detection (P11)
- [ ] **Goal progress metric** — phẳng = stuck (P13)

---

## 4. Các Anti-Pattern Cần Tránh

| Anti-Pattern | Mô tả | Cách phòng |
|---|---|---|
| **Infinite loop** | Agent refine mãi không dừng | `maxSteps` + circuit breaker |
| **Goal drift** | Agent đuổi theo goal khác sau nhiều bước | Goal pinning: goal gốc LUÔN trong system prompt |
| **Context rot** | Context window đầy → output quality giảm | Compress old history, isolate sub-agents |
| **Premature termination** | Agent tự tin nói "done" khi mới làm 30% | Reflection node, evaluator check |
| **Hallucinated tool calls** | Gọi tool không tồn tại hoặc sai params | JSON Schema validation + `NotFoundError` |
| **Recovery storms** | Retry mù quáng → rate limit + cost explosion | Exponential backoff + max retries |
| **Reward hacking** | Agent học cách "có vẻ done" thay vì "thực sự done" | External evaluator, không self-assessment |

---

## 5. Loop Complexity Decision Tree

```
Bắt đầu ở đây ↓

Cần tool không?
├── NO  → Prompt Chaining (deterministic, rẻ)
└── YES → ReAct Loop (Pattern 1)
           │
           Cần tự sửa lỗi?
           ├── NO  → ReAct là đủ
           └── YES → Thêm Reflection (Pattern 2)
                      │
                      Cần verifiable pass/fail?
                      ├── NO  → Reflection là đủ
                      └── YES → Ralph Loop (Pattern 5: test/compiler check)
                                 │
                                 Nhiều domain khác nhau?
                                 ├── NO  → Single agent + tools
                                 └── YES → Multi-Agent (Pattern 7)
                                            │
                                            LUÔN LUÔN thêm:
                                            ├── Bounded Execution (Pattern 10)
                                            └── Circuit Breaker (Pattern 8)
```

**Quy tắc vàng:** Start with ReAct. Hầu hết task "có vẻ cần" Multi-Agent hoặc Tree-of-Thoughts có thể giải bằng **well-prompted ReAct loop với chi phí thấp hơn nhiều.**

---

## 6. Cost Reality

| Pattern | Token Cost (so với chat thường) |
|---|---|
| Single chat | 1× (baseline) |
| Single-agent loop | ~4× |
| Multi-agent system | ~15× |

**Insight:** Multi-agent không phải lúc nào cũng tốt hơn. Chỉ thêm khi single-agent loop thực sự không giải được task.

---

## 7. Tổng Kết: Loop Engineering = Kỹ Năng CỐT LÕI 2026

```
2023: Prompt Engineer      → "Viết prompt hay"
2024: Agent Developer      → "Dùng framework"
2025: Loop Engineer        → "Thiết kế vòng lặp"
2026: Harness Engineer     → "Build factory chạy nhiều loop"
```

**Em đang ở vị trí ĐẶC BIỆT:** Vừa học loop engineering (P2), vừa học Go concurrency (goroutine/channel/errgroup), vừa hiểu được MỌI DÒNG CODE trong loop của mình — điều mà dev chỉ dùng LangGraph không làm được.

**Câu trả lời phỏng vấn cho "Em hiểu gì về agent loop?":**

> "Em tự build agent loop trong Go theo ReAct pattern. Loop có 4 lớp termination: explicit (model hết tool_calls), resource cap (maxSteps=12), behavioral (circuit breaker phát hiện stuck), và evaluator (reflection node kiểm tra task completion). Tool execution dùng fan-out errgroup — mỗi goroutine ghi vào pre-allocated slice by index, không lock. Context được quản lý theo thứ tự cố định: system → tools → memory → data → history, với DATA tách khỏi INSTRUCTION để chống prompt injection. Em có thể trace từng dòng từ HTTP request → engine.Run() → dispatch → model → route → tools → SSE response."
