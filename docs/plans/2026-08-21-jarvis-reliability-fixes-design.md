# JARVIS agent-go Reliability Fixes — Design

**Nguồn**: phát hiện từ log production `jarvis-agent-go` (VPS `hr-vps`, container `jarvis-agent-go`) trong phiên test GitHub MCP server ngày 2026-08-19/21. Xem chi tiết trace trong lịch sử hội thoại — không lặp lại ở đây.

## Bối cảnh

3 vấn đề tìm thấy qua log thật, cộng 1 yêu cầu bổ sung từ user:

1. **Routing sai**: `orchestrator.route()` chỉ match keyword thô trên đúng 1 tin nhắn hiện tại, không biết agent nào đang xử lý hội thoại. Root cause cụ thể: keyword `"tìm hiểu"` trong `TriggerKeywords` của agent `research` khớp ngay vào câu hỏi mà JARVIS tự đặt ra trước đó, bị lặp lại trong format `"Q: ...\nA: ..."` khi user trả lời `ask_user`.
2. **Reflection đốt quota Gemini**: gate `worthLearning()` gần vô hiệu (điều kiện OR "assistant dài > 400 rune" hầu như luôn đúng), và reflection dùng chung provider chain 9+ Gemini variant với chat chính → mỗi lượt cascade qua nhiều biến thể khi 429 trước khi rơi xuống DeepSeek/Anthropic.
3. **Hallucination không grounding**: model đọc đúng tool result (đã verify qua SHA khớp) nhưng vẫn bịa tech stack khác hoàn toàn. System prompt (`BuildSystemPrompt`) không có bất kỳ rule nào yêu cầu bám sát tool output.
4. **[Bổ sung]** User có sẵn 1 API key Claude muốn tận dụng làm fallback cho chain chat chính, và muốn có key Claude thứ 2 làm lớp fallback tiếp theo nếu key 1 hết quota.

## Quyết định thiết kế (đã duyệt qua brainstorming)

### 1. Routing — `internal/orchestrator/orchestrator.go`
- Parse input dạng `"Q: ...\nA: ..."`: chỉ dùng phần sau `"A: "` để match keyword, bỏ phần `"Q: "` (câu hỏi do JARVIS tự đặt).
- Sticky agent: `stickyAgent map[string]stickyEntry{agentName string, lastUsed time.Time}` trong `Orchestrator`, bảo vệ bằng `sync.RWMutex` (cùng pattern `triggerRegexCache` đã có). Trong `Run()`: input dạng Q:/A: + đã có sticky cho `conversationID` → dùng luôn sticky, bỏ qua keyword matching. Ngược lại route như cũ rồi ghi sticky. Entry > 24h khi đọc lại coi như chưa có (không cần sweep định kỳ, YAGNI cho cap kích thước map).

### 2. Quota (reflection) — `internal/memory/{learner_gate,reflection,learner}.go`, `internal/provider/factory/factory.go`
- Bỏ nhánh OR "assistant dài > 400 rune" trong `worthLearning()`. Chỉ giữ: keyword match HOẶC user message dài > `trivialUserRunes`.
- `factory.NewReflectionProvider(cfg)`: DeepSeek đơn nếu có `DEEPSEEK_API_KEY`, else fallback `factory.New(cfg)` (giữ hành vi cũ) + log warning. `cmd/server/main.go` dùng cho `NewLearner` thay vì tái dùng `prov`.
- Batch N lượt (default 3, env `REFLECTION_BATCH_TURNS`): `Learner` giữ `turnCounter map[string]int` (mutex). Chưa đạt N → bỏ qua. Đạt N → reset, chạy thật, **và** truyền `windowMessages = 2*N` cho `ReflectAndExtract` (thay hằng số cứng `maxReflectionMessages = 4`) để không mất fact của các lượt bị gộp.

### 3. Claude 2-key fallback — `internal/config/config.go`, `internal/provider/factory/factory.go`
- `cfg.AnthropicKey2` (env `ANTHROPIC_API_KEY_2`, optional). `factory.newAuto()`: nếu có, build thêm `anthropic.New(key2, model)` append ngay sau Claude key 1. Chain cuối: `Gemini (mọi variant) → DeepSeek → Claude key 1 → Claude key 2`.
- Không cần sửa `fallback.go` — cooldown/circuit-breaker theo **vị trí trong chain**, không theo tên (`namedProvider` array, index-addressed).
- Cosmetic: wrapper nhỏ trong `factory.go` override `Name()` → `"anthropic-1"`/`"anthropic-2"` cho log rõ ràng.

### 4. Hallucination — `internal/agent/context.go`
- Thêm bullet `GROUNDING VÀO KẾT QUẢ TOOL` vào section `[QUY TẮC]` (phần cacheable) của `BuildSystemPrompt()`, đặt gần cụm rule "CHỌN TOOL TRA CỨU LINH HOẠT". Áp dụng chung cho cả 3 agent (general/code/research) vì hallucination có thể xảy ra ở bất kỳ agent nào dùng tool.

## Thứ tự implement & deploy

Routing → Quota → Claude 2-key → Hallucination. Mỗi phần deploy riêng qua `deploy/deploy-to-vps.sh`, verify qua `ssh hr-vps "docker logs --tail 200 --timestamps jarvis-agent-go"` trước khi làm phần tiếp theo.

**Rủi ro cao nhất**: Routing (sticky agent) — parse Q:/A: sai có thể kẹt agent sai cả hội thoại. Quan sát log ổn định trước khi tiếp tục các phần sau.

## Ngoài phạm vi (không làm trong đợt này)
- Cap kích thước map `stickyAgent` (YAGNI ở quy mô hiện tại).
- Đổi model mặc định cho agent `code` (Hallucination hướng C) — chỉ cân nhắc nếu A+B (prompt) chưa đủ sau khi test thật.
- Mã hoá `auth_token` MCP server tại rest (nợ kỹ thuật đã biết từ trước, không thuộc phạm vi đợt này).
