# Báo cáo load test production — 2026-08-18

Mục tiêu: trả lời câu hỏi "nếu 100 hoặc 1000 người dùng prompting cùng lúc thì
JARVIS chịu nổi hay lăn đùng ra tèo?"

Target: `https://ai.ethansoftwaredeveloper.com` (production thật, không phải staging).
Công cụ: `tools/loadtest` (Go, trong repo này).

---

## TL;DR

**Hạ tầng không tèo. Nhưng hệ thống hiện tại không phục vụ được 100 user
đồng thời, và nút thắt nằm ở nơi ít ai ngờ: một dòng cấu hình thiếu.**

Ba trần chặn, xếp theo thứ tự user sẽ đụng phải:

| # | Trần | Giá trị đo được | Ai gây ra |
|---|------|-----------------|-----------|
| 1 | Rate limit chat | **20 request/phút cho TOÀN hệ thống** (≈0,33 req/s) | bug thiếu `X-Forwarded-For` → xem §5.1 |
| 2 | LLM provider | 503 "high demand" xuất hiện từ **30 concurrent**, p95 nhảy 5,4s → 21,7s | Gemini/DeepSeek |
| 3 | hr-nginx | 25 r/s + burst 50; **30 kết nối/IP**; ~1000 conn cho cả VPS | reverse proxy dùng chung |

Còn agent-go — thứ ai cũng nghĩ sẽ chết trước — **xử lý 27.000 request ở tới
2000 concurrent, 100% thành công, RAM 97 MB**. Nó không phải vấn đề.

---

## 1. Hạ tầng đang chạy

VPS: **2 vCPU, 3,9 GB RAM**, 13 container (dùng chung với stack `hr-*`).

```
Internet → hr-nginx (host, TLS, limit_req/limit_conn)
         → jarvis-web:8090 (nginx container, SPA + proxy /api/)
         → jarvis-api:3001 (Fastify BFF: auth, Mongo, rate limit, Redis cache)
         → jarvis-agent-go:3002 (agent runtime, LLM, RAG, memory)
         → Gemini 3.1 flash-lite → Gemini 3.5 → DeepSeek → Claude (fallback chain)
```

## 2. Cửa trước — hr-nginx

Config (`/etc/nginx/conf.d/jarvis.conf` trong container `hr-nginx`):

```nginx
limit_req_zone  $binary_remote_addr zone=jarvis_req_limit:10m rate=25r/s;
limit_conn_zone $binary_remote_addr zone=jarvis_conn_limit:10m;
limit_req  zone=jarvis_req_limit burst=50 nodelay;
limit_conn jarvis_conn_limit 30;
```

Đo từ máy ngoài (`ab` + `tools/loadtest -mode=health`):

| Tốc độ bắn | Tỉ lệ lỗi | Lỗi trả về |
|---|---|---|
| 42 req/s | 0% | — |
| 121 req/s | 31% | 503 (limit_req) |
| 194 req/s | 65% | 503 |
| 922 req/s | 82% | 503 |

Cả static file và `/api/` đều bị chặn như nhau → đúng là `limit_req`, không phải
app quá tải.

**Diễn giải quan trọng:** hai giới hạn này khoá theo `$binary_remote_addr`, tức
**per-IP**. 1000 user thật có 1000 IP → mỗi người một hạn mức riêng, cửa này
KHÔNG chặn họ. Nó chỉ chặn việc test từ một máy — và `limit_conn 30` là lý do
không thể mô phỏng 1000 SSE stream từ bên ngoài.

Trần cứng của cả VPS: `worker_processes auto` (=2) × `worker_connections 1024`
= 2048 kết nối. Mỗi request qua proxy tốn 1 conn client + 1 conn upstream, nên
thực tế **~1000 request đang mở đồng thời** cho toàn bộ VPS, chia sẻ với `hr-*`.

## 3. BFF (Fastify) — nút thắt thật

Test: 60 chat request thật, 10 concurrent, qua public URL, token JWT hợp lệ.

```
success = 31,7% (19/60)
TTFB   p50=240ms  p95=341ms
total  p50=4387ms p95=10828ms
kết quả: 429_gateway_ratelimit=41  ok=19
lỗi: {"error":"Rate limit exceeded, retry in 55 seconds"}
```

Đúng 19 request + 1 smoke = **20** — khớp hạn mức `max: 20, timeWindow: 1 minute`
của route `/conversations/:id/chat`.

Vấn đề không phải con số 20, mà là **20 đó dùng chung cho tất cả mọi người**.
Bằng chứng từ log `jarvis-api`:

```
145 "remoteAddress":"172.18.0.7"     ← nginx container, KHÔNG phải IP user
 43 "remoteAddress":"127.0.0.1"      ← healthcheck
```

