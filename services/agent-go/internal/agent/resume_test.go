package agent

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/provider"
	"github.com/ai-agent-tut/agent-go/internal/tools"
)

// memInterruptStore là fake InterruptStore trong bộ nhớ — đủ để test luồng
// save/load/resume mà không cần sqlite thật (sqlite.Store đã có test riêng ở
// internal/storage/sqlite/paused_runs_test.go). Cũng ghi lại TOÀN BỘ lịch sử
// các lần Save (không chỉ bản mới nhất) — cần cho
// TestEngine_Resume_CrashMidNodeTools_KhongGoiLaiTool để lấy lại một checkpoint
// TRUNG GIAN đã bị ghi đè/xoá sau đó, giả lập việc "process crash right at
// that point" (test không thể kill process thật giữa chừng).
type memInterruptStore struct {
	mu        sync.Mutex
	saved     map[string][]byte
	agentName map[string]string
	history   [][]byte
	deleted   int
}

func newMemInterruptStore() *memInterruptStore {
	return &memInterruptStore{saved: map[string][]byte{}, agentName: map[string]string{}}
}

func (m *memInterruptStore) SaveInterruptedState(runID, agentName string, stateJSON []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := append([]byte(nil), stateJSON...)
	m.saved[runID] = cp
	m.agentName[runID] = agentName
	m.history = append(m.history, cp)
	return nil
}

// DeleteInterruptedState implements agent.InterruptStore (Engine gọi khi run
// kết thúc tự nhiên ở NodeEnd — xem Engine.deleteCheckpoint). Không xoá khỏi
// `history`: test cần đọc lại checkpoint trung gian NGAY CẢ SAU KHI run đã
// kết thúc bình thường và checkpoint cuối cùng đã bị dọn.
func (m *memInterruptStore) DeleteInterruptedState(runID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.saved, runID)
	m.deleted++
	return nil
}

func (m *memInterruptStore) get(runID string) ([]byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.saved[runID]
	return data, ok
}

func (m *memInterruptStore) getHistory() [][]byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([][]byte, len(m.history))
	copy(out, m.history)
	return out
}

