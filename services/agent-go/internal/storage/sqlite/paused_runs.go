package sqlite

import (
	"database/sql"
	"fmt"
)

// migratePausedRuns tạo bảng paused_runs — KHÔNG đụng schema
// conversations/messages/memories trong sqlite.go (migrate()). Bảng này lưu
// SNAPSHOT MỚI NHẤT (checkpoint) của agent.State cho MỘT run_id — không chỉ
// khi Engine dừng ở NodeInterrupt (HITL) mà SAU MỖI LẦN CHUYỂN NODE của cả
// lượt chạy (xem internal/agent/engine.go: checkpoint/saveInterruptedState),
// để POST /chat/resume đọc lại và tiếp tục lượt chạy dù dừng vì lý do gì (hỏi
// user, hay tiến trình agent-go crash/restart giữa chừng) — xem
// internal/agent/resume.go. Tên bảng/hàm giữ nguyên "paused_runs"/
// "*InterruptedState" (không đổi) để tránh phá vỡ code/test đang gọi các tên
// này ở nhiều nơi — coi đây là "bảng checkpoint mới nhất theo run_id", KHÔNG
// còn đúng nghĩa hẹp "chỉ run đang tạm dừng chờ user" như tên gợi ý nữa.
//
// 1 dòng / run_id (UPSERT theo run_id — SaveInterruptedState) — bản ghi được
// ghi ĐÈ liên tục trong lúc run đang chạy, và bị XOÁ khi run kết thúc theo 1
// trong 2 cách: (a) resume xong xuôi (thành công hay lỗi, dùng 1 lần) qua
// handler /chat/resume, hoặc (b) run tự nhiên chạy tới NodeEnd không qua
// interrupt (xem Engine.deleteCheckpoint) — tránh state rác tích luỹ trong cả
// 2 trường hợp.
//
// Tách riêng khỏi migrate() và gọi từ Open() (không sửa migrate()) để tính
// năng resume không chạm vào schema đã ổn định.
func migratePausedRuns(db *sql.DB) error {
	_, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS paused_runs (
		run_id TEXT PRIMARY KEY,
		agent_name TEXT NOT NULL DEFAULT '',
		state_json TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT (datetime('now'))
	);
	`)
	return err
}

// SaveInterruptedState lưu (hoặc ghi đè — UPSERT theo run_id) snapshot JSON
// mới nhất của agent.State. Được gọi LẶP LẠI nhiều lần trong đời 1 run (mỗi
// lần chuyển node — checkpoint định kỳ) VÀ khi engine dừng ở NodeInterrupt.
// Implements agent.InterruptStore.
//
// agentName ghi lại agent nào (general/code/research — xem AgentSpec.Name)
// đã tạo ra run này: orchestrator dùng NHIỀU Engine với registry tool KHÁC
// NHAU (vd chỉ codeRegistry có shell.exec/git), nên resume PHẢI dùng đúng
// Engine gốc — nếu không, ResolveInterrupt có thể không tìm thấy tool trong
// registry của Engine sai.
func (s *Store) SaveInterruptedState(runID, agentName string, stateJSON []byte) error {
	_, err := s.db.Exec(
		`INSERT INTO paused_runs (run_id, agent_name, state_json) VALUES (?, ?, ?)
		 ON CONFLICT(run_id) DO UPDATE SET agent_name = excluded.agent_name, state_json = excluded.state_json`,
		runID, agentName, string(stateJSON),
	)
	if err != nil {
		return fmt.Errorf("sqlite: save interrupted state: %w", err)
	}
	return nil
}

// LoadInterruptedState đọc lại state JSON + tên agent gốc đã lưu theo run_id.
// Trả lỗi rõ ràng nếu không tìm thấy (run đã resume xong, hết hạn dọn dẹp thủ
// công, hoặc run_id sai) — caller (chat_resume handler) trả 404 cho client
// trong TH này.
func (s *Store) LoadInterruptedState(runID string) (stateJSON []byte, agentName string, err error) {
	var raw string
	err = s.db.QueryRow("SELECT state_json, agent_name FROM paused_runs WHERE run_id = ?", runID).Scan(&raw, &agentName)
	if err == sql.ErrNoRows {
		return nil, "", fmt.Errorf("sqlite: paused run not found: %s", runID)
	}
	if err != nil {
		return nil, "", fmt.Errorf("sqlite: load interrupted state: %w", err)
	}
	return []byte(raw), agentName, nil
}

// DeleteInterruptedState xoá bản ghi paused_runs — hoặc sau khi resume xử lý
// xong (thành công hay lỗi, dùng 1 lần — chat_resume.go), hoặc khi Engine
// thấy run kết thúc TỰ NHIÊN ở NodeEnd không qua interrupt (Engine.
// deleteCheckpoint). Cả 2 trường hợp đều tránh state rác tích luỹ trong bảng.
func (s *Store) DeleteInterruptedState(runID string) error {
	if _, err := s.db.Exec("DELETE FROM paused_runs WHERE run_id = ?", runID); err != nil {
		return fmt.Errorf("sqlite: delete interrupted state: %w", err)
	}
	return nil
}
