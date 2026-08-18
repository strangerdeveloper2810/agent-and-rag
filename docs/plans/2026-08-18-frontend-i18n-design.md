# Frontend i18n (vi/en) — Design

Date: 2026-08-18
Status: Approved (design), pending implementation plan

## Mục tiêu

Thêm tính năng chuyển đổi ngôn ngữ (tiếng Việt / tiếng Anh) cho `apps/web`, bao gồm:
1. Dịch toàn bộ UI copy (label, button, toast, error message tĩnh).
2. AI (JARVIS, chạy trên `services/agent-go`) trả lời theo đúng ngôn ngữ UI đang chọn.

Phạm vi được xác nhận: **cả UI + AI response** (không chỉ dịch giao diện), dùng **react-i18next**, **không** thêm locale prefix vào URL.

## Khảo sát hiện trạng (tóm tắt)

- `apps/web`: React 19 + Vite, router `<Routes>` phẳng, `main.tsx` = `BrowserRouter > ToastProvider > App`. Chưa có thư viện i18n nào trong toàn bộ monorepo.
- Có sẵn pattern lưu preference qua `localStorage` (`ThemeToggle` + `initTheme()`, key `jarvis-theme`) — dùng làm khuôn mẫu cho việc lưu locale.
- ~19/55 file trong `apps/web/src` có text hardcode (chủ yếu tiếng Việt, vài chỗ tiếng Anh không nhất quán ở `Header.tsx`, `DocumentsView.tsx`). Không có lớp constants/strings tập trung. Top file nhiều hardcode nhất: `modules/chat/components/MessageBubble.tsx`, `design-system/organisms/Sidebar.tsx`, `modules/chat/components/Composer.tsx`, `design-system/organisms/Header.tsx`, `pages/auth/*`.
- `ApiError` (`apps/web/src/lib/http.ts`) đã có sẵn field `code` bên cạnh `message` — dùng để mapping lỗi sang i18n thay vì hiện message thô từ backend.
- `packages/ui` (@app/ui, dùng chung, Vite resolve trực tiếp source qua alias — không có bước build/dist riêng) có 3 chỗ hardcode text tiếng Anh: `ConfirmDialog` (`confirmLabel="Confirm"`, `cancelLabel="Cancel"`), `ThemeToggle` ("Light"/"Dark" + aria-label), `Toast` (aria-label="Dismiss"). Không nên import i18n lib trực tiếp vào package dùng chung vì sẽ ép mọi consumer tương lai phải kéo theo dependency.
- `apps/web/src/design-system` trùng lặp gần như copy-paste với `packages/ui` (Button, Badge, Avatar, Card, Kbd, ThemeToggle, NavTab, SuggestionChip, Toast, ConfirmDialog) — style khác nhau (Tailwind token kiểu shadcn vs CSS variables). Rủi ro: sửa hardcode có thể phải sửa ở cả 2 nơi.
- Enum không có bảng label sẵn: `Task.status/priority`, `ToolCallState.status`, `ChatEvent.type` (định nghĩa ở `packages/types`) — mapping sang text hiển thị hiện nằm rải rác ở `ChatPage.tsx`, `ToolCallCard.tsx`, `Sidebar.tsx`.
- Backend: không có xử lý `Accept-Language`/locale nào ở `apps/api` (BFF Fastify) hay `services/agent-go`. **`services/agent-go/internal/agent/context.go:39`** (hàm `BuildSystemPrompt`) hard-code chỉ thị `"LUÔN trả lời bằng tiếng Việt (trừ khi user yêu cầu ngôn ngữ khác)."` — không có tham số locale. Không có cột lưu preference ngôn ngữ trong DB `users`.

## Kiến trúc & luồng dữ liệu

Nguồn sự thật cho locale: giá trị `"vi" | "en"` lưu tại `localStorage` key `jarvis-lang`, mặc định `"vi"` (khớp nội dung gốc hiện tại).

```
main.tsx
  → initLocale()  // đọc localStorage, fallback navigator.language, default "vi"
  → i18n.init({ lng, fallbackLng: "vi", resources: {...} })
  → createRoot(...).render(<I18nextProvider i18n={i18n}><BrowserRouter>...</BrowserRouter></I18nextProvider>)
```

Đổi ngôn ngữ (LanguageSwitcher trong Header, cạnh ThemeToggle):
```
click → i18n.changeLanguage("en") → localStorage.setItem("jarvis-lang", "en")
      → mọi component dùng useTranslation() tự re-render (react-i18next built-in)
```

AI trả lời đúng ngôn ngữ (không cần lưu locale ở DB — mỗi request tự mang theo `lang` hiện tại, agent-go xử lý stateless):
```
FE: mỗi POST chat (SSE) → kèm field lang: i18n.language
BFF (apps/api): nhận lang từ body → forward nguyên vẹn tới agent-go
agent-go: handler đọc lang → BuildSystemPrompt(memories, skills, lang)
  → context.go:39 chọn dòng chỉ thị tương ứng theo lang
```

## Cấu trúc file i18n (frontend)

Chia namespace theo module, mỗi namespace = 1 file JSON riêng trong từng thư mục locale (không gộp chung 1 file to):

```
apps/web/src/i18n/
  index.ts              # init i18next, import resources, initLocale()
  locales/
    vi/
      common.json       # Xác nhận, Hủy, Xóa, Lưu, Thử lại, Đăng xuất, Đang tải, Tìm kiếm, Sáng/Tối, Đã chép...
      auth.json         # LoginPage, RegisterPage, VerifyEmailPage
      chat.json         # MessageBubble, Composer, EmptyState, ChatPage, ModeSelector, Markdown,
                         # ToolCallCard, CitationList + label ChatEvent.type/ToolCallState.status
      documents.json    # DocumentsView + label Task.status/priority
      layout.json       # Header, Sidebar, AppLayout (nav, search placeholder, logout confirm...)
      errors.json       # mapping theo ApiError.code
    en/
      common.json
      auth.json
      chat.json
      documents.json
      layout.json
      errors.json
```

