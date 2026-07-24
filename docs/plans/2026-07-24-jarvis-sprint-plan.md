# J.A.R.V.I.S. Personal — Sprint Plan (2-4 Tuần)
# J.A.R.V.I.S. Personal — Kế Hoạch Rút Gọn

> **Ngày/Date:** 2026-07-24
> **Nguyên tắc/Principle:** YAGNI tối đa. Chỉ build thứ DÙNG ĐƯỢC NGAY. Mọi thứ khác → v2.
> **Mục tiêu/Goal:** Cuối tuần 4, JARVIS chạy local, chat được, nhớ được, tool được.

---

## 0. CẮT GÌ? / WHAT'S CUT?

| Phase gốc | Quyết định | Lý do |
|---|---|---|
| P8 Planner + Reflection | ❌ CẮT | Model 2026 tự reasoning — không cần external planner |
| P9 Skills | ❌ CẮT | System prompt đủ dùng. Skills = v2 |
| P10 Guardrails + HITL | ✂️ GIỮ TỐI THIỂU | Chỉ circuit breaker + maxSteps. Bỏ HITL (single user) |
| P17 MCP Device Layer | ❌ CẮT | Tool cứng trong code trước. MCP = v2 |
| P19 Proactive Engine | ❌ CẮT | Reactive trước. Proactive = v2 |
| P20 Personality Engine | ❌ CẮT | System prompt cứng. Learn = v2 |
| P21 Desktop App | ❌ CẮT | CLI + Web UI là đủ |
| P22 Mobile | ❌ CẮT | v2 |
| P12 Gateway Integration | ❌ CẮT | Bỏ hẳn Fastify/TS. Go serve direct |
| P5 RAG (Voyage) | ✂️ ĐỔI | Ollama embedding thay Voyage |

---

## 1. LỘ TRÌNH 4 TUẦN / 4-WEEK SPRINT

```
TUẦN 1: ENGINE CHẠY ĐƯỢC
TUẦN 2: CHAT THẬT VỚI OLLAMA
TUẦN 3: NHỚ + TOOLS
TUẦN 4: HOÀN THIỆN + POLISH
```

---

## TUẦN 1: ENGINE LÕI + CHAT ĐƯỢC (FAKE LLM)

### Mục tiêu: `curl POST /chat` → SSE stream response

```
NGÀY 1-2: Agent Engine
□ P2.3 Node Model (FakeProvider + TDD)
□ P2.4 Node Tools (fan-out + echo tool)
□ P2.5 Engine Run Loop (full scenario test)
□ P2.6 Transport /chat SSE endpoint

NGÀY 3: Provider thật
□ P1.x FakeProvider ← EM ĐANG LÀM DỞ
□ Test: full loop với FakeProvider → SSE events đúng thứ tự

NGÀY 4-5: CLI + Wire up
□ P18 CLI: `jarvis chat` (interactive) + `jarvis ask "..."` (one-shot)
□ Go serve HTTP + SSE TRỰC TIẾP (bỏ Fastify)
□ Test end-to-end: `echo "hello" | jarvis ask` → response

CUỐI TUẦN 1 DELIVERABLE:
$ jarvis serve
$ curl -N -X POST localhost:8080/chat \
  -H "Content-Type: application/json" \
  -d '{"userMessage":"hello"}'
→ data: {"type":"text","text":"Hello!"}
→ data: {"type":"done",...}
```

### Task chi tiết:

#### T1.1 — FakeProvider (15 phút)
```go
// internal/provider/fake.go
// EM TỰ CODE — anh đã hướng dẫn ở P2.3 Part A
```

#### T1.2 — Node Model (30 phút)
```go
// internal/agent/node_model.go
// EM TỰ CODE — test đã có ở node_model_test.go
```

#### T1.3 — Node Tools (30 phút)
```go
// internal/agent/node_tools.go
// Lấy tool_calls từ LastAssistant() → registry.RunParallel() → AppendObservation
```

#### T1.4 — Engine Run Loop (45 phút)
```go
// internal/agent/engine.go
type Engine struct {
    provider provider.Provider
    registry *tools.Registry
}

func (e *Engine) Run(ctx context.Context, in RunInput, emit EmitFunc) (provider.Usage, error) {
    s := newState(in)
    node := NodeModel

    for {
        select {
        case <-ctx.Done():
            return s.Usage, ctx.Err()
        default:
        }

        emit(StepEvent(node))
        next, err := e.dispatch(ctx, node, s, emit)
        if err != nil {
            emit(ErrorEvent(err.Error()))
            return s.Usage, err
        }
        if next == NodeEnd {
            break
        }
        node = next
    }

    emit(DoneEvent(s.Usage))
    return s.Usage, nil
}

func (e *Engine) dispatch(ctx context.Context, node NodeID, s *State, emit EmitFunc) (NodeID, error) {
    switch node {
    case NodeModel:
        return nodeModel(ctx, e.provider, e.registry, s, emit)
    case NodeTools:
        return nodeTools(ctx, e.registry, s, emit)
    default:
        return NodeEnd, nil
    }
}
```

