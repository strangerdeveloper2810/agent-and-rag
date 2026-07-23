# Go Agent (Polyglot) — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Chuyển agent runtime sang Go (`services/agent-go`), giữ Fastify/TS làm gateway; tự dựng agent engine (không LangGraph), pluggable Gemini+Claude, đủ tool/memory/context/guardrails.

**Architecture:** Xem [design doc](./2026-07-23-go-agent-polyglot-design.md). Polyglot: `apps/api` (TS gateway) ↔ `services/agent-go` (Go) qua HTTP+SSE. Go truy cập Mongo trực tiếp cho `documents`(read)/`tasks`/`memories`.

**Tech Stack:** Go 1.23+ · `go.mongodb.org/mongo-driver` · `google.golang.org/genai` (Gemini) · `github.com/anthropics/anthropic-sdk-go` (Claude) · `golang.org/x/sync/errgroup` · OpenTelemetry · stdlib `net/http` + `testing`.

---

## Quy ước chung (đọc trước)

- **Ngôn ngữ test:** Go stdlib `testing` + `httptest`. Không dùng framework assert (giữ ít dep); helper so sánh tự viết khi cần.
- **TDD nghiêm ngặt:** mỗi task = viết test đỏ → chạy đỏ → code tối thiểu → chạy xanh → commit. Dùng @superpowers:test-driven-development.
- **Chạy test:** `cd services/agent-go && go test ./...` (hoặc `go test ./internal/agent -run TestName -v`).
- **Commit nhỏ, thường xuyên.** Prefix: `feat(agent-go):`, `chore(agent-go):`, `test(agent-go):`.
- **Không gọi mạng thật trong test:** provider adapter test qua `http.Client` với `RoundTripper` giả; Mongo test qua interface + fake (hoặc `mongodb-memory`/testcontainers ở phase sau).
- **Không xoá agent LangGraph TS** cho tới P12 (giữ để so sánh/fallback).
- Verify cuối mỗi phase: `go vet ./...` + `go test ./...` + (khi có) `golangci-lint run` phải xanh.

---

# PHASE 0 — Scaffold agent-go + monorepo + CI

**Mục tiêu:** service Go tối thiểu chạy được, `/healthz` xanh, wire vào Turbo + CI path-filtered + Docker.

### Task 0.1: Khởi tạo Go module + healthz (TDD)

**Files:**
- Create: `services/agent-go/go.mod`
- Create: `services/agent-go/internal/transport/http/health.go`
- Create: `services/agent-go/internal/transport/http/health_test.go`
- Create: `services/agent-go/cmd/server/main.go`

**Step 1 — Tạo module**
```bash
mkdir -p services/agent-go/cmd/server services/agent-go/internal/transport/http
cd services/agent-go && go mod init github.com/ai-agent-tut/agent-go
```

**Step 2 — Viết test đỏ** `internal/transport/http/health_test.go`
```go
package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthz(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	Healthz(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	if got := rec.Body.String(); got != `{"status":"ok"}` {
		t.Fatalf("unexpected body: %s", got)
	}
}
```

**Step 3 — Chạy đỏ:** `go test ./internal/transport/http/` → FAIL (`Healthz` undefined).

**Step 4 — Code tối thiểu** `internal/transport/http/health.go`
```go
package http

import "net/http"

func Healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
```

**Step 5 — Chạy xanh:** `go test ./...` → PASS.

**Step 6 — main tối thiểu** `cmd/server/main.go`
```go
package main

import (
	"log/slog"
	"net/http"
	"os"

	agenthttp "github.com/ai-agent-tut/agent-go/internal/transport/http"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", agenthttp.Healthz)

	addr := ":" + envOr("PORT", "3002")
	slog.Info("agent-go listening", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
```

**Step 7 — Commit**
```bash
git add services/agent-go
git commit -m "feat(agent-go): scaffold module + healthz endpoint"
```

---

### Task 0.2: Turbo shim (package.json)

**Files:** Create `services/agent-go/package.json`

