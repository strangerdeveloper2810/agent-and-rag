// Package sqlite cung cấp SQLite storage cho JARVIS: conversations, messages, memories.
// Dùng modernc.org/sqlite (pure Go, không CGO). Auto-migrate schema khi mở DB.
package sqlite

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Store wraps SQLite connection with application-specific methods.
type Store struct {
	db *sql.DB
}

// Conversation represents a chat conversation.
type Conversation struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Message represents a single message in a conversation.
type Message struct {
	ID             int64     `json:"id"`
	ConversationID string    `json:"conversationId"`
	Role           string    `json:"role"`
	Content        string    `json:"content"`
	ToolCallsJSON  string    `json:"toolCalls,omitempty"`
	ToolCallID     string    `json:"toolCallId,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
}

// Memory represents a Tier 3 semantic memory entry.
type Memory struct {
	ID         int64     `json:"id"`
	Type       string    `json:"type"`
	Key        string    `json:"key"`
	Value      string    `json:"value"`
	Confidence float64   `json:"confidence"`
	Source     string    `json:"source"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// Open mở (hoặc tạo) SQLite database tại path và auto-migrate schema.
// Dùng ":memory:" cho test.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("sqlite: open: %w", err)
	}

	// Performance tuning
	_, _ = db.Exec("PRAGMA journal_mode=WAL")
	_, _ = db.Exec("PRAGMA foreign_keys=ON")

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite: migrate: %w", err)
	}
	// Bảng paused_runs (resume tối giản cho NodeInterrupt) — tách khỏi migrate()
	// ở trên để không chạm schema conversations/messages/memories đã ổn định.
	// Xem paused_runs.go.
	if err := migratePausedRuns(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite: migrate paused_runs: %w", err)
	}

	return &Store{db: db}, nil
}

// Close đóng database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// --- Schema Migration ---

