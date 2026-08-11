package memory

import (
	"context"
	"math"
	"strings"
	"sync"
)

type storeEntry struct {
	value     string
	embedding []float64
}

// Store is a thread-safe in-memory key-value store for agent memories.
type Store struct {
	mu       sync.RWMutex
	data     map[string]storeEntry
	embedder Embedder
}

func NewStore() *Store {
	return &Store{data: make(map[string]storeEntry)}
}

// SetEmbedder configures an optional embedding provider for semantic search.
// Pass nil to disable semantic capabilities (keyword-only fallback).
func (s *Store) SetEmbedder(e Embedder) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.embedder = e
}

func (s *Store) Get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data[key]
	if !ok {
		return "", false
	}
	return v.value, true
}

func (s *Store) Set(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := storeEntry{value: value}

	if s.embedder != nil {
		vecs, err := s.embedder.Embed(context.Background(), []string{value})
		if err == nil && len(vecs) > 0 {
			entry.embedding = vecs[0]
		}
	}

	s.data[key] = entry
}

func (s *Store) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
}

func (s *Store) Search(query string) map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	lowerQuery := strings.ToLower(query)
	results := make(map[string]string)
	for k, v := range s.data {
		if strings.Contains(strings.ToLower(k), lowerQuery) ||
			strings.Contains(strings.ToLower(v.value), lowerQuery) {
			results[k] = v.value
		}
	}
	return results
}

// SemanticSearch performs embedding-based cosine similarity search.
// Returns the topK most similar items. Requires an embedder to be set.
// If embedder is nil or embeddings are missing, returns empty.
func (s *Store) SemanticSearch(query string, topK int) ([]Item, error) {
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

	for key, entry := range s.data {
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
			Embedding:  s.data[candidates[i].key].embedding,
		}
	}
	return results, nil
}

func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data)
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
