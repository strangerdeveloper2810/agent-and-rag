# Fix rate limit + tối ưu chi phí LLM — 2026-08-18

Đi kèm `docs/2026-08-18-loadtest-production-report.md`. Bản load test tìm ra
vấn đề; bản này sửa chúng.

Năm fix, mỗi fix có test hồi quy đã kiểm chứng là **fail khi chưa sửa**.

Fix 1–4 sửa bug (rate limit gộp cả hệ thống, đếm token phồng 8 lần, request
Gemini rác, learner chạy cho lượt tán gẫu). **Fix 5 mới là phần giảm tiền thật:
input mỗi lượt chat 8.229 → 5.238 token, −36%.**

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

## Fix 5 — Cắt token thật mỗi lượt chat: −36% input

Bốn fix trên chặn phí quota và sửa số đo, nhưng **tiền token mỗi lượt gần như
không giảm**. Phần này mới là cắt chi phí.

Đo trước bằng `cmd/promptsize` để biết cắt ở đâu, thay vì cắt theo cảm giác:

| Thành phần | Trước | Sau | |
|---|---|---|---|
| base system prompt | 1.726 | 1.726 | giữ (là quy tắc hành vi) |
| danh sách 30 skill | **1.095** | **186** | chỉ còn tên |
| skill được kích hoạt | 2.027 | 2.027 | giữ (chỉ nạp khi khớp) |
| tool schema (đã filter) | 380 | 380 | giữ |
| **→ lượt gọi model** | **5.260** | **4.352** | **−17%** |
| reflection system prompt | 683 | 683 | giữ |
| transcript gửi cho learner | **~2.286** | **~203** | chỉ lượt cuối |
| **→ lượt gọi learner** | **~2.969** | **886** | **−70%** |
| **TỔNG input một lượt chat** | **~8.229** | **5.238** | **−36%** |

### 5a. Danh sách skill: bỏ description, giữ tên (−909 token/request)

Phát hiện quyết định: **skill KHÔNG do model chọn.** `skills.Loader.MatchSkill`
chấm điểm bằng code Go trên input người dùng (khớp tên + `triggers` trong
frontmatter), rồi `node_model` nạp nguyên văn SKILL.md của skill thắng.

Nghĩa là 30 dòng description gửi kèm **mọi** request không mua được khả năng nào
— model không dùng chúng để kích hoạt gì cả. Chúng chỉ giúp model trả lời câu
"bạn làm được gì", và danh sách tên là đủ cho việc đó.

Vẫn giữ danh sách trong prompt (thay vì bỏ hẳn) để prefix đầu prompt còn ổn định
cho prompt caching — chèn động theo câu hỏi sẽ phá cache.

### 5b. Learner chỉ nhận lượt trao đổi cuối (−2.083 token/lượt)

Learner chạy sau **mỗi** câu trả lời, nhưng trước đây nhận cả transcript (cắt ở
8.000 rune). Các lượt cũ đã được reflect ở những lần gọi trước → gửi lại là trả
tiền nhiều lần cho cùng một đoạn text.

Giờ chỉ lấy `maxReflectionMessages = 4` tin nhắn cuối (2 cặp trao đổi: lượt hiện
tại + 1 lượt trước làm ngữ cảnh cho câu kiểu "cái đó tên là X"), trần rune hạ
8.000 → 2.500. Lọc user/assistant **trước** khi cắt, nếu không một lượt nhiều
tool call sẽ đẩy hết hội thoại thật ra ngoài — có test cho đúng ca này.

### 5c. Bỏ gọi embedding khi không cần (−1 API call/lượt)

`RecallNode` gọi `SemanticSearch` → API embedding (Voyage) cho **mọi** câu user,
kể cả khi lookup theo keyword và full-text đã tìm được đúng fact — kết quả
semantic sau đó bị bỏ qua vì vòng lặp chỉ thêm key CHƯA có. Giờ chỉ gọi khi hai
bước rẻ không ra gì (đúng lúc semantic search có giá trị: câu hỏi diễn đạt khác
cách fact được lưu). Tiền embedding nhỏ, nhưng cũng bớt 100–300ms latency và một
điểm lỗi mỗi lượt.