`@fastify/rate-limit` mặc định khoá theo `req.ip`. Fastify không bật
`trustProxy`, nên `req.ip` = địa chỉ socket = IP của nginx container. **Mọi user
trên Internet đổ vào cùng một bucket.** Với 100 người dùng, người thứ 21 gửi
tin trong phút đó ăn 429 dù chưa gửi gì trước đó.

Trần thực tế của production hôm nay: **20 chat/phút ≈ 0,33 req/s cho toàn hệ thống.**

## 4. agent-go — đo trần thật

Vì `limit_conn 30`/IP không cho giữ 1000 stream từ ngoài, phần này bắn từ chính
VPS vào `172.18.0.2:3002`, bỏ qua nginx + Fastify.

### 4.1 Tầng vận chuyển (0 token)

Dùng input mà guardrails chắc chắn từ chối (`ignore all previous instructions`)
→ agent-go trả 400 **trước khi gọi LLM**, nên đo được sức chịu kết nối mà không
tốn một đồng nào. `mode=probe` trong `tools/loadtest`.

| Concurrency | Request | Throughput | p50 | p95 | p99 | max | Thành công |
|---|---|---|---|---|---|---|---|
| 100 | 2.000 | 4.962 req/s | 15ms | 45ms | 77ms | 94ms | **100%** |
| 500 | 5.000 | 4.444 req/s | 92ms | 204ms | 248ms | 401ms | **100%** |
| 1.000 | 10.000 | 5.282 req/s | 164ms | 309ms | 410ms | 784ms | **100%** |
| 2.000 | 10.000 | 3.919 req/s | 385ms | 834ms | 942ms | 1.370ms | **100%** |

27.000 request, **không một lỗi nào**. Tài nguyên trong lúc bắn:

- agent-go: CPU ≤ 4,5%, RAM 17 → 97 MB
- load average đỉnh: **1,32** trên 2 vCPU
- canary `/healthz`: 41 mẫu, 0 fail

Kết luận: HTTP layer của Go không phải nút thắt. Ở 1000 concurrent nó còn rảnh.

### 4.2 Với LLM thật

50 request thật (giữ chi phí thấp theo yêu cầu — DeepSeek còn ~$9):

| Concurrency | Thành công | p50 | p95 | p99 | max | Lỗi |
|---|---|---|---|---|---|---|
| 5 | 100% | 3,8s | 5,4s | 5,7s | 5,8s | — |
| 15 | 93,3% | 6,2s | **25,3s** | 36,4s | 39,1s | 1× empty response từ provider |
| 30 | 96,7% | 4,4s | **21,7s** | 23,9s | 24,6s | 1× Gemini 503 "high demand" |

Tài nguyên: agent-go CPU ≤ 7,9%, RAM 22–98 MB, load ≤ 0,45. **Máy vẫn rảnh** —
độ trễ đến từ phía LLM provider, không phải VPS.

Từ 15 concurrent trở lên, p95 đã gấp **4–5 lần** so với 5 concurrent, và lỗi
provider bắt đầu xuất hiện lẻ tẻ. Đó là điểm chất lượng bắt đầu vỡ.

## 5. Ba vấn đề tìm thấy

### 5.1 Rate limit áp cho cả hệ thống thay vì từng user — NGHIÊM TRỌNG

Chuỗi IP bị đứt ở nginx **trong container**, không phải ở hr-nginx:

- `hr-nginx` (host): CÓ set `X-Forwarded-For $proxy_add_x_forwarded_for` ✅
- `apps/web/nginx.conf` (container): chỉ set `Host` + `X-Real-IP`, **không
  chuyển tiếp `X-Forwarded-For`** ❌
- `apps/api/src/app.ts`: `Fastify({ logger, bodyLimit })` — **không có `trustProxy`** ❌

Nên bật `trustProxy` một mình là vô tác dụng — phải sửa cả hai. Và sửa xong thì
rate limit theo IP vẫn chưa đúng: user đã đăng nhập nên nên khoá theo `sub` của
JWT, mới đúng ý "mỗi người 20 tin/phút".

### 5.2 Con số token bị phồng ~8 lần — LỖI ĐO, không phải context phình

> **Đã sửa** — xem `docs/2026-08-18-cost-and-ratelimit-fixes.md`. Phần dưới giữ
> lại nguyên trạng quá trình lần ra nguyên nhân, kèm kết luận đúng.

Log `jarvis-agent-go` với prompt "Xin chào, bạn là ai và làm được gì?":

```
gemini: calling API model=gemini-3.1-flash-lite tools_count=9
model: LLM response tokens_in=41400 tokens_out=2506 elapsed_ms=2860
engine: run done steps=1 tokens_in=41400 tokens_out=2506 total_tokens=43906
```

Một request khác báo tới `tokens_in=90528`. Ban đầu tôi đọc đây là context phình
(skills + memory nạp vô điều kiện). **Đo lại thì không phải.**

