# AI Agent Tut

Dự án **học tập** build AI Agent chatbot (RAG + task management) với Claude, LangGraph và MongoDB Atlas. Đi từ chatbot có memory → agent gọi tool → orchestration bằng state machine.

- 📐 **Thiết kế tổng:** [`docs/plans/2026-06-25-ai-agent-chatbot-design.md`](docs/plans/2026-06-25-ai-agent-chatbot-design.md)
- 🏗️ **Kiến trúc Backend & Agent (chi tiết + sơ đồ flow):** [`docs/architecture-backend-agent.md`](docs/architecture-backend-agent.md)

## Lộ trình
| Mốc | Nội dung | Trạng thái | Plan |
|-----|----------|:---------:|------|
| 0 | Nền móng: monorepo, Fastify, Mongo, React | ✅ | [milestone-0](docs/plans/2026-06-25-milestone-0-foundation.md) |
| 1 | Chatbot có memory + SSE streaming | ✅ | [milestone-1](docs/plans/2026-06-25-milestone-1-chatbot-memory.md) |
| 2 | Agent + Tools: RAG + task management | ✅ | [milestone-2](docs/plans/2026-06-25-milestone-2-agent-tools.md) |
| 3 | LangGraph multi-step agent (StateGraph) | ✅ | [milestone-3](docs/plans/2026-06-25-milestone-3-langgraph.md) |
| 4 | Structured memory (hybrid) | 📝 có plan | [milestone-4](docs/plans/2026-07-23-milestone-4-structured-memory.md) |

## Tính năng hiện có
- **Chat streaming** qua SSE — token chảy real-time, hiển thị chip khi agent đang gọi tool.
- **Agent LangGraph** — vòng `agent ↔ tools` tự lặp; 7 tool: `ragSearch`, `listDocuments`, `readDocument`, `createTask`, `listTasks`, `updateTask`, `deleteTask`.
- **RAG** — upload `.txt` / `.md` / `.pdf` → trích text → chunk → embed (Voyage) → Atlas Vector Search.
- **Versioning tài liệu** — cập nhật tạo version mới, xem lại nội dung bản cũ; search luôn dùng bản mới nhất.
- **Task management** — agent tự tạo/sửa/xem/xoá task qua tool.
- **Quan sát** — `pnpm graph:print` (sơ đồ Mermaid) + LangSmith tracing (token/tool/latency mỗi lượt).

## Techstack
pnpm + Turborepo · Fastify + TypeScript · Vite + React + Tailwind · MongoDB Atlas (Vector Search) · Anthropic Claude **hoặc** Google Gemini (chọn qua `LLM_PROVIDER`) · Voyage AI (embedding) · LangChain + LangGraph · Zod · Vitest

## Chạy local
1. **Chuẩn bị** (chi tiết trong plan Mốc 0):
   - Tạo MongoDB Atlas cluster, lấy connection string
   - Lấy API key: Anthropic + Voyage AI (tuỳ chọn: LangSmith để tracing)
2. `pnpm install`
3. Tạo `apps/api/.env` từ `apps/api/.env.example`, điền key thật
4. **Tạo Atlas Vector Search Index** `vector_index` (hướng dẫn trong plan Mốc 2) — bắt buộc cho RAG
5. `pnpm dev` → API tại `:3001`, Web tại `:3000`

## Scripts hữu ích
| Lệnh | Việc |
|------|------|
| `pnpm dev` | chạy cả API + Web (Turborepo) |
| `pnpm dev:api` / `pnpm dev:web` | chạy riêng từng app |
| `pnpm test` | chạy test (Vitest) |
| `pnpm typecheck` | kiểm tra type toàn repo |
| `pnpm --filter @app/api graph:print` | in sơ đồ Mermaid của agent graph |
| `pnpm --filter @app/api migrate:docs` | backfill documentId/version cho tài liệu cũ |

## Cấu trúc
```
apps/
  api/   # Fastify backend
    src/
      agent/        # LangGraph: graph.ts, lc-tools.ts, graph-runner.ts
      middleware/   # error handler tập trung
      modules/      # mỗi tính năng: controllers/ services/ repositories/ + routes
      lib/          # mongo, claude, voyage, errors
  web/   # Vite + React (chat + documents)
docs/
  architecture-backend-agent.md   # kiến trúc chi tiết
  plans/                          # design + implementation plans
```

> **Lưu ý:** Anthropic không cung cấp API embedding — dùng Voyage AI cho text → vector.
> Atlas Vector Search Index phải tạo thủ công (hướng dẫn trong plan Mốc 2).
