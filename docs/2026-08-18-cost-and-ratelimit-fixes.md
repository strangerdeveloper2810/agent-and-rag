# Fix rate limit + tối ưu chi phí LLM — 2026-08-18

Đi kèm `docs/2026-08-18-loadtest-production-report.md`. Bản load test tìm ra
vấn đề; bản này sửa chúng.

Ba fix, mỗi fix có test hồi quy đã kiểm chứng là **fail khi chưa sửa**.

---

## Fix 1 — Rate limit: từ "20 tin/phút toàn hệ thống" thành "mỗi người một hạn mức"

**Vấn đề.** Đo trên production: 60 chat request từ một máy chỉ 19 request lọt,
41 request ăn `429 "Rate limit exceeded, retry in 55 seconds"`. Log `jarvis-api`
chỉ thấy đúng một địa chỉ cho mọi traffic Internet:

```
145 "remoteAddress":"172.18.0.7"   ← IP của nginx container
 43 "remoteAddress":"127.0.0.1"    ← healthcheck
```

`@fastify/rate-limit` khoá theo `req.ip`. Reverse proxy ngoài cùng có set
`X-Forwarded-For` đúng, nhưng **nginx trong container `jarvis-web` không chuyển
tiếp header đó**, và Fastify cũng không bật `trustProxy` → `req.ip` = IP container
với mọi người. Kết quả: hạn mức 20 chat/phút là của **toàn hệ thống**. Với 100
user, người thứ 21 trong phút đó bị 429 dù chưa gửi gì.

**Sửa.**

| File | Thay đổi |
|---|---|
| `apps/web/nginx.conf` | thêm `proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;` + `X-Forwarded-Proto` cho `location /api/` |
| `apps/api/src/app.ts` | `trustProxy: 2` |
| `apps/api/src/common/guards/rate-limit-key.ts` | keyGenerator mới: user đã đăng nhập → `user:<sub>`, chưa đăng nhập → `ip:<ip>` |

Hai lựa chọn đáng giải thích:

- **`trustProxy: 2` chứ không phải `true`.** `true` = tin mọi hop, và Fastify khi
  đó lấy entry ngoài cùng bên trái của `X-Forwarded-For` — mà client tự gửi được
  header đó, nên ai cũng bơm IP giả để reset hạn mức. Đếm đúng 2 hop
  (reverse proxy → nginx container → api) thì IP lấy ra luôn là IP mà proxy đầu
  tiên **thực sự** thấy.
- **Khoá theo `sub` của JWT, và VERIFY token.** Nếu chỉ decode, ai cũng tự ký một
  `sub` bất kỳ để nhận bucket mới. Khoá theo user cũng xử lý luôn hai ca mà khoá
  theo IP làm sai: cả office sau NAT dùng chung một IP, và một người đổi wifi
  sang 4G lại được cấp thêm hạn mức.

**Tác động.** Trần hệ thống chuyển từ ~20 user lên vùng ~200–300 user hoạt động
(giới hạn mới nằm ở LLM provider, không phải rate limiter), không thêm hạ tầng.

**Test.** `apps/api/src/common/guards/rate-limit-key.test.ts` — 6 case: hai user
khác bucket, cùng user đổi IP vẫn một bucket, chưa đăng nhập theo IP, token ký
sai secret không được cấp bucket riêng, token hết hạn, thiếu cookie.

---

## Fix 2 — Đếm token: hết phồng 8 lần

**Vấn đề.** Log báo `tokens_in=41400` cho một câu chat đơn giản, nhưng
`cmd/promptsize` cân payload thật thì agent chỉ gửi **~5.260 token**.

Nguyên nhân ở `internal/agent/node_model.go`:

```go
case provider.ChunkUsage:
    s.Usage.InputTokens += chunk.Usage.InputTokens   // ← sai
```

Gemini gửi `usageMetadata` kèm `promptTokenCount` **đầy đủ ở MỌI chunk stream** —
mỗi `ChunkUsage` là SNAPSHOT cộng dồn của lượt gọi, không phải delta. Cộng chúng
lại = nhân input token với số chunk (8 chunk → 41.400; câu dài hơn → 90.528).
Anthropic/DeepSeek gửi usage một lần ở cuối nên không lộ bug.

**Sửa.** `node_model.go` và `node_plan.go`: lấy `max()` các snapshot trong một
lượt gọi, rồi cộng vào tổng **một lần** sau khi stream kết thúc. Semantics
snapshot đúng cho cả 4 provider; cộng dồn qua các **bước** (tool loop) vẫn giữ.

Kèm theo: `node_plan.go` trước đây `return` ngay khi gặp `ChunkError`, bỏ luôn
usage đã tiêu và để stream dở dang — giờ đọc hết stream, tính token, rồi mới
thoát.

**Tác động.** Không tự nó giảm tiền, nhưng nếu con số sai 8 lần thì mọi quyết định
tối ưu sau đó đều là đoán. Nó cũng sửa một lỗi người dùng thấy được:
`totalTokens` chảy vào `contextTokens`/`contextBudget` mà FE dùng để gợi ý "nên
bắt đầu chat mới" → trước fix, người dùng bị nhắc tạo chat mới sớm gấp ~8 lần.

