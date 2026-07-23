package mongo

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Task — collection `tasks` (agent-go sở hữu qua tool CRUD). Khớp schema TS
// (apps/api/src/schemas/task.ts).
type Task struct {
	ID          bson.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	Title       string        `bson:"title" json:"title"`
	Status      string        `bson:"status" json:"status"`
	Priority    string        `bson:"priority,omitempty" json:"priority,omitempty"`
	Tags        []string      `bson:"tags,omitempty" json:"tags,omitempty"`
	DueDate     *time.Time    `bson:"dueDate,omitempty" json:"dueDate,omitempty"`
	RemindAt    *time.Time    `bson:"remindAt,omitempty" json:"remindAt,omitempty"`
	Source      string        `bson:"source,omitempty" json:"source,omitempty"`
	CreatedAt   time.Time     `bson:"createdAt" json:"createdAt"`
	UpdatedAt   time.Time     `bson:"updatedAt" json:"updatedAt"`
	CompletedAt *time.Time    `bson:"completedAt,omitempty" json:"completedAt,omitempty"`
}

// DocChunk — collection `documents` (api ghi khi ingest; agent-go ĐỌC cho RAG).
type DocChunk struct {
	DocumentID string    `bson:"documentId" json:"documentId"`
	Source     string    `bson:"source" json:"source"`
	Version    int       `bson:"version" json:"version"`
	ChunkIndex int       `bson:"chunkIndex" json:"chunkIndex"`
	Text       string    `bson:"text" json:"text"`
	Embedding  []float64 `bson:"embedding,omitempty" json:"embedding,omitempty"`
	CreatedAt  time.Time `bson:"createdAt" json:"createdAt"`
}

// Memory — collection `memories` (semantic long-term, agent-go sở hữu).
type Memory struct {
	ID             bson.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	Type           string        `bson:"type" json:"type"`
	Key            string        `bson:"key" json:"key"`
	Value          string        `bson:"value" json:"value"`
	Source         string        `bson:"source" json:"source"`
	Confidence     float64       `bson:"confidence" json:"confidence"`
	Embedding      []float64     `bson:"embedding,omitempty" json:"embedding,omitempty"`
	ConversationID string        `bson:"conversationId,omitempty" json:"conversationId,omitempty"`
	CreatedAt      time.Time     `bson:"createdAt" json:"createdAt"`
	UpdatedAt      time.Time     `bson:"updatedAt" json:"updatedAt"`
}
