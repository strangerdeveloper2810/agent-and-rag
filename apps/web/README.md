# @app/web — React SPA

Single-page app cho JARVIS: chat (SSE streaming), quan ly tai lieu (RAG), va authentication (login/register/verify-email). Xay bang React 19 + Vite + Tailwind CSS, theme **shadcn (indigo/Inter)**.

Xem tong quan toan he thong o [README goc](../../README.md).

---

## Vai tro trong kien truc

```
Browser
   │
   ▼
React SPA (apps/web)  ← file nay
   │  fetch (cookie access/refresh token) + EventSource (SSE)
   ▼
Fastify Gateway (apps/api)  →  services/agent-go
```

Dev server chay tren `:3000`, Vite proxy `/api` sang `http://localhost:3001` (xem `vite.config.ts`). Bien `VITE_AGENT_URL` (trong `.env`) dung khi frontend can goi thang endpoint cua Go agent (vi du `/suggestions`) thay vi qua gateway.

---

## Cau truc thu muc

```
apps/web/src/
├── main.tsx                      # entrypoint (mount React root)
├── App.tsx                       # React Router routes (lazy-load + code-splitting)
├── index.css                     # Tailwind + bien CSS shadcn (theme indigo, light/dark)
├── modules/                      # feature module (theo domain)
│   ├── chat/                     # ChatPage, Composer, MessageBubble, Markdown, ModeSelector,
│   │                              # SlashCommandMenu, EmptyState + chat.api.ts
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
├── context/ConversationContext.tsx # state hoi thoai hien tai (React Context)
├── stores/auth.store.ts           # zustand — auth state (user, token)
├── hooks/useDocumentTitle.ts
├── lib/                           # http client wrapper, utils (cn), validation (zod)
└── types/                         # types cho component/function props (tach khoi @app/types dung chung)
```

## Routes (`App.tsx`)

| Path | Guard | Trang |
|------|-------|-------|
| `/login` | `GuestGuard` | Dang nhap |
| `/register` | `GuestGuard` | Dang ky |
| `/verify-email` | `GuestGuard` | Xac thuc OTP email |
| `/` | `AuthGuard` | ChatPage (hoi thoai moi) |
| `/messages/:id` | `AuthGuard` | ChatPage (hoi thoai co san) |
| `/documents` | `AuthGuard` | DocumentsView (quan ly tai lieu RAG) |

Moi route protected deu boc trong `AppLayout` (Header + Sidebar); cac page duoc `lazy()`-load de code-split theo route.

---

## Shared workspace packages

`apps/web` dung chung 4 package trong `packages/` (pnpm workspace):

| Package | Vai tro |
|---------|---------|
| `@app/types` | Type dung chung: `Conversation`, `Message`, `AttachmentPayload`, `ChatEvent`, `UsageData`, ... |
| `@app/http` | Singleton HTTP client: timeout, retry, request/response interceptor |
| `@app/api-client` | Client goi API co type, wrap `@app/http` + `@app/types` |
| `@app/ui` | Component design-system dung chung (atoms/molecules) — song song voi `src/design-system/` cuc bo cua web |

---

## Scripts

```bash
pnpm --filter @app/web dev         # vite dev server (:3000)
pnpm --filter @app/web build       # tsc (app + node config) → vite build
pnpm --filter @app/web preview     # preview bundle production
pnpm --filter @app/web typecheck   # tsc --noEmit (app + node config)
```

Hoac tu goc repo: `pnpm dev:web`, `pnpm build:web`.

> Chua co test tu dong (`*.test.*`) cho `apps/web` tinh den hien tai — kiem thu thu cong qua trinh duyet la cach xac minh chinh.

---

## Environment Variables

Copy `.env.example` → `.env`. Chi bien co tien to `VITE_` duoc expose vao bundle client (khong dat secret o day).

| Bien | Mac dinh | Mo ta |
|------|---------|-------|
| `VITE_AGENT_URL` | `http://localhost:3002` | Base URL goi truc tiep Go agent runtime (vd endpoint suggestions). Khi chay qua Docker/nginx proxy, de rong de dung same-origin |

---

## Tech Stack

- **React 19** + React Compiler (tu dong memo hoa, khong can `useMemo`/`useCallback` thu cong) qua `@rolldown/plugin-babel`
- **Vite 8** (Rolldown) — dev server + build, manual chunk splitting (tach `react-vendor` va `markdown` de cache tot hon)
- **React Router 7** — routing + lazy loading
- **Tailwind CSS 4** + shadcn UI (theme indigo, CSS variables cho light/dark) + `class-variance-authority` + `tailwind-merge`
- **Zustand** — state quan ly nhe (auth store)
- **react-hook-form** + **zod** (qua `@hookform/resolvers`) — form + validate
- **react-markdown** + **remark-gfm** — render markdown trong chat (tach chunk rieng vi nang)
- **Heroicons** / **lucide-react** — icon
