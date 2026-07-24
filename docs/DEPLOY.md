# JARVIS Deployment Guide

Huong dan trien khai JARVIS agent runtime tu local development den production Docker.

Deployment guide from local development to production Docker.

---

## Local Development

### Prerequisites

- **Go 1.25+** — [go.dev/dl](https://go.dev/dl/)
- **MongoDB Atlas** cluster (free tier: [cloud.mongodb.com](https://cloud.mongodb.com))
- API key from one of:
  - **Gemini** — [aistudio.google.com](https://aistudio.google.com) (free tier, 1,500 req/day)
  - **Anthropic** — [console.anthropic.com](https://console.anthropic.com)
- (Optional) **Voyage AI** key — [voyageai.com](https://voyageai.com) for embeddings/RAG

### Environment Variables

Create `.env` in `services/agent-go/` (or export them directly):

```bash
# Required
export MONGODB_URI="mongodb+srv://user:pass@cluster.mongodb.net/?retryWrites=true&w=majority"

# LLM Provider (choose one)
export LLM_PROVIDER="gemini"           # or "anthropic"
export GEMINI_API_KEY="your-gemini-key"
export GEMINI_MODEL="gemini-3.1-flash-lite"
# export ANTHROPIC_API_KEY="your-anthropic-key"
# export CLAUDE_MODEL="claude-haiku-4-5-20251001"

# Optional
export PORT="3002"                     # default: 3002
export MONGODB_DB="ai_agent_tut"       # default: ai_agent_tut
export GOOGLE_THINKING_LEVEL="LOW"     # OFF | LOW | MEDIUM | HIGH
export VOYAGE_API_KEY="your-voyage-key"
```

### Run

```bash
# From monorepo root
cd services/agent-go

# HTTP server mode
go run ./cmd/server

# CLI mode — one-shot question
go run ./cmd/jarvis ask "Thoi tiet hom nay the nao?"

# CLI mode — interactive chat REPL
go run ./cmd/jarvis chat

# With env file (if using direnv or similar)
source .env && go run ./cmd/server
```

### Verify

```bash
# Health check
curl http://localhost:3002/healthz
# {"status":"ok"}

# Test chat
curl -X POST http://localhost:3002/chat \
  -H "Content-Type: application/json" \
  -N \
  -d '{"userMessage":"Xin chao, ban la ai?"}'
```

Expected output: SSE stream of JSON events, starting with `step` events and ending with `done`.

---

## Docker Deployment

### Build Image

The Dockerfile uses multi-stage builds with distroless for a minimal (~15MB) image:

```dockerfile
# services/agent-go/Dockerfile
# syntax=docker/dockerfile:1
FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/server ./cmd/server

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/server /server
EXPOSE 3002
ENTRYPOINT ["/server"]
```

Build and run:

```bash
# Build
cd services/agent-go
docker build -t jarvis-agent:latest .

# Run
docker run -d \
  --name jarvis \
  -p 3002:3002 \
  -e MONGODB_URI="mongodb+srv://..." \
  -e LLM_PROVIDER="gemini" \
  -e GEMINI_API_KEY="your-key" \
  -e GEMINI_MODEL="gemini-3.1-flash-lite" \
  jarvis-agent:latest

# Check logs
docker logs -f jarvis

# Verify
curl http://localhost:3002/healthz
```

---

## Docker Compose (JARVIS + Ollama)

Run JARVIS with Ollama for fully local LLM (no API keys needed):

```yaml
# docker-compose.yml in project root
version: "3.9"

services:
  ollama:
    image: ollama/ollama:latest
    container_name: jarvis-ollama
    ports:
      - "11434:11434"
    volumes:
      - ollama_data:/root/.ollama
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:11434/api/tags"]
      interval: 10s
      timeout: 5s
      retries: 5

  # Pull model on first run
  ollama-init:
    image: ollama/ollama:latest
    container_name: jarvis-ollama-init
    depends_on:
      ollama:
        condition: service_healthy
    entrypoint: ["/bin/sh", "-c"]
    command:
      - |
        ollama pull gemma3:4b
    environment:
      - OLLAMA_HOST=ollama:11434

  jarvis:
    build:
      context: ./services/agent-go
      dockerfile: Dockerfile
    container_name: jarvis-agent
    ports:
      - "3002:3002"
    environment:
      - PORT=3002
      - MONGODB_URI=mongodb://mongo:27017/ai_agent_tut
      - MONGODB_DB=ai_agent_tut
      - LLM_PROVIDER=ollama
      - OLLAMA_HOST=http://ollama:11434
      - OLLAMA_MODEL=gemma3:4b
    depends_on:
      ollama-init:
        condition: service_completed_successfully
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:3002/healthz"]
      interval: 10s
      timeout: 5s
      retries: 5
      start_period: 15s

  # Optional: local MongoDB (instead of Atlas)
  mongo:
    image: mongo:8
    container_name: jarvis-mongo
    ports:
      - "27017:27017"
    volumes:
      - mongo_data:/data/db
    restart: unless-stopped

volumes:
  ollama_data:
  mongo_data:
```

### Running with Docker Compose

```bash
# Start all services (Ollama + JARVIS + MongoDB)
docker compose up -d

# Check status
docker compose ps

# View logs
docker compose logs -f jarvis

# Test
curl http://localhost:3002/healthz
```

**Note:** The Ollama provider for JARVIS uses a custom adapter (`internal/provider/ollama/`) compatible with OpenAI-compatible chat completion APIs. Configure `LLM_PROVIDER=ollama` and set `OLLAMA_HOST` + `OLLAMA_MODEL`.

---

## Environment Variables Reference

| Variable | Required | Default | Description |
|----------|:--------:|---------|-------------|
| `PORT` | No | `3002` | HTTP server port |
| `MONGODB_URI` | **Yes** | — | MongoDB connection string (Atlas or local) |
| `MONGODB_DB` | No | `ai_agent_tut` | MongoDB database name |
| `LLM_PROVIDER` | No | `gemini` | LLM provider: `gemini`, `anthropic`, or `ollama` |
| `GEMINI_API_KEY` | If gemini | — | Google AI Studio API key |
| `GEMINI_MODEL` | No | `gemini-3.1-flash-lite` | Gemini model name |
| `GOOGLE_THINKING_LEVEL` | No | `LOW` | Gemini thinking budget: `OFF`, `LOW`, `MEDIUM`, `HIGH` |
| `ANTHROPIC_API_KEY` | If anthropic | — | Anthropic API key |
| `CLAUDE_MODEL` | No | `claude-haiku-4-5-20251001` | Claude model name |
| `OLLAMA_HOST` | If ollama | — | Ollama server URL (e.g., `http://localhost:11434`) |
| `OLLAMA_MODEL` | No | `gemma3:4b` | Ollama model name |
| `VOYAGE_API_KEY` | For RAG | — | Voyage AI API key for embeddings |

### Model Selection Guide

| Model | Cost (per 1M tokens) | Best For |
|-------|---------------------|----------|
| `gemini-3.1-flash-lite` | ~$0.075 input | Default — cheapest, fast, good quality |
| `gemini-3.1-flash` | ~$0.15 input | Better quality, still fast |
| `gemini-3.1-pro` | ~$1.25 input | Complex reasoning, multi-step plans |
| `claude-haiku-4-5-20251001` | ~$0.80 input | Fast Claude, good for simple tasks |
| `claude-sonnet-4-20250514` | ~$3.00 input | Best quality, complex agentic tasks |
| `gemma3:4b` (Ollama) | Free (local) | Offline, privacy, no API costs |

---

## Health Check Configuration

### Kubernetes

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: jarvis
spec:
  containers:
  - name: jarvis
    image: jarvis-agent:latest
    ports:
    - containerPort: 3002
    env:
    - name: MONGODB_URI
      valueFrom:
        secretKeyRef:
          name: jarvis-secrets
          key: mongodb-uri
    livenessProbe:
      httpGet:
        path: /healthz
        port: 3002
      initialDelaySeconds: 5
      periodSeconds: 10
    readinessProbe:
      httpGet:
        path: /healthz
        port: 3002
      initialDelaySeconds: 3
      periodSeconds: 5
```

### Nomad

```hcl
job "jarvis" {
  group "agent" {
    task "server" {
      driver = "docker"
      config {
        image = "jarvis-agent:latest"
        ports = ["http"]
      }
      service {
        name = "jarvis"
        port = "http"
        check {
          type     = "http"
          path     = "/healthz"
          interval = "10s"
          timeout  = "2s"
        }
      }
    }
  }
}
```

### Systemd

```ini
# /etc/systemd/system/jarvis.service
[Unit]
Description=JARVIS AI Agent Runtime
After=network.target

[Service]
Type=simple
User=jarvis
WorkingDirectory=/opt/jarvis
EnvironmentFile=/opt/jarvis/.env
ExecStart=/opt/jarvis/server
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

---

## Production Checklist

Before deploying to production:

- [ ] Set `GOOGLE_THINKING_LEVEL=OFF` or `LOW` for faster responses (thinking adds 30-60s latency with Gemini 3.x)
- [ ] Use a dedicated MongoDB database (not shared with other apps)
- [ ] Set up MongoDB Atlas network access — whitelist your production IP ranges
- [ ] Configure API key rotation schedule
- [ ] Set up monitoring on `/healthz` endpoint (uptime check every 30s)
- [ ] Configure log aggregation (slog writes JSON to stdout — compatible with any log collector)
- [ ] Run `go test ./...` before deploying
- [ ] Build with `-trimpath` for reproducible builds
- [ ] Use distroless base image (already in Dockerfile) — no shell, smaller attack surface
- [ ] Set resource limits: 256MB RAM, 0.5 CPU per instance is sufficient for 10-50 concurrent chats
