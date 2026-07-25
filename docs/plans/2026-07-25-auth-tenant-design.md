# Auth + Multi-Tenant Design

> **Date:** 2026-07-25
> **Decision:** Option A — mỗi user = 1 tenant, auth trong BFF, hybrid PostgreSQL + MongoDB

---

## 1. Tổng quan kiến trúc

```
┌─────────────────────────────────────────────────────────────────────┐
│                          FRONTEND (React)                            │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────┐   │
│  │ /login       │  │ /register    │  │ /admin/users (admin)     │   │
│  │ /chat        │  │ /documents   │  │ /settings                │   │
│  └──────────────┘  └──────────────┘  └──────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────┘
                                    │
                          Cookie: access_token + refresh_token
                                    │
┌─────────────────────────────────────────────────────────────────────┐
│                      BFF (Fastify — Port 3001)                       │
│                                                                      │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │  Middleware: authGuard → extract JWT → req.tenantId           │   │
│  │  Middleware: adminGuard → check role=admin                    │   │
│  └──────────────────────────────────────────────────────────────┘   │
│                                                                      │
│  ┌──────────────────────┐  ┌──────────────────────────────────────┐ │
│  │ modules/auth/         │  │ modules/chat/ + documents/ + tasks/  │ │
│  │ modules/users/        │  │ • Mọi query có { tenantId } filter   │ │
│  │ (auth flow + admin)   │  │ • Go agent proxy: gắn X-Tenant-ID    │ │
│  └──────────────────────┘  └──────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────┘
           │                              │
    ┌──────┴──────┐              ┌───────┴────────┐
    │ PostgreSQL  │              │   MongoDB       │
    │ (VPS)       │              │   Atlas         │
    │             │              │                 │
    │ • users     │              │ • conversations │
    │ • credentials│             │ • messages      │
    │ • sessions  │              │ • tasks         │
    │ • refresh_  │              │ • documents     │
    │   tokens    │              │ • memories      │
    └─────────────┘              └─────────────────┘
           │
    tenant_id = user_id ──────────────────────────┘
```

### Quyết định chính

| Quyết định | Lựa chọn |
|------------|----------|
| Tenant model | User = Tenant (1 user → 1 tenant tự động) |
| Auth methods | Email/Password + Google OAuth |
| Auth location | BFF (Fastify) — Go agent chỉ nhận `X-Tenant-ID` |
| Auth DB | PostgreSQL (tận dụng VPS có sẵn) |
| AI Data DB | MongoDB Atlas (giữ nguyên) |
| Token strategy | Access token 15ph + Refresh token 7 ngày, httpOnly cookie |
| Token kết hợp OAuth | JWT trong httpOnly cookie |

---

## 2. Data Model

### 2.1 PostgreSQL — Auth Database

```sql
-- Bảng users: thông tin người dùng
CREATE TABLE users (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email       VARCHAR(255) UNIQUE NOT NULL,
    name        VARCHAR(255) NOT NULL,
    avatar_url  VARCHAR(512),
    role        VARCHAR(20) NOT NULL DEFAULT 'user',        -- 'user' | 'admin'
    status      VARCHAR(20) NOT NULL DEFAULT 'active',      -- 'active' | 'disabled' | 'deleted'
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Bảng credentials: auth methods (1 user có thể có nhiều method)
CREATE TABLE credentials (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    method          VARCHAR(20) NOT NULL,                   -- 'email' | 'google'

    -- Email/password fields
    password_hash   VARCHAR(255),

    -- Google OAuth fields
    google_id       VARCHAR(255),
    google_email    VARCHAR(255),

    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(user_id, method),
    UNIQUE(google_id)
);

-- Bảng refresh_tokens: quản lý refresh token family
CREATE TABLE refresh_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  VARCHAR(255) UNIQUE NOT NULL,
    family      UUID NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_refresh_tokens_user ON refresh_tokens(user_id);
CREATE INDEX idx_refresh_tokens_expires ON refresh_tokens(expires_at);
```

