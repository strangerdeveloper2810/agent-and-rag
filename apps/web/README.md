# @app/web — React SPA

Single-page app cho JARVIS: chat (SSE streaming), quản lý tài liệu (RAG), và authentication (login/register/verify-email). Xây bằng React 19 + Vite + Tailwind CSS, theme **shadcn (indigo/Inter)**.

Xem tổng quan toàn hệ thống ở [README gốc](../../README.md).

---

## Vai trò trong kiến trúc

```
Browser
   │
   ▼
React SPA (apps/web)  ← file này
   │  fetch (cookie access/refresh token) + EventSource (SSE)
   ▼
Fastify Gateway (apps/api)  →  services/agent-go
```

Dev server chạy trên `:3000`, Vite proxy `/api` sang `http://localhost:3001` (xem `vite.config.ts`). Biến `VITE_AGENT_URL` (trong `.env`) dùng khi frontend cần gọi thẳng endpoint của Go agent (ví dụ `/suggestions`) thay vì qua gateway.

ChatPage đọc `contextTokens`/`contextBudget` từ event `done` (Go tính, forward qua BFF) — khi context vượt ~80% ngân sách, hiện banner gợi ý bắt đầu hội thoại mới (nút "Bắt đầu chat mới"); vẫn giữ được thông tin quan trọng đã học vì memory của agent theo tenant, không theo từng hội thoại.

---

## Cấu trúc thư mục

```
apps/web/src/
├── main.tsx                      # entrypoint (mount React root)
├── App.tsx                       # React Router routes (lazy-load + code-splitting)
├── index.css                     # Tailwind + biến CSS shadcn (theme indigo, light/dark)
├── modules/                      # feature module (theo domain)
│   ├── chat/                     # ChatPage (+ banner gợi ý chat mới), Composer, MessageBubble,
│   │                              # Markdown, ModeSelector, SlashCommandMenu, EmptyState + chat.api.ts
│   └── documents/                # DocumentsView + documents.api.ts
├── pages/auth/                   # LoginPage, RegisterPage, VerifyEmailPage
├── components/
│   ├── guards/                   # AuthGuard, GuestGuard, AdminGuard (route-level)
│   ├── ui/                       # avatar, badge, button, card, input, skeleton (shadcn primitives)
│   └── ErrorBoundary.tsx
├── design-system/                 # atomic design layer
│   ├── atoms/                    # Button, Badge, Card, Avatar, Kbd, ThemeToggle, AgentBadge
│   ├── molecules/                 # NavTab, SearchBar, SuggestionChip, Toast, ConfirmDialog,
│   │                              # ChatSkeleton, CitationList, ToolCallCard
│   ├── organisms/                 # Header, Sidebar
│   └── templates/                 # AppLayout (Outlet + Suspense cho lazy route)
├── context/ConversationContext.tsx # state hội thoại hiện tại (React Context)
├── stores/auth.store.ts           # zustand — auth state (user, token)
├── hooks/useDocumentTitle.ts
├── lib/                           # http client wrapper, utils (cn), validation (zod)
└── types/                         # types cho component/function props (tách khỏi @app/types dùng chung)
```

## Routes (`App.tsx`)

| Path | Guard | Trang |
|------|-------|-------|
| `/login` | `GuestGuard` | Đăng nhập |
| `/register` | `GuestGuard` | Đăng ký |
| `/verify-email` | `GuestGuard` | Xác thực OTP email |
| `/` | `AuthGuard` | ChatPage (hội thoại mới) |
| `/messages/:id` | `AuthGuard` | ChatPage (hội thoại có sẵn) |
| `/documents` | `AuthGuard` | DocumentsView (quản lý tài liệu RAG) |

Mọi route protected đều bọc trong `AppLayout` (Header + Sidebar); các page được `lazy()`-load để code-split theo route.

---

## Shared workspace packages

`apps/web` dùng chung 4 package trong `packages/` (pnpm workspace):

| Package | Vai trò |
|---------|---------|
| `@app/types` | Type dùng chung: `Conversation`, `Message`, `AttachmentPayload`, `ChatEvent` (bao gồm `contextTokens`/`contextBudget` trên event `done`), `UsageData`, ... |
| `@app/http` | Singleton HTTP client: timeout, retry, request/response interceptor |
| `@app/api-client` | Client gọi API có type, wrap `@app/http` + `@app/types` (bao gồm `streamChat` + `streamContinue`) |
| `@app/ui` | Component design-system dùng chung (atoms/molecules) — song song với `src/design-system/` cục bộ của web |

---

## Scripts

```bash
pnpm --filter @app/web dev         # vite dev server (:3000)
pnpm --filter @app/web build       # tsc (app + node config) → vite build
pnpm --filter @app/web preview     # preview bundle production
pnpm --filter @app/web typecheck   # tsc --noEmit (app + node config)
```

Hoặc từ gốc repo: `pnpm dev:web`, `pnpm build:web`.

> Chưa có test tự động (`*.test.*`) cho `apps/web` tính đến hiện tại — kiểm thử thủ công qua trình duyệt là cách xác minh chính.

---

## Environment Variables

Copy `.env.example` → `.env`. Chỉ biến có tiền tố `VITE_` được expose vào bundle client (không đặt secret ở đây).

| Biến | Mặc định | Mô tả |
|------|---------|-------|
| `VITE_AGENT_URL` | `http://localhost:3002` | Base URL gọi trực tiếp Go agent runtime (vd endpoint suggestions). Khi chạy qua Docker/nginx proxy, để rỗng để dùng same-origin |

---

## Tech Stack

- **React 19** + React Compiler (tự động memo hoá, không cần `useMemo`/`useCallback` thủ công) qua `@rolldown/plugin-babel`
- **Vite 8** (Rolldown) — dev server + build, manual chunk splitting (tách `react-vendor` và `markdown` để cache tốt hơn)
- **React Router 7** — routing + lazy loading
- **Tailwind CSS 4** + shadcn UI (theme indigo, CSS variables cho light/dark) + `class-variance-authority` + `tailwind-merge`
- **Zustand** — state quản lý nhẹ (auth store)
- **react-hook-form** + **zod** (qua `@hookform/resolvers`) — form + validate
- **react-markdown** + **remark-gfm** — render markdown trong chat (tách chunk riêng vì nặng)
- **Heroicons** / **lucide-react** — icon
