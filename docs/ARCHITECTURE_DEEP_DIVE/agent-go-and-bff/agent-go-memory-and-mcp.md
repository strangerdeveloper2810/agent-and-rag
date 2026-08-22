# agent-go — Memory 3 tầng + Learner, và MCP (client 2 chế độ + server mới)

← [Về mục lục](./README.md)

## Memory — 3 tầng + Learner nền

- **Working**: `State.Messages` trong 1 lượt chạy.
- **Episodic**: tóm tắt khi hội thoại dài (node `summarize`).
- **Semantic**: `memory.Store` (in-memory, nạp lại từ Mongo `memories` lúc khởi động) — facts sống xuyên hội thoại, theo tenant.

**Learner** chạy NGẦM sau mỗi lượt chat (`LearnFromConversation`, không block response): gọi 1 lượt LLM riêng (provider riêng, xem [`agent-go-providers.md`](./agent-go-providers.md)) để trích fact/knowledge item mới. Có 2 lớp giảm chi phí:

1. **Gate** `worthLearning()`: chỉ học nếu câu user có keyword gợi ý fact HOẶC đủ dài — bỏ điều kiện cũ "assistant trả lời dài" (gần như luôn đúng, làm gate vô hiệu).
2. **Batch theo N lượt** (`SetBatchTurns`, default 3 qua `REFLECTION_BATCH_TURNS`): gộp N lượt mới thực sự gọi LLM 1 lần, thay vì mỗi lượt gọi 1 lần. Cửa sổ tin nhắn đưa vào prompt co giãn theo **số lượt RAW đã trôi qua kể từ lần reflect trước** (kể cả lượt bị gate chặn) — không dùng thẳng N, để không bỏ sót nội dung khi có lượt tán gẫu xen giữa.

---

## MCP — 3 chế độ, khác hẳn nhau

JARVIS vừa là **MCP client** (2 chế độ) vừa là **MCP server** (mới) — 3 mô hình hoàn toàn độc lập, đừng nhầm lẫn.

### Client — subprocess (admin)

| | Mô tả |
|---|---|
| Cấu hình | YAML tĩnh, chỉ admin sửa được |
| Lưu ở | file YAML |
| Cơ chế | spawn subprocess, JSON-RPC qua stdio |
| Rủi ro | Cao (chạy lệnh trên máy chủ) → chỉ admin |
| Namespacing tool | `mcp__<server>__<tool>` |

### Client — remote user (Streamable HTTP)

| | Mô tả |
|---|---|
| Cấu hình | User tự thêm qua Settings UI ("MCP Servers" tab, `apps/api` `modules/users`) |
| Lưu ở | Postgres `user_mcp_servers` (+ `auth_token` — **plaintext tại rest**, nợ kỹ thuật đã biết) |
| Cơ chế | HTTP POST tới URL user cung cấp, `Authorization: Bearer <token>` |
| Rủi ro | Thấp hơn (chỉ gọi ra ngoài) → mọi user |
| Namespacing tool | `mcp__<server>__<tool>` (cùng quy ước, tránh đụng tên) |

### Server (mới) — JARVIS expose tool CỦA MÌNH cho client khác

Đây là chiều **NGƯỢC LẠI** 2 mô hình trên: thay vì JARVIS gọi ra MCP server khác, giờ JARVIS TỰ LÀ 1 MCP server để Claude Desktop/IDE/1 JARVIS instance khác gọi vào (`internal/mcp/server.go`).

- **Transport**: "Streamable HTTP" (spec `modelcontextprotocol.io`, bản `2025-06-18`) — nhánh đơn giản nhất: mỗi JSON-RPC request là 1 HTTP POST, server trả về đúng 1 response JSON (không mở SSE stream, vì JARVIS không cần đẩy notification chủ động).
- **Endpoint**: `POST /mcp` trong `cmd/server/main.go`.
- **Tool expose**: whitelist tối giản, dựng RIÊNG (không tái dùng registry `code`/`general`): `calculator, datetime, echo, version, web.search, web.fetch, notes.search, notes.create`.
- **Bảo mật — điểm quan trọng nhất**: đây là đường vào MỚI, hoàn toàn KHÔNG đi qua `internal/agent/node_tools.go` (nơi chặn tool đặc quyền theo owner-tenant cho kênh `/chat` bình thường). Vì MCP server không có khái niệm "owner tenant" nào, `Server.allowed(name)` **hard-exclude tuyệt đối** `tools.IsPrivilegedTool` (`shell.exec`, `file.*`, `git`) khỏi cả `tools/list` lẫn `tools/call` — không có ngoại lệ, không có cấu hình bật lại, và KHÔNG tin caller filter truyền vào (defense-in-depth: dù filter cho phép, Server vẫn tự chặn lại).
- **Auth**: không có khái niệm owner-tenant như kênh chat, nên tự có lớp riêng — mặc định (không set `MCP_API_KEY`) chỉ chấp nhận request từ **loopback** (127.0.0.1/::1); set `MCP_API_KEY` thì yêu cầu header `Authorization: Bearer <key>` khớp đúng (so sánh constant-time, tránh timing attack), áp dụng cho MỌI nguồn kể cả loopback nếu key sai.

Xem thêm [`docs/security-model.md`](../../../services/agent-go/docs/security-model.md) (trong `services/agent-go/`) cho phần giải thích đầy đủ về mô hình tin cậy này.

---

## Bảng so sánh nhanh 3 chế độ MCP

| | Client — subprocess | Client — remote user | **Server (mới)** |
|---|---|---|---|
| Hướng | JARVIS gọi ra | JARVIS gọi ra | Client khác gọi vào JARVIS |
| Ai cấu hình | Admin (YAML) | User (Settings UI) | Không cần cấu hình phía "client" — JARVIS tự expose |
| Auth | Không cần (local subprocess) | Bearer token do user cung cấp | Loopback mặc định, hoặc `MCP_API_KEY` |
| Rủi ro chính | Chạy lệnh trên máy chủ (chỉ admin) | Gọi ra ngoài thay mặt user | Ai gọi được `/mcp` dùng được tool non-privileged — CHƯA có auth theo user/tenant |