Key convention: flat key trong từng file, vd `common.json`: `{ "confirm": "Xác nhận", "cancel": "Hủy", "retry": "Thử lại" }`. Dùng `const { t } = useTranslation("chat"); t("retry")`, hoặc chéo namespace `t("common:confirm")`.

Quy mô app hiện tại (55 file) đủ nhỏ để bundle tĩnh toàn bộ resources (import trực tiếp JSON), không cần `i18next-http-backend` hay lazy-load namespace theo route ở giai đoạn này.

## `packages/ui` (shared) & mapping lỗi

**`packages/ui`:** không import i18n lib vào package dùng chung. Sửa `ConfirmDialog`/`ThemeToggle`/`Toast` nhận label qua props (giữ default tiếng Anh hiện tại làm fallback, không breaking change), `apps/web` luôn truyền chuỗi đã dịch qua props bằng `t()` (namespace `common`). Trước khi sửa: audit xem `Header.tsx`/`Sidebar.tsx` đang import bản `@app/ui` hay bản trùng lặp trong `apps/web/src/design-system`, sửa đúng bản đang thực sự được dùng.

**Mapping lỗi (`errors.json`):** thay vì hiện thẳng `ApiError.message` (`lib/http.ts:105`):
```
catch (ApiError) → t(`errors:${err.code}`, { defaultValue: t("common:genericError") })
```
Code lạ/không map được → fallback về câu chung đã dịch, không hiện `message` thô từ backend (tránh lẫn ngôn ngữ). Áp dụng cho `LoginPage`, `RegisterPage`, `VerifyEmailPage`, `DocumentsView`, `ChatPage`.

`ErrorBoundary.tsx` (lỗi runtime JS, không có `code`): không dịch `error.message` thô — thay bằng câu chung tĩnh đã dịch ("Đã có lỗi xảy ra, vui lòng tải lại trang"), ẩn chi tiết kỹ thuật (chỉ hiện ở dev mode nếu cần).

Giả định cần verify lúc code: BFF phải trả `code` nhất quán ở mọi lỗi để FE map được; nếu thiếu, tự rơi về fallback chung nên không vỡ nhưng nên rà lại.

## Backend: BFF + agent-go (AI trả lời theo ngôn ngữ UI)

**agent-go (`services/agent-go/internal/agent/context.go`):** đổi chữ ký `BuildSystemPrompt(memories []string, skillSummaries []skills.SkillSummary)` → thêm `lang string`. Dòng 39 đổi thành nhánh điều kiện:
```go
if lang == "en" {
    b.WriteString("- ALWAYS respond in English (unless the user explicitly asks otherwise).\n")
} else {
    b.WriteString("- LUÔN trả lời bằng tiếng Việt (trừ khi user yêu cầu ngôn ngữ khác).\n")
}
```
`lang == ""` → coi như `"vi"` (không đổi hành vi hiện tại). `cmd/jarvis/main.go` (CLI, không qua web UI) truyền `"vi"` cứng.

**BFF (`apps/api`):** endpoint nhận tin nhắn chat thêm field `lang?: "vi" | "en"` vào schema request, forward nguyên vẹn khi gọi agent-go. Contract request chính xác giữa BFF ↔ agent-go cần xác định lúc implement (chưa nằm trong khảo sát đã có).

**`packages/api-client`:** hàm gửi tin nhắn thêm `lang: i18n.language` vào body mỗi request.

**Giới hạn phạm vi chấp nhận được:** chỉ đổi chỉ thị ngôn ngữ trả lời, không dịch `skillSummaries`/`memories` — LLM xử lý ngữ cảnh đa ngôn ngữ tốt, rủi ro thấp.

## Kế hoạch triển khai theo phase

1. **Phase 0 — Hạ tầng i18n (frontend):** cài `i18next` + `react-i18next`, tạo `apps/web/src/i18n/` theo cấu trúc trên, `initLocale()` (mẫu `initTheme()`), wire `I18nextProvider` vào `main.tsx`, thêm `LanguageSwitcher` (atom mới) cạnh `ThemeToggle` trong `Header.tsx`.
2. **Phase 1 — Extract string** theo thứ tự: `modules/chat/*` → `design-system/organisms` (Header, Sidebar) → `pages/auth/*` → `modules/documents/DocumentsView` → label enum.
3. **Phase 2 — Chuẩn hóa lỗi:** audit BFF trả `code` nhất quán, viết `errors.json`, sửa 5 chỗ hiện `message` thô, sửa `ErrorBoundary`.
4. **Phase 3 — `packages/ui` shared:** audit import thực tế, sửa props `ConfirmDialog`/`ThemeToggle`/`Toast`, truyền chuỗi dịch từ `apps/web`.
5. **Phase 4 — Backend locale passthrough:** `BuildSystemPrompt` thêm `lang`, BFF thêm field `lang` + forward, `api-client` gửi kèm `lang`.
6. **Phase 5 — QA thủ công:** đổi ngôn ngữ → F5 giữ nguyên → AI trả lời đúng tiếng Anh khi chọn `en` → lỗi hiển thị đúng ngôn ngữ → rà lại 19 file đã liệt kê để không sót hardcode.

## Rủi ro/tech debt đã biết, không xử lý trong phase này

`apps/web/src/design-system` trùng lặp `packages/ui` là tech debt có sẵn — chỉ sửa các component liên quan trực tiếp tới i18n ở cả 2 nơi nếu cả 2 đang thực sự được dùng, không dedupe toàn bộ trong lần này.