**Step 1** — nội dung:
```json
{
  "name": "@app/agent-go",
  "version": "0.1.0",
  "private": true,
  "scripts": {
    "dev": "go run ./cmd/server",
    "build": "go build -o bin/server ./cmd/server",
    "test": "go test ./...",
    "lint": "golangci-lint run || true",
    "typecheck": "go vet ./..."
  }
}
```
> `lint` để `|| true` tạm cho tới khi cài golangci-lint (Task 0.4 CI cài sẵn). `typecheck` = `go vet` để khớp task name của Turbo.

**Step 2 — Verify:** `pnpm install` rồi `pnpm --filter @app/agent-go test` → chạy `go test ./...` xanh.

**Step 3 — Commit:** `git add services/agent-go/package.json && git commit -m "chore(agent-go): turbo shim scripts"`

---

### Task 0.3: CI — thêm filter + job agent-go

**Files:** Modify `.github/workflows/ci.yml`

**Step 1** — thêm vào `detect.filters` (khối `filters: |`):
```yaml
            agent_go:
              - *shared
              - 'services/agent-go/**'
```
và thêm output: `agent_go: ${{ steps.filter.outputs.agent_go }}`.

**Step 2** — thêm job:
```yaml
  agent-go:
    needs: detect
    if: ${{ needs.detect.outputs.agent_go == 'true' }}
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: services/agent-go/go.mod
      - name: Vet + Test + Build
        working-directory: services/agent-go
        run: go vet ./... && go test ./... && go build ./...
      - name: Lint
        uses: golangci/golangci-lint-action@v6
        with:
          working-directory: services/agent-go
```

**Step 3** — thêm `agent-go` vào `ci-success.needs`: `needs: [detect, backend, frontend, agent-go]` và thêm điều kiện fail của nó vào script gate.

**Step 4 — Commit:** `git add .github/workflows/ci.yml && git commit -m "ci: add path-filtered agent-go job"`

---

### Task 0.4: Dockerfile multi-stage

**Files:** Create `services/agent-go/Dockerfile`, `services/agent-go/.dockerignore`
```dockerfile
FROM golang:1.23 AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/server

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/server /server
EXPOSE 3002
ENTRYPOINT ["/server"]
```
**Commit:** `chore(agent-go): multi-stage Dockerfile`

---

### Task 0.5: Config loader (env) — TDD

**Files:** Create `internal/config/config.go` + `internal/config/config_test.go`

**Step 1 — Test đỏ:** parse env → struct; thiếu `MONGODB_URI` → error; default `PORT=3002`, `LLM_PROVIDER=gemini`.
```go
func TestLoad_Defaults(t *testing.T) {
	t.Setenv("MONGODB_URI", "mongodb://x")
	t.Setenv("GEMINI_API_KEY", "k")
	c, err := Load()
	if err != nil { t.Fatal(err) }
	if c.Port != "3002" || c.Provider != "gemini" {
		t.Fatalf("bad defaults: %+v", c)
	}
}
func TestLoad_MissingMongo(t *testing.T) {
	t.Setenv("MONGODB_URI", "")
	if _, err := Load(); err == nil { t.Fatal("want error") }
}
```

**Step 2-5:** implement `Load() (Config, error)` đọc `os.Getenv`, validate, default; chạy xanh; commit `feat(agent-go): env config loader`.

**✅ Done P0:** `go test ./...` xanh; `pnpm --filter @app/agent-go build` ra binary; CI có job agent-go; `docker build` được.

---

# PHASE 1 — Provider layer (pluggable Gemini + Claude)

**Mục tiêu:** `Provider` interface + normalized types + adapter Gemini & Anthropic (streaming + tool-calling), test không gọi mạng.

### Task 1.1: Normalized types — TDD

**Files:** Create `internal/provider/types.go` + `types_test.go`

Types: `Role`, `Message{Role, Content, ToolCalls, ToolResults}`, `ToolDef{Name, Description, Schema json.RawMessage}`, `ToolCall{ID, Name, Args json.RawMessage}`, `ChunkKind` (enum), `StreamChunk`, `Usage`, `GenerateRequest`, `ProviderOptions{MaxTokens, ThinkingLevel, Cache bool}`.

**Test:** một hàm thuần cần test ngay — `BuildToolResultMessage(callID, result string) Message` (map tool result → message). Viết test đỏ → implement → xanh → commit `feat(agent-go): provider normalized types`.

