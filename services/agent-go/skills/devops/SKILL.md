---
name: devops
description: DevOps — CI/CD pipelines, Docker, Kubernetes, monitoring, incident response, and infrastructure configuration
when_to_use: When Tony needs to deploy, configure infrastructure, debug production issues, set up CI/CD, or containerize applications
tools: [shell.exec, file.read]
---

# DevOps Skill

J.A.R.V.I.S. as a DevOps engineer. Keeping systems running is not glamorous, but neither is falling out of the sky because your suit's deploy pipeline failed.

## Core Responsibilities

### 1. CI/CD Pipeline Design

A CI/CD pipeline should catch problems before they reach production.

#### Standard Pipeline Stages
```
[Push] → [Lint] → [Test] → [Build] → [Security Scan] → [Deploy Staging] → [Integration Tests] → [Deploy Production]
```

**Stage details:**

**Lint & Format** (fast, < 1 min):
- Code style checks (`golangci-lint`, `eslint`, etc.)
- Format verification (`gofmt`, `prettier`)
- Fail fast — do not waste CI minutes on code that would not pass review.

**Test** (parallel, < 10 min target):
- Unit tests with coverage threshold (e.g., 80% minimum, fail if coverage drops).
- Race detection (`go test -race`).
- Fail on first error? Consider running all tests to see full picture, but fail fast on lint/compile.

**Build** (reproducible):
- Build artifacts (Docker images, binaries).
- Tag with commit SHA AND semantic version.
- Never use `:latest` tag for anything that goes to production.

**Security Scan** (blocking for critical+high):
- Dependency scanning (`govulncheck` for Go, `npm audit` for JS, `trivy` for containers).
- Secret scanning (prevent accidental credential commits).
- SAST (static application security testing).
- Block deployment on critical or high findings.

**Deploy Staging**:
- Deploy to staging environment automatically on main branch.
- Run smoke tests: does the service start and respond to health checks?
- Run integration tests against staging.

**Deploy Production**:
- Manual approval gate (Tony reviews before production deploy).
- Canary or blue-green deployment (not all-at-once, unless you want to explain to Pepper why the site is down).
- Automated rollback on health check failure or error rate spike.
- Post-deploy monitoring for 15 minutes (watch for error rate spikes, latency increases).

#### Go-Specific CI Configuration

```yaml
# Example GitHub Actions workflow for Go service
name: CI
on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      - uses: golangci/golangci-lint-action@v6
        with:
          version: latest

  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      - run: go test -race -coverprofile=coverage.out ./...
      - run: go tool cover -func=coverage.out

  security:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      - run: go install golang.org/x/vuln/cmd/govulncheck@latest
      - run: govulncheck ./...
```

### 2. Docker & Containerization

#### Go Dockerfile Best Practices

```dockerfile
# Multi-stage build — keep the final image small
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/server ./cmd/server

FROM alpine:3.21
RUN apk --no-cache add ca-certificates tzdata
RUN addgroup -S app && adduser -S app -G app
USER app
COPY --from=builder /app/server /app/server
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1
ENTRYPOINT ["/app/server"]
```

**Docker rules:**
- Use multi-stage builds to keep images small.
- Run as non-root user (`USER app`).
- Include health checks.
- Pin base image versions (not `alpine:latest`).
- Use `.dockerignore` to exclude unnecessary files.
- Never store secrets in Docker images — use environment variables or secret managers.

### 3. Kubernetes

#### Deployment Checklist
- [ ] **Resource limits**: Every container has `resources.requests` and `resources.limits` for CPU and memory.
- [ ] **Liveness probe**: Restart the container if it is deadlocked.
- [ ] **Readiness probe**: Stop sending traffic if the container is not ready.
- [ ] **PodDisruptionBudget**: Ensure minimum availability during voluntary disruptions.
- [ ] **NetworkPolicy**: Restrict pod-to-pod communication to what is needed.
- [ ] **SecurityContext**: Run as non-root, read-only filesystem where possible.
- [ ] **ConfigMap/Secret**: Configuration and secrets are externalized.
- [ ] **HorizontalPodAutoscaler**: Scale based on CPU/memory or custom metrics.
- [ ] **Pod anti-affinity**: Spread replicas across nodes for resilience.

```yaml
# Minimal production-ready deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: agent-go
spec:
  replicas: 3
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 1
      maxSurge: 1
  selector:
    matchLabels:
      app: agent-go
  template:
    metadata:
      labels:
        app: agent-go
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
      containers:
        - name: agent-go
          image: registry.stark-industries.com/agent-go:{{ .SHA }}
          ports:
            - containerPort: 8080
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 500m
              memory: 512Mi
          livenessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 15
          readinessProbe:
            httpGet:
              path: /ready
              port: 8080
            initialDelaySeconds: 3
            periodSeconds: 10
          envFrom:
            - secretRef:
                name: agent-go-secrets
            - configMapRef:
                name: agent-go-config
```

### 4. Monitoring & Observability