`cmd/promptsize` dựng đúng payload mà agent gửi lên và cân từng phần:

| Thành phần | Byte | ≈ token |
|---|---|---|
| base system prompt | 6.042 | 1.726 |
| danh sách 30 skill (name + description) | 3.833 | 1.095 |
| skill được kích hoạt (learning-tutor, full SKILL.md) | 7.097 | 2.027 |
| tool schema sau filter (4 tool) | 1.333 | 380 |
| tin nhắn user | 58 | 16 |
| **TỔNG** | **18.413** | **≈5.260** |

Agent gửi ~5.260 token nhưng log báo 41.400 — chênh đúng 8 lần, và request dài
hơn thì chênh 17 lần. Nguyên nhân nằm ở `node_model.go`:

```go
case provider.ChunkUsage:
    s.Usage.InputTokens += chunk.Usage.InputTokens   // ← cộng dồn
```

Gemini gửi `usageMetadata` kèm `promptTokenCount` **đầy đủ ở MỌI chunk stream**
(`gemini.go`: emit `ChunkUsage` bên trong vòng lặp) — mỗi chunk là một SNAPSHOT
cộng dồn, không phải delta. Cộng chúng lại = nhân input token với số chunk.
Anthropic và DeepSeek chỉ gửi usage một lần ở cuối nên không lộ ra bug này.

Hệ quả không chỉ là log sai:

- `totalTokens` chảy ra UI và vào `contextTokens`/`contextBudget` mà FE dùng để
  gợi ý "nên bắt đầu chat mới" → người dùng bị nhắc tạo chat mới quá sớm.
- Mọi ước lượng chi phí (kể cả ước lượng trong bản đầu của báo cáo này) đều sai
  theo hướng phóng đại.

Điều KHÔNG bị ảnh hưởng: `trimContext` tự ước lượng token từ `s.Messages`
(`estimateTokens`) chứ không đọc `s.Usage`, nên compaction vẫn chạy đúng ngưỡng.

Kết luận đúng: context ~5,3k token/request là hợp lý cho một agent có tool +
skill, **không phải chỗ cần cắt gấp**. Chỗ thật sự đốt tiền là §5.4.

### 5.3 Chat cache không phân theo tenant

`apps/api/src/common/cache/chat-cache.ts`: key = `md5(model + temperature +
messages)`, TTL 1 giờ, **không có tenant/user trong key**. Hai user hỏi câu y
hệt nhau sẽ nhận cùng một câu trả lời cached.

Rủi ro rò rỉ ở đây thấp (chỉ trùng khi toàn bộ `messages` giống nhau, mà history
thường khác), nhưng nó cũng làm **sai lệch mọi bài load test** nếu prompt lặp
lại: request thứ hai trả về từ Redis trong 258ms thay vì gọi LLM 3,1s. Vì vậy
`tools/loadtest` có cờ `-unique` (mặc định bật) gắn mã tham chiếu vào mỗi prompt.

### 5.4 Mỗi lượt learner/summarize đốt 2 request Gemini chắc chắn thất bại — TỐN TIỀN THẬT

> **Đã sửa** — xem `docs/2026-08-18-cost-and-ratelimit-fixes.md`.

Log production, đọc kỹ mới thấy điều vô lý:

```
gemini:   calling API model=deepseek-v4-flash ...   ← provider gemini, model deepseek?!
gemini:   calling API model=deepseek-v4-flash ...
deepseek: calling API model=deepseek-v4-flash ...
```

Chuỗi nguyên nhân:

1. `cmd/server/main.go:fastModel()` trả `deepseek-v4-flash` khi có DeepSeek key —
   dùng cho learner (`ReflectAndExtract`) và `SummarizeMessages`.
2. Tên model đó đi vào `GenerateRequest.Options.Model` và `fallback.Provider`
   truyền **nguyên request** cho từng provider trong chuỗi.
3. `gemini.go` lại tôn trọng override: `if req.Options.Model != "" { model = ... }`
   → gọi Gemini API với một model không tồn tại. Chắc chắn lỗi.

Nên mỗi lần learner chạy (tức **sau mỗi câu trả lời**) hệ thống bắn 2 request
Gemini vô ích trước khi rơi xuống DeepSeek. Ba thứ mất cùng lúc: quota free tier
Gemini, độ trễ, và — tệ nhất — 2 lỗi giả đó cộng vào circuit breaker/cooldown nên
Gemini bị đánh dấu "hỏng" oan, đẩy cả traffic thật sang provider trả tiền.

## 6. Vậy chịu được bao nhiêu user đồng thời?

Ba con số khác nhau, đừng lẫn:

**Hôm nay (chưa fix gì):** 20 chat/phút cho toàn hệ thống. Khoảng **20 người**
mỗi người gửi 1 tin/phút. Người thứ 21 ăn 429.

