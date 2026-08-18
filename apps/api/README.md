# @app/api — Fastify Gateway (BFF)

Fastify 5 + TypeScript gateway dung lam **Backend-For-Frontend** cho JARVIS: xac thuc nguoi dung, upload/trich xuat file, CRUD hoi thoai/tai lieu, va **proxy SSE** sang [Go agent runtime](../../services/agent-go) de xu ly hoi thoai AI.

Xem tong quan toan he thong o [README goc](../../README.md).

---

## Vai tro trong kien truc

```
React SPA (apps/web)
        │  HTTP + cookie (access/refresh token)
        ▼
Fastify Gateway (apps/api)  ← file nay
  ├─ Auth: JWT + Google OAuth + OTP email (Resend), multi-tenant
  ├─ Chat: proxy POST /conversations/:id/chat → SSE → Go agent
  ├─ Documents: upload, PDF extract, versioning
  ├─ Tasks: GET /tasks (chi doc, Mongo)
  ├─ Upload: presigned URL + object storage (S3/MinIO)
  └─ Cache: chat/embedding/tool cache qua Redis
        │
        ├──► MongoDB Atlas   (conversations, messages, documents + vector search, tasks, memories)
        ├──► PostgreSQL      (users, credentials, refresh tokens — auth DB)
        ├──► Redis           (rate-limit, cache)
        ├──► MinIO / S3      (file upload)
        └──► services/agent-go (HTTP + SSE, khi AGENT_BACKEND=go)
```

`AGENT_BACKEND` co 2 gia tri:
- `go` (khuyen nghi) — proxy request sang Go agent runtime qua HTTP/SSE.
- `langgraph` (legacy) — chay agent LangGraph in-process trong chinh service nay (xem `src/agent/deprecated/`, dang duoc thay the dan bang Go).

---

## Cau truc thu muc

```
apps/api/src/
├── server.ts                   # entrypoint (init Postgres → buildApp → listen)
├── app.ts                      # compose Fastify plugin + route + health endpoints
├── config.ts                   # zod env schema (fail-fast khi thieu bien bat buoc)
├── agent/
│   ├── client/                 # AgentClient interface + Go agent client (SSE) + LangGraph client
│   └── deprecated/              # agent loop LangGraph cu (graph-runner, tools) — legacy
├── modules/
│   ├── auth/                   # register, verify-email (OTP), login, Google OAuth, refresh, logout, me
│   ├── users/                  # admin: list/search/get/patch user (role, tenant)
│   ├── chat/                   # conversations CRUD + POST .../chat (SSE proxy sang Go agent)
│   ├── documents/               # upload, versioning, extract, delete
│   ├── tasks/                   # GET /tasks (read-only)
│   └── upload/                  # presigned URL, direct upload, list, delete (S3/MinIO)
├── database/
│   ├── mongo/                   # collections, tenant filter
│   ├── postgres/                # pool cho auth DB
│   └── redis/                   # cache/rate-limit client
├── common/
│   ├── cache/                   # chat-cache, embedding-cache, tool-cache (Redis)
│   ├── storage/                 # s3 client + storage service
│   ├── email/                   # Resend client (OTP email)
│   ├── guards/                  # authGuard, adminGuard
│   └── filters/                 # error filter tap trung
└── schemas/                     # zod schema dung chung (chat-request, message, task)
```

---

## API Endpoints

Moi route (tru `/api/health*`) deu prefix `/api`.

### Health
| Method | Path | Mo ta |
|--------|------|-------|
| `GET` | `/api/health` | Liveness — process con song |
| `GET` | `/api/healthz` | Deep health — ping Mongo + Go agent (neu `AGENT_BACKEND=go`) |
| `GET` | `/api/ready` | Readiness — Mongo ping duoc chua |

### Auth (`modules/auth`)
| Method | Path | Auth | Mo ta |
|--------|------|:---:|-------|
| `POST` | `/api/auth/register` | - | Dang ky + gui OTP xac thuc email |
| `POST` | `/api/auth/verify-email` | - | Xac nhan OTP |
| `POST` | `/api/auth/resend-otp` | - | Gui lai OTP |
| `POST` | `/api/auth/login` | - | Dang nhap, tra access/refresh token |
| `GET` | `/api/auth/google` | - | Redirect Google OAuth |
| `GET` | `/api/auth/google/callback` | - | Google OAuth callback |
| `POST` | `/api/auth/refresh` | - | Lam moi access token |
| `POST` | `/api/auth/logout` | - | Thu hoi refresh token |
| `GET` | `/api/auth/me` | JWT | Thong tin user hien tai |

### Admin (`modules/users`)
| Method | Path | Auth |
|--------|------|:---:|
| `GET` | `/api/admin/users` | admin |
| `GET` | `/api/admin/users/search` | admin |
| `GET` | `/api/admin/users/:id` | admin |
| `PATCH` | `/api/admin/users/:id` | admin |

### Chat (`modules/chat`)
| Method | Path | Auth | Mo ta |
|--------|------|:---:|-------|
| `POST` | `/api/conversations` | JWT | Tao hoi thoai moi |
| `GET` | `/api/conversations` | JWT | Liet ke hoi thoai |
| `GET` | `/api/conversations/:id/messages` | JWT | Lay lich su tin nhan |
| `DELETE` | `/api/conversations/:id` | JWT | Xoa hoi thoai |
| `POST` | `/api/conversations/:id/chat` | JWT | Gui tin nhan → SSE stream tu Go agent (rate-limit 20/phut) |