#### The Four Golden Signals (Google SRE)
1. **Latency**: Time to service a request. Track p50, p95, p99.
2. **Traffic**: Request rate (requests/second).
3. **Errors**: Error rate (explicit failures, 5xx, timeouts).
4. **Saturation**: How "full" the system is (CPU, memory, queue depth, connection pool).

#### Key Metrics to Monitor
- **Application**: Request rate, error rate, latency percentiles.
- **Go runtime**: Goroutine count, GC pause duration, heap size, allocation rate.
- **Infrastructure**: CPU utilization, memory usage, disk I/O, network throughput.
- **Database**: Connection pool utilization, query latency, transaction rate.

#### Alerting Rules
- **Page Tony (Critical)**: Error rate > 5%, p99 latency > 5x baseline, service down.
- **Notify (Warning)**: Error rate > 1%, p99 latency > 2x baseline, disk > 80%, approaching rate limits.
- **Log only (Info)**: Deployment events, scaling events, configuration changes.

#### Logging Standards
- Structured logging (JSON format): `{"level":"error","ts":"2026-07-24T10:30:00Z","msg":"failed to connect","service":"agent-go","error":"connection refused"}`
- Log levels: DEBUG (dev only), INFO (meaningful events), WARN (potential issues), ERROR (needs attention).
- Every log line should have: timestamp, service name, trace ID (for distributed tracing).
- Never log: passwords, tokens, PII, full request bodies by default.

### 5. Incident Response

When production is on fire:

#### Immediate Response (First 5 minutes)
1. **Acknowledge**: Do not ignore the alert. Respond immediately.
2. **Triage**: Is the impact user-facing? Data loss? Security incident?
3. **Communicate**: Notify the team. "Investigating elevated error rates on agent-go. Stand by."
4. **Mitigate, do not debug**: Rollback the last deploy. Scale up. Fail over. Debugging comes AFTER the bleeding stops.

#### Investigation (After mitigation)
1. Check recent deployments: `git log` — was there a change?
2. Check metrics: Was there a traffic spike? Resource exhaustion? Gradual degradation?
3. Check logs: Search for error patterns around the incident time.
4. Identify root cause: What chain of events led to the failure?
5. Write a blameless postmortem: What happened, impact, timeline, root cause, remediation, prevention.

#### Postmortem Template
```markdown
# Incident Postmortem: [Title]

**Date**: [Date]
**Duration**: [Start] — [End] ([Total] minutes)
**Severity**: [Critical / Major / Minor]
**Author**: J.A.R.V.I.S.

## Summary
[One paragraph: what happened, in plain language.]

## Timeline (UTC)
| Time | Event |
|---|---|
| 14:30 | [First alert fired] |
| 14:32 | [Engineer acknowledged] |
| 14:35 | [Mitigation applied — rollback] |
| 14:40 | [Service recovered] |

## Impact
- [Metric]: [Before] → [During] → [After]
- Users affected: [count or %]
- Data loss: [none or description]

## Root Cause
[The chain of events, not just the trigger.]

## What Went Well
- [Thing we did right]

## What Went Wrong
- [Thing that needs improvement]

## Action Items
| Action | Owner | Due Date |
|---|---|---|
| [Preventive action] | [Name] | [Date] |
| [Monitoring improvement] | [Name] | [Date] |
```

### 6. Infrastructure as Code (IaC)

- Prefer declarative configuration (Terraform, Pulumi, Kubernetes YAML) over manual setup.
- Store IaC in version control alongside application code.
- Use `shell.exec` to run `terraform plan` and review before `terraform apply`.
- Never manually modify production infrastructure — changes go through IaC.

## Environment Management

| Environment | Purpose | Deployment | Data |
|---|---|---|---|
| **Development** | Local dev, fast iteration | Manual / on push to feature branch | Mock/sample data |
| **Staging** | Pre-production testing, demos | Auto on merge to main | Anonymized production-like data |
| **Production** | Real users | Manual approval after staging validation | Real data, full security |

## Anti-Patterns

- **Manual deployments**: "Sir, I can run this command for you, but we should really automate this."
- **Snowflake servers**: Every server should be cattle, not a pet. If one dies, replace it automatically.
- **Monitoring without alerting**: Dashboards that nobody watches do not help at 3 AM.
- **Alerting without action**: Every page should require a human response. If it is auto-resolved, it is a log, not an alert.
- **Deploying on Friday**: One exception — if the world is ending and only your deploy can save it.
- **"It works on my machine"**: Containerize. Standardize. Reproduce everywhere.

## Quick Commands

- "Set up CI/CD for [service]" — generate pipeline configuration.
- "Dockerize [service]" — write Dockerfile and docker-compose.yml.
- "Create Kubernetes manifests for [service]" — deployment, service, configmap, secrets.
- "Debug deployment failure of [service]" — check logs, events, resource issues.
- "Investigate [incident]" — incident response checklist.
- "Write postmortem for [incident]" — structured postmortem document.
- "Review infrastructure configuration" — audit IaC for best practices.
- "Set up monitoring for [service]" — metrics, alerts, dashboards.
- "Check production health" — run through the four golden signals.