### Task 1.2: Provider interface + FakeProvider (cho test engine sau)

**Files:** `internal/provider/provider.go`, `internal/provider/fake.go` + `fake_test.go`
```go
type Provider interface {
	Generate(ctx context.Context, req GenerateRequest) (<-chan StreamChunk, error)
	Name() string
}
```
`FakeProvider` trả sẵn 1 kịch bản chunk (text/tool_call/done) do test cấu hình. **Test:** drain channel → đúng thứ tự chunk; tôn trọng `ctx.Done()`. Commit `feat(agent-go): provider interface + fake`.

### Task 1.3: Gemini adapter — TDD (mock transport)

**Files:** `internal/provider/gemini/gemini.go` + `gemini_test.go`
- Dịch `[]Message`/`[]ToolDef` → request Gemini; gom stream Gemini → `StreamChunk`.
- **Test:** dùng `option.WithHTTPClient(&http.Client{Transport: fakeRT})` (hoặc tách hàm dịch thuần `toGeminiContents`/`fromGeminiChunk` để test **không cần SDK network**). Ưu tiên test **hàm dịch thuần** (translate request + parse 1 SSE chunk) → đỏ → implement → xanh.
- Commit `feat(agent-go): gemini provider adapter`.

### Task 1.4: Anthropic adapter — TDD

**Files:** `internal/provider/anthropic/anthropic.go` + test. Tương tự 1.3: test hàm dịch thuần `toAnthropicMessages` / `parseToolUse` / stream→chunk. Commit `feat(agent-go): anthropic provider adapter`.

### Task 1.5: Provider factory

**Files:** `internal/provider/factory.go` + test: `New(cfg) (Provider, error)` chọn theo `cfg.Provider` (`gemini|anthropic`), lỗi nếu thiếu key tương ứng. Commit `feat(agent-go): provider factory`.

**✅ Done P1:** factory tạo được cả 2 provider; adapter dịch request + parse stream đúng (test thuần); `FakeProvider` sẵn cho P2.

---

# PHASE 2 — Agent engine lõi (loop engineering)

**Mục tiêu:** state machine chạy vòng model→route→tools→model với loop control + streaming, verify bằng `FakeProvider` + 1 tool giả. **Đây là trái tim — làm kỹ, test từng node + router.**

### Task 2.1: State + Event + EmitFunc

**Files:** `internal/agent/state.go`, `internal/agent/event.go` (+ test cho helper thuần nếu có). Types theo design mục 7. Commit `feat(agent-go): agent state + event types`.

### Task 2.2: Router (hàm thuần) — TDD ⭐

**Files:** `internal/agent/router.go` + `router_test.go`
```go
// route quyết định node kế tiếp từ state (thuần, dễ test).
func route(s *State) NodeID {
	last := s.lastAssistant()
	switch {
	case s.Interrupt != nil:      return NodeInterrupt
	case len(last.ToolCalls) > 0: return NodeTools
	case s.Step >= s.MaxSteps:    return NodeEnd
	default:                      return NodeEnd // final answer
	}
}
```
**Test:** bảng case (có tool_calls → Tools; vượt maxSteps → End; final → End; interrupt → Interrupt). Đỏ → implement → xanh → commit.

### Task 2.3: Node `model` — TDD (dùng FakeProvider)

**Files:** `internal/agent/node_model.go` + test. Gọi `provider.Generate`, stream `TextDelta` → `emit(text)`, gom `ToolCall` vào assistant message, cộng `Usage`. **Test:** FakeProvider trả "text + 1 tool_call" → assert emit đúng + state.Messages có assistant với tool_calls. Commit.

### Task 2.4: Node `tools` + tool registry giả — TDD

**Files:** `internal/agent/node_tools.go`, `internal/tools/registry.go`, `internal/tools/tool.go` + test. Node đọc tool_calls của assistant cuối → chạy qua registry (**fan-out `errgroup`**) → append `Observation`/tool-result message. Tool giả `echo` trả lại args. **Test:** 2 tool_call song song → 2 kết quả đúng thứ tự; tool lỗi → observation error (không crash). Commit.