**Sau khi fix rate limit (§5.1):** trần chuyển sang LLM provider. Ở 30 concurrent
prompt, p95 đã 21,7s và có 503 lẻ tẻ → vùng an toàn khoảng **20–30 prompt đang
chạy đồng thời**. Nếu mỗi user gửi 1 tin/phút và mỗi tin mất ~6s, con số này
tương đương **200–300 user hoạt động** (200 user × 6s / 60s = 20 prompt đồng thời).

**Với đúng 1000 người bấm Enter cùng lúc:** tốc độ tiêu thụ đo được ~1,2–3,8
request/s. Drain 1000 request cần **~4–14 phút**. Nhưng `proxy_read_timeout 120s`
(nginx) và `AGENT_GO_TIMEOUT=120000` (BFF) cắt ở 2 phút → chỉ khoảng **150–450
người đầu** nhận được câu trả lời, phần còn lại timeout. Server **không sập**,
không OOM, không container nào restart — nhưng phần lớn user thấy lỗi.

Trả lời trực tiếp câu hỏi ban đầu: **không lăn đùng ra tèo — nó vẫn đứng, thở
đều, chỉ là hầu hết user không được phục vụ.**

## 7. Khuyến nghị, theo thứ tự đáng làm

1. **Sửa chuỗi IP + rate limit theo user** (§5.1). Ba thay đổi nhỏ:
   - `apps/web/nginx.conf`, trong `location /api/`:
     `proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;`
   - `apps/api/src/app.ts`: `Fastify({ trustProxy: true, ... })`
   - `keyGenerator: (req) => req.user?.sub ?? req.ip` cho rate limit chat
   Đây là thay đổi biến hệ thống từ "20 tin/phút toàn cục" thành "20 tin/phút
   mỗi người" — tức là từ ~20 user lên ~200–300 user, không cần thêm một đồng
   hạ tầng nào.

2. **Sửa đếm token + chặn model override rò rỉ** (§5.2, §5.4). Không sửa được
   đếm token thì mọi nỗ lực tối ưu cost đều là đoán; không chặn override thì mỗi
   câu trả lời vẫn kèm 2 request Gemini rác.

3. **Backpressure thay vì để user chờ rồi timeout.** Khi số request đang chạy
   vượt ngưỡng (ví dụ 30), trả 429 kèm `Retry-After` ngay lập tức. Chờ 120 giây
   rồi nhận lỗi là trải nghiệm tệ nhất trong mọi lựa chọn.

4. **Rate limit dùng Redis store.** Hiện `@fastify/rate-limit` chạy in-memory,
   scale lên nhiều instance là mất hiệu lực. Redis đã có sẵn trong stack.

5. **Thêm tenant vào chat cache key** (§5.3).

6. **Cân nhắc `limit_conn 30`/IP.** Với user thật thì đủ, nhưng nếu nhiều người
   sau cùng một NAT (office, trường học) thì 30 kết nối là ít — SSE giữ kết nối
   lâu nên dễ đụng trần.

---

## Cách chạy lại

```bash
# Cửa trước + BFF, từ máy ngoài (an toàn, rẻ)
export LOADTEST_JWT_SECRET="<JWT_SECRET của prod>"
go run ./tools/loadtest -mode=health -stages "50:300"
go run ./tools/loadtest -mode=chat   -stages "10:60" -users 20

# Trần thật của agent-go — phải chạy TỪ TRONG VPS (limit_conn 30/IP chặn từ ngoài)
#   ulimit -n 65535
#   ./loadtest -mode=probe -base http://172.18.0.2:3002 -stages "1000:10000"   # 0 token
#   ./loadtest -mode=agent -base http://172.18.0.2:3002 -stages "30:30"        # LLM thật
```

Lưu ý khi đo: luôn để `-unique` bật (mặc định), nếu không Redis cache của BFF sẽ
trả lời thay LLM và mọi số liệu đều đẹp giả.

## Ghi chú về phạm vi

- Test bắn vào production thật, có sự đồng ý của chủ hệ thống.
- Binary test đã được xoá khỏi `/tmp` của VPS sau khi đo.
- Không container nào restart; `hr-api-staging` giữ nguyên uptime 8 ngày →
  stack dùng chung không bị gián đoạn.
- Chi phí LLM: log báo ~4,06 triệu token trên 50 request, nhưng con số đó bị
  phồng ~8–17 lần do bug đếm ở §5.2. Thực tế vào khoảng 300–500 nghìn token
  (~5,3k input mỗi request cộng output), chủ yếu trên gemini-3.1-flash-lite và
  một phần deepseek-v4-flash.
- Phần 1000 concurrent **với LLM thật** không được chạy (sẽ tốn 60–90M token);
  con số ở §6 là ngoại suy từ throughput đo được, đã ghi rõ là ước lượng.
