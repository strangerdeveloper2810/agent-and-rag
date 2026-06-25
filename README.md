# AI Agent Tut

Dự án học tập build AI Agent chatbot (RAG + task management) với Claude, LangGraph, MongoDB Atlas.

📐 **Thiết kế:** [`docs/plans/2026-06-25-ai-agent-chatbot-design.md`](docs/plans/2026-06-25-ai-agent-chatbot-design.md)

## Lộ trình (4 mốc)
| Mốc | Nội dung | Plan |
|-----|----------|------|
| 0 | Nền móng: monorepo, Fastify, Mongo, React | [milestone-0](docs/plans/2026-06-25-milestone-0-foundation.md) |
| 1 | Chatbot có memory + SSE streaming | [milestone-1](docs/plans/2026-06-25-milestone-1-chatbot-memory.md) |
| 2 | Agent + Tools: RAG + task management | [milestone-2](docs/plans/2026-06-25-milestone-2-agent-tools.md) |
| 3 | LangGraph multi-step agent | [milestone-3](docs/plans/2026-06-25-milestone-3-langgraph.md) |

## Techstack
pnpm + Turborepo · Fastify + TypeScript · Vite + React + Tailwind · MongoDB Atlas (Vector Search) · Anthropic Claude · Voyage AI (embedding) · LangChain + LangGraph · Zod · Vitest

## Chạy local
1. **Chuẩn bị** (xem chi tiết trong plan Mốc 0):
   - Tạo MongoDB Atlas cluster M0, lấy connection string
   - Lấy API key: Anthropic + Voyage AI
2. `pnpm install`
3. Tạo `apps/api/.env` từ `apps/api/.env.example` và điền key thật
4. `pnpm dev` → API tại `:3001`, Web tại `:5173`

## Cấu trúc
```
apps/
  api/   # Fastify backend (agent, RAG, tools)
  web/   # Vite React frontend (chat + documents)
docs/
  plans/ # design + implementation plans
```

> **Lưu ý:** Anthropic không cung cấp API embedding — dùng Voyage AI cho phần text → vector.
> Atlas Vector Search Index phải tạo thủ công (hướng dẫn trong plan Mốc 2).