### 2.2 MongoDB — Thêm tenantId

Mọi collection hiện có đều thêm field:
```typescript
{ tenantId: string }  // reference đến PostgreSQL users.id
```

Index mới cần tạo:
```typescript
{ tenantId: 1, createdAt: -1 }    // conversations, messages, tasks
{ tenantId: 1, documentId: 1 }    // documents
{ tenantId: 1, key: 1 }           // memories
```

### 2.3 Mối quan hệ user_id ↔ tenant_id

```
PostgreSQL users.id (UUID) ──→ MongoDB tenantId (string)

JWT payload: { sub: user.id, email, role }
authGuard decode → req.tenantId = payload.sub
MongoDB: .find({ tenantId: req.tenantId })
```

---

## 3. Auth Flows

### 3.1 Email/Password Register

```
POST /api/auth/register
Body: { email, password, name }

1. Validate email format, password strength (min 8)
2. Check email UNIQUE → 409 Conflict nếu trùng
3. hash = bcrypt(password, 12 rounds)
4. INSERT INTO users → user
5. INSERT INTO credentials (user_id, method='email', password_hash=hash)
6. issueTokens(user) → set httpOnly cookies
7. Return { user: { id, email, name, role } } — 201 Created
```

### 3.2 Email/Password Login

```
POST /api/auth/login
Body: { email, password }

1. Tìm user theo email
2. Tìm credential WHERE method='email'
3. bcrypt.compare(password, credential.password_hash) → sai → 401
4. Check user.status !== 'disabled' → 403
5. issueTokens(user) → set httpOnly cookies
6. Return { user: { id, email, name, role } } — 200 OK
```

### 3.3 Google OAuth Login

```
GET /api/auth/google          → redirect Google OAuth consent
GET /api/auth/google/callback → exchange code → userinfo → find/create user
                               → issueTokens → redirect frontend

Hợp nhất account: nếu user đã đăng ký bằng email,
login bằng Google cùng email → thêm credential method='google'
cho user hiện có (không tạo user mới).
```

### 3.4 Token Refresh

```
POST /api/auth/refresh
Cookie: refresh_token

1. hash = SHA256(token)
2. Tìm refresh_tokens WHERE token_hash = hash → 401 nếu không thấy
3. Check expires_at > NOW() → 401 nếu hết hạn
4. XOÁ token cũ
5. issueTokens(user) → set cookie mới
6. Return { message: 'ok' }

Security:
- Token rotation: mỗi refresh → token mới, token cũ bị xoá
- Family-based detection: reuse attack → xoá toàn bộ family
```

### 3.5 Cookie Configuration

```typescript
{
  access_token: {
    value: jwt.sign({ sub, email, role }, secret, { expiresIn: '15m' }),
    httpOnly: true, secure: true, sameSite: 'lax',
    path: '/', maxAge: 15 * 60
  },
  refresh_token: {
    value: crypto.randomBytes(48).toString('base64url'),
    httpOnly: true, secure: true, sameSite: 'lax',
    path: '/api/auth', maxAge: 7 * 24 * 60 * 60
  }
}
```

---

## 4. BFF Refactor — Module Pattern (kiểu NestJS)

### 4.1 Cấu trúc thư mục mới

