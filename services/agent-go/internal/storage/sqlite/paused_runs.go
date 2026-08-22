package sqlite

import (
	"database/sql"
	"fmt"
)

// migratePausedRuns tạo bảng paused_runs — KHÔNG đụng schema
// conversations/messages/memories trong sqlite.go (migrate()). Bảng này lưu
// snapshot JSON của agent.State khi Engine dừng ở NodeInterrupt (resume tối
// giản CHỈ cho node đó — xem internal/agent/resume.go), để POST /chat/resume
// đọc lại và tiếp tục lượt chạy. Dùng 1 lần: xoá ngay sau khi resume xong
// (thành công hay lỗi) qua DeleteInterruptedState, tránh state rác tích luỹ.
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

// SaveInterruptedState lưu (hoặc ghi đè, nếu cùng run_id resume rồi lại dừng
// ở interrupt tiếp theo) snapshot JSON của agent.State theo run_id. Implements
// agent.InterruptStore.
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

// DeleteInterruptedState xoá bản ghi paused_runs sau khi resume xử lý xong
// (thành công hay lỗi) — dùng 1 lần, tránh state rác tích luỹ trong bảng.
func (s *Store) DeleteInterruptedState(runID string) error {
	if _, err := s.db.Exec("DELETE FROM paused_runs WHERE run_id = ?", runID); err != nil {
		return fmt.Errorf("sqlite: delete interrupted state: %w", err)
	}
	return nil
}