#### T1.5 — SSE Chat Endpoint (45 phút)
```go
// internal/transport/http/chat.go
func (h *ChatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // 1. Parse JSON body → RunInput
    // 2. Set SSE headers:
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    // 3. Create emit function → flush JSON mỗi event
    flusher := w.(http.Flusher)
    emit := func(e agent.Event) {
        data, _ := json.Marshal(e)
        fmt.Fprintf(w, "data: %s\n\n", data)
        flusher.Flush()
    }
    // 4. Run engine với context từ request (auto-cancel khi client disconnect)
    _, err := h.engine.Run(r.Context(), input, emit)
}
```

#### T1.6 — Go Serve Direct (bỏ Fastify) (30 phút)
```go
// cmd/server/main.go — cập nhật
func main() {
    // ... load config, create engine, registry ...
    
    mux := http.NewServeMux()
    mux.HandleFunc("GET /healthz", Healthz)
    mux.HandleFunc("POST /chat", chatHandler.ServeHTTP)
    
    // Serve static web UI (optional)
    mux.Handle("GET /", http.FileServer(http.Dir("./web/dist")))
    
    server := &http.Server{Addr: ":8080", Handler: mux}
    // ... graceful shutdown ...
    server.ListenAndServe()
}
```

---

## TUẦN 2: OLLAMA + CHAT THẬT

### Mục tiêu: JARVIS gọi LLM thật (local), hiểu và trả lời

```
NGÀY 1-2: Ollama Adapter
□ P15.1 Ollama provider (Generate + Embed)
□ Test với httptest.Server giả lập Ollama API
□ Factory: thêm case "ollama"

NGÀY 3: SQLite cơ bản
□ P16.1 SQLite conversations + messages (thay MongoDB)
□ Schema auto-migrate
□ Test với :memory:

NGÀY 4-5: Tích hợp
□ Wire Engine với Ollama + SQLite
□ Chat thật: "Hello JARVIS" → Ollama trả lời
□ Test manual: chạy server, curl chat, thấy response từ LLM thật

CUỐI TUẦN 2 DELIVERABLE:
$ jarvis serve
$ jarvis ask "What is 2+2?"
→ "2+2 equals 4."
$ jarvis chat
You> Nhớ là tôi thích cafe đen nhé
JARVIS> Đã ghi nhớ. Lần sau tôi sẽ nhắc.
```

### Task mới cần viết:

#### T2.1 — Ollama Generate (TDD)
```go
// internal/provider/ollama/ollama.go
type Client struct {
    baseURL    string        // http://localhost:11434
    model      string        // llama3.1:8b
    httpClient *http.Client
}

func New(baseURL, model string) *Client { ... }

func (c *Client) Generate(ctx context.Context, req provider.GenerateRequest) (<-chan provider.StreamChunk, error) {
    // POST /api/chat → parse streaming JSON → channel StreamChunk
    // Giống hệt pattern trong gemini.go nhưng gọi Ollama REST API
}
```

#### T2.2 — Ollama Embed (cho memory)
```go
func (c *Client) Embed(ctx context.Context, texts []string) ([][]float32, error) {
    // POST /api/embed → parse {"embeddings": [[...], ...]}
}
```

#### T2.3 — SQLite Store
```go
// internal/storage/sqlite/sqlite.go
type Store struct {
    db *sql.DB  // database/sql + SQLite driver
}

func (s *Store) CreateConversation(title string) (*Conversation, error) { ... }
func (s *Store) AddMessage(convID string, msg Message) error { ... }
func (s *Store) GetMessages(convID string) ([]Message, error) { ... }
```

---

## TUẦN 3: MEMORY + TOOLS THẬT

### Mục tiêu: JARVIS nhớ được + gọi được tool

```
NGÀY 1-2: Memory 3-tier
□ P7.1 Memory store (SQLite)
□ P7.2 Node recall (structured + vector lookup)
□ P7.3 Node extract (LLM trích facts sau mỗi lượt chat)
□ P7.4 Node summarize (nén history dài)

NGÀY 3-4: Tools thật (5 tools)
□ File search/read (grep, cat — local filesystem)
□ Web search (SearXNG local hoặc API)
□ Memory CRUD (saveMemory, recallMemory)
□ Shell exec (có confirm)

NGÀY 5: Context engineering
□ P6 Prompt assembly: system → tools → memory → data → history
□ DATA vs INSTRUCTION separation (chống prompt injection)
□ Token budget + trim

CUỐI TUẦN 3 DELIVERABLE:
You> Tìm file config của dự án X
JARVIS> [gọi file.search] Tìm thấy 3 file: ...
You> Tôi thích cafe đen, nhớ nhé
JARVIS> [gọi saveMemory] Đã lưu.
...2 ngày sau...
You> Tôi uống gì nhỉ?
JARVIS> [gọi recallMemory] Bạn thích cafe đen không đường.
```

---

## TUẦN 4: POLISH + HOÀN THIỆN

### Mục tiêu: JARVIS xài được hằng ngày

