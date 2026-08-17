# OTP Email Verification + Hoàn Thiện Tenant Isolation — Design

> **Date:** 2026-08-17
> **Mở rộng của:** `docs/plans/2026-07-25-auth-tenant-design.md` (giữ nguyên toàn bộ kiến trúc auth: Fastify + PostgreSQL + bcrypt + JWT trong httpOnly cookie, 1 user = 1 tenant, `tenantId = user.id`)
> **Quyết định:** Thêm bước OTP verify email vào flow register/login; hoàn thiện 2 phần đã lên kế hoạch nhưng chưa triển khai ở thiết kế cũ (documents tenant filter, X-Tenant-ID sang agent-go)

---

## 1. Bối cảnh

Thiết kế `2026-07-25-auth-tenant-design.md` đã định nghĩa kiến trúc auth + multi-tenant đầy đủ và phần lớn đã được triển khai (Phase 1-4, 7). Tuy nhiên khảo sát code hiện tại (2026-08-17) cho thấy 2 phần trong plan cũ **chưa từng được hoàn thiện**:

1. **Phase 5.2 (documents tenant filter)** — module `documents` (Mongo, RAG document store) hoàn toàn không lọc theo `tenantId` ở bất kỳ query nào (`listDocuments`, `searchSimilar`, `getDocumentContent`...). Mọi user đã login đang thấy chung 1 kho tài liệu. Module `upload` (khác `documents`) thì đã làm đúng qua `getTenantId(req)`.
2. **Phase 6 (Go Agent nhận X-Tenant-ID)** — BFF (`apps/api`) hiện không gửi header `X-Tenant-ID` khi gọi sang `agent-go`, nên mọi request RAG search bên Go đều rơi vào tenant `"default"` (middleware Go có fallback này).

Ngoài ra, flow register hiện tại **chưa có xác minh email**: user nhập email/password/tên → được cấp token ngay lập tức, không có bước OTP.

Việc hôm nay gồm 2 phần độc lập nhưng làm chung một lượt vì cùng đụng vào module `auth`:

- **A. OTP email verification** (tính năng MỚI): register → gửi OTP qua email → user nhập OTP để verify → mới được cấp token/login.
- **B. Hoàn thiện tenant isolation còn dang dở** (nợ kỹ thuật từ thiết kế cũ): fix `documents` module + gắn `X-Tenant-ID` cho `agent-go`.

Tenant model giữ nguyên như thiết kế cũ: **1 user = 1 tenant**, `tenantId = user.id`, không có bảng `tenants` riêng.

---

## 2. Quyết định thiết kế (chốt qua brainstorm)

| Quyết định | Lựa chọn |
|---|---|
| OTP hết hạn | 10 phút |
| OTP sai | Khoá sau 5 lần sai, phải bấm gửi lại |
| Cooldown resend | 2 phút |
| Re-register khi chưa verify | Cho phép, ghi đè OTP + password cũ |
| Nơi lưu OTP | Redis (đã có sẵn `ioredis` trong repo), không thêm bảng Postgres |
| Email provider | Resend |
| UX verify | Trang riêng `/verify-email` (không multi-step trên cùng RegisterPage) |
| Login khi chưa verify | Chặn (403), tự động gửi OTP mới, redirect sang `/verify-email` |
| Scope documents tenant filter | Có, làm luôn trong đợt này |

---

## 3. Data Model

### 3.1 PostgreSQL — migration mới

```sql
-- 002-add-email-verification.sql
ALTER TABLE users ADD COLUMN email_verified BOOLEAN NOT NULL DEFAULT false;
```

Không cần cột `tenant_id` — `tenantId` luôn suy ra từ `user.id` (đúng pattern `auth.guard.ts` hiện tại).

### 3.2 Redis — OTP state (dùng `cacheSet`/`cacheGet`/`cacheDel` sẵn có)

| Key | Value | TTL | Mục đích |
|---|---|---|---|
| `otp:{email}` | JSON `{ codeHash: string, attempts: number }` | 600s (10p) | Lưu OTP hiện hành (hash SHA-256, giống pattern refresh-token) |
| `otp:{email}:cooldown` | `"1"` (chỉ cần tồn tại) | 120s (2p) | Chặn spam nút "gửi lại" |

Không cần job dọn dẹp — Redis tự xoá khi hết TTL.

### 3.3 MongoDB — thêm `tenantId` vào documents

`DocChunkDoc` và `DocVersionDoc` thêm field `tenantId: string`. Index mới: `{ tenantId: 1, documentId: 1 }`.

---

## 4. Backend — Auth Flow

### 4.1 `POST /api/auth/register`

```
Body: { email, password, name }

1. Validate DTO (zod, như hiện tại)
2. Tìm user theo email:
   - Có, email_verified = true  → 409 Conflict "Email đã tồn tại"
   - Có, email_verified = false → UPDATE name + password_hash (ghi đè), dùng lại user.id cũ
   - Không có                    → INSERT user mới (email_verified mặc định false)
3. Sinh OTP 6 số (crypto.randomInt), hash SHA-256, lưu Redis (otp:{email}, TTL 600s, attempts=0)
   set cooldown key (otp:{email}:cooldown, TTL 120s)
4. Gửi email OTP qua Resend (sendOtpEmail)
5. Response 201 { email } — KHÔNG set cookie, KHÔNG issueTokens
```

### 4.2 `POST /api/auth/verify-email`