```
apps/api/src/
├── main.ts                      ← entry: create app + listen
├── app.ts                       ← build app (testable, export buildApp)
├── config/
│   ├── index.ts                 ← env schema (zod)
│   └── env.ts
├── common/
│   ├── guards/
│   │   ├── auth.guard.ts        ← onRequest hook: verify JWT → req.tenantId
│   │   └── admin.guard.ts       ← onRequest hook: check role=admin
│   ├── filters/
│   │   └── error.filter.ts     ← error handler tập trung
│   ├── interfaces/
│   │   └── auth-context.ts      ← AuthContext, TenantContext types
│   └── pipes/
│       └── validation.pipe.ts   ← Zod validation wrapper
├── database/
│   ├── mongo/
│   │   ├── mongo.module.ts      ← connect + health + ensureIndexes
│   │   ├── collections.ts       ← tên collections
│   │   └── tenant.filter.ts     ← helper: tenantFilter(req, extra?)
│   ├── postgres/
│   │   ├── postgres.module.ts   ← pg Pool singleton
│   │   └── migrations/
│   │       └── 001-create-auth-tables.sql
│   └── index.ts
├── modules/
│   ├── auth/
│   │   ├── auth.module.ts       ← Fastify plugin: đăng ký routes
│   │   ├── auth.controller.ts   ← Handler: parse request → service
│   │   ├── auth.service.ts      ← Business logic
│   │   ├── auth.repository.ts   ← PostgreSQL queries
│   │   ├── dto/
│   │   │   ├── register.dto.ts
│   │   │   ├── login.dto.ts
│   │   │   └── token.dto.ts
│   │   └── strategies/
│   │       ├── jwt.strategy.ts      ← sign + verify + cookie setter
│   │       ├── google.strategy.ts   ← OAuth2 exchange + userinfo
│   │       └── token.service.ts     ← refresh token CRUD + rotation
│   ├── users/
│   │   ├── users.module.ts
│   │   ├── users.controller.ts  ← Admin: list, get, disable users
│   │   ├── users.service.ts
│   │   ├── users.repository.ts
│   │   └── dto/
│   ├── chat/                    ← (giữ nguyên, thêm tenant filter)
│   ├── documents/               ← (giữ nguyên, thêm tenant filter)
│   └── tasks/                   ← (giữ nguyên, thêm tenant filter)
└── agent/
    ├── agent.module.ts
    └── agent.client.ts          ← proxy Go agent, gắn X-Tenant-ID
```

### 4.2 Module pattern

Mỗi module export 1 Fastify plugin:

```typescript
// modules/auth/auth.module.ts
export async function authModule(app: FastifyInstance) {
  const repo = new AuthRepository(getPgPool());
  const tokenService = new TokenService(config, repo);
  const authService = new AuthService(repo, tokenService);
  const controller = new AuthController(authService);

  // Public routes
  app.post('/api/auth/register', controller.register);
  app.post('/api/auth/login', controller.login);
  app.get('/api/auth/google', controller.googleRedirect);
  app.get('/api/auth/google/callback', controller.googleCallback);
  app.post('/api/auth/refresh', controller.refresh);
  app.post('/api/auth/logout', controller.logout);

  // Protected route
  app.get('/api/auth/me', { onRequest: [authGuard] }, controller.me);
}
```

### 4.3 Dependency Injection (manual)

```typescript
// app.ts
export function buildApp(): FastifyInstance {
  const app = Fastify({ logger: true });

  // Database modules (singleton)
  const pgPool = await initPostgres(config);
  const mongoDb = await initMongo(config);

  // Feature modules
  app.register(authModule, { pgPool });
  app.register(usersModule, { pgPool });
  app.register(chatModule, { mongoDb, agentClient });
  app.register(documentsModule, { mongoDb });
  app.register(tasksModule, { mongoDb });

  return app;
}
```

### 4.4 Guard pattern

```typescript
// common/guards/auth.guard.ts
export const authGuard: preHandlerHookHandler = async (req, reply) => {
  const token = req.cookies?.access_token;
  if (!token) throw new UnauthorizedError('Authentication required');
  try {
    const payload = jwt.verify(token, config.JWT_SECRET);
    req.tenantId = payload.sub;
    req.user = payload;
  } catch {
    throw new UnauthorizedError('Token expired');
  }
};
```

### 4.5 Tenant filter helper

```typescript
// database/mongo/tenant.filter.ts
export function tenantFilter(req: FastifyRequest, extra: Record<string, unknown> = {}) {
  return { tenantId: req.tenantId, ...extra };
}

// Usage: db.collection('conversations').find(tenantFilter(req))
```

