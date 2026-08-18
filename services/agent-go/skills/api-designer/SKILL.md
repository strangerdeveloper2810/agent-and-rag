---
name: api-designer
description: API design — RESTful principles, GraphQL schema, endpoint design, error handling, rate limiting, versioning, OpenAPI specs
when_to_use: When the user needs to design a new API, review an existing API design, or create specifications for endpoints and data contracts
triggers: [thiết kế api, thiet ke api, design api, api design, rest api, graphql schema, openapi, swagger, thiết kế endpoint, thiet ke endpoint]
tools: [file.read, file.write]
---

# API Designer Skill

## Nguyên tắc vàng

**Thiết kế cho người DÙNG API, không cho người viết nó.** Mọi quyết định lọc qua
câu: "người lần đầu đọc endpoint này có đoán đúng không?"

1. **Nhất quán** — một chỗ dùng `snake_case` thì mọi chỗ dùng `snake_case`; một
   format lỗi cho toàn bộ API.
2. **Đoán được** — dev chưa từng thấy endpoint vẫn đoán đúng cách dùng.
3. **Không gây bất ngờ** — `DELETE /users/123` là xoá, không phải archive.
4. **Suy giảm mềm** — trả kết quả một phần thay vì fail toàn bộ; thêm field không
   được làm vỡ client cũ.
5. **Tự tài liệu** — URL, tên field, status code tự kể câu chuyện.

## Resource naming

- Danh từ số nhiều cho collection: `/users`, không `/user`.
- Kebab-case cho tên nhiều từ: `/battle-damage-reports`.
- Không động từ trong URL (`/users` không `/getUsers`), trừ action không phải
  CRUD: `POST /suits/:id/activate`.
- Không dấu `/` ở cuối. Sub-resource: `/users/:id/suits`.
- `PUT` = thay toàn bộ, `PATCH` = sửa một phần.

## Status code — chỗ hay dùng sai

201 POST thành công (kèm `Location`) · 202 nhận rồi, xử lý async · 204 DELETE
thành công không body · 401 chưa xác thực vs **403 đã xác thực nhưng không có
quyền** · 409 xung đột trạng thái · 422 đúng cú pháp sai nghĩa · 429 vượt rate
limit · 500 **không bao giờ** lộ chi tiết nội bộ.

## Request / response

- Chọn MỘT kiểu đặt tên field (`snake_case` hoặc `camelCase`) và giữ suốt.
- Không prefix tên field bằng tên resource (`user_name` trong `/users` là dư).
- Enum là **string**, không phải số (`"role": "pilot"`, không `"role": 3`).
- Phẳng hơn là lồng — không quá 2 tầng.
- Timestamp theo RFC 3339: `"2026-07-24T14:30:00Z"`.
- Response luôn có `id`; resource có thể sửa thì thêm `created_at` + `updated_at`.
- Không lộ ID nội bộ, không lộ field nhạy cảm (hash mật khẩu, ghi chú nội bộ).

## Pagination

- **Cursor** (`?cursor=abc&limit=20`) — **khuyến nghị**: ổn định khi dữ liệu đang
  thay đổi, tốt cho tập lớn.
- **Offset** (`?offset=0&limit=20`) — đơn giản, nhưng nhảy/lặp item khi dữ liệu đổi.
- **Page** (`?page=1&per_page=20`) — thân thiện cho UI.

Response kèm khối `pagination` có tổng số và link `next`/`prev`.

## Filter / sort / search

`GET /suits?status=active&sort=-created_at&q=thruster`

Sort tăng `?sort=field`, giảm `?sort=-field`. Full-text `?q=`. Luôn ghi rõ field
nào cho phép filter/sort.

## Format lỗi

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid request parameters.",
    "details": [
      { "field": "email", "issue": "invalid_format", "message": "Must be a valid email address." }
    ],
    "request_id": "req_abc123"
  }
}
```

Luôn có `code` (máy đọc) + `message` (người đọc) + `request_id` (đối chiếu log);
`details` cho lỗi validate theo từng field. Không lộ stack trace.

## Versioning

- URL path `/v1/users` — **khuyến nghị** cho API public (rõ, dễ test).
- Header `Accept: version=1` — URL sạch, dùng cho API nội bộ.
- Query `?version=1` — tránh.

Chỉ bump version khi có breaking change; đỡ version N-1 trong thời gian deprecate
(vd 6 tháng); báo qua header `Deprecation` + `Sunset`. Không phát hành API mà
không có version, kể cả `v1`. Thêm một field **bắt buộc** cũng là breaking change.

## Rate limit

Trả header `RateLimit-Limit`, `RateLimit-Remaining`, `RateLimit-Reset`, và
`Retry-After` khi đã chặn. Vượt hạn mức → `429`.

## Auth

API key cho service-to-service (`Authorization: Bearer` hoặc `X-API-Key`), JWT
cho API của user, OAuth 2.0 cho bên thứ ba theo flow chuẩn. **Không bao giờ**:
key trong URL, basic auth không TLS, tự phát minh scheme auth. Mọi response có
security header (HSTS, `nosniff`, `X-Frame-Options: DENY`, CSP).

## GraphQL

Connection kiểu Relay cho list phân trang · mutation dùng input type trả payload
type · đánh `!` ở field chắc chắn có · description cho mọi type/field · giới hạn
độ sâu và độ phức tạp query để chống abuse.

## OpenAPI

Khi user xin spec: sinh OpenAPI **3.1** đầy đủ — paths, parameters, schema
request/response, security scheme, ví dụ từng endpoint.

## Checklist review

- [ ] Resource số nhiều, kebab-case, không động từ
- [ ] HTTP method đúng nghĩa (GET an toàn, PUT idempotent)
- [ ] Lỗi theo format chuẩn, có `request_id`
- [ ] Mọi endpoint list đều có pagination
- [ ] Có versioning
- [ ] Rate limit được định nghĩa và thông báo qua header
- [ ] Tên field nhất quán một kiểu
- [ ] Timestamp RFC 3339, ID nhất quán một loại
- [ ] Có security header
- [ ] Spec có ví dụ request/response

## Anti-pattern

- GET làm thay đổi state — không bao giờ.
- Dùng POST cho mọi thứ; method mang ý nghĩa, đừng bỏ.
- Lồng sâu: `/users/1/suits/2/missions/3/logs/4` — tách endpoint.
- Bẫy boolean: `?active=true` thì `?active=false` nghĩa là gì? Dùng `?status=`.
- Mọi thứ trả 200 — 201/204 mang thông tin.
- URL kiểu RPC: `/api/getUserById` là RPC đội lốt REST.
