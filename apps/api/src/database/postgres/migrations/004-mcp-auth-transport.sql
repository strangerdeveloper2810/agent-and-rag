-- Migration: 004-mcp-auth-transport
-- Mô tả: Thêm cột auth_token cho user_mcp_servers (token xác thực remote MCP
-- server -- Notion/GitHub/Linear/Sentry... đều đòi header
-- `Authorization: Bearer <token>`). Nới CHECK transport để chấp nhận cả
-- 'http' (Streamable HTTP -- giao thức chuẩn cho MCP server remote hiện nay)
-- lẫn 'sse' (giá trị cũ, giữ để tương thích ngược với dữ liệu đã tạo trước
-- migration này), default chuyển sang 'http' cho server mới tạo.
--
-- QUAN TRỌNG: migration runner (postgres.module.ts runMigrations) REPLAY
-- toàn bộ migration mỗi lần khởi động app -- MỌI câu lệnh dưới đây phải chạy
-- lại được nhiều lần mà không lỗi (idempotent).

-- 1. Thêm cột auth_token. Cột api_key cũ (migration 003) được GIỮ NGUYÊN tại
--    DB (không DROP) để không phá dữ liệu hiện có, nhưng kể từ migration này
--    ứng dụng (repository/service/controller) không còn đọc/ghi cột api_key
--    nữa -- auth_token là nguồn sự thật duy nhất cho token từ nay.
--
--    LƯU Ý BẢO MẬT: auth_token đang lưu PLAINTEXT tại rest (giống api_key
--    cũ). Đây là nợ kỹ thuật tạm chấp nhận để không tự chế crypto vội --
--    bước sau nên mã hoá cột này (vd pgcrypto hoặc KMS phía ứng dụng) trước
--    khi dùng với dữ liệu thật/production.
ALTER TABLE user_mcp_servers ADD COLUMN IF NOT EXISTS auth_token TEXT DEFAULT NULL;

-- 2. Backfill: nếu server nào đã có api_key từ trước mà auth_token chưa có
--    giá trị, copy sang để không "mất" token khi nâng cấp lên schema mới.
--    An toàn để chạy lại nhiều lần: lần chạy sau auth_token đã có giá trị
--    nên điều kiện `auth_token IS NULL` không còn đúng -- không ghi đè lần
--    nữa.
UPDATE user_mcp_servers
SET auth_token = api_key
WHERE auth_token IS NULL AND api_key IS NOT NULL;

-- 3. Nới CHECK transport: cho phép 'http' (Streamable HTTP) và 'sse'
--    (legacy). Postgres không có "ADD CONSTRAINT IF NOT EXISTS" nên phải
--    DROP rồi ADD lại -- cách này idempotent (chạy lại nhiều lần vẫn ra
--    đúng 1 constraint).
ALTER TABLE user_mcp_servers DROP CONSTRAINT IF EXISTS user_mcp_servers_transport_check;
ALTER TABLE user_mcp_servers ADD CONSTRAINT user_mcp_servers_transport_check
    CHECK (transport IN ('http', 'sse'));

-- 4. Default transport cho server mới tạo chuyển sang 'http'.
ALTER TABLE user_mcp_servers ALTER COLUMN transport SET DEFAULT 'http';
