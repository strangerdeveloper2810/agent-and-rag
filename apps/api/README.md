# @app/api — Fastify Gateway (BFF)

Fastify 5 + TypeScript gateway dùng làm **Backend-For-Frontend** cho JARVIS: xác thực người dùng, upload/trích xuất file, CRUD hội thoại/tài liệu, và **proxy SSE** sang [Go agent runtime](../../services/agent-go) để xử lý hội thoại AI.

Xem tổng quan toàn hệ thống ở [README gốc](../../README.md) — bao gồm lý do dự án dùng Go agent tự xây thay vì LangChain/LangGraph ([Why Not LangChain?](../../README.md#why-not-langchain--langgraph)).

---

## Vai trò trong kiến trúc

```
React SPA (apps/web)
        │  HTTP + cookie (access/refresh token)
        ▼
Fastify Gateway (apps/api)  ← file này
  ├─ Auth: JWT + Google OAuth + OTP email (Resend), multi-tenant
  ├─ Chat: proxy POST /conversations/:id/chat → SSE → Go agent
  ├─ Documents: upload, PDF extract, versioning
  ├─ Tasks: GET /tasks (chỉ đọc, Mongo)
  ├─ Upload: presigned URL + object storage (S3/MinIO)
  └─ Cache: chat/embedding/tool cache qua Redis
        │
        ├──► MongoDB Atlas   (conversations, messages, documents + vector search, tasks, memories)
        ├──► PostgreSQL      (users, credentials, refresh tokens — auth DB)
        ├──► Redis           (rate-limit, cache)
        ├──► MinIO / S3      (file upload)
        └──► services/agent-go (HTTP + SSE, khi AGENT_BACKEND=go)
```

`AGENT_BACKEND` có 2 giá trị:
- `go` (khuyến nghị) — proxy request sang Go agent runtime qua HTTP/SSE.
- `langgraph` (legacy) — chạy agent LangGraph in-process trong chính service này (xem `src/agent/deprecated/`), **vẫn giữ lại chủ ý** để so sánh/fallback, không phải dead code chờ xoá. Xem [Why Not LangChain?](../../README.md#why-not-langchain--langgraph) ở README gốc để biết lý do agent chính chuyển sang Go.

---

## Cấu trúc thư mục

```
apps/api/src/
├── server.ts                   # entrypoint (init Postgres → buildApp → listen)
├── app.ts                      # compose Fastify plugin + route + health endpoints
├── config.ts                   # zod env schema (fail-fast khi thiếu biến bắt buộc)
├── agent/
│   ├── client/                 # AgentClient interface + Go agent client (SSE) + LangGraph client
│   └── deprecated/              # agent loop LangGraph cũ (graph-runner, tools) — legacy, giữ để so sánh/fallback
├── modules/
│   ├── auth/                   # register, verify-email (OTP), login, Google OAuth, refresh, logout, me
│   ├── users/                  # admin: list/search/get/patch user (role, tenant)
│   ├── chat/                   # conversations CRUD + POST .../chat (SSE proxy sang Go agent) + POST .../continue
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
│   └── filters/                 # error filter tập trung
└── schemas/                     # zod schema dùng chung (chat-request, message, task)
```

---

## API Endpoints

Mọi route (trừ `/api/health*`) đều prefix `/api`.

### Health
| Method | Path | Mô tả |
|--------|------|-------|
| `GET` | `/api/health` | Liveness — process còn sống |
| `GET` | `/api/healthz` | Deep health — ping Mongo + Go agent (nếu `AGENT_BACKEND=go`) |
| `GET` | `/api/ready` | Readiness — Mongo ping được chưa |

### Auth (`modules/auth`)
| Method | Path | Auth | Mô tả |
|--------|------|:---:|-------|
| `POST` | `/api/auth/register` | - | Đăng ký + gửi OTP xác thực email |
| `POST` | `/api/auth/verify-email` | - | Xác nhận OTP |
| `POST` | `/api/auth/resend-otp` | - | Gửi lại OTP |
| `POST` | `/api/auth/login` | - | Đăng nhập, trả access/refresh token |
| `GET` | `/api/auth/google` | - | Redirect Google OAuth |
| `GET` | `/api/auth/google/callback` | - | Google OAuth callback |
| `POST` | `/api/auth/refresh` | - | Làm mới access token |
| `POST` | `/api/auth/logout` | - | Thu hồi refresh token |
| `GET` | `/api/auth/me` | JWT | Thông tin user hiện tại |

### Admin (`modules/users`)
| Method | Path | Auth |
|--------|------|:---:|
| `GET` | `/api/admin/users` | admin |
| `GET` | `/api/admin/users/search` | admin |
| `GET` | `/api/admin/users/:id` | admin |
| `PATCH` | `/api/admin/users/:id` | admin |

### Chat (`modules/chat`)
| Method | Path | Auth | Mô tả |
|--------|------|:---:|-------|
| `POST` | `/api/conversations` | JWT | Tạo hội thoại mới |
| `GET` | `/api/conversations` | JWT | Liệt kê hội thoại |
| `GET` | `/api/conversations/:id/messages` | JWT | Lấy lịch sử tin nhắn |
| `DELETE` | `/api/conversations/:id` | JWT | Xoá hội thoại |
| `POST` | `/api/conversations/:id/chat` | JWT | Gửi tin nhắn → SSE stream từ Go agent (rate-limit 20/phút) |
| `POST` | `/api/conversations/:id/continue` | JWT | Tiếp tục câu trả lời bị cắt vì chạm giới hạn token — nối trực tiếp vào message assistant cũ (không tạo user turn mới), rate-limit 20/phút |

### Documents (`modules/documents`)
| Method | Path | Auth | Mô tả |
|--------|------|:---:|-------|
| `POST` | `/api/documents/upload` | JWT | Upload + trích xuất + embed (Voyage), rate-limit 20/phút |
| `PUT` | `/api/documents/:documentId` | JWT | Cập nhật nội dung (tạo version mới) |
| `GET` | `/api/documents` | JWT | Liệt kê tài liệu |
| `GET` | `/api/documents/:documentId/versions` | JWT | Lịch sử version |
| `GET` | `/api/documents/:documentId/versions/:version` | JWT | Nội dung 1 version |
| `DELETE` | `/api/documents/:documentId` | JWT | Xoá tài liệu |

### Tasks (`modules/tasks`)
| Method | Path | Auth | Mô tả |
|--------|------|:---:|-------|
| `GET` | `/api/tasks` | JWT | Liệt kê task (**chỉ đọc** — chưa có route/tool tạo-sửa-xoá) |

### Upload (`modules/upload`)
| Method | Path | Mô tả |
|--------|------|-------|
| `GET` | `/api/upload/presigned` | Lấy presigned URL để upload thẳng lên S3/MinIO |
| `POST` | `/api/upload` | Upload trực tiếp qua server |
| `GET` | `/api/upload` | Liệt kê file |
| `DELETE` | `/api/upload/:key` | Xoá file |

---

## Environment Variables

Xác thực qua `zod` trong `src/config.ts` — server **fail-fast** (throw ngay khi start) nếu thiếu biến bắt buộc. Chưa có `apps/api/.env.example` riêng; tham khảo [`env/.env.example`](../../env/.env.example) ở repo root để biết giá trị mẫu, rồi tạo `apps/api/.env` (đọc bởi `tsx --env-file=.env`).

| Biến | Bắt buộc | Mô tả |
|------|:---:|-------|
| `PORT` | - | Mặc định `3001` |
| `MONGODB_URI` | ✓ | Kết nối MongoDB Atlas |
| `PG_CONNECTION_STRING` | ✓ | Postgres — auth DB |
| `REDIS_URL` | ✓ | Redis — cache + rate-limit |
| `S3_ENDPOINT` / `S3_ACCESS_KEY` / `S3_SECRET_KEY` / `S3_BUCKET` | - | MinIO/S3, có default cho dev local |
| `ANTHROPIC_API_KEY`, `CLAUDE_MODEL` | ✓ (anthropic) | LLM cho legacy LangGraph agent |
| `LLM_PROVIDER`, `GOOGLE_API_KEY`, `GOOGLE_MODEL`, `GOOGLE_THINKING_LEVEL` | tuỳ | Provider Gemini cho legacy agent |
| `VOYAGE_API_KEY` | ✓ | Embedding cho RAG |
| `JWT_SECRET`, `JWT_REFRESH_SECRET` | ✓ | Tối thiểu 32 ký tự |
| `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_REDIRECT_URI` | ✓ | Google OAuth |
| `RESEND_API_KEY`, `EMAIL_FROM` | tuỳ chọn | Gửi email OTP (không có thì log cảnh báo, không chặn startup) |
| `AGENT_BACKEND` | - | `langgraph` (mặc định) hoặc `go` |
| `AGENT_GO_URL`, `AGENT_GO_TIMEOUT` | - | URL + timeout gọi services/agent-go |
| `CHAT_CACHE_VERSION` | - | Bump khi đổi model/prompt bên Go agent để invalidate cache |
| `CORS_ORIGIN` | - | Danh sách origin, phân tách dấu phẩy (rỗng = cho mọi origin) |
| `FRONTEND_URL` | - | Dùng cho redirect Google OAuth |
| `LANGSMITH_*` | tuỳ chọn | Tracing cho LangGraph legacy |

---

## Scripts

```bash
pnpm --filter @app/api dev              # tsx watch --env-file=.env src/server.ts
pnpm --filter @app/api build             # tsc → dist/
pnpm --filter @app/api start             # node dist/server.js
pnpm --filter @app/api test              # vitest run
pnpm --filter @app/api test:coverage     # vitest run --coverage
pnpm --filter @app/api typecheck         # tsc --noEmit
pnpm --filter @app/api migrate:docs      # backfill document versions (script riêng)
```

Hoặc từ gốc repo: `pnpm dev:api`, `pnpm build:api`, `pnpm test:api`.

## Testing

17 file `*.test.ts` (Vitest) bao phủ: auth (OTP), chat routes/service/stream (bao gồm luồng `continue`), documents (chunk/extract/service), tasks repository, voyage embedding client, go-agent client (SSE mapping), app-level smoke test. Chạy `pnpm --filter @app/api test`.

## Tech Stack

Fastify 5, Zod (validate env + DTO), MongoDB driver, `pg` (Postgres), `ioredis`, AWS SDK v3 (S3), `@fastify/multipart` (upload), `@fastify/rate-limit`, `jsonwebtoken`, `bcrypt`, Resend (email), `unpdf` (PDF extraction). Phần legacy còn giữ `@langchain/*` cho agent LangGraph (`AGENT_BACKEND=langgraph`), đang được thay dần bằng proxy sang Go agent.
