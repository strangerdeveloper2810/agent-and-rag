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
| skill được kích hoạt | **2.027** | **1.151** | bỏ frontmatter + trần token |
| tool schema (đã filter) | 380 | 380 | giữ |
| **→ lượt gọi model** | **5.260** | **3.475** | **−34%** |
| reflection system prompt | 683 | 683 | giữ |
| transcript gửi cho learner | **~2.286** | **~203** | chỉ lượt cuối |
| **→ lượt gọi learner** | **~2.969** | **886** | **−70%** |
| **TỔNG input một lượt chat** | **~8.229** | **4.448** | **−45,9%** |

> Mốc trung gian: sau 5a–5d là 5.238 token (−36%); sau 5e–5f là 4.362 (−47%)
> nhưng lúc đó 13 skill vẫn bị gọt mất nội dung. Sau 5g (viết lại 8 SKILL.md dài
> nhất + nâng trần lên 5.500) là **4.448 token và 0 skill bị gọt** — đắt hơn mốc
> trước 86 token nhưng lấy lại toàn bộ 29.464 byte hướng dẫn đang bị mất, và có
> thêm 2 skill mới. Đây là con số cuối.

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

### 5e. Skill không còn kéo YAML frontmatter vào prompt (−161 token/lượt có skill)

`skills.Loader` gán `skill.Content = raw` — tức **toàn bộ file, gồm cả YAML
frontmatter**. Mỗi lần skill kích hoạt, prompt nhận thêm `name`, `description`,
`when_to_use`, `tools`, và `triggers` — danh sách 20+ từ khoá chỉ dùng cho
`MatchSkill` bên Go (learning-tutor: ~564 byte frontmatter).

Vừa tốn token, vừa dở về chất lượng: nhét một danh sách từ khoá vào prompt dễ
làm model nhắc lại chúng. Giờ `Content` chỉ còn phần thân.

### 5f. Trần token cho thân skill (−715 token/lượt có skill)

Nội dung skill được chèn lại ở **mỗi** lượt chat có skill khớp:
`State.activatedSkills` chỉ chặn chèn lặp trong CÙNG một lượt chạy, lượt sau là
State mới nên chèn lại từ đầu. Mà SKILL.md dao động 2.000–11.600 byte.

`skills.MaxPromptBytes = 4500` (≈1.285 token), cắt theo **ranh giới section**
(`## `) chứ không theo số byte — một hướng dẫn bị chặt giữa câu còn tệ hơn là
không có. Ca biên đã xử lý: nếu chính section đầu đã vượt trần thì giữ một phần
nó (cắt theo dòng) thay vì chỉ còn lại tiêu đề; và cắt cứng thì cắt theo rune để
không hỏng ký tự tiếng Việt. Có thêm dòng ghi chú nói thẳng với model là phần sau
đã được lược, thay vì im lặng cắt.

**Đánh đổi, nói rõ:** 12 trong ~30 skill vượt trần, `devops` (11.604 byte) mất
~60% thân. Skill bị gọt nhiều nghĩa là nên viết lại SKILL.md cho phần cốt lõi lên
đầu — vì vậy log `model: skill activated` giờ có thêm `body_bytes` và
`truncated`, để biết skill nào đang bị cắt chứ không mất đuôi âm thầm. Muốn nới
thì sửa một hằng số.

### 5g. Viết lại 8 SKILL.md dài nhất + nâng trần cho khớp dữ liệu thật

Trần 4.500 byte ban đầu làm **13/32 skill bị gọt, mất 29.464 byte** hướng dẫn
không bao giờ tới được model (`devops` mất 6.678, `api-designer` mất 6.069).

Cách xử lý: nén nội dung theo tinh thần caveman (bỏ boilerplate mà model tự sinh
được — Dockerfile, YAML GitHub Actions, K8s manifest, template postmortem, schema
GraphQL; bỏ ví dụ trùng ý; gộp bullet lặp; **giữ nguyên code block**), rồi nâng
trần lên 5.500 cho khớp file lớn nhất còn lại.

| Skill | Trước | Sau |
|---|---|---|
| devops | 11.178 | 4.620 |
| api-designer | 10.569 | 5.462 |
| productivity | 8.748 | 5.110 |
| security-audit | 7.764 | 4.598 |
| performance-optimizer | 7.366 | 4.581 |
| learning-tutor | 6.533 | 4.272 |
| data-analysis | 6.380 | 4.632 |
| planning | 5.568 | ~4.900 |

Kết quả: **0/32 skill bị gọt** — mọi hướng dẫn đều tới được model.

Vì sao nâng trần không làm tăng cost: **trần không phải chi phí mỗi request**,
kích thước thật của skill được kích hoạt mới là. Trần chỉ cắt phần vượt. Sau khi
nén, skill trung bình còn ~4.600 byte nên chi phí thực tế còn GIẢM so với lúc
trần 4.500 mà file to.

Sửa kèm khi viết lại: `productivity` thiếu `triggers` (activation dựa vào matching
tiếng Anh, rất yếu) → đã thêm; nhiều ví dụ trong `productivity`/`learning-tutor`
gọi user là **"sir"** — trái trực tiếp base prompt đang cấm gọi "sir" → đã bỏ.

### 5h. Thêm 2 skill từ obra/superpowers (MIT), gộp 3 ý vào `debug`

