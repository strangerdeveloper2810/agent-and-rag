# BFF (`apps/api`) — vai trò thật, không phải "agent"

← [Về mục lục](./README.md)

Sau khi tách agent ra Go, BFF chỉ còn 5 việc:

| Việc | Module | Ghi chú |
|---|---|---|
| **Auth** | `modules/auth`, `common/guards/auth.guard.ts` | JWT (access + refresh cookie httpOnly) + Google OAuth + OTP email (Resend). `authGuard` decode JWT (`jwt.verify`) → `req.tenantId = payload.sub`, set trước khi chạm controller. |
| **CRUD hội thoại** | `modules/chat` (trừ phần chat/suggestions) | `conversations`/`messages` (Mongo) — BFF **sở hữu**, ghi trực tiếp. |
| **RAG ingest** | `modules/documents` | Upload → extract text → chunk → embed (Voyage) → lưu `documents` (Mongo, có vector search Atlas), có versioning (`document_versions`). agent-go chỉ **đọc** collection này qua tool `rag.search`/`rag.read`/`rag.list`. |
| **File upload** | `modules/upload`, `common/storage` | MinIO/S3, dùng cho ảnh/file đính kèm chat + tài liệu RAG. |
| **Proxy sang agent-go** | `agent/client/go-agent.client.ts` | `stream()` (chat SSE), `getSuggestions()`, `testMcpConnection()`, `checkGoAgentHealth()` — MỌI lời gọi đều tự thêm `X-Tenant-ID` từ `tenantId` đã xác thực. |

BFF **không còn** business logic "agent" nào (không prompt, không tool, không LLM call trực tiếp — trừ nhánh legacy `AGENT_BACKEND=langgraph`, xem [`docs/architecture-backend-agent.md`](../../architecture-backend-agent.md)). Nó là 1 lớp mỏng: xác thực → map path → gọi agent-go → forward kết quả.

`go-agent.client.ts` giờ có thêm hàm `resume()` (optional trên interface `AgentClient` — chỉ agent-go implement, LangGraph legacy không có khái niệm checkpoint) proxy sang `POST /chat/resume` của agent-go, dùng chung logic map SSE→AgentEvent với `stream()` qua helper `mapGoAgentEvents`. Khác `stream()`: **không retry** khi lỗi trước response, vì agent-go xoá checkpoint ngay sau khi load để resume — gọi lại lần 2 với cùng `runId` chỉ nhận lỗi "not found".

## Circuit breaker phía BFF

`go-agent.client.ts` giữ 1 circuit breaker module-level (đếm lỗi liên tiếp, mở mạch tạm thời khi agent-go down) — tách biệt hoàn toàn với circuit breaker BÊN TRONG agent-go (`guardrails.NewCircuitBreaker`, chống LLM loop — xem [`agent-go-resilience.md`](./agent-go-resilience.md)). Hai cơ chế bảo vệ 2 tầng khác nhau: một chống "agent-go không phản hồi được" (network/process), một chống "model tự lặp vô hạn trong 1 lượt chạy" (logic).

---

## Danh sách route BFF hiện có

```
POST   /api/conversations                          tạo hội thoại
GET    /api/conversations                          danh sách
GET    /api/conversations/:id/messages              tin nhắn trong 1 hội thoại
DELETE /api/conversations/:id                       xoá hội thoại
POST   /api/conversations/:id/chat        ★         CHAT (SSE → agent-go, rate-limit 20/phút)
POST   /api/conversations/:id/continue    ★         tiếp tục câu trả lời bị cắt (cùng rate-limit)
POST   /api/conversations/:id/resume      ★         resume 1 run đã dừng giữa chừng (interrupt/crash, cùng rate-limit)
GET    /api/suggestions                   ★         gợi ý mở hội thoại (proxy có tenant, rate-limit 20/phút)

POST   /api/documents/upload              ★         upload mới (.txt/.md/.pdf, rate-limit 20/phút — gọi Voyage embedding)
PUT    /api/documents/:documentId         ★         cập nhật → version mới (cùng rate-limit)
GET    /api/documents                               danh sách (kèm version)
GET    /api/documents/:documentId/versions           lịch sử version của 1 tài liệu
GET    /api/documents/:documentId/versions/:version  nội dung 1 version cụ thể
DELETE /api/documents/:documentId                   xoá (cả lịch sử)

GET    /api/tasks                                   debug: xem task agent tạo (chỉ đọc)
GET    /api/health                                  health check
```

(★ = gọi LLM/embedding provider ngoài qua agent-go hoặc Voyage, có `preHandler: authGuard` + rate-limit riêng chặt hơn mức toàn cục — xem `apps/api/src/modules/chat/chat.routes.ts` và `apps/api/src/modules/documents/documents.routes.ts` để xem cấu hình rate-limit chính xác.)

Cấu hình MCP server của user (Settings UI, "MCP Servers" tab) nằm trong `modules/users` (`dto/mcp-server.dto.ts`), lưu ở Postgres `user_mcp_servers` — xem [`data-model.md`](./data-model.md) và [`agent-go-memory-and-mcp.md`](./agent-go-memory-and-mcp.md) cho chi tiết cơ chế 2 chế độ MCP.