func migrate(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS conversations (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT (datetime('now')),
		updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
	);

	CREATE TABLE IF NOT EXISTS messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		conversation_id TEXT NOT NULL REFERENCES conversations(id),
		role TEXT NOT NULL CHECK (role IN ('user','assistant','system','tool')),
		content TEXT NOT NULL DEFAULT '',
		tool_calls TEXT DEFAULT '',
		tool_call_id TEXT DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT (datetime('now'))
	);
	CREATE INDEX IF NOT EXISTS idx_messages_conv ON messages(conversation_id, created_at);

	CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(content, content=messages, content_rowid=id);

	CREATE TABLE IF NOT EXISTS memories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		type TEXT NOT NULL CHECK (type IN ('preference','fact','entity','relationship')),
		key TEXT NOT NULL,
		value TEXT NOT NULL,
		confidence REAL NOT NULL DEFAULT 1.0,
		source TEXT NOT NULL DEFAULT 'ai_extracted',
		created_at DATETIME NOT NULL DEFAULT (datetime('now')),
		updated_at DATETIME NOT NULL DEFAULT (datetime('now')),
		UNIQUE(type, key)
	);
	CREATE INDEX IF NOT EXISTS idx_memories_type ON memories(type);

	CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(value, content=memories, content_rowid=id);
	`
	_, err := db.Exec(schema)
	return err
}

// --- Conversations ---

// CreateConversation tạo conversation mới, trả về Conversation với ID được generate.
func (s *Store) CreateConversation(title string) (*Conversation, error) {
	id := fmt.Sprintf("conv_%d", time.Now().UnixNano())
	_, err := s.db.Exec("INSERT INTO conversations (id, title) VALUES (?, ?)", id, title)
	if err != nil {
		return nil, fmt.Errorf("sqlite: create conversation: %w", err)
	}
	return &Conversation{ID: id, Title: title, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}

// GetConversation lấy conversation theo ID.
func (s *Store) GetConversation(id string) (*Conversation, error) {
	var c Conversation
	err := s.db.QueryRow("SELECT id, title, created_at, updated_at FROM conversations WHERE id = ?", id).
		Scan(&c.ID, &c.Title, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("sqlite: conversation not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite: get conversation: %w", err)
	}
	return &c, nil
}

// ListConversations trả về danh sách conversations, mới nhất trước.
func (s *Store) ListConversations(limit int) ([]*Conversation, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query("SELECT id, title, created_at, updated_at FROM conversations ORDER BY updated_at DESC LIMIT ?", limit)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list conversations: %w", err)
	}
	defer rows.Close()

	var out []*Conversation
	for rows.Next() {
		var c Conversation
		if err := rows.Scan(&c.ID, &c.Title, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("sqlite: scan conversation: %w", err)
		}
		out = append(out, &c)
	}
	return out, rows.Err()
}

// --- Messages ---

// AddMessage thêm message vào conversation.
func (s *Store) AddMessage(convID, role, content, toolCallsJSON, toolCallID string) (*Message, error) {
	now := time.Now()
	result, err := s.db.Exec(
		"INSERT INTO messages (conversation_id, role, content, tool_calls, tool_call_id, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		convID, role, content, toolCallsJSON, toolCallID, now,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: add message: %w", err)
	}
	id, _ := result.LastInsertId()

	// Update conversation timestamp
	_, _ = s.db.Exec("UPDATE conversations SET updated_at = ? WHERE id = ?", now, convID)

	return &Message{ID: id, ConversationID: convID, Role: role, Content: content, ToolCallsJSON: toolCallsJSON, ToolCallID: toolCallID, CreatedAt: now}, nil
}

// GetMessages lấy tất cả messages của conversation, cũ nhất trước.
func (s *Store) GetMessages(convID string) ([]Message, error) {
	rows, err := s.db.Query(
		"SELECT id, conversation_id, role, content, tool_calls, tool_call_id, created_at FROM messages WHERE conversation_id = ? ORDER BY created_at ASC",
		convID,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: get messages: %w", err)
	}
	defer rows.Close()

	var out []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &m.ToolCallsJSON, &m.ToolCallID, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("sqlite: scan message: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// SearchMessages tìm kiếm full-text trong messages.
func (s *Store) SearchMessages(query string, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.db.Query(
		"SELECT m.id, m.conversation_id, m.role, m.content, m.tool_calls, m.tool_call_id, m.created_at FROM messages m JOIN messages_fts fts ON m.id = fts.rowid WHERE messages_fts MATCH ? ORDER BY rank LIMIT ?",
		query, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: search messages: %w", err)
	}
	defer rows.Close()

	var out []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &m.ToolCallsJSON, &m.ToolCallID, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("sqlite: scan message: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// --- Memories ---

// UpsertMemory thêm hoặc cập nhật memory (giữ confidence cao nhất).
func (s *Store) UpsertMemory(memType, key, value string, confidence float64, source string) error {
	now := time.Now()
	_, err := s.db.Exec(`
		INSERT INTO memories (type, key, value, confidence, source, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(type, key) DO UPDATE SET
			value = CASE WHEN ? >= confidence THEN ? ELSE value END,
			confidence = MAX(confidence, ?),
			updated_at = ?`,
		memType, key, value, confidence, source, now, now,
		confidence, value, confidence, now,
	)
	if err != nil {
		return fmt.Errorf("sqlite: upsert memory: %w", err)
	}
	return nil
}

// LookupMemory tìm memory chính xác theo type + key.
func (s *Store) LookupMemory(memType, key string) (*Memory, error) {
	var m Memory
	err := s.db.QueryRow(
		"SELECT id, type, key, value, confidence, source, created_at, updated_at FROM memories WHERE type = ? AND key = ?",
		memType, key,
	).Scan(&m.ID, &m.Type, &m.Key, &m.Value, &m.Confidence, &m.Source, &m.CreatedAt, &m.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil // not found = no error, nil result
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite: lookup memory: %w", err)
	}
	return &m, nil
}

// SearchMemories tìm kiếm full-text trong memories.
func (s *Store) SearchMemories(query string, limit int) ([]Memory, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.db.Query(
		"SELECT m.id, m.type, m.key, m.value, m.confidence, m.source, m.created_at, m.updated_at FROM memories m JOIN memories_fts fts ON m.id = fts.rowid WHERE memories_fts MATCH ? ORDER BY rank LIMIT ?",
		query, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: search memories: %w", err)
	}
	defer rows.Close()

	var out []Memory
	for rows.Next() {
		var m Memory
		if err := rows.Scan(&m.ID, &m.Type, &m.Key, &m.Value, &m.Confidence, &m.Source, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("sqlite: scan memory: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ListMemoriesByType trả về tất cả memories của một type.
func (s *Store) ListMemoriesByType(memType string) ([]Memory, error) {
	rows, err := s.db.Query("SELECT id, type, key, value, confidence, source, created_at, updated_at FROM memories WHERE type = ? ORDER BY key", memType)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list memories: %w", err)
	}
	defer rows.Close()

	var out []Memory
	for rows.Next() {
		var m Memory
		if err := rows.Scan(&m.ID, &m.Type, &m.Key, &m.Value, &m.Confidence, &m.Source, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("sqlite: scan memory: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