// TestEngine_InterruptSaveAndResume_EndToEnd chứng minh trọn vẹn cơ chế resume
// tối giản cho NodeInterrupt (xem resume.go):
//
//  1. Run() gọi 1 tool KindDestructive → guardrails chặn → engine dừng ở
//     NodeInterrupt, emit event interrupt kèm RunID, và (vì có
//     SetInterruptStore) lưu State qua InterruptStore.
//  2. Test giả lập "client gọi POST /chat/resume": load lại State đã lưu
//     (DeserializeState), gọi ResolveInterrupt(answer="yes") — THỰC THI THẬT
//     tool bị chặn, rồi gọi Resume() để engine chạy tiếp NGAY TỪ SAU
//     NodeInterrupt (không chạy lại từ đầu — provider chỉ được gọi thêm
//     ĐÚNG 1 lần nữa, không phải 2 lần cho lượt model đầu).
//  3. Kết quả cuối (final text) phải đúng như provider trả về ở "lượt 2".
func TestEngine_InterruptSaveAndResume_EndToEnd(t *testing.T) {
	destructiveTool := &stubTool{name: "task.delete", kind: tools.KindDestructive, output: "deleted: report.txt"}
	reg := tools.NewRegistry()
	reg.Register(destructiveTool)

	// Lượt 1: model yêu cầu chạy task.delete. Lượt 2 (SAU resume): model thấy
	// tool đã chạy xong (observation trong Messages) và trả lời bằng text —
	// scriptedProvider (engine_usage_integration_test.go, cùng package) trả
	// kịch bản KHÁC NHAU cho mỗi lần gọi, không lặp lại như provider.NewFake.
	step1 := []provider.StreamChunk{
		{Kind: provider.ChunkToolCall, ToolCall: &provider.ToolCall{
			ID: "call-1", Name: "task.delete", Args: []byte(`{"path":"report.txt"}`),
		}},
		{Kind: provider.ChunkDone},
	}
	step2 := []provider.StreamChunk{
		{Kind: provider.ChunkText, Text: "Đã xoá report.txt xong."},
		{Kind: provider.ChunkDone},
	}
	prov := &scriptedProvider{scripts: [][]provider.StreamChunk{step1, step2}}

	eng := NewEngine(prov, reg)
	store := newMemInterruptStore()
	eng.SetInterruptStore(store)
	eng.SetName("code")

	var (
		mu         sync.Mutex
		firstEvts  []Event
		interruptE *Event
	)
	emit1 := func(e Event) {
		mu.Lock()
		defer mu.Unlock()
		firstEvts = append(firstEvts, e)
		if e.Type == "interrupt" {
			ev := e
			interruptE = &ev
		}
	}

	usage1, err := eng.Run(context.Background(), RunInput{
		UserMessage: "xoá report.txt giúp tôi",
		MaxSteps:    5,
	}, emit1)
	if err != nil {
		t.Fatalf("Run (lượt 1): %v", err)
	}
	_ = usage1

	// --- 1. Verify: engine dừng ở interrupt, event mang RunID, state đã lưu ---
	if interruptE == nil {
		t.Fatal("thiếu event interrupt")
	}
	if interruptE.RunID == "" {
		t.Fatal("event interrupt thiếu RunID — client không biết resume bằng run_id nào")
	}
	if doneEvt := hasEvent(firstEvts, "done"); doneEvt == nil {
		t.Error("thiếu event done sau khi dừng ở interrupt (engine vẫn phải emit done cho request hiện tại)")
	}

	savedData, ok := store.get(interruptE.RunID)
	if !ok {
		t.Fatalf("State không được lưu vào InterruptStore cho runID=%s", interruptE.RunID)
	}
	if store.agentName[interruptE.RunID] != "code" {
		t.Errorf("agentName lưu kèm state = %q, want %q", store.agentName[interruptE.RunID], "code")
	}

	// Tool destructive KHÔNG được chạy trước khi user xác nhận.
	if destructiveTool.err == nil && len(destructiveTool.name) > 0 {
		// (không có cách trực tiếp đếm số lần Execute trên stubTool — kiểm
		// tra gián tiếp qua prov.calls dưới đây: nếu tool đã chạy ở lượt 1,
		// model vẫn đứng ở step1 vì route() không quay lại NodeModel khi còn
		// unanswered tool call — do đó prov.calls == 1 là bằng chứng đủ.)
	}
	if prov.calls != 1 {
		t.Fatalf("prov.calls sau lượt 1 = %d, want 1 (chưa được gọi tiếp vì đang dừng ở interrupt)", prov.calls)
	}

	// --- 2. Giả lập client gọi POST /chat/resume ---
	state, err := DeserializeState(savedData)
	if err != nil {
		t.Fatalf("DeserializeState: %v", err)
	}
	if state.RunID != interruptE.RunID {
		t.Errorf("state.RunID = %q sau deserialize, want %q", state.RunID, interruptE.RunID)
	}
	if state.Interrupt == nil {
		t.Fatal("state.Interrupt phải còn tồn tại NGAY SAU khi load lại (trước khi ResolveInterrupt)")
	}

	if err := eng.ResolveInterrupt(context.Background(), state, "yes"); err != nil {
		t.Fatalf("ResolveInterrupt: %v", err)
	}
	if state.Interrupt != nil {
		t.Error("state.Interrupt phải nil sau ResolveInterrupt")
	}

	var (
		mu2       sync.Mutex
		secondTxt []string
	)
	emit2 := func(e Event) {
		mu2.Lock()
		defer mu2.Unlock()
		if e.Type == "text" {
			secondTxt = append(secondTxt, e.Text)
		}
	}

	usage2, err := eng.Resume(context.Background(), state, emit2)
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	_ = usage2

	// --- 3. Verify: engine chạy tiếp (không chạy lại từ đầu) và ra đúng kết quả ---
	if prov.calls != 2 {
		t.Fatalf("prov.calls sau resume = %d, want 2 (đúng 1 lượt gọi model mới, không chạy lại từ đầu)", prov.calls)
	}
	gotText := ""
	for _, tstr := range secondTxt {
		gotText += tstr
	}
	if gotText != "Đã xoá report.txt xong." {
		t.Errorf("text sau resume = %q, want %q", gotText, "Đã xoá report.txt xong.")
	}

	// Observation của tool bị chặn phải có mặt (đã thực thi thật qua ResolveInterrupt).
	found := false
	for _, obs := range state.Scratchpad {
		if obs.Name == "task.delete" && obs.Output == "deleted: report.txt" {
			found = true
		}
	}
	if !found {
		t.Errorf("Scratchpad thiếu observation của task.delete đã thực thi: %+v", state.Scratchpad)
	}
}