```
Body: { email, otp }

1. Đọc otp:{email} từ Redis. Không có/hết hạn → 400 "OTP đã hết hạn, vui lòng gửi lại"
2. attempts >= 5 → 400 "OTP không hợp lệ, vui lòng gửi lại" (coi như đã khoá)
3. So sánh SHA256(otp) với codeHash:
   - Sai → tăng attempts trong Redis (giữ nguyên TTL còn lại), 400 "OTP không đúng"
     Nếu attempts vừa chạm 5 → xoá otp:{email} luôn (bắt buộc resend)
   - Đúng → xoá otp:{email} + otp:{email}:cooldown
            UPDATE users SET email_verified = true
            issueTokens(user) → set cookie (giống login hiện tại)
            Response 200 { user }
```

### 4.3 `POST /api/auth/resend-otp`

```
Body: { email }

1. Tìm user theo email. Không có → 404 (chấp nhận lộ thông tin tồn tại email vì
   đây là dự án cá nhân quy mô nhỏ, không cần chống user-enumeration)
2. email_verified = true → 400 "Email đã được xác minh"
3. Còn cooldown → 429 { secondsRemaining }
4. Sinh OTP mới (ghi đè), reset attempts = 0, set cooldown mới
5. Gửi email → 200 { message: "OTP đã được gửi lại" }
```

### 4.4 `POST /api/auth/login` — sửa

```
... (giữ nguyên check email/password như hiện tại) ...
Sau khi bcrypt.compare thành công:
  - user.email_verified === false:
      - Không còn cooldown → sinh OTP mới + gửi email (như resend-otp)
      - Còn cooldown → không gửi thêm (OTP trước vẫn còn hiệu lực)
      - Response 403 { code: "EMAIL_NOT_VERIFIED", email: user.email }
  - user.email_verified === true → issueTokens như hiện tại, 200 { user }
```

---

## 5. Backend — Email Service (Resend)

Module mới `apps/api/src/common/email/`:
- `resend.client.ts` — khởi tạo `Resend` SDK với `RESEND_API_KEY`
- `email.service.ts` — `sendOtpEmail(to: string, name: string, otp: string): Promise<void>`, template HTML đơn giản (dark bg, amber accent theo theme Luxury Dark), log lỗi nếu Resend fail nhưng KHÔNG throw chặn luồng register (user vẫn có thể bấm "gửi lại" sau)

Config mới trong `.env`/`config.ts`: `RESEND_API_KEY`, `EMAIL_FROM` (vd `"JARVIS <onboarding@resend.dev>"` khi dev — domain test của Resend chỉ gửi được tới email đã verify trong dashboard Resend khi ở chế độ chưa xác minh domain riêng).

---

## 6. Backend — Hoàn thiện Tenant Isolation

### 6.1 `documents` module

Thêm `tenantId` vào mọi query trong `documents.repository.ts` (list, search, get content, create, update, delete) — copy pattern từ `getTenantId(req)` đã dùng đúng trong module `upload`. Ghi `tenantId` khi tạo document mới; migrate dữ liệu cũ (nếu có) gán `tenantId = "default"` để không mất dữ liệu hiện tại trong môi trường dev.

### 6.2 Go Agent — nhận `X-Tenant-ID`

Client gọi sang Go agent (BFF) thêm header `X-Tenant-ID: req.tenantId` cho mọi request `/chat`. Bên Go, middleware `TenantMiddleware` đã đọc header này sẵn (không cần sửa Go) — chỉ cần BFF gửi đúng.

---

## 7. Frontend

- `RegisterPage.tsx`: bỏ set-user-ngay-sau-submit; thành công → `navigate('/verify-email?email=' + encodeURIComponent(email))`
- `VerifyEmailPage.tsx` (mới): đọc `email` từ query param (read-only hiển thị), ô nhập OTP 6 số, nút "Xác minh", nút "Gửi lại" disabled + đếm ngược 2 phút (khởi tạo từ giây trả về của server khi 429, để chịu được reload trang)
- `LoginPage.tsx`: bắt response `403 EMAIL_NOT_VERIFIED` → toast "Vui lòng xác minh email, mã OTP mới đã được gửi" → `navigate('/verify-email?email=...')`
- Auth store (Zustand): `register()` không set `user` nữa (chỉ trả về, không đăng nhập); thêm `verifyEmail(email, otp)` và `resendOtp(email)`
- Route mới: `/verify-email` (public, giống `/login`, `/register`)

---

## 8. Rủi ro & Edge Case

- **Resend domain chưa verify**: khi dev, dùng domain test `resend.dev` — chỉ gửi được tới email đã thêm vào danh sách test của tài khoản Resend. Cần lưu ý khi test thật với email bất kỳ.
- **Email gửi thất bại**: register vẫn tạo user (email_verified=false), không rollback — user có thể bấm "gửi lại" để thử lại.
- **Đồng thời register 2 lần rất nhanh**: cooldown key Redis tự nhiên chặn tạo OTP mới liên tục.
- **Dữ liệu documents cũ (nếu có) không có tenantId**: cần migrate gán `tenantId = "default"` khi thêm field, tránh mất quyền truy cập dữ liệu cũ.

---

## 9. Ngoài phạm vi (không làm đợt này)

- Chưa hỗ trợ nhiều user trong 1 tenant (mời thành viên, phân quyền) — giữ đúng quyết định "1 user = 1 tenant".
- Chưa chống user-enumeration ở `resend-otp`/`register` (chấp nhận được với quy mô dự án cá nhân).
- Chưa có job dọn dữ liệu documents cũ không có tenantId ngoài việc gán mặc định `"default"`.
