# Mô hình dữ liệu — MongoDB **CHUNG** giữa BFF và agent-go, cộng 2 kho riêng của agent-go

← [Về mục lục](./README.md)

Đây là điểm dễ hiểu nhầm nhất: BFF (Node) và agent-go (Go) **không** có database riêng cho phần lõi — cả 2 trỏ tới **cùng 1 MongoDB** (`MONGODB_URI`/`MONGODB_DB` giống nhau trong `env/.env`, cùng `docker-compose.yml`). Ai ghi/đọc collection nào:

| Collection | Ghi bởi | Đọc bởi | Ghi chú |
|---|---|---|---|
| `conversations`, `messages` | **BFF** (`chat.repository.ts`) | **BFF** (hiển thị lịch sử) + **agent-go** (đọc `RecentUserMessages` để cá nhân hoá `/suggestions`) | agent-go chỉ ĐỌC, không bao giờ ghi 2 collection này. |
| `documents` | **BFF** (ingest: upload/update) | **agent-go** (tool `rag.search`/`rag.read`/`rag.list`) | Schema Go (`internal/mongo/models.go` struct `DocChunk`) phải tự giữ khớp tay với schema TS (Zod) — không có migration chung. |
| `document_versions` | BFF (archive khi update) | — | Chỉ BFF dùng, agent-go không chạm. |
| `tasks` | **agent-go** (qua tool CRUD) | **BFF** (`GET /api/tasks`, chỉ đọc để debug/hiển thị) | Ngược hướng với `documents`: agent-go SỞ HỮU, BFF chỉ đọc. |
| `memories` | **agent-go** (`internal/memory.Learner`) | **agent-go** | Facts đã học (autonomous learner) + knowledge items — BFF không đụng. |
| `user_mcp_servers` | **BFF** (Postgres, không phải Mongo) | **agent-go** (đọc runtime khi build danh sách MCP tool remote của user) | Lưu ở **Postgres**, khác các dòng trên (Mongo) — user tự thêm remote MCP server qua Settings UI (`modules/users`, `dto/mcp-server.dto.ts`). |

**Vì sao cùng 1 Mongo**: tránh đồng bộ 2 database, và cho phép agent-go đọc trực tiếp dữ liệu BFF ghi ra (RAG documents, lịch sử hội thoại cho personalization) mà không cần một API nội bộ riêng để "hỏi mượn" dữ liệu. Đánh đổi: schema phải định nghĩa 2 lần (Go struct + Zod/TS interface) và tự giữ đồng bộ tay — không có migration tool chung giữa 2 ngôn ngữ.

---

## Kho dữ liệu RIÊNG của agent-go (không chia sẻ với BFF)

Ngoài Mongo chung ở trên, agent-go còn có **SQLite cục bộ** (`JARVIS_DB_PATH`, mặc định `jarvis.db`) hoàn toàn riêng — BFF không biết, không đọc, không ghi:

| Bảng | Mục đích | Ghi/đọc bởi | Ghi chú |
|---|---|---|---|
| `paused_runs` | Checkpoint/resume — state của 1 run đang dừng giữa chừng (interrupt HITL hoặc crash) | `Engine.SetInterruptStore` (ghi mỗi lần chuyển node), `POST /chat/resume` (đọc + xoá sau khi resume xong) | Xem [`agent-go-resilience.md`](./agent-go-resilience.md). Đây là lý do resume KHÔNG cần Mongo — tự đủ trong 1 file SQLite cục bộ của chính process agent-go. |
| `cost_ledger` | Chi phí ước tính mỗi lượt gọi LLM, gắn theo tenant | `Engine.SetCostLedger` (ghi, chỉ khi `ENABLE_COST_LEDGER=true`), CLI `jarvis cost <tenantID>` (đọc) | Xem [`agent-go-providers.md`](./agent-go-providers.md). TẮT mặc định — side-effect ghi thêm cho mọi request nên phải opt-in. |
| `conversations`/`messages` (SQLite, khác Mongo) | Bản offline/local cho dev không cần Mongo Atlas | `internal/storage/sqlite` | KHÔNG phải cùng bảng với Mongo `conversations`/`messages` ở BFF — đây là 2 kho tách biệt hoàn toàn, chỉ trùng tên khái niệm, dùng cho 2 mục đích khác nhau (BFF: nguồn thật cho production; SQLite agent-go: dev/offline). |

**Vì sao tách riêng thay vì dùng chung Mongo**: `paused_runs` và `cost_ledger` là dữ liệu **nội bộ vận hành của agent-go**, không phải dữ liệu nghiệp vụ mà BFF/frontend cần hiển thị trực tiếp — dùng SQLite cục bộ tránh phụ thuộc Mongo Atlas còn sống hay không mỗi khi checkpoint, và tránh phải định nghĩa thêm schema Zod phía BFF cho thứ BFF không bao giờ đọc.