Không ảnh hưởng compaction: `trimContext` tự ước lượng từ `s.Messages`, không đọc
`s.Usage`.

**Test.** `internal/agent/usage_accounting_test.go` — kiểu Gemini (3 snapshot →
tính 1 lần), kiểu Anthropic/DeepSeek (usage một lần ở cuối), và cộng dồn qua
nhiều bước tool loop.

---

## Fix 3 — Không rò rỉ tên model sang provider khác họ

**Vấn đề.** Log production:

```
gemini:   calling API model=deepseek-v4-flash ...   ← vô lý
gemini:   calling API model=deepseek-v4-flash ...
deepseek: calling API model=deepseek-v4-flash ...
```

`fastModel()` trả `deepseek-v4-flash` (dùng cho learner + summarize), tên đó vào
`Options.Model`, `fallback.Provider` truyền nguyên request cho từng provider, và
`gemini.go` tôn trọng override → gọi Gemini với model không tồn tại, **chắc chắn
lỗi**, rồi mới rơi xuống DeepSeek.

Vì learner chạy sau **mỗi** câu trả lời, mỗi lượt chat kèm 2 request Gemini rác.

**Sửa.** `internal/provider/fallback/fallback.go`: `scopeModel(req, providerName)`
bỏ `Options.Model` khi tên model không thuộc provider đang gọi (`gemini-*`/`gemma-*`
→ gemini, `deepseek-*` → deepseek, `claude-*` → anthropic). Tên không nhận ra
(model tự host, Ollama) thì giữ nguyên, không đoán.

**Tác động.** Mỗi lượt chat bớt 2 request Gemini thất bại. Quan trọng hơn tiền:
2 lỗi giả đó đang cộng vào circuit breaker/cooldown, khiến Gemini bị đánh dấu
"hỏng" oan và traffic thật bị đẩy sang provider trả tiền.

**Test.** `internal/provider/fallback/model_scope_test.go` — gemini không nhận
model của deepseek, deepseek vẫn nhận đúng override của mình, `modelFamily()`
nhận diện 9 dạng tên, model lạ giữ nguyên.

---

## Fix 4 — Learner không chạy cho lượt tán gẫu

**Vấn đề.** `Learner.LearnFromConversation` gọi LLM (reflection) sau **mỗi** câu
trả lời để trích `user_facts` + `knowledge_items`. Với "xin chào" hay "cảm ơn
nhé" nó luôn trả rỗng — nhưng vẫn là một lượt gọi LLM đầy đủ. Tức mỗi câu tán
gẫu đang trả tiền hai lần.

**Sửa.** `internal/memory/learner_gate.go` — `worthLearning()`, bảo toàn phía học,
chỉ bỏ khi chắc chắn không có gì:

- câu user chứa từ khoá gợi ý fact → **luôn học** (dùng chung `keywordToKeys` với
  `RecallNode`, một nguồn sự thật)
- câu user > 25 rune → học
- câu trả lời > 400 rune → học (câu dài thường là giải pháp kỹ thuật đáng lưu)
- còn lại (ngắn + ngắn + không từ khoá) → bỏ qua

Bỏ sót một lượt đáng học chỉ mất một fact (lượt sau nhắc lại là học được); học
mọi lượt tán gẫu thì nhân đôi hoá đơn LLM của toàn hệ thống.

**Test.** `internal/memory/learner_gate_test.go` — 7 case cho `worthLearning`,
cộng hai test hành vi: lượt tán gẫu **không** gọi provider, lượt có fact
("tôi tên An") **vẫn** gọi.

---

## Công cụ đo đi kèm

`services/agent-go/cmd/promptsize` — cân từng thành phần payload gửi lên LLM
(base prompt, skill summary, skill kích hoạt, tool schema, messages):

```bash
cd services/agent-go && go run ./cmd/promptsize
```

Chính nó lật lại kết luận ban đầu của bản load test: 41k token là lỗi đo, không
phải context phình. Giữ lại trong repo để lần sau nghi ngờ chi phí thì đo thay
vì đoán.

---

## Kiểm chứng

```
services/agent-go:  go vet ./... && go test ./...   → toàn bộ pass
apps/api:           pnpm test                        → 18 file, 74 test pass
```

Mỗi test hồi quy đã được chạy thử với code CHƯA sửa để chắc là nó thật sự bắt
được bug:

- `TestScopeModel_KhongRoRiTenModelSangProviderKhacHo` → fail: `gemini nhận model "deepseek-v4-flash"`
- `TestNodeModel_UsageCongDonQuaNhieuBuoc` → fail: `InputTokens = 400, want 200`

## Việc còn lại (chưa làm)

Theo thứ tự đáng làm tiếp, từ báo cáo load test:

1. **Backpressure**: khi >30 request đang chạy, trả 429 + `Retry-After` ngay thay
   vì để user chờ 120s rồi timeout.
2. **Rate limit dùng Redis store**: hiện in-memory, scale nhiều instance là mất
   hiệu lực. Redis đã có trong stack.
3. **Thêm tenant vào chat cache key** (`apps/api/src/common/cache/chat-cache.ts`).
4. Cân nhắc rút gọn danh sách 30 skill summary (~1.095 token mỗi request) — nhỏ
   hơn nhiều so với 3 fix trên, và có đánh đổi về chất lượng chọn skill.
