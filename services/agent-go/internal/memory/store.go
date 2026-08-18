package memory

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/ai-agent-tut/agent-go/internal/mongo"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type storeEntry struct {
	value     string
	embedding []float64
}

// Store is a thread-safe in-memory key-value store for agent memories.
//
// Data is partitioned by tenantID (outer map key) — mirroring the pattern
// already used by internal/tools/memory_tools.go's memoryStore. Without this
// partitioning, memories learned for one tenant would be visible to (or
// clobbered by) every other tenant sharing the same process, since Store is
// wired as a single process-wide singleton in cmd/server/main.go.
//
// Callers MUST pass the tenantID resolved via middleware.GetTenantID(ctx) —
// Store itself has no notion of "no tenant"; an empty string is just another
// namespace bucket like any other.
type Store struct {
	mu       sync.RWMutex
	data     map[string]map[string]storeEntry // tenantID -> key -> entry
	embedder Embedder
}

func NewStore() *Store {
	return &Store{data: make(map[string]map[string]storeEntry)}
}

// SetEmbedder configures an optional embedding provider for semantic search.
// Pass nil to disable semantic capabilities (keyword-only fallback).
func (s *Store) SetEmbedder(e Embedder) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.embedder = e
}

func (s *Store) Get(tenantID, key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data[tenantID][key]
	if !ok {
		return "", false
	}
	return v.value, true
}

func (s *Store) Set(tenantID, key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := storeEntry{value: value}

	if s.embedder != nil {
		vecs, err := s.embedder.Embed(context.Background(), []string{value})
		if err == nil && len(vecs) > 0 {
			entry.embedding = vecs[0]
		}
	}

	if s.data[tenantID] == nil {
		s.data[tenantID] = make(map[string]storeEntry)
	}
	s.data[tenantID][key] = entry
}

func (s *Store) Delete(tenantID, key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data[tenantID], key)
}

func (s *Store) Search(tenantID, query string) map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	lowerQuery := strings.ToLower(query)
	results := make(map[string]string)
	for k, v := range s.data[tenantID] {
		if strings.Contains(strings.ToLower(k), lowerQuery) ||
			strings.Contains(strings.ToLower(v.value), lowerQuery) {
			results[k] = v.value
		}
	}
	return results
}

// SemanticSearch performs embedding-based cosine similarity search scoped to
// a single tenant. Returns the topK most similar items. Requires an embedder
// to be set. If embedder is nil or embeddings are missing, returns empty.
func (s *Store) SemanticSearch(tenantID, query string, topK int) ([]Item, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.embedder == nil || topK <= 0 {
		return nil, nil
	}

	vecs, err := s.embedder.Embed(context.Background(), []string{query})
	if err != nil || len(vecs) == 0 {
		return nil, err
	}
	queryVec := vecs[0]

	type scored struct {
		key   string
		value string
		score float64
	}
	var candidates []scored

	for key, entry := range s.data[tenantID] {
		if len(entry.embedding) == 0 {
			continue
		}
		s := cosineSimilarity(queryVec, entry.embedding)
		candidates = append(candidates, scored{key: key, value: entry.value, score: s})
	}

	// Sort descending by score (simple insertion sort since topK is small).
	for i := 0; i < len(candidates) && i < topK; i++ {
		best := i
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].score > candidates[best].score {
				best = j
			}
		}
		if best != i {
			candidates[i], candidates[best] = candidates[best], candidates[i]
		}
	}

	limit := topK
	if limit > len(candidates) {
		limit = len(candidates)
	}

	results := make([]Item, limit)
	for i := 0; i < limit; i++ {
		results[i] = Item{
			Key:        candidates[i].key,
			Value:      candidates[i].value,
			Confidence: candidates[i].score,
			Embedding:  s.data[tenantID][candidates[i].key].embedding,
		}
	}
	return results, nil
}

// Len returns the number of memories stored for a single tenant.
func (s *Store) Len(tenantID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data[tenantID])
}

// All trả về TOÀN BỘ key-value đã lưu cho 1 tenant — dùng bởi tool
// memory.list để agent tự liệt kê được những gì đã "nhớ", thay vì phải đoán
// đúng keyword để memory.recall tìm ra (cùng lý do rag.list được thêm cho
// RAG trước đây).
func (s *Store) All(tenantID string) map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make(map[string]string, len(s.data[tenantID]))
	for k, v := range s.data[tenantID] {
		out[k] = v.value
	}
	return out
}

// memoryDoc ánh xạ 1 document trong collection "memories" (ghi bởi
// Learner.saveFactToMongo) — dùng để nạp lại vào Store lúc khởi động.
type memoryDoc struct {
	Key       string    `bson:"key"`
	Value     string    `bson:"value"`
	TenantID  string    `bson:"tenantId"`
	Embedding []float64 `bson:"embedding"`
}

// applyLoadedDocs điền các fact đã tải từ Mongo vào Store, bỏ qua document
// thiếu key/value/tenantId. Tách khỏi LoadFromMongo để test được logic thuần
// (merge/skip) mà không cần kết nối Mongo thật.
func (s *Store) applyLoadedDocs(docs []memoryDoc) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	loaded := 0
	for _, d := range docs {
		if d.Key == "" || d.Value == "" || d.TenantID == "" {
			continue
		}
		if s.data[d.TenantID] == nil {
			s.data[d.TenantID] = make(map[string]storeEntry)
		}
		s.data[d.TenantID][d.Key] = storeEntry{value: d.Value, embedding: d.Embedding}
		loaded++
	}
	return loaded
}

// LoadFromMongo nạp lại toàn bộ fact đã học từ collection "memories" (ghi bởi
// Learner.saveFactToMongo) vào Store trong RAM — gọi 1 lần lúc server khởi
// động.
//
// Trước fix này: saveFactToMongo ghi bền xuống Mongo nhưng KHÔNG có bước đọc
// ngược nào ở bất kỳ đâu trong codebase, nên mọi fact "đã học" chỉ sống tới
// lần restart/deploy kế tiếp — kho bền chỉ để ghi, không bao giờ được đọc lại.
//
// mongoClient=nil (Mongo không được cấu hình) → no-op, không lỗi, không chặn
// khởi động server.
func (s *Store) LoadFromMongo(ctx context.Context, mongoClient *mongo.Client) (int, error) {
	if mongoClient == nil {
		return 0, nil
	}

	coll := mongoClient.Collection("memories")
	cursor, err := coll.Find(ctx, bson.M{})
	if err != nil {
		return 0, fmt.Errorf("memory: load from mongo: find: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []memoryDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return 0, fmt.Errorf("memory: load from mongo: decode: %w", err)
	}

	return s.applyLoadedDocs(docs), nil
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