// TestEngine_ResolveInterrupt_Decline chứng minh answer KHÔNG khớp
// approveAnswers → tool KHÔNG được chạy, observation ghi lỗi rõ ràng thay vì
// âm thầm bỏ qua tool call.
func TestEngine_ResolveInterrupt_Decline(t *testing.T) {
	destructiveTool := &stubTool{name: "task.delete", kind: tools.KindDestructive, output: "should not run"}
	reg := tools.NewRegistry()
	reg.Register(destructiveTool)

	eng := NewEngine(provider.NewFake(), reg)
	s := stateWithToolCalls("task.delete")
	s.Interrupt = &Interrupt{Reason: "confirm_destructive", Tool: "task.delete", CallID: "c0", Args: "{}"}

	if err := eng.ResolveInterrupt(context.Background(), s, "no thanks"); err != nil {
		t.Fatalf("ResolveInterrupt: %v", err)
	}
	if s.Interrupt != nil {
		t.Error("state.Interrupt phải nil sau ResolveInterrupt dù bị từ chối")
	}
	if len(s.Scratchpad) != 1 || s.Scratchpad[0].Error == "" {
		t.Fatalf("Scratchpad phải ghi 1 observation LỖI (từ chối): %+v", s.Scratchpad)
	}
}

// TestEngine_Resume_ErrorsIfStillInterrupted đảm bảo Resume() fail loud thay
// vì lặp vô hạn nếu caller quên gọi ResolveInterrupt trước.
func TestEngine_Resume_ErrorsIfStillInterrupted(t *testing.T) {
	eng := NewEngine(provider.NewFake(), tools.NewRegistry())
	s := newState(RunInput{UserMessage: "hi", MaxSteps: 5})
	s.Interrupt = &Interrupt{Reason: "confirm_destructive", Tool: "x", CallID: "c0"}

	if _, err := eng.Resume(context.Background(), s, func(Event) {}); err == nil {
		t.Fatal("Resume() phải lỗi khi s.Interrupt vẫn còn khác nil")
	}
}