### Task 2.5: Vòng `Run` (loop control) — TDD ⭐

**Files:** `internal/agent/engine.go` + `engine_test.go`
```go
func (e *Engine) Run(ctx context.Context, in RunInput, emit EmitFunc) (Usage, error) {
	s := newState(in)
	node := NodeModel
	for {
		if ctx.Err() != nil { return s.Usage, ctx.Err() }
		var err error
		node, err = e.dispatch(ctx, node, s, emit)
		if err != nil { emit(errEvent(err)); return s.Usage, err }
		if node == NodeEnd { break }
	}
	emit(doneEvent(s.Usage))
	return s.Usage, nil
}
```
**Test kịch bản đầy đủ** với FakeProvider: lượt 1 trả tool_call `echo` → node tools chạy → lượt 2 (FakeProvider script thứ 2) trả final text → END. Assert: chuỗi event `text? → tool_start → tool_end → text → done`; dừng đúng ở `maxSteps`; `ctx` cancel giữa chừng → trả sớm. Commit `feat(agent-go): agent engine run loop`.

### Task 2.6: Transport `/chat` (SSE) — TDD

**Files:** `internal/transport/http/chat.go` + test (`httptest` + FakeProvider engine). Parse body → `Engine.Run` với `emit` ghi `data: <json>\n\n` vào `http.ResponseWriter` (flush mỗi event); `r.Context()` = cancel. **Test:** POST → đọc stream → thấy events kết thúc bằng `done`. Wire route vào `main.go`. Commit `feat(agent-go): SSE /chat endpoint`.

**✅ Done P2:** POST `/chat` chạy engine với FakeProvider + tool giả, stream SSE đúng, loop control + cancel hoạt động, mỗi node + router có test.

---

# PHASE 3–14 (spec — chi tiết hoá thành plan riêng khi tới)

> Mỗi phase sẽ được viết thành 1 implementation plan bite-sized riêng (như P0–P2) ngay trước khi thực thi. Dưới đây là **spec**: goal · key files · key tasks · Done.

### P3 — Tool system hoàn chỉnh
**Goal:** interface/registry chuẩn hoá, phân loại Read/Write/Destructive, guardrail mỗi tool. **Files:** `internal/tools/*`. **Tasks:** `Kind()`; validate args theo schema; timeout `context.WithTimeout`; cắt result lớn; đăng ký tool thật (stub trả cứng để test loop). **Done:** registry sinh `[]ToolDef` cho provider; fan-out + guardrail có test.

### P4 — Mongo (Go) + task tools
**Goal:** Go Mongo driver + CRUD `tasks`. **Files:** `internal/mongo/*`, `internal/tools/tasks_*.go`. **Tasks:** client + `toObjectId` (400-safe); struct `Task`; tools createTask/listTasks/updateTask/deleteTask (test qua interface + fake store; integration test optional testcontainers). **Done:** 4 task tool chạy thật trên Mongo; index qua api `ensureIndexes`.

### P5 — RAG retrieval
**Goal:** `ragSearch`/`listDocuments`/`readDocument` trong Go. **Files:** `internal/rag/*`, `internal/tools/rag_*.go`. **Tasks:** Voyage client (REST, batch, retry 429) — test hàm dựng request thuần; Atlas `$vectorSearch` (đọc `documents`); `getDocumentContent` theo `documentId` (cắt 24k); relevance gate `minScore`; trả citation. **Done:** 3 RAG tool chạy; ragSearch trả kèm `{documentId,source,score}`.

### P6 — Context engineering + prompt caching
**Goal:** assembler ráp system+tools+skills+memory+RAG+history theo thứ tự cố định, tách DỮ LIỆU vs CHỈ THỊ. **Files:** `internal/agent/context.go`. **Tasks:** prompt assembler (thuần, test); token budget + trim; đánh dấu `Cache` cho phần ổn định; wire vào node model. **Done:** context lắp đúng khối; caching giảm input token (đo bằng usage).

