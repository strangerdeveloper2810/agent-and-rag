---
name: devops
description: DevOps — CI/CD pipelines, Docker, Kubernetes, monitoring, incident response, and infrastructure configuration
when_to_use: When the user needs to deploy, configure infrastructure, debug production issues, set up CI/CD, or containerize applications
triggers: [deploy, triển khai, trien khai, docker, kubernetes, k8s, ci/cd, pipeline, hạ tầng, ha tang]
tools: [shell.exec, file.read]
---

# DevOps Skill

Giữ hệ thống chạy. Ưu tiên quy tắc quyết định; template Dockerfile/YAML/manifest
thì tự sinh theo yêu cầu cụ thể, không nhắc lại ở đây.

## 1. CI/CD

```
Push → Lint → Test → Build → Security Scan → Deploy Staging → Integration Test → Deploy Prod
```

| Stage | Ngưỡng | Quy tắc |
|---|---|---|
| Lint/Format | < 1 phút | Fail fast, đừng đốt CI cho code không qua review |
| Test | < 10 phút | Có ngưỡng coverage, fail khi coverage tụt; bật race detector |
| Build | reproducible | Tag bằng commit SHA **và** semver. Không bao giờ dùng `:latest` cho production |
| Security scan | blocking | Quét dependency + secret + SAST. Chặn deploy khi có critical/high |
| Deploy staging | tự động trên main | Smoke test: service start + health check; rồi integration test |
| Deploy prod | **cần người duyệt** | Canary hoặc blue-green, không all-at-once |

Sau deploy production: tự rollback khi health check fail hoặc error rate vọt;
theo dõi 15 phút đầu (error rate, latency).

## 2. Docker

- Multi-stage build cho image nhỏ.
- Chạy non-root (`USER app`).
- Có `HEALTHCHECK`.
- Pin version base image, không `:latest`.
- Có `.dockerignore`.
- **Không** nhúng secret vào image — dùng env hoặc secret manager.

## 3. Kubernetes — checklist deploy

- [ ] `resources.requests` + `resources.limits` cho CPU và memory (mọi container)
- [ ] Liveness probe (restart khi deadlock) và readiness probe (ngắt traffic khi chưa sẵn sàng)
- [ ] PodDisruptionBudget — giữ mức khả dụng tối thiểu khi có disruption tự nguyện
- [ ] NetworkPolicy — chỉ mở đúng đường pod-to-pod cần thiết
- [ ] SecurityContext — non-root, filesystem read-only nếu được
- [ ] ConfigMap/Secret — cấu hình và secret nằm ngoài image
- [ ] HorizontalPodAutoscaler
- [ ] Pod anti-affinity — rải replica ra nhiều node
- [ ] RollingUpdate với `maxUnavailable: 1`

## 4. Monitoring

**Bốn tín hiệu vàng (Google SRE):** Latency (p50/p95/p99) · Traffic (req/s) ·
Errors (5xx, timeout) · Saturation (CPU, memory, queue, connection pool).

**Metric cần có:** request/error rate + latency percentile; goroutine count, GC
pause, heap (Go runtime); CPU/memory/disk/network; connection pool + query
latency (DB).

**Ngưỡng alert:**

| Mức | Điều kiện |
|---|---|
| Page (critical) | error rate > 5%, p99 > 5× baseline, service down |
| Notify (warning) | error rate > 1%, p99 > 2× baseline, disk > 80%, gần chạm rate limit |
| Log (info) | deploy, scaling, đổi cấu hình |

**Log:** JSON có cấu trúc; mỗi dòng phải có timestamp + service + trace ID.
Level: DEBUG (chỉ dev) · INFO · WARN · ERROR. **Không** log password, token, PII,
hay full request body.

## 5. Incident response

**5 phút đầu:** acknowledge → triage (ảnh hưởng user? mất dữ liệu? sự cố bảo mật?)
→ thông báo team → **mitigate trước, debug sau** (rollback deploy gần nhất, scale
up, fail over). Cầm máu xong mới điều tra.

**Sau khi đã mitigate:** xem deploy gần đây (`git log`) → xem metric (traffic
spike? cạn tài nguyên? xuống dốc dần?) → tìm pattern lỗi trong log quanh thời
điểm sự cố → xác định root cause (cả chuỗi sự kiện, không chỉ cái kích hoạt) →
viết postmortem không quy trách nhiệm cá nhân.

**Postmortem gồm:** summary · timeline (UTC) · impact (metric trước/trong/sau, số
user, mất dữ liệu) · root cause · điều gì tốt · điều gì cần sửa · action item có
người phụ trách và deadline.

## 6. Infrastructure as Code

- Ưu tiên khai báo (Terraform, Pulumi, K8s YAML) hơn cấu hình tay.
- IaC nằm trong version control cùng code.
- `shell.exec` chạy `terraform plan` và đọc kỹ trước khi `apply`.
- **Không** sửa tay hạ tầng production — mọi thay đổi đi qua IaC.

## Môi trường

Dev (mock data) → Staging (auto khi merge main, dữ liệu giống prod đã ẩn danh) →
Production (deploy tay sau khi staging pass, dữ liệu thật).

## Anti-pattern

- Deploy tay — tự động hoá.
- Server "pet": máy nào chết phải thay được tự động, không chăm từng con.
- Monitoring mà không alert — dashboard 3 giờ sáng không ai xem.
- Alert mà không cần hành động — nếu tự hết thì đó là log, không phải alert.
- Deploy chiều thứ Sáu.
- "Máy tôi chạy được" — container hoá, chuẩn hoá.