### Documents (`modules/documents`)
| Method | Path | Auth | Mo ta |
|--------|------|:---:|-------|
| `POST` | `/api/documents/upload` | JWT | Upload + trich xuat + embed (Voyage), rate-limit 20/phut |
| `PUT` | `/api/documents/:documentId` | JWT | Cap nhat noi dung (tao version moi) |
| `GET` | `/api/documents` | JWT | Liet ke tai lieu |
| `GET` | `/api/documents/:documentId/versions` | JWT | Lich su version |
| `GET` | `/api/documents/:documentId/versions/:version` | JWT | Noi dung 1 version |
| `DELETE` | `/api/documents/:documentId` | JWT | Xoa tai lieu |

### Tasks (`modules/tasks`)
| Method | Path | Auth | Mo ta |
|--------|------|:---:|-------|
| `GET` | `/api/tasks` | JWT | Liet ke task (**chi doc** — chua co route/tool tao-sua-xoa) |

### Upload (`modules/upload`)
| Method | Path | Mo ta |
|--------|------|-------|
| `GET` | `/api/upload/presigned` | Lay presigned URL de upload thang len S3/MinIO |
| `POST` | `/api/upload` | Upload truc tiep qua server |
| `GET` | `/api/upload` | Liet ke file |
| `DELETE` | `/api/upload/:key` | Xoa file |

---

## Environment Variables

Xac thuc qua `zod` trong `src/config.ts` — server **fail-fast** (throw ngay khi start) neu thieu bien bat buoc. Chua co `apps/api/.env.example` rieng; tham khao [`env/.env.example`](../../env/.env.example) o repo root de biet gia tri mau, roi tao `apps/api/.env` (doc boi `tsx --env-file=.env`).

| Bien | Bat buoc | Mo ta |
|------|:---:|-------|
| `PORT` | - | Mac dinh `3001` |
| `MONGODB_URI` | ✓ | Ket noi MongoDB Atlas |
| `PG_CONNECTION_STRING` | ✓ | Postgres — auth DB |
| `REDIS_URL` | ✓ | Redis — cache + rate-limit |
| `S3_ENDPOINT` / `S3_ACCESS_KEY` / `S3_SECRET_KEY` / `S3_BUCKET` | - | MinIO/S3, co default cho dev local |
| `ANTHROPIC_API_KEY`, `CLAUDE_MODEL` | ✓ (anthropic) | LLM cho legacy LangGraph agent |
| `LLM_PROVIDER`, `GOOGLE_API_KEY`, `GOOGLE_MODEL`, `GOOGLE_THINKING_LEVEL` | tuy | Provider Gemini cho legacy agent |
| `VOYAGE_API_KEY` | ✓ | Embedding cho RAG |
| `JWT_SECRET`, `JWT_REFRESH_SECRET` | ✓ | Toi thieu 32 ky tu |
| `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_REDIRECT_URI` | ✓ | Google OAuth |
| `RESEND_API_KEY`, `EMAIL_FROM` | tuy chon | Gui email OTP (khong co thi log canh bao, khong chan startup) |
| `AGENT_BACKEND` | - | `langgraph` (mac dinh) hoac `go` |
| `AGENT_GO_URL`, `AGENT_GO_TIMEOUT` | - | URL + timeout goi services/agent-go |
| `CHAT_CACHE_VERSION` | - | Bump khi doi model/prompt ben Go agent de invalidate cache |
| `CORS_ORIGIN` | - | Danh sach origin, phan tach dau phay (rong = cho moi origin) |
| `FRONTEND_URL` | - | Dung cho redirect Google OAuth |
| `LANGSMITH_*` | tuy chon | Tracing cho LangGraph legacy |

---

## Scripts

```bash
pnpm --filter @app/api dev              # tsx watch --env-file=.env src/server.ts
pnpm --filter @app/api build             # tsc → dist/
pnpm --filter @app/api start             # node dist/server.js
pnpm --filter @app/api test              # vitest run
pnpm --filter @app/api test:coverage     # vitest run --coverage
pnpm --filter @app/api typecheck         # tsc --noEmit
pnpm --filter @app/api migrate:docs      # backfill document versions (script rieng)
```

Hoac tu goc repo: `pnpm dev:api`, `pnpm build:api`, `pnpm test:api`.

## Testing

17 file `*.test.ts` (Vitest) bao phu: auth (OTP), chat routes/service, documents (chunk/extract/service), tasks repository, voyage embedding client, app-level smoke test. Chay `pnpm --filter @app/api test`.

## Tech Stack

Fastify 5, Zod (validate env + DTO), MongoDB driver, `pg` (Postgres), `ioredis`, AWS SDK v3 (S3), `@fastify/multipart` (upload), `@fastify/rate-limit`, `jsonwebtoken`, `bcrypt`, Resend (email), `unpdf` (PDF extraction). Phan legacy con giu `@langchain/*` cho agent LangGraph (`AGENT_BACKEND=langgraph`), dang duoc thay dan bang proxy sang Go agent.
