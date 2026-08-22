package http

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/ai-agent-tut/agent-go/internal/agent"
	"github.com/ai-agent-tut/agent-go/internal/orchestrator"
)

// PausedRunStore đọc/xoá checkpoint MỚI NHẤT của State đã lưu cho một run_id
// (không chỉ khi Engine dừng ở NodeInterrupt — xem internal/agent/engine.go
// checkpoint/saveInterruptedState) — implement bởi *sqlite.Store (bảng
// paused_runs, xem internal/storage/sqlite/paused_runs.go). Interface NHỎ ở
// đây để package http không phải phụ thuộc thẳng internal/storage/sqlite —
// cùng idiom với MongoPinger/ConversationLearner trong chat.go.
type PausedRunStore interface {
	LoadInterruptedState(runID string) (stateJSON []byte, agentName string, err error)
	DeleteInterruptedState(runID string) error
}

// ChatResumeHandler xử lý POST /chat/resume — tiếp tục MỘT lượt chạy đã dừng
// giữa chừng, ở BẤT KỲ node nào:
//   - NodeInterrupt (guardrails chặn tool destructive, chờ user xác nhận) —
//     cần "answer" trong body.
//   - Node khác (tiến trình agent-go crash/restart giữa chừng, không phải
//     HITL) — "answer" không cần thiết, bỏ qua nếu client có gửi.
//
// Xem internal/agent/resume.go cho cơ chế đầy đủ + giới hạn còn lại.
type ChatResumeHandler struct {
	orch  *orchestrator.Orchestrator
	store PausedRunStore
}

// NewChatResumeHandler tạo ChatResumeHandler. orch dùng để tìm lại ĐÚNG Engine
// (general/code/research) đã tạo ra run gốc — xem comment field
// agent.Engine.name — vì mỗi agent có tool registry khác nhau.
func NewChatResumeHandler(orch *orchestrator.Orchestrator, store PausedRunStore) *ChatResumeHandler {
	return &ChatResumeHandler{orch: orch, store: store}
}

// ChatResumeRequest là body JSON client gửi lên: run_id nhận từ Event{Type:
// "interrupt", RunID: "..."} của lượt chat gốc (hoặc run_id đã biết từ trước
// nếu resume sau crash, không qua interrupt). answer là câu trả lời của user
// cho câu hỏi xác nhận tool destructive (vd "yes"/"có" để đồng ý chạy, bất kỳ
// giá trị nào khác bị coi là từ chối — xem agent.ResolveInterrupt) — CHỈ BẮT
// BUỘC nếu state đã lưu cho run_id này đang chờ interrupt (state.Interrupt !=
// nil); bỏ trống nếu resume từ 1 checkpoint không phải interrupt (vd sau khi
// tiến trình agent-go crash/restart giữa chừng ở NodeModel/NodeTools...).
type ChatResumeRequest struct {
	RunID  string `json:"run_id"`
	Answer string `json:"answer"`
}

// ServeHTTP implements http.Handler. Response là SSE giống /chat (client dùng
// chung code xử lý stream), NGOẠI TRỪ các lỗi xảy ra TRƯỚC khi engine chạy
// tiếp (run_id không tồn tại, state hỏng, thiếu answer khi đang chờ
// interrupt...) — những lỗi đó trả JSON thường (không phải SSE) vì tại điểm
// đó còn chưa có gì để stream.
func (h *ChatResumeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req ChatResumeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"invalid json: %v"}`, err), http.StatusBadRequest)
		return
	}
	if req.RunID == "" {
		http.Error(w, `{"error":"run_id is required"}`, http.StatusBadRequest)
		return
	}

	stateJSON, agentName, err := h.store.LoadInterruptedState(req.RunID)
	if err != nil {
		slog.Warn("chat_resume: paused run not found", "run_id", req.RunID, "err", err)
		http.Error(w, fmt.Sprintf(`{"error":"paused run not found: %s"}`, req.RunID), http.StatusNotFound)
		return
	}

	// Xoá NGAY sau khi load thành công — dùng 1 lần, tránh state rác tích
	// luỹ trong paused_runs. Xoá TRƯỚC khi chạy resume (không phải sau): nếu
	// resume thất bại giữa chừng (panic, client disconnect...), bản ghi cũ dễ
	// gây nhầm lẫn hơn là mất khả năng retry — user cần bắt đầu lượt hỏi mới.
	if delErr := h.store.DeleteInterruptedState(req.RunID); delErr != nil {
		slog.Warn("chat_resume: xoá paused run thất bại (không chặn resume)", "run_id", req.RunID, "err", delErr)
	}

	spec := h.orch.GetAgent(agentName)
	if spec == nil || spec.Engine == nil {
		slog.Error("chat_resume: agent gốc của run này không còn đăng ký trong orchestrator", "run_id", req.RunID, "agent", agentName)
		http.Error(w, fmt.Sprintf(`{"error":"agent %q not found for this paused run"}`, agentName), http.StatusInternalServerError)
		return
	}

	state, err := agent.DeserializeState(stateJSON)
	if err != nil {
		slog.Error("chat_resume: deserialize state thất bại", "run_id", req.RunID, "err", err)
		http.Error(w, `{"error":"corrupted paused state"}`, http.StatusInternalServerError)
		return
	}

	// state.Interrupt != nil → run này dừng ở NodeInterrupt (HITL), BẮT BUỘC
	// answer để biết đồng ý/từ chối trước khi tiếp tục. state.Interrupt == nil
	// → run dừng ở checkpoint của node KHÁC (vd tiến trình agent-go
	// crash/restart giữa chừng) — bỏ qua ResolveInterrupt hoàn toàn, answer
	// không cần thiết, Resume() bên dưới tự route tiếp từ đúng chỗ.
	if state.Interrupt != nil {
		if strings.TrimSpace(req.Answer) == "" {
			slog.Warn("chat_resume: thiếu answer cho run đang chờ interrupt", "run_id", req.RunID)
			http.Error(w, `{"error":"answer is required: this run is paused waiting for user confirmation"}`, http.StatusBadRequest)
			return
		}
		if err := spec.Engine.ResolveInterrupt(r.Context(), state, req.Answer); err != nil {
			slog.Error("chat_resume: resolve interrupt thất bại", "run_id", req.RunID, "err", err)
			http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusBadRequest)
			return
		}
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	emit := func(e agent.Event) {
		data, _ := json.Marshal(e)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	_, _ = spec.Engine.Resume(r.Context(), state, emit)
}
