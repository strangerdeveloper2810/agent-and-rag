# J.A.R.V.I.S. — Docker

## Environment

Tất cả biến môi trường được gom về 1 chỗ:

```
env/
├── .env.development    # Dev defaults (committed, safe)
└── .env.example        # Production template → copy thành .env
```

## Development (local)

Chỉ chạy PostgreSQL + Redis trong Docker. Code (api, web, agent-go) chạy qua `pnpm dev` trên máy host. MongoDB dùng Atlas cloud.

```bash
# Start infrastructure
pnpm docker:dev

# Hoặc thủ công:
docker compose -f docker/development/docker-compose.yml up -d

# Xem logs
docker compose -f docker/development/docker-compose.yml logs -f

# Stop
pnpm docker:dev:down
```

| Service | Port | Ghi chú |
|---|---|---|
| **postgres** | 5432 | Auth DB (users, credentials, refresh tokens) |
| **redis** | 6379 | Rate limiting, cache (embedding/chat/tool) |

## Deployment (production)

Full stack: PostgreSQL + Redis + Go Agent + API BFF + Frontend.

```bash
# 1. Tạo env file từ template
cp env/.env.example env/.env
# → Sửa giá trị thật trong env/.env

# 2. Build & start
pnpm docker:up

# Hoặc thủ công:
docker compose -f docker/deployment/docker-compose.yml up -d --build

# View logs
docker compose -f docker/deployment/docker-compose.yml logs -f

# Stop
pnpm docker:down
```

| Service | Port | Image | Size |
|---|---|---|---|
| **postgres** | 5432 | postgres:17-alpine | ~10MB |
| **redis** | 6379 | redis:7-alpine | ~10MB |
| **agent-go** (Go engine) | 3002 | jarvis-agent-go | ~12MB |
| **api** (Fastify/TS BFF) | 3001 | jarvis-api | ~200MB |
| **web** (nginx + React) | 80 | jarvis-web | ~30MB |

## Architecture (deployment)

```
Browser :80 → nginx (React SPA)
              ├── /api/* → api:3001 (SSE proxy → agent-go:3002)
              └── /suggestions → agent-go:3002

api:3001 → postgres:5432 (auth data)
         → redis:6379 (cache + rate limit)
         → MongoDB Atlas (AI data: chat, documents, tasks)
```

## Volumes

- `jarvis_data` — SQLite database for Go agent
- `pgdata` / `pgdata_dev` — PostgreSQL persistent data
- `redis_data` / `redis_dev` — Redis persistent data

## Health Checks

- `postgres`: pg_isready (10s interval)
- `redis`: redis-cli ping (10s interval)
- `agent-go`: GET /healthz (10s interval)
- `api`: GET /api/health (15s interval, depends on postgres + redis + agent-go)