// TestEngine_Resume_CrashMidNodeTools_KhongGoiLaiTool chứng minh phần TỔNG
// QUÁT HOÁ của resume: một run dừng giữa chừng ở node KHÁC NodeInterrupt
// (mô phỏng tiến trình agent-go crash/restart NGAY SAU KHI NodeTools chạy
// xong 1 tool, TRƯỚC lượt gọi model kế tiếp) phải resume tiếp tục ĐÚNG chỗ mà
// KHÔNG gọi lại tool đã chạy xong — khác hẳn case NodeInterrupt (tool CHƯA hề
// chạy khi dừng, xem TestEngine_InterruptSaveAndResume_EndToEnd).
//
// Vì Go test không thể kill process thật giữa chừng, test lấy lại checkpoint
// TRUNG GIAN mà Engine.checkpoint() đã ghi (sau MỖI lần chuyển node — xem
// engine.go runLoop) ngay sau khi NodeTools hoàn tất, dùng memInterruptStore.
// history để đọc lại checkpoint đó NGAY CẢ SAU KHI lượt chạy gốc đã tới
// NodeEnd bình thường và bản ghi cuối cùng đã bị Engine.deleteCheckpoint xoá —
// đây chỉ là cách quan sát; bản thân cơ chế checkpoint/xoá trong Engine không
// hề biết hay quan tâm tới việc test đang "nhìn trộm" lịch sử này.
func TestEngine_Resume_CrashMidNodeTools_KhongGoiLaiTool(t *testing.T) {
	counter := &countingTool{name: "counter", kind: tools.KindRead, output: "counted"}
	reg := tools.NewRegistry()
	reg.Register(counter)

	step1 := []provider.StreamChunk{
		{Kind: provider.ChunkToolCall, ToolCall: &provider.ToolCall{
			ID: "call-1", Name: "counter", Args: []byte(`{}`),
		}},
		{Kind: provider.ChunkDone},
	}
	step2 := []provider.StreamChunk{
		{Kind: provider.ChunkText, Text: "Xong rồi."},
		{Kind: provider.ChunkDone},
	}
	prov := &scriptedProvider{scripts: [][]provider.StreamChunk{step1, step2}}

	eng := NewEngine(prov, reg)
	store := newMemInterruptStore()
	eng.SetInterruptStore(store)
	eng.SetName("code")

	// --- 1. Lượt chạy "trước khi crash": chạy TRỌN VẸN (test không kill được
	// process thật) — nhưng engine vẫn checkpoint sau MỖI lần chuyển node
	// trong lúc chạy, và store.history giữ lại toàn bộ, kể cả bản đã bị xoá
	// ở cuối khi run tới NodeEnd.
	if _, err := eng.Run(context.Background(), RunInput{
		UserMessage: "đếm giúp tôi",
		MaxSteps:    5,
	}, func(Event) {}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := counter.calls.Load(); got != 1 {
		t.Fatalf("counter.calls sau lượt chạy gốc = %d, want 1", got)
	}
	if store.deleted != 1 {
		t.Fatalf("checkpoint phải được xoá đúng 1 lần khi run kết thúc tự nhiên ở NodeEnd, deleted=%d", store.deleted)
	}

	// --- 2. Tìm lại checkpoint "tool đã chạy xong, sẵn sàng gọi model lần 2"
	// — route() trên state đó phải trả về NodeModel (KHÔNG phải NodeTools).
	var crashState *State
	for _, data := range store.getHistory() {
		s, err := DeserializeState(data)
		if err != nil {
			t.Fatalf("DeserializeState: %v", err)
		}
		if len(s.Scratchpad) == 1 && s.Scratchpad[0].Name == "counter" && route(s) == NodeModel {
			crashState = s
			break
		}
	}
	if crashState == nil {
		t.Fatalf("không tìm thấy checkpoint ngay sau khi NodeTools chạy xong (history có %d bản ghi)", len(store.getHistory()))
	}
	if crashState.Interrupt != nil {
		t.Fatal("checkpoint mô phỏng crash này KHÔNG PHẢI interrupt — Interrupt phải nil")
	}

	// --- 3. Giả lập process restart: resume THẲNG từ checkpoint, KHÔNG qua
	// ResolveInterrupt (không cần answer) — đúng nhánh "node khác NodeInterrupt"
	// trong Engine.Resume().
	var (
		mu2   sync.Mutex
		texts []string
	)
	emit2 := func(e Event) {
		mu2.Lock()
		defer mu2.Unlock()
		if e.Type == "text" {
			texts = append(texts, e.Text)
		}
	}

	if _, err := eng.Resume(context.Background(), crashState, emit2); err != nil {
		t.Fatalf("Resume: %v", err)
	}

	// --- 4. Kết quả: tool KHÔNG được gọi lại, và câu trả lời cuối vẫn đúng.
	if got := counter.calls.Load(); got != 1 {
		t.Errorf("counter.calls sau CẢ lượt chạy gốc lẫn resume = %d, want 1 (tool KHÔNG được gọi lại lần 2)", got)
	}
	if got := strings.Join(texts, ""); got != "Xong rồi." {
		t.Errorf("text sau resume = %q, want %q", got, "Xong rồi.")
	}
}
