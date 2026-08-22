# agent-go — Kênh giao tiếp: HTTP/SSE (chính) + Telegram (mới)

← [Về mục lục](./README.md)

## HTTP/SSE — kênh chính, đi qua BFF

`POST /chat` là kênh chính, LUÔN đi qua BFF (xem [`bff.md`](./bff.md) và [`flows.md`](./flows.md)) — tenant đến từ header `X-Tenant-ID` do BFF gắn sau khi xác thực JWT. Đây là kênh duy nhất frontend React dùng.

## Telegram — kênh mới, KHÔNG đi qua BFF

`internal/transport/telegram` — bot long-polling thuần `net/http` (không SDK Telegram ngoài), bật khi `TELEGRAM_BOT_TOKEN` khác rỗng. Khởi động như 1 goroutine trong `cmd/server/main.go`, chạy song song với HTTP server, dùng CHUNG orchestrator (`orch`) đã dựng cho `/chat` — không có engine/wiring riêng.

**Điểm khác biệt quan trọng với kênh HTTP**: Telegram KHÔNG đi qua BFF, KHÔNG có JWT — tenant được TỰ GÁN bằng `"telegram:" + chatID` (qua `middleware.WithTenantID`). Nhờ tái dùng đúng cơ chế multi-tenant sẵn có, hệ quả là:

- Mỗi user Telegram tự động bị cô lập trong tenant riêng của họ (không đọc/ghi chéo memory/file với user Telegram khác).
- Tool đặc quyền (`shell.exec`, `file.*`, `git`) tự động bị chặn cho MỌI user Telegram, vì `telegram:<chatID>` không thể nào nằm trong `OWNER_TENANT_IDS` trừ khi admin cố tình thêm — không cần code chặn riêng cho kênh này.

**Giới hạn đã biết**:
- Lịch sử hội thoại giữ trong RAM (`map[int64][]provider.Message`, có mutex) — mất khi restart process, không persist SQLite/Mongo (chấp nhận cho v1).
- Chỉ hỗ trợ long-polling, không có webhook — đơn giản hơn (không cần domain/SSL công khai) nhưng tốn 1 HTTP connection giữ mở liên tục.
- Không streaming từng chữ về Telegram — gom hết response rồi gửi 1 lần (hoặc nhiều chunk nếu > 4096 ký tự, giới hạn của Telegram `sendMessage`).
- Không xử lý update khác ngoài text message (ảnh, document, callback_query bị bỏ qua im lặng).
