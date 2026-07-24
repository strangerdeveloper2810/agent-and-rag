// Package personality quản lý personality profile: name, formality, humor, verbosity.
// PersonalityEngine dùng profile để AdaptPrompt (thêm giọng điệu) và Learn (tích luỹ).
package personality

import (
	"fmt"
	"strings"
	"sync"
)

// Formality mức độ trang trọng.
type Formality int

const (
	FormalityCasual   Formality = iota // bạn bè, suồng sã
	FormalityNeutral                   // trung tính (default)
	FormalityFormal                    // lịch sự, chuyên nghiệp
)

func (f Formality) String() string {
	switch f {
	case FormalityCasual:
		return "casual"
	case FormalityNeutral:
		return "neutral"
	case FormalityFormal:
		return "formal"
	default:
		return "unknown"
	}
}

// Humor mức độ hài hước.
type Humor int

const (
	HumorNone    Humor = iota // nghiêm túc
	HumorDry                  // hài khô
	HumorPlayful              // vui vẻ, dí dỏm
)

func (h Humor) String() string {
	switch h {
	case HumorNone:
		return "none"
	case HumorDry:
		return "dry"
	case HumorPlayful:
		return "playful"
	default:
		return "unknown"
	}
}

// Verbosity mức độ chi tiết.
type Verbosity int

const (
	VerbosityConcise Verbosity = iota // ngắn gọn
	VerbosityNormal                   // vừa phải (default)
	VerbosityDetailed                 // chi tiết
)

func (v Verbosity) String() string {
	switch v {
	case VerbosityConcise:
		return "concise"
	case VerbosityNormal:
		return "normal"
	case VerbosityDetailed:
		return "detailed"
	default:
		return "unknown"
	}
}

// Stats thống kê về quá trình học.
type Stats struct {
	Interactions int
	Preferences  map[string]int // key=preference name, value=count
}

// Profile định nghĩa tính cách của agent.
type Profile struct {
	Name      string    `json:"name"`
	Formality Formality `json:"formality"`
	Humor     Humor     `json:"humor"`
	Verbosity Verbosity `json:"verbosity"`
}

// DefaultProfile trả về profile mặc định: JARVIS, neutral, dry, normal.
func DefaultProfile() Profile {
	return Profile{
		Name:      "JARVIS",
		Formality: FormalityNeutral,
		Humor:     HumorDry,
		Verbosity: VerbosityNormal,
	}
}

// PersonalityEngine quản lý profile và học từ tương tác.
type PersonalityEngine struct {
	profile Profile
	stats   Stats
	mu      sync.RWMutex
}

// New tạo PersonalityEngine từ profile.
func New(p Profile) *PersonalityEngine {
	return &PersonalityEngine{
		profile: p,
		stats: Stats{
			Preferences: make(map[string]int),
		},
	}
}

// Profile trả về bản sao profile hiện tại.
func (e *PersonalityEngine) Profile() Profile {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.profile
}

// Update cập nhật profile.
func (e *PersonalityEngine) Update(p Profile) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.profile = p
}

// Stats trả về thống kê hiện tại.
func (e *PersonalityEngine) Stats() Stats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	s := Stats{
		Interactions: e.stats.Interactions,
		Preferences:  make(map[string]int, len(e.stats.Preferences)),
	}
	for k, v := range e.stats.Preferences {
		s.Preferences[k] = v
	}
	return s
}

// AdaptPrompt nhận prompt gốc và trả về prompt đã điều chỉnh theo profile.
// Thêm system instructions về formality, humor, verbosity.
func (e *PersonalityEngine) AdaptPrompt(base string) string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var b strings.Builder

	// Personality header
	b.WriteString("[TÍNH CÁCH]\n")
	b.WriteString(fmt.Sprintf("Bạn là %s.\n", e.profile.Name))

	switch e.profile.Formality {
	case FormalityCasual:
		b.WriteString("- Phong cách: thân mật, gần gũi, sử dụng ngôn ngữ tự nhiên.\n")
	case FormalityNeutral:
		b.WriteString("- Phong cách: trung tính, lịch sự nhưng không quá trang trọng.\n")
	case FormalityFormal:
		b.WriteString("- Phong cách: trang trọng, chuyên nghiệp, dùng kính ngữ phù hợp.\n")
	}

	switch e.profile.Humor {
	case HumorNone:
		b.WriteString("- Hài hước: không, giữ thái độ nghiêm túc.\n")
	case HumorDry:
		b.WriteString("- Hài hước: hài khô, châm biếm nhẹ nhàng khi phù hợp.\n")
	case HumorPlayful:
		b.WriteString("- Hài hước: dí dỏm, có thể đùa vui khi thích hợp.\n")
	}

	switch e.profile.Verbosity {
	case VerbosityConcise:
		b.WriteString("- chi tiết: ngắn gọn, đi thẳng vào vấn đề, tránh lan man.\n")
	case VerbosityNormal:
		b.WriteString("- chi tiết: vừa phải, giải thích khi cần nhưng không dài dòng.\n")
	case VerbosityDetailed:
		b.WriteString("- chi tiết: đầy đủ, giải thích cặn kẽ từng bước.\n")
	}

	b.WriteString("\n")
	b.WriteString(base)

	return b.String()
}

// Learn học từ một cặp input/response. Tăng interaction count và có thể
// trích xuất preference (vd người dùng nói "ngắn gọn" → tăng concise preference).
func (e *PersonalityEngine) Learn(input, response string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.stats.Interactions++

	// Trích xuất preference từ input người dùng (keyword matching đơn giản).
	lower := strings.ToLower(input)
	prefs := map[string]string{
		"ngắn gọn":   "concise",
		"ngan gon":   "concise",
		"brief":      "concise",
		"concise":    "concise",
		"chi tiết":   "detailed",
		"chi tiet":   "detailed",
		"detailed":   "detailed",
		"thân mật":   "casual",
		"than mat":   "casual",
		"casual":     "casual",
		"trang trọng": "formal",
		"trang trong": "formal",
		"formal":     "formal",
		"nghiêm túc": "serious",
		"nghiem tuc": "serious",
		"vui vẻ":     "playful",
		"vui ve":     "playful",
	}

	for keyword, pref := range prefs {
		if strings.Contains(lower, keyword) {
			e.stats.Preferences[pref]++
		}
	}
}

// Reset đưa stats về 0 (giữ nguyên profile).
func (e *PersonalityEngine) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stats = Stats{
		Preferences: make(map[string]int),
	}
}
