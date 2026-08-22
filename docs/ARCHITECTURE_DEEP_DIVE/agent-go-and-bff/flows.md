# Trace 3 flow thật (end-to-end)

← [Về mục lục](./README.md)

## 1. Chat (`POST /api/conversations/:id/chat`)

1. BFF: `authGuard` → `tenantId`. Lưu message user vào Mongo (`messages`) TRƯỚC khi mở SSE (lỗi validate/DB còn trả JSON thường được).
2. BFF: `reply.hijack()`, mở SSE thô, gọi `goAgentClient.stream(history, {tenantId, lang, ...})`.
3. agent-go: `TenantMiddleware` đọc `X-Tenant-ID` → context. `Orchestrator.Run`: kiểm tra sticky agent (nếu là reply `ask_user`) → chọn `Engine` đúng agent.
4. `Engine`: recall → summarize → model (gọi LLM qua fallback chain hoặc RouterProvider) → tools (nếu có tool_calls, chạy song song) → lặp lại model → ... → extract → END. Mỗi bước phát `Event` (text/tool_start/tool_end/done) qua channel, đồng thời checkpoint state vào SQLite sau mỗi lần chuyển node (xem [`agent-go-resilience.md`](./agent-go-resilience.md)).
5. agent-go stream `Event` → BFF forward nguyên văn thành `data: {...}\n\n` → browser render real-time.
6. Sau khi response xong: BFF lưu message assistant vào Mongo; agent-go xoá checkpoint (`NodeEnd` tự nhiên) và (độc lập, không chờ) spawn goroutine `Learner.LearnFromConversation` — không block response user; nếu `ENABLE_COST_LEDGER=true`, ghi thêm 1 dòng cost ledger.

## 2. Suggestions (`GET /api/suggestions`) — ví dụ cụ thể cho hợp đồng tenant

1. FE gọi `api.get("/api/suggestions")` (cookie session, qua BFF — **không** gọi thẳng agent-go).
2. BFF: `authGuard` → `tenantId` → `chat.controller.getSuggestions` → `go-agent.client.getSuggestions(tenantId)` → `GET {AGENT_GO_URL}/suggestions` kèm `X-Tenant-ID: <tenantId>`.
3. agent-go: đọc tenant từ context, tự query Mongo (`RecentUserMessages`, collection `messages` — đọc thứ BFF ghi) + facts (`Store.All(tenantID)`) + thời gian thật (`time.Now()`), dựng 1 prompt cá nhân hoá, gọi LLM 1 lượt (`MaxSteps: 1`), parse JSON `[{text, category}]`.
4. Trả về BFF → FE. FE lọc theo tab đang chọn ở **client-side** (không gọi lại LLM mỗi lần đổi tab — tôn trọng ngân sách LLM hẹp của dự án).

*(Trước khi fix: FE gọi thẳng agent-go, không header nào → tenant luôn `"default"`; prompt hoàn toàn tĩnh (không time/history/facts) → gợi ý quanh quẩn vài chủ đề bất kể user/thời điểm.)*

## 3. Resume (`POST /api/conversations/:id/resume` → `POST /chat/resume`)

1. 1 run đang chạy dừng giữa chừng — vì `NodeInterrupt` (agent chủ động hỏi lại user qua tool `ask_user`) HOẶC vì crash/restart process ở bất kỳ node nào khác.
2. `Engine` đã checkpoint state (kèm `RunID`, `agent_name`) vào SQLite `paused_runs` từ trước đó (checkpoint sau mỗi lần chuyển node — xem [`agent-go-resilience.md`](./agent-go-resilience.md)); event SSE `interrupt` (nếu có) mang thêm `runId` — forward nguyên vẹn qua BFF (`packages/types` `ChatEvent`) để FE biết dùng `runId` nào.
3. FE gọi `POST /api/conversations/:id/resume {runId, answer}` (`:id` = conversationId, chỉ dùng để biết lưu response vào hội thoại nào — KHÔNG phải `runId`). BFF: `authGuard` → `tenantId` → `chat.controller.postResume` → `chatService.resumeReply` → `goAgentClient.resume(runId, answer, {tenantId})` → `POST {AGENT_GO_URL}/chat/resume {run_id, answer}` kèm `X-Tenant-ID`.
4. `internal/transport/http.NewChatResumeHandler` load lại state theo `run_id` + `agent_name` (tra đúng `Engine` trong `Orchestrator`), gọi `ResolveInterrupt` nếu cần (điền `answer`), xoá checkpoint, gọi `Engine.Resume` chạy tiếp đúng từ node đã dừng — trả SSE giống hệt `/chat`.
5. BFF forward SSE về FE giống `/chat`; response mới được NỐI vào cuối assistant message hiện có qua `appendToLastAssistantMessage` (giống cơ chế `continueReply`) — không tạo message mới, vì phần text đã sinh trước khi interrupt (nếu có) đã được lưu thành 1 message ở lượt gốc.

**Giới hạn còn lại**: BFF đã có proxy đầy đủ, nhưng **frontend (`apps/web`) chưa có UI** để hiển thị câu hỏi interrupt và gọi `/resume` — hiện tại `runId` đã có mặt trong event SSE (type-safe qua `ChatEvent`) nhưng `ChatPage.tsx` mới chỉ dùng event `interrupt` để hiện toast cảnh báo (xem `case "interrupt"`), chưa lưu `runId` hay hiển thị form trả lời. Đây là 1 tính năng UI riêng cần thiết kế (dialog xác nhận/trả lời câu hỏi), chưa làm.