```
NGÀY 1-2: Web UI + Frontend
□ Giữ apps/web (React), connect thẳng vào Go server (bỏ api gateway)
□ Stream chat UI (đã có SSE parsing)
□ Conversation list + memory viewer

NGÀY 3: Circuit breaker + Rate limit
□ P10 minimal: circuit breaker (phát hiện stuck loop)
□ maxSteps + token budget

NGÀY 4-5: Test + Docs + Install
□ E2E test: 10 kịch bản chat thật
□ README hoàn chỉnh (song ngữ)
□ Install script: `curl ... | sh`
□ Docker image (single binary)

CUỐI TUẦN 4 DELIVERABLE:
- 1 binary `jarvis` (~30MB)
- `jarvis serve` → web UI ở localhost:8080
- Chat + memory + tools hoạt động
- README + install script
- Demo video 2 phút
```

---

## 2. SO SÁNH: TRƯỚC VS SAU

| | Plan gốc (22 phases, 6 tháng) | Sprint plan (4 tuần) |
|---|---|---|
| **Phases** | P0-P22 | P0-P7 + P15-P16 + P18 |
| **Thời gian** | 6 tháng | 4 tuần |
| **Engine** | Đầy đủ (ReAct + Plan + Reflect) | Chỉ ReAct (đủ dùng) |
| **Memory** | 4-tier (working + episodic + semantic + procedural) | 3-tier (working + semantic) |
| **Tools** | 25+ MCP devices | 5-7 tools built-in |
| **LLM** | Tiered (Ollama → Gemini → Claude) | Ollama (local) + optional cloud |
| **Gateway** | Fastify/TS | Go serve direct |
| **UI** | Web + CLI + Desktop + Mobile | CLI + Web |
| **DB** | MongoDB Atlas | SQLite local |
| **V2 sẽ thêm** | — | MCP, Proactive, Personality, Desktop app |

---

## 3. CẤU TRÚC FILE SAU 4 TUẦN

```
services/jarvis/                    # đổi tên từ agent-go
├── cmd/jarvis/
│   └── main.go                     # entry point: CLI + HTTP server
├── internal/
│   ├── agent/                      # Engine (P2)
│   │   ├── state.go, event.go      # ✅ done
│   │   ├── router.go               # ✅ done
│   │   ├── node_model.go           # 🔜 em làm
│   │   ├── node_tools.go           # 🔜 tuần 1
│   │   ├── engine.go               # 🔜 tuần 1
│   │   └── *_test.go
│   ├── provider/
│   │   ├── provider.go, types.go   # ✅ done
│   │   ├── fake.go                 # 🔜 em làm
│   │   ├── ollama/                 # 🔜 tuần 2
│   │   ├── gemini/                 # ✅ done (cloud fallback)
│   │   └── factory/
│   ├── tools/
│   │   ├── tool.go, registry.go    # ✅ done
│   │   ├── files.go                # 🔜 tuần 3
│   │   ├── memory.go               # 🔜 tuần 3
│   │   ├── web.go                  # 🔜 tuần 3
│   │   └── shell.go               # 🔜 tuần 3
│   ├── memory/
│   │   ├── store.go                # 🔜 tuần 3
│   │   ├── recall.go, extract.go   # 🔜 tuần 3
│   │   └── summarize.go            # 🔜 tuần 3
│   ├── storage/
│   │   └── sqlite/                 # 🔜 tuần 2
│   ├── config/
│   └── transport/http/
│       ├── health.go               # ✅ done
│       ├── chat.go                 # 🔜 tuần 1
│       └── ui.go                   # 🔜 tuần 4 (serve React)
├── web/                            # React UI (move từ apps/web)
├── skills/                         # cắt → v2
├── go.mod, go.sum
├── Dockerfile
└── README.md                       # 🔜 tuần 4
```

---

## 4. CHECKLIST HÀNG NGÀY

Mỗi ngày kết thúc với:

```bash
cd services/jarvis
go vet ./...          # Không warning
go test ./... -race   # Tất cả xanh, không race
go build ./...        # Build thành công
git diff --stat       # Review thay đổi
git commit -m "feat(jarvis): ..."  # Commit nhỏ, thường xuyên
```

Mỗi tuần kết thúc với:
```bash
$ jarvis serve                        # Server chạy
$ jarvis ask "hello"                  # Chat được
$ curl localhost:8080/healthz         # Health check OK
```

---

## 5. TỔNG KẾT

```
BẮT ĐẦU (hiện tại)              KẾT THÚC (4 tuần)
─────────────────────          ─────────────────────
P2 dở dang                     JARVIS v1.0
Engine chưa chạy               Engine ReAct hoàn chỉnh
Gemini/Claude API              Ollama local LLM
MongoDB Atlas (cloud)          SQLite (local)
Fastify/TS gateway             Go serve trực tiếp
Chạy test, chưa dùng được      Dùng hằng ngày: chat, nhớ, tools
```

**Quy tắc vàng của sprint này:** Mỗi ngày code xong 1 thứ CHẠY ĐƯỢC. Không code 3 ngày rồi mới test. Cuối mỗi ngày, `jarvis ask "hello"` phải trả về câu trả lời (dù là fake).
