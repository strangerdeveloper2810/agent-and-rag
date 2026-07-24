# J.A.R.V.I.S. — Docker Deployment

## Quick Start

```bash
# Build & start all services
docker compose -f docker/docker-compose.yml up -d

# View logs
docker compose -f docker/docker-compose.yml logs -f

# Stop
docker compose -f docker/docker-compose.yml down
```

## Services

| Service | Port | Image | Size |
|---|---|---|---|
| **web** (nginx + React) | 80 | jarvis-web | ~30MB |
| **api** (Fastify/TS BFF) | 3001 | jarvis-api | ~200MB |
| **agent-go** (Go engine) | 3002 | jarvis-agent-go | ~12MB |

## Architecture

```
Browser :80 → nginx (React SPA)
              ├── /api/* → api:3001 (SSE proxy → agent-go:3002)
              └── /suggestions → agent-go:3002
```

## Prerequisites

- Docker & Docker Compose v2
- `.env` files configured:
  - `services/agent-go/.env` — LLM keys, MongoDB URI, Voyage key
  - `apps/api/.env` — AGENT_BACKEND=go, AGENT_GO_URL

## Environment Variables

See `.env.example` in each service directory for all available options.

## Volumes

- `jarvis_data` — SQLite database persisted at `/data/jarvis.db`

## Health Checks

- `agent-go`: GET /healthz (10s interval)
- `api`: GET /api/health (15s interval, depends on agent-go being healthy)
