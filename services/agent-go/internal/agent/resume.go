// Resume TỔNG QUÁT — tiếp tục MỘT lượt chạy đã dừng giữa chừng ở BẤT KỲ node
// nào, không chỉ NodeInterrupt (HITL). Đây là bản mở rộng của thiết kế gốc
// (resume tối giản, chỉ áp dụng cho NodeInterrupt) — xem engine.go
// (checkpoint/saveInterruptedState/deleteCheckpoint, gọi từ runLoop) cho cơ
// chế lưu snapshot SAU MỖI LẦN CHUYỂN NODE.
//
// Có 2 lý do một run dừng giữa chừng và cần resume:
//
//  1. HITL: guardrails chặn 1 tool call KindDestructive ("confirm_destructive"
//     — xem node_tools.go). Engine dừng ở NodeInterrupt, state.Interrupt != nil.
//     Cần Answer của user để biết đồng ý/từ chối trước khi tiếp tục.
//  2. Crash/restart: tiến trình agent-go bị dừng đột ngột (crash, deploy,
//     OOM...) giữa lúc đang chạy 1 node BẤT KỲ khác (NodeModel, NodeTools,
//     NodeReflect...). state.Interrupt == nil — không cần Answer, chỉ cần
//     route() tính lại đúng node kế tiếp dựa trên state đã checkpoint.
//
// Luồng xử lý chung (xem internal/transport/http/chat_resume.go):
//
//  1. Load lại State đã lưu theo run_id (sqlite.Store.LoadInterruptedState).
//  2. NẾU state.Interrupt != nil: gọi Engine.ResolveInterrupt(ctx, state,
//     answer) — diễn giải answer (đồng ý/từ chối), THỰC THI THẬT tool bị chặn
//     nếu đồng ý, ghi tool result vào state.Messages, rồi xoá state.Interrupt.
//     answer là BẮT BUỘC trong trường hợp này (400 nếu thiếu).
//     NẾU state.Interrupt == nil: bỏ qua bước này hoàn toàn — answer không
//     cần thiết (bỏ qua nếu client có gửi).
//  3. Gọi Engine.Resume(ctx, state, emit) — route() tự quyết định node kế
//     tiếp dựa trên state hiện tại (đã cập nhật ở bước 2 nếu có), KHÔNG chạy
//     lại từ đầu (NodeRecall).
//  4. Xoá bản ghi paused_runs (dùng 1 lần) — dù bước 2/3 thành công hay lỗi.
//
// GIỚI HẠN CÒN LẠI (chưa giải quyết, cần idempotency key cho tool call ở
// sprint sau nếu muốn triệt để): checkpoint được lưu Ở RANH GIỚI GIỮA 2 NODE
// (sau khi 1 node dispatch xong, TRƯỚC khi node kế tiếp bắt đầu — xem
// engine.go checkpoint()). Nếu crash xảy ra TRONG LÚC dispatch(NodeTools)
// đang chạy (vd 1 trong nhiều tool call song song đã thực thi thành công,
// tool khác chưa xong), checkpoint gần nhất vẫn là bản TRƯỚC khi NodeTools bắt
// đầu (tool call chưa có kết quả) — resume sẽ chạy lại TOÀN BỘ NodeTools,
// có thể gọi lại (side-effect kép) tool ĐÃ chạy thành công ở lần trước cho
// các tool KindWrite/KindDestructive (vd shell.exec, file.write chạy 2 lần).
// Case "crash NGAY SAU KHI NodeTools chạy xong toàn bộ" (checkpoint đã ghi
// nhận đủ observation) thì AN TOÀN — resume() sẽ route đúng sang NodeModel,
// không gọi lại tool nào — xem TestEngine_Resume_CrashMidNodeTools_KhongGoiLaiTool.
//
// GIỚI HẠN CÓ CHỦ ĐÍCH (từ thiết kế gốc, không đổi): nếu 1 batch tool call
// chặn NHIỀU HƠN 1 tool destructive cùng lúc, State.Interrupt chỉ giữ tool
// ĐẦU TIÊN (xem node_tools.go). Sau khi ResolveInterrupt xử lý xong cái đầu
// tiên và Resume() chạy tiếp, các tool destructive còn lại (nếu có) sẽ bị
// guardrails chặn lại thành 1 Interrupt MỚI — client resume tuần tự từng cái,
// mỗi lần 1 tool.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ai-agent-tut/agent-go/internal/guardrails"
	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// approveAnswers liệt kê các câu trả lời (đã lowercase + trim) được coi là
// ĐỒNG Ý cho tool destructive chạy. Bất kỳ answer nào KHÔNG khớp đều bị coi
// là TỪ CHỐI — an toàn hơn (fail-closed) so với đoán ý định người dùng.
var approveAnswers = map[string]bool{
	"yes": true, "y": true, "ok": true, "okay": true,
	"approve": true, "approved": true, "confirm": true, "confirmed": true,
	"có": true, "co": true, "đồng ý": true, "dong y": true,
	"được": true, "duoc": true, "ừ": true, "u": true,
}

func isApproveAnswer(answer string) bool {
	return approveAnswers[strings.ToLower(strings.TrimSpace(answer))]
}