### P7 — Memory 3 tầng
**Goal:** working + summary + long-term semantic hybrid. **Files:** `internal/memory/*`, node `recall`/`summarize`/`extract`, tools `saveMemory`/`recallMemory`. **Tasks:** `memories` schema + repo (structured + vector); node recall (merge structured+vector, thuần `mergeMemories` test); node summarize (`selectMessagesToSummarize` test); node extract (parseExtracted test); Atlas `memory_index`. **Done:** preference "dạy" ở hội thoại A áp dụng ở B; engine có recall/summarize/extract.

### P8 — Planner + reflection
**Goal:** node `plan` (phân rã trước act) + node `reflect` (self-critique trước final). **Files:** `internal/agent/node_plan.go`, `node_reflect.go`, router cập nhật. **Tasks:** plan node (LLM rẻ → các bước → vào context); reflect node (kiểm câu trả lời, route lại model nếu cần) với giới hạn số vòng reflect. **Done:** router hỗ trợ plan/reflect có test; loop vẫn có chốt dừng.

### P9 — Skills (progressive disclosure)
**Goal:** SKILL.md discovery + nạp theo ngữ cảnh. **Files:** `internal/skills/*`, `services/agent-go/skills/*/SKILL.md`. **Tasks:** loader đọc frontmatter (name/description/when_to_use/tools); đưa danh sách vào system prompt; router/plan nạp thân skill khi khớp. **Done:** thêm 1 skill mẫu; agent nạp đúng khi task khớp (test loader thuần).

### P10 — Guardrails + HITL
**Goal:** input/output guardrail + interrupt tool phá hủy (resumable). **Files:** `internal/guardrails/*`, engine interrupt, `/chat/resume`. **Tasks:** output guardrail bắt buộc citation khi dùng ragSearch; input guardrail đánh dấu data; tool Destructive → emit `interrupt` + persist State (Mongo) → `POST /chat/resume {runId, decision}` tiếp tục/hủy. **Done:** deleteTask cần xác nhận; resume chạy tiếp đúng.

### P11 — Observability
**Goal:** OTel span + token/cost/latency. **Files:** `internal/observability/*`, instrument engine/provider/tools. **Tasks:** tracer setup (OTLP optional); span mỗi run/node/LLM/tool; đo usage + latency; slog structured. **Done:** thấy trace 1 lượt (node/tool/LLM) + số token/cost.

### P12 — Gateway integration (TS ↔ Go)
**Goal:** `apps/api` proxy `/chat` sang agent-go, gỡ agent LangGraph khỏi đường chạy. **Files:** `apps/api/src/modules/chat/*`. **Tasks:** service gọi `agent-go /chat` (fetch SSE) → proxy ra browser; truyền history + provider; lưu assistant khi `done`; env `AGENT_GO_URL`; **giữ** LangGraph agent sau cờ để so sánh. **Done:** chat E2E qua Go; frontend không đổi (format AgentEvent giữ tương thích).

### P13 — Eval harness
**Goal:** đo RAG + agent. **Files:** `services/agent-go/eval/*` hoặc script. **Tasks:** bộ câu hỏi vàng → recall@k/faithfulness (LLM-judge); agent-eval (dùng đúng tool?). **Done:** chạy `go test -tags eval` ra số liệu.

### P14 — Dọn dẹp + docs + e2e
**Goal:** hoàn thiện. **Tasks:** cập nhật `docs/architecture-*`, README (thêm agent-go, sơ đồ), docker-compose (api+web+agent-go), e2e happy path; cân nhắc gỡ hẳn LangGraph nếu Go đã tương đương. **Done:** README + kiến trúc phản ánh polyglot; compose chạy cả stack.

---

## Định nghĩa "Done" toàn dự án
- Chat chạy qua **agent-go** với engine tự dựng (không LangGraph trên đường chạy chính).
- Đủ **9 tool**; **memory 3 tầng** xuyên hội thoại; **skills** nạp theo ngữ cảnh.
- **Guardrails + HITL** cho tool phá hủy; **observability** token/cost/latency.
- **Pluggable** Gemini+Claude qua Provider interface.
- **CI xanh cả 3 phía** (api/web/agent-go), path-filtered.
- Hiểu được: agent loop, tool-calling, context engineering, memory 3 tầng, guardrails/HITL — **từ gốc**.
