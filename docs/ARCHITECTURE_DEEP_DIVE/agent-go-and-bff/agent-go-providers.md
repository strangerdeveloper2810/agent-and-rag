# agent-go — Provider layer: auto-fallback, RouterProvider, cost ledger

← [Về mục lục](./README.md)

## Auto-fallback chain

`factory.New(cfg)` với `LLM_PROVIDER=auto` dựng chain theo thứ tự **cố định**:

```
Gemini (toàn bộ free-tier pool: primary → secondary → fallback models)
  → DeepSeek (rẻ, pay-as-you-go)
    → Claude key 1
      → Claude key 2 (optional, ANTHROPIC_API_KEY_2 — thêm sau khi 1 key hết quota giữa lúc demo)
```

`internal/provider/fallback.Provider` theo dõi cooldown/lỗi liên tiếp **theo VỊ TRÍ trong chain** (`&chain[i]`), không theo tên provider — nên 2 client Claude khác key vẫn độc lập nhau dù cùng gọi là "anthropic". Khi có 2 key, cả 2 được bọc qua `namedOverride` để log hiện `anthropic-1`/`anthropic-2` phân biệt được đang chạy key nào (chỉ áp dụng khi có key thứ 2 — 1 key thì tên vẫn là `"anthropic"` như cũ).

Riêng **reflection nền** (Learner, xem [`agent-go-memory-and-mcp.md`](./agent-go-memory-and-mcp.md)) dùng provider **RIÊNG** (`factory.NewReflectionProvider` — DeepSeek đơn, không qua chain Gemini) để không cạnh tranh quota Gemini với chat chính của user đang chờ trực tiếp.

---

## Provider local: Ollama + openai_compat

Ngoài 3 provider cloud trên, `internal/provider/ollama` (local Ollama, dùng đúng role `"tool"` chuẩn — trước đây giả role `"user"`, đã fix) và `internal/provider/openai_compat` (clone từ adapter DeepSeek, tổng quát hoá cho bất kỳ server local nào nói chuẩn OpenAI chat/completions — vLLM, llama.cpp server, LM Studio) cho phép chạy hoàn toàn offline/miễn phí. Cả hai được wire vào `factory.New` qua `LLM_PROVIDER=ollama` hoặc `LLM_PROVIDER=openai_compat`.

## RouterProvider — kết hợp local + cloud theo request

`LLM_PROVIDER=router` dựng `internal/provider/router.Router`, bọc 2 provider:

- **Local** (`ROUTER_LOCAL_BACKEND=ollama|openai_compat`, mặc định `ollama`) — dùng khi request `ThinkingLevel == ThinkingOff` **và** không có tool nào (`len(req.Tools) == 0`). Đây là case rẻ nhất về mặt suy luận (không cần tool-calling phức tạp, không cần suy luận sâu) nên đẩy sang model local miễn phí là hợp lý.
- **Cloud** (tái dùng nguyên `newAuto(cfg)` — chuỗi fallback ở trên) — mọi request còn lại.

**Fallback local → cloud chỉ trong 1 trường hợp**: `local.Generate()` lỗi NGAY LẬP TỨC (trước khi có channel trả về — ví dụ Ollama server không chạy). Nếu lỗi xuất hiện GIỮA CHỪNG stream (dưới dạng `ChunkError` trên channel đã trả về, kể cả ở chunk đầu tiên), Router **KHÔNG** fallback — forward nguyên trạng lỗi đó cho caller. Lý do: một khi đã trao channel, phía trên có thể đang stream nội dung cho end-user; đổi provider giữa chừng sẽ tạo ra 1 response lẫn lộn 2 nguồn khác nhau, tệ hơn là trả lỗi rõ ràng.

---

## Pricing & Cost Ledger (per-tenant)

`internal/provider/pricing` giữ 1 bảng giá ước tính USD/1M token cho từng `provider:model` (Gemini/Anthropic/DeepSeek — Ollama/openai_compat coi như $0 vì tự host). **Bảng giá KHÔNG được coi là sự thật tuyệt đối** — giá LLM thay đổi liên tục, code có comment nhắc verify lại trang pricing chính thức, và cho phép override qua `PRICING_OVERRIDE_JSON` (path tới file JSON merge đè lên default).

`Engine.SetCostLedger(ledger)` (interface nhỏ định nghĩa cục bộ trong package `agent`, giống mẫu `InterruptStore` — tránh import cycle ngược vào `internal/storage/sqlite`) ghi 1 dòng vào bảng `cost_ledger` sau mỗi lượt `runLoop` **có phát sinh usage mới** (dùng delta token so với lúc bắt đầu lượt chạy, không phải tổng cộng dồn — quan trọng để resume sau interrupt không tính trùng token đã tính trước khi dừng). Tính cả `HypotheticalMaxCostUSD` (nếu lượt đó dùng provider đắt nhất trong bảng giá) để suy ra `SavingsUSD`.

**TẮT MẶC ĐỊNH** (`ENABLE_COST_LEDGER=false`) — đây là side-effect ghi SQLite thêm cho MỌI request, không phải chỉ khi cần, nên phải opt-in tường minh (bài học từ 1 lần lỡ merge tính năng này bật ngầm định — xem changelog PR liên quan).

CLI `jarvis cost <tenantID>` đọc `TenantCostSummary` từ cùng file SQLite (`JARVIS_DB_PATH`), in tổng chi phí, tiết kiệm ước tính, và breakdown theo provider — không cần dựng LLM provider/orchestrator đầy đủ, chỉ mở DB.