// ResolveInterrupt diễn giải answer của user cho Interrupt đang chờ trong s,
// rồi XOÁ s.Interrupt để route() cho phép Resume() chạy tiếp. PHẢI gọi hàm
// này TRƯỚC Resume — Resume sẽ báo lỗi nếu s.Interrupt vẫn còn khác nil.
//
// answer khớp approveAnswers (case-insensitive) → THỰC THI THẬT tool đã bị
// guardrails chặn (đúng args ban đầu model đã sinh), ghi tool result như một
// tool call bình thường đã chạy xong. Answer khác → ghi observation LỖI kèm
// nguyên văn answer, để model thấy lý do bị từ chối và tự đề xuất hướng khác
// ở bước kế tiếp — KHÔNG âm thầm bỏ qua tool call đó (assistant message vẫn
// còn ToolCallID chưa trả lời, route() sẽ kẹt ở NodeTools nếu không ghi gì).
func (e *Engine) ResolveInterrupt(ctx context.Context, s *State, answer string) error {
	if s == nil {
		return fmt.Errorf("engine: resolve interrupt: state is nil")
	}
	interrupted := s.Interrupt
	if interrupted == nil {
		return fmt.Errorf("engine: resolve interrupt: state has no pending interrupt (run_id=%s)", s.RunID)
	}
	s.Interrupt = nil

	if !isApproveAnswer(answer) {
		s.AppendObservation(Observation{
			CallID: interrupted.CallID,
			Name:   interrupted.Tool,
			Error:  fmt.Sprintf("User declined to run this tool (answer: %q). Do not retry the same call; suggest an alternative or ask the user how to proceed.", answer),
		})
		return nil
	}

	t, ok := e.registry.Get(interrupted.Tool)
	if !ok {
		s.AppendObservation(Observation{
			CallID: interrupted.CallID,
			Name:   interrupted.Tool,
			Error:  fmt.Sprintf("tool %q not found on resume (registry changed since original call?)", interrupted.Tool),
		})
		return nil
	}

	res, err := t.Execute(ctx, json.RawMessage(interrupted.Args))
	obs := Observation{CallID: interrupted.CallID, Name: interrupted.Tool}
	if err != nil {
		obs.Error = err.Error()
	} else {
		obs.Output = res.Content
	}
	s.AppendObservation(obs)
	return nil
}

// Resume tiếp tục MỘT lượt chạy đã dừng giữa chừng — ở NodeInterrupt (HITL,
// gọi SAU KHI ResolveInterrupt đã xử lý xong answer) HOẶC ở BẤT KỲ node nào
// khác state đã được checkpoint (crash/restart, không cần ResolveInterrupt —
// xem comment đầu file). Trong CẢ HAI trường hợp, s.Interrupt PHẢI là nil khi
// gọi hàm này (Resume tự kiểm tra, trả lỗi ngay nếu không — chốt an toàn
// tránh vòng lặp vô hạn nếu caller quên gọi ResolveInterrupt).
//
// route(s) (hàm THUẦN, chỉ đọc State) tự quyết định node kế tiếp dựa trên
// state hiện tại — KHÔNG cần biết state được checkpoint ở node nào: nếu tool
// call cuối đã có kết quả (đã chạy xong TRƯỚC lúc checkpoint) → NodeModel;
// nếu chưa → NodeTools; v.v. Đây chính là cơ chế cho phép Resume() hoạt động
// đúng cho MỌI checkpoint mà không cần biết "checkpoint này thuộc node nào".
//
// Khác Run(): Resume KHÔNG discovery lại MCP servers (registry riêng của lượt
// gốc — s.mcpRegistry — đã mất khi State bị serialize/deserialize qua JSON,
// xem SerializeForResume) và KHÔNG mở LangSmith root run mới (root run của
// lượt gốc coi như đã kết thúc ở lần dừng).
func (e *Engine) Resume(ctx context.Context, s *State, emit EmitFunc) (provider.Usage, error) {
	if s == nil {
		return provider.Usage{}, fmt.Errorf("engine: resume: state is nil")
	}
	if s.Interrupt != nil {
		return s.Usage, fmt.Errorf("engine: resume: state still has a pending interrupt (call ResolveInterrupt first)")
	}

	// loopBreaker không sống sót qua serialize (unexported field) — dựng lại
	// giống Run() để circuit breaker vẫn hoạt động cho phần còn lại của run.
	if e.circuitBreaker != nil && s.loopBreaker == nil {
		s.loopBreaker = guardrails.NewCircuitBreaker(e.circuitBreaker.MaxRepeats())
	}

	node := route(s)
	if node == NodeInterrupt {
		// Không nên xảy ra (đã kiểm tra s.Interrupt == nil ở trên), nhưng giữ
		// chốt an toàn tường minh thay vì lặp vô hạn nếu route() có logic mới
		// sau này khiến Interrupt được set lại từ điều kiện khác.
		return s.Usage, fmt.Errorf("engine: resume: route() trả về NodeInterrupt dù s.Interrupt đã nil — dừng để tránh vòng lặp")
	}

	return e.runLoop(ctx, node, s, emit, time.Now())
}