---

## 5. Go Agent Integration

### 5.1 BFF → Go Agent proxy

```typescript
// agent/agent.client.ts
export async function proxyChatToGoAgent(req: FastifyRequest, body: ChatRequest) {
  return fetch(`${config.AGENT_GO_URL}/chat`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-Tenant-ID': req.tenantId,
    },
    body: JSON.stringify({ ...body, tenantId: req.tenantId }),
  });
}
```

### 5.2 Go Agent changes

```go
// internal/transport/http/chat.go
func NewChatHandler(orch *orchestrator.Orchestrator) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        tenantID := r.Header.Get("X-Tenant-ID")
        ctx := agent.NewContext(r.Context(), tenantID)
        // engine.Run(ctx, messages...)
    }
}
```

Các tool RAG, memory, tasks trong Go agent sẽ dùng `tenantID` từ context để filter.

---

## 6. Frontend Changes

### 6.1 Route structure

```tsx
<Routes>
  <Route path="/login" element={<LoginPage />} />
  <Route path="/register" element={<RegisterPage />} />
  <Route element={<AuthGuard />}>
    <Route element={<AppLayout />}>
      <Route path="/" element={<ChatPage />} />
      <Route path="/messages/:id" element={<ChatPage />} />
      <Route path="/documents" element={<DocumentsView />} />
      <Route element={<AdminGuard />}>
        <Route path="/admin/users" element={<UsersManagement />} />
      </Route>
    </Route>
  </Route>
</Routes>
```

### 6.2 Auth store (Zustand)

```typescript
// shared/stores/auth.store.ts
interface AuthState {
  user: User | null;
  isLoading: boolean;
  login(email, password): Promise<void>;
  register(email, password, name): Promise<void>;
  loginWithGoogle(): void;            // redirect
  logout(): Promise<void>;
  fetchMe(): Promise<void>;
  refreshToken(): Promise<boolean>;
}
```

### 6.3 HTTP interceptor (auto refresh)

```typescript
http.interceptors.response.use(
  (res) => res,
  async (error) => {
    if (error.response?.status === 401 && !error.config._retry) {
      error.config._retry = true;
      const ok = await useAuthStore.getState().refreshToken();
      if (ok) return http(error.config);
    }
    return Promise.reject(error);
  }
);
```

---

## 7. Endpoint Summary

| Method | Path | Auth | Mô tả |
|--------|------|------|-------|
| POST | `/api/auth/register` | Public | Đăng ký email/pass |
| POST | `/api/auth/login` | Public | Đăng nhập email/pass |
| GET | `/api/auth/google` | Public | Redirect Google OAuth |
| GET | `/api/auth/google/callback` | Public | Google callback |
| POST | `/api/auth/refresh` | Public (cookie) | Refresh access token |
| POST | `/api/auth/logout` | Public | Xoá cookie + token |
| GET | `/api/auth/me` | Required | Lấy user hiện tại |
| GET | `/api/admin/users` | Admin | Danh sách users |
| PATCH | `/api/admin/users/:id` | Admin | Disable/update user |
| Tất cả `/api/chat`, `/api/documents`, `/api/tasks` | Required | Có tenant filter |

---

## 8. Decision Log

| Quyết định | Lý do |
|------------|-------|
| Auth ở BFF, không ở Go Agent | Đúng BFF pattern: auth là cross-cutting concern của API gateway |
| PostgreSQL cho auth | Tận dụng VPS có sẵn, ACID, unique constraints mạnh |
| MongoDB cho AI data | Document model phù hợp chat/knowledge, vector search |
| Module pattern kiểu NestJS | User muốn tổ chức code có cấu trúc, dễ mở rộng |
| Manual DI thay vì framework | Đơn giản, không thêm dependency |
| httpOnly cookie thay vì Authorization header | An toàn hơn: chống XSS, cookie tự động gửi |
| Refresh token rotation + family detection | Phát hiện token theft |