Đã so nội dung hai bên trước khi import. Kết luận: superpowers **không tốt hơn**,
nó làm việc khác — nó là process discipline cho agent đang code trong repo có
người duyệt, còn skill JARVIS là gói phương pháp cho trợ lý đa năng, có tool
binding và trigger tiếng Việt.

Phần lớn superpowers **trùng** skill JARVIS đã có (brainstorming, debug, planning,
code-review) nên KHÔNG import — hai skill cùng chủ đề sẽ tranh điểm trong
`MatchSkill`, kích hoạt sai vừa tốn ~1.300 token vừa trả lời lệch.

**Gộp vào `skills/debug`** (2.032 → 3.531 byte, vẫn dưới trần) 3 ý JARVIS thiếu:
Luật sắt "không sửa gì trước khi tìm ra root cause" (trước chỉ là gợi ý mức
anti-pattern) · **Pattern Analysis** — so với chỗ code đang chạy ĐÚNG rồi diff
working vs broken · **quy tắc 3 lần** — ba lần fix không xong thì dừng, nghi kiến trúc.

**Import 2 skill JARVIS chưa có**, adapt hẳn sang ngữ cảnh (trigger tiếng Việt,
`tools` binding, dưới trần, ghi attribution MIT):
`test-driven-development` (3.312 byte) và `verification-before-completion` (2.962 byte).

Catalogue tăng 2 tên = **+16 token/request**.

**caveman**: không import làm skill runtime — văn phong telegraphic áp lên câu trả
lời tiếng Việt là UX tệ, và base prompt đã yêu cầu ngắn gọn. Chỉ dùng *quy tắc
nén* của nó cho §5g (nén file prompt nội bộ), đúng chỗ nó mạnh. Không vendor phần
engine/proxy (BSL-1.1).

### Còn có thể cắt tiếp (chưa làm)

- `reflectionSystemPrompt` 683 token/lượt learner — rút gọn được nhưng đánh đổi
  chất lượng trích xuất, cần eval trước khi sửa.
- base system prompt 1.726 token — phần lớn là quy tắc format bảng + quy tắc
  chọn RAG/web. Sửa là đổi hành vi, phải có eval.
- `datetime` tool schema 249 token, lớn nhất trong 4 tool của chat thường.
- Viết lại 12 SKILL.md đang bị gọt cho gọn (hoặc tóm tắt một lần lúc load) — vừa
  lấy lại phần nội dung đang bị cắt, vừa giảm token thêm.

## Fix 6 — Learner chưa bao giờ lưu được gì vào MongoDB

**Bug production, tìm ra khi viết integration test có Mongo thật.**

`saveFactToMongo` và `saveKnowledgeItemToMongo` gọi:

```go
_, _ = coll.UpdateOne(ctx, filter, update)   // sai 2 chỗ
```

Filter là `{key, tenantId}` (hoặc `{documentId, tenantId}`) — lần học đầu tiên của
một key thì **chưa có document nào khớp**, và `UpdateOne` **không có
`SetUpsert(true)`** nên nó không ghi gì cả. Error thì bị `_, _ =` ném đi, nên việc
không ghi được diễn ra hoàn toàn im lặng: log vẫn in
`learner: learned user fact` như thể đã lưu.

Hệ quả: mọi thứ learner học được chỉ nằm trong `Store` in-memory và **mất sạch sau
mỗi lần restart container**; `LoadFromMongo` lúc khởi động không có gì để nạp. Tức
tính năng "agent tự học và nhớ qua các phiên" chưa từng hoạt động qua đường này.

Sửa: thêm `options.UpdateOne().SetUpsert(true)` ở cả hai chỗ, và log warning khi
ghi lỗi thay vì bỏ qua.

Test chứng minh: `TestLearner_LuuFactVaoMongo` (fail 5s timeout trước fix, pass
0,07s sau fix), `TestLearner_LuuKnowledgeItemVaoMongo`, và
`TestLearner_UpsertKhongTronLanGiuaTenant` (hai tenant học cùng key `user_name`
phải ra hai document, không ghi đè nhau).

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
| `internal/agent` | **95,5%** | gate ≥95% |
| `internal/memory` | **95,5%** | gate ≥95% (cần MongoDB, xem bên dưới) |
| `internal/skills` | **96,9%** | gate ≥95% |
| `internal/provider/fallback` | **95,1%** | gate ≥95% |
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

**`internal/memory` 87,9% → 95,5% nhờ integration test có MongoDB thật.** Ba hàm
I/O Mongo (`saveFactToMongo`, `saveKnowledgeItemToMongo`, `LoadFromMongo`) không
fake được vì `mongo.Client` chỉ dựng qua `Connect()` (ping ngay lúc tạo, struct có
field unexported). Cách chạy:

```bash
docker run -d --rm --name jarvis-test-mongo -p 27117:27017 mongo:7
MONGODB_TEST_URI=mongodb://localhost:27117 go test ./internal/memory/
```

Không set biến đó thì nhóm test này skip (CI cũ vẫn xanh), nhưng coverage tụt về
~88% và gate 95% sẽ fail — nên CI đã được thêm service `mongo:7` kèm
`MONGODB_TEST_URI`.

**Bug production tìm ra nhờ nhóm test này** — xem Fix 6.

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
