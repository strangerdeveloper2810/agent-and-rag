// Package memory — in-memory key-value store for agent memories (MVP).
package memory

import (
	"strings"
	"sync"
)

// Store is a thread-safe in-memory key-value store for agent memories.
// MVP dùng map[string]string; phase sau thay bằng Mongo mà không đổi interface.
type Store struct {
	mu   sync.RWMutex
	data map[string]string
}

// NewStore tạo Store rỗng.
func NewStore() *Store {
	return &Store{data: make(map[string]string)}
}

// Get tra cứu một key (thread-safe).
func (s *Store) Get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data[key]
	return v, ok
}

// Set gán giá trị cho key (thread-safe).
func (s *Store) Set(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
}

// Delete xoá một key (thread-safe).
func (s *Store) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
}

// Search tìm các mục có key hoặc value chứa query (case-insensitive).
// Trả về map key→value, rỗng nếu không khớp.
func (s *Store) Search(query string) map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	lowerQuery := strings.ToLower(query)
	results := make(map[string]string)
	for k, v := range s.data {
		if strings.Contains(strings.ToLower(k), lowerQuery) ||
			strings.Contains(strings.ToLower(v), lowerQuery) {
			results[k] = v
		}
	}
	return results
}

// Len trả về số lượng mục trong store.
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data)
}