### 5d. Cho thấy phần chi phí đang bị che

`gemini.go` giờ log `cached` và `thoughts` token ở chunk cuối. `cached_tokens` là
tập con của `promptTokenCount` và được tính giá rẻ hơn nhiều — không thấy con số
này thì không biết prompt caching có ăn hay không, tức là mù về khoản lớn nhất
trong hoá đơn (system prompt ~4k token lặp lại mọi request).

### Còn có thể cắt tiếp (chưa làm)

- `reflectionSystemPrompt` 683 token/lượt learner — rút gọn được nhưng đánh đổi
  chất lượng trích xuất, cần eval trước khi sửa.
- base system prompt 1.726 token — phần lớn là quy tắc format bảng + quy tắc
  chọn RAG/web. Sửa là đổi hành vi, phải có eval.
- `datetime` tool schema 249 token, lớn nhất trong 4 tool của chat thường.

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
services/agent-go:  gofmt -l . (sạch) && go vet ./... && go test ./...   → toàn bộ pass
                    ./scripts/coverage-gate.sh                           → pass
apps/api:           pnpm test (22 file, 84 test)  → pass
                    pnpm typecheck / eslint / prettier --check           → sạch
                    pnpm test:coverage (ngưỡng theo file)                 → pass
```

Mỗi test hồi quy đã được chạy thử với code CHƯA sửa để chắc là nó thật sự bắt
được bug (test không fail được thì không chứng minh điều gì):

| Test | Khi bỏ fix ra |
|---|---|
| `TestScopeModel_KhongRoRiTenModelSangProviderKhacHo` | `gemini nhận model "deepseek-v4-flash"` |
| `TestNodeModel_UsageCongDonQuaNhieuBuoc` | `InputTokens = 400, want 200` |
| `TestEngineRun_TokenKhongBiNhanLen_QuaToolLoop` | `Usage.InputTokens = 6000, want 2500`; `done.TotalTokens = 6071, want 2550` |
| `app.ratelimit.integration.test.ts` | user B: `expected 429 to be 400`; khách chưa đăng nhập: `expected 429 to be 401` |

## Test đã viết

**Unit test**

- `apps/api/src/common/guards/rate-limit-key.test.ts` (6) — sinh khoá theo user/IP,
  token giả mạo, token hết hạn, thiếu cookie.
- `services/agent-go/internal/provider/fallback/model_scope_test.go` (4) —
  `modelFamily` 9 dạng tên, giữ/bỏ override, model lạ.
- `services/agent-go/internal/agent/usage_accounting_test.go` (3) — kiểu Gemini,
  kiểu Anthropic/DeepSeek, cộng dồn qua nhiều bước.
- `services/agent-go/internal/memory/learner_gate_test.go` (7 + 2) — bảng
  `worthLearning`, và learner không gọi provider khi tán gẫu.
- `slugify` (7 case) trong `learner_integration_test.go`.

**Integration test**

- `apps/api/src/app.ratelimit.integration.test.ts` (3) — qua `buildApp()` THẬT,
  route chat thật: user A hết 20/phút → 429, user B vẫn 400 (không bị 429 oan).
- `apps/api/src/common/guards/rate-limit-route.test.ts` (2) — hạn mức riêng của
  route có kế thừa `keyGenerator` toàn cục hay không (nếu không thì cả bản fix vô
  nghĩa; đây là giả định phải test chứ không được tin).
- `apps/api/src/app.health.test.ts` (5) — `/api/health`, `/api/healthz`,
  `/api/ready` cho cả nhánh Mongo sống và Mongo chết. CD dựa vào healthz để
  quyết định deploy thành công, trước đây không có test nào chạm tới.
- `services/agent-go/internal/agent/engine_usage_integration_test.go` (2) — chạy
  trọn `engine.Run()` qua tool loop 2 bước với provider stream kiểu Gemini, kiểm
  số token ở event `done` mà client thật sự nhận.
- `services/agent-go/internal/memory/learner_integration_test.go` (4) —
  `LearnFromConversation` → `ReflectAndExtract` → `Store`, có kiểm tra fact được
  scope theo tenant.

## Coverage — số thật

Đo bằng `go test -cover` và `vitest --coverage` (provider v8):

| Phạm vi | Coverage | Ghi chú |
|---|---|---|
| `internal/agent` | **94,8%** | gate ≥90% |
| `internal/provider/fallback` | **95,1%** | gate ≥90% |
| `internal/memory` | **87,9%** | gate ≥85% — xem bên dưới |
| `apps/api/src/app.ts` | **92,98%** (funcs 100%) | trước: 59,6% |
| `apps/api/.../rate-limit-key.ts` | **100%** cả 4 chỉ số | |
| agent-go toàn bộ | 81,0% | có sẵn từ trước |
| apps/api toàn bộ | 37,6% | có sẵn từ trước |

**Nói rõ: KHÔNG đạt >90% ở phạm vi toàn repo.** Code do lần sửa này thêm vào thì
100% (`scopeModel`, `modelFamily`, `worthLearning`, `lastByRole`, `nodePlan`,
`rate-limit-key.ts`). Phần kéo tổng xuống là code có sẵn chưa từng có test:
`apps/api` có `server.ts`, `common/email`, `src/agent/deprecated/*` (0%);
agent-go có `internal/mongo` 40,9%, `internal/tools` 76,3%. Đưa cả hai lên 90% là
một việc riêng, lớn hơn nhiều lần bản fix này.

**Vì sao `internal/memory` dừng ở 87,9%:** ba hàm còn lại là I/O Mongo thuần —
`saveFactToMongo`, `saveKnowledgeItemToMongo` (0%), `LoadFromMongo` (18,2%).
Không fake được vì `mongo.Client` chỉ dựng qua `Connect()` (ping ngay lúc tạo, và
struct có field unexported), nên cần MongoDB thật. Máy dev lúc đo không có
`mongod` và Docker daemon chưa bật, nên tôi **không viết test cho phần này** —
viết mà không chạy được thì không kiểm chứng được. Bật Docker (hoặc cấp
`MONGODB_TEST_URI`) là làm được ngay.

**Gate coverage** (chống tụt về sau, không phải gate toàn repo cho đỏ CI):

- `services/agent-go/scripts/coverage-gate.sh` — ratchet theo từng package, chạy
  trong CI dùng lại `coverage.out` của bước test (không chạy test 2 lần).
- `apps/api/vitest.config.ts` — `coverage.thresholds` theo từng file; CI có step
  `Coverage gate (API)`.

Cả hai gate đã được thử với ngưỡng cố tình đặt cao để chắc là chúng thật sự fail
(`ERROR: Coverage for statements (92.98%) does not meet "src/app.ts" threshold (99%)`
và `❌ internal/memory: 87.9% < ngưỡng 95%`), rồi mới đặt lại mức thật.

## Việc còn lại (chưa làm)

Theo thứ tự đáng làm tiếp, từ báo cáo load test:

1. **Backpressure**: khi >30 request đang chạy, trả 429 + `Retry-After` ngay thay
   vì để user chờ 120s rồi timeout.
2. **Rate limit dùng Redis store**: hiện in-memory, scale nhiều instance là mất
   hiệu lực. Redis đã có trong stack.
3. **Thêm tenant vào chat cache key** (`apps/api/src/common/cache/chat-cache.ts`).
4. Cân nhắc rút gọn danh sách 30 skill summary (~1.095 token mỗi request) — nhỏ
   hơn nhiều so với 3 fix trên, và có đánh đổi về chất lượng chọn skill.
