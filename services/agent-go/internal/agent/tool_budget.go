package agent

import (
	"encoding/json"
	"fmt"

	"github.com/ai-agent-tut/agent-go/internal/provider"
	"github.com/ai-agent-tut/agent-go/internal/tools"
)

// canonicalToolCallKey trả về khoá chuẩn hoá (tên tool + args đã chuẩn hoá) để
// phát hiện tool call TRÙNG LẶP HỆT NHAU trong CÙNG một batch (log dev thật đã
// thấy model tự gọi "notes.search notes.search" 2-3 lần với args giống hệt
// trong CÙNG một lượt phản hồi).
//
// Chuẩn hoá args bằng round-trip qua interface{} rồi marshal lại:
// encoding/json.Marshal sắp xếp key map theo alphabet, nên 2 JSON tương đương
// nhưng khác thứ tự key (`{"a":1,"b":2}` vs `{"b":2,"a":1}`) vẫn khớp — model
// sinh JSON không đảm bảo thứ tự key ổn định giữa các lần gọi.
//
// Nếu args không parse được (JSON hỏng), dùng thẳng chuỗi gốc làm khoá — an
// toàn hơn là crash, chỉ mất cơ hội dedup cho case hiếm gặp này (JSON hỏng đã
// được tool tự báo lỗi ở tầng Execute, không liên quan tới dedup).
func canonicalToolCallKey(name string, args json.RawMessage) string {
	var v any
	if err := json.Unmarshal(args, &v); err != nil {
		return name + "|" + string(args)
	}
	canon, err := json.Marshal(v)
	if err != nil {
		return name + "|" + string(args)
	}
	return name + "|" + string(canon)
}

// dedupGroups gom các ToolCall theo canonicalToolCallKey. Trả về:
//   - order: thứ tự các khoá xuất hiện lần đầu (để duyệt kết quả ổn định)
//   - exec: 1 ToolCall ĐẠI DIỆN cho mỗi khoá — chỉ những call này cần THỰC THI
//   - groups: khoá → TẤT CẢ ToolCall gốc thuộc nhóm đó (kể cả đại diện), theo
//     đúng thứ tự xuất hiện — dùng để gán lại kết quả cho từng bản sao sau khi
//     đại diện đã chạy xong.
type dedupGroups struct {
	order  []string
	exec   []provider.ToolCall
	groups map[string][]provider.ToolCall
}

// dedupeSafeCalls nhóm các tool call TRÙNG LẶP HỆT NHAU lại, NHƯNG CHỈ cho tool
// KindRead (không side-effect). Tool KindWrite/KindDestructive KHÔNG bao giờ bị
// dedupe — dù hiếm, một số tool có thể cố ý được gọi lặp lại vì side-effect (vd
// ghi 2 note riêng biệt tình cờ có cùng nội dung), nên chỉ dedupe khi chắc chắn
// an toàn: đọc dữ liệu không đổi kết quả giữa 2 lần gọi trong cùng 1 batch.
func dedupeSafeCalls(reg *tools.Registry, calls []provider.ToolCall) dedupGroups {
	groups := make(map[string][]provider.ToolCall, len(calls))
	order := make([]string, 0, len(calls))
	exec := make([]provider.ToolCall, 0, len(calls))

	for _, tc := range calls {
		key := tc.ID // mặc định: mỗi call là 1 nhóm riêng (ID luôn duy nhất, không bao giờ dedupe)
		if t, ok := reg.Get(tc.Name); ok && t.Kind() == tools.KindRead {
			key = canonicalToolCallKey(tc.Name, tc.Args)
		}
		if _, seen := groups[key]; !seen {
			order = append(order, key)
			exec = append(exec, tc) // call ĐẦU TIÊN của nhóm sẽ là đại diện thực thi
		}
		groups[key] = append(groups[key], tc)
	}
	return dedupGroups{order: order, exec: exec, groups: groups}
}

// duplicateToolResultNote là output thay thế cho các bản sao trùng lặp (không
// phải đại diện được thực thi). NGẮN có chủ đích: đây là khoản tiết kiệm CONTEXT
// TOKEN thật — nếu lặp lại y nguyên nội dung cho mỗi bản sao, model vẫn phải trả
// token đọc lại y hệt dữ liệu nhiều lần dù không tốn thêm 1 lần thực thi nào.
func duplicateToolResultNote(toolName string) string {
	return fmt.Sprintf("[Trùng với một lệnh gọi %q khác ở trên (cùng tham số) trong cùng lượt này — "+
		"kết quả giống hệt, xem output của lần gọi đầu tiên.]", toolName)
}

// applyToolOutputBudget áp 2 lớp giới hạn lên output của MỘT tool call:
//  1. perCallMax: trần cho riêng call này (như capToolOutput cũ).
//  2. totalBudget: ngân sách TỔNG cộng dồn qua CẢ LƯỢT CHẠY — chặn trường hợp
//     nhiều tool call, mỗi cái riêng lẻ đều dưới trần, nhưng CỘNG DỒN vượt quá
//     ngân sách cho phép (log dev thật: 46.542 input token ở step 4, không có
//     tool call đơn lẻ nào chạm MaxToolOutput=24.000, nhưng 4 lần rag.search +
//     nhiều notes.search cộng dồn qua các step thì có).
//
// alreadyUsed là số ký tự đã tính vào ngân sách TỪ TRƯỚC (các tool call trước đó
// trong cùng lượt). Trả về: output đã xử lý, số ký tự output đó chiếm dụng thêm
// (để caller cộng dồn vào alreadyUsed cho tool call tiếp theo), và cờ có bị cắt
// thêm vì ngân sách tổng hay không (riêng biệt với việc bị cắt vì perCallMax).
func applyToolOutputBudget(output string, perCallMax, totalBudget, alreadyUsed int) (result string, usedRunes int, budgetTruncated bool) {
	capped, _ := capToolOutput(output, perCallMax)

	if totalBudget <= 0 {
		return capped, len([]rune(capped)), false
	}

	remaining := totalBudget - alreadyUsed
	if remaining <= 0 {
		// Ngân sách đã cạn TỪ TRƯỚC call này — tool vẫn đã chạy (không biết
		// trước kích thước kết quả để mà chặn sớm), nhưng nội dung KHÔNG được
		// đưa vào context nữa, chỉ còn ghi chú giải thích.
		note := fmt.Sprintf("[Ngân sách tool-output của lượt này đã dùng hết (%d ký tự). "+
			"Tool đã chạy xong nhưng kết quả KHÔNG được đưa vào ngữ cảnh — "+
			"hãy tổng hợp câu trả lời từ những gì đã có, hoặc hỏi người dùng thu hẹp phạm vi thay vì gọi thêm tool.]",
			totalBudget)
		return note, len([]rune(note)), true
	}

	runes := []rune(capped)
	if len(runes) <= remaining {
		return capped, len(runes), false
	}

	note := fmt.Sprintf("\n\n[... bị cắt thêm vì gần hết ngân sách tool-output CỦA CẢ LƯỢT (còn %d/%d ký tự). "+
		"Hãy tổng hợp câu trả lời, hạn chế gọi thêm tool.]", remaining, totalBudget)
	truncatedResult := string(runes[:remaining]) + note
	return truncatedResult, len([]rune(truncatedResult)), true
}
