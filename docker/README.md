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

Full stack: PostgreSQL + Redis + MinIO + Go Agent + API BFF + Frontend.

### Deploy lên VPS (khuyến nghị — script tự động)

```bash
# 1 lần duy nhất trên VPS (Docker đã cài sẵn):
scp deploy/setup-vps.sh your-vps:/tmp/ && ssh your-vps sudo bash /tmp/setup-vps.sh

# Từ máy local, mỗi lần deploy:
./deploy/deploy-to-vps.sh
```

`deploy-to-vps.sh` tự rsync code (không cần VPS có quyền pull GitHub riêng),
ghép `env/.env.production` (bạn tự điền, KHÔNG commit git) với secret hạ tầng
tự sinh 1 lần, rồi build + `up -d` từ xa qua SSH. Xem chi tiết + biến môi
trường (`JARVIS_SSH_HOST`, `WEB_HOST_PORT`) trong header của script.

**Chỉ `web` được publish port ra host** (mặc định `8090`, đổi qua
`WEB_HOST_PORT` nếu VPS dùng chung với app khác đã chiếm port đó) —
postgres/redis/minio/agent-go/api chỉ giao tiếp NỘI BỘ qua Docker network,
không publish ra host (tránh lộ ra internet trên VPS dùng chung, và tránh
đụng port với service khác — xem comment trong `docker-compose.yml`).

### Chạy thủ công (VPS riêng, không dùng chung)

```bash
# 1. Tạo env file từ template
cp env/.env.example env/.env
# → Sửa giá trị thật trong env/.env

# 2. Build & start (đổi port web nếu 80 đã bị chiếm)
WEB_HOST_PORT=80 docker compose -f docker/deployment/docker-compose.yml up -d --build

# View logs
docker compose -f docker/deployment/docker-compose.yml logs -f

# Stop
docker compose -f docker/deployment/docker-compose.yml down
```

| Service | Port ra host | Image | Size |
|---|---|---|---|
| **postgres** | *(nội bộ, không publish)* | postgres:17-alpine | ~10MB |
| **redis** | *(nội bộ, không publish)* | redis:7-alpine | ~10MB |
| **minio** | *(nội bộ, không publish)* | minio/minio | — |
| **agent-go** (Go engine) | *(nội bộ, không publish)* | jarvis-agent-go | ~12MB |
| **api** (Fastify/TS BFF) | *(nội bộ, không publish)* | jarvis-api | ~200MB |
| **web** (nginx + React) | `${WEB_HOST_PORT:-8090}` | jarvis-web | ~30MB |

## Architecture (deployment)

```
Browser :${WEB_HOST_PORT} → nginx (React SPA, trong container web)
              ├── /api/* → api:3001 (SSE proxy → agent-go:3002, network nội bộ)
              └── /suggestions → agent-go:3002 (network nội bộ)

api:3001 → postgres:5432 (network nội bộ — auth data)
         → redis:6379 (network nội bộ — cache + rate limit)
         → minio:9000 (network nội bộ — object storage)
         → MongoDB Atlas (AI data: chat, documents, tasks — không phải container)
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
