package personality

import (
	"strings"
	"testing"
)

func TestDefaultProfile(t *testing.T) {
	p := DefaultProfile()
	if p.Name != "JARVIS" {
		t.Errorf("Name = %q, want %q", p.Name, "JARVIS")
	}
	if p.Formality != FormalityNeutral {
		t.Errorf("Formality = %v, want %v", p.Formality, FormalityNeutral)
	}
	if p.Humor != HumorDry {
		t.Errorf("Humor = %v, want %v", p.Humor, HumorDry)
	}
	if p.Verbosity != VerbosityNormal {
		t.Errorf("Verbosity = %v, want %v", p.Verbosity, VerbosityNormal)
	}
}

func TestFormalityString(t *testing.T) {
	tests := []struct {
		f    Formality
		want string
	}{
		{FormalityCasual, "casual"},
		{FormalityNeutral, "neutral"},
		{FormalityFormal, "formal"},
		{Formality(99), "unknown"},
	}
	for _, tt := range tests {
		got := tt.f.String()
		if got != tt.want {
			t.Errorf("Formality(%d).String() = %q, want %q", tt.f, got, tt.want)
		}
	}
}

func TestHumorString(t *testing.T) {
	tests := []struct {
		h    Humor
		want string
	}{
		{HumorNone, "none"},
		{HumorDry, "dry"},
		{HumorPlayful, "playful"},
		{Humor(99), "unknown"},
	}
	for _, tt := range tests {
		got := tt.h.String()
		if got != tt.want {
			t.Errorf("Humor(%d).String() = %q, want %q", tt.h, got, tt.want)
		}
	}
}

func TestVerbosityString(t *testing.T) {
	tests := []struct {
		v    Verbosity
		want string
	}{
		{VerbosityConcise, "concise"},
		{VerbosityNormal, "normal"},
		{VerbosityDetailed, "detailed"},
		{Verbosity(99), "unknown"},
	}
	for _, tt := range tests {
		got := tt.v.String()
		if got != tt.want {
			t.Errorf("Verbosity(%d).String() = %q, want %q", tt.v, got, tt.want)
		}
	}
}

func TestAdaptPrompt_Defaults(t *testing.T) {
	engine := New(DefaultProfile())
	base := "Hãy trả lời câu hỏi của người dùng."

	result := engine.AdaptPrompt(base)

	// Must contain personality header
	if !strings.Contains(result, "[TÍNH CÁCH]") {
		t.Error("AdaptPrompt missing [TÍNH CÁCH] header")
	}
	if !strings.Contains(result, "JARVIS") {
		t.Error("AdaptPrompt missing agent name")
	}
	// Must preserve base prompt
	if !strings.Contains(result, base) {
		t.Error("AdaptPrompt missing base prompt")
	}
}

func TestAdaptPrompt_AllTones(t *testing.T) {
	tests := []struct {
		name      string
		profile   Profile
		wantWords []string
	}{
		{
			name:      "casual+dry+concise",
			profile:   Profile{Name: "Bot", Formality: FormalityCasual, Humor: HumorDry, Verbosity: VerbosityConcise},
			wantWords: []string{"thân mật", "hài khô", "ngắn gọn"},
		},
		{
			name:      "formal+none+detailed",
			profile:   Profile{Name: "Bot", Formality: FormalityFormal, Humor: HumorNone, Verbosity: VerbosityDetailed},
			wantWords: []string{"trang trọng", "nghiêm túc", "chi tiết"},
		},
		{
			name:      "neutral+playful+normal",
			profile:   Profile{Name: "Bot", Formality: FormalityNeutral, Humor: HumorPlayful, Verbosity: VerbosityNormal},
			wantWords: []string{"trung tính", "dí dỏm", "vừa phải"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := New(tt.profile)
			result := engine.AdaptPrompt("base")
			for _, word := range tt.wantWords {
				if !strings.Contains(result, word) {
					t.Errorf("AdaptPrompt missing word %q for profile %s", word, tt.name)
				}
			}
		})
	}
}

func TestLearn(t *testing.T) {
	engine := New(DefaultProfile())

	engine.Learn("trả lời ngắn gọn thôi", "ok")
	engine.Learn("hãy chi tiết hơn", "được thôi")
	engine.Learn("vui vẻ lên nào", "haha")

	stats := engine.Stats()
	if stats.Interactions != 3 {
		t.Errorf("Interactions = %d, want 3", stats.Interactions)
	}
	if stats.Preferences["concise"] != 1 {
		t.Errorf("concise = %d, want 1", stats.Preferences["concise"])
	}
	if stats.Preferences["detailed"] != 1 {
		t.Errorf("detailed = %d, want 1", stats.Preferences["detailed"])
	}
	if stats.Preferences["playful"] != 1 {
		t.Errorf("playful = %d, want 1", stats.Preferences["playful"])
	}
}

func TestLearn_MultipleKeywords(t *testing.T) {
	engine := New(DefaultProfile())

	// Input chứa cả "ngắn gọn" và "chi tiết"
	engine.Learn("cần vừa ngắn gọn vừa chi tiết", "ok")

	stats := engine.Stats()
	if stats.Preferences["concise"] != 1 {
		t.Errorf("concise = %d, want 1", stats.Preferences["concise"])
	}
	if stats.Preferences["detailed"] != 1 {
		t.Errorf("detailed = %d, want 1", stats.Preferences["detailed"])
	}
}

func TestLearn_NoKeywords(t *testing.T) {
	engine := New(DefaultProfile())
	engine.Learn("hôm nay thế nào?", "tốt")

	stats := engine.Stats()
	if stats.Interactions != 1 {
		t.Errorf("Interactions = %d, want 1", stats.Interactions)
	}
	if len(stats.Preferences) != 0 {
		t.Errorf("Preferences len = %d, want 0 for no-keyword input", len(stats.Preferences))
	}
}

func TestReset(t *testing.T) {
	engine := New(DefaultProfile())
	engine.Learn("ngắn gọn", "ok")
	engine.Learn("vui vẻ", "ok")

	engine.Reset()
	stats := engine.Stats()
	if stats.Interactions != 0 {
		t.Errorf("Interactions = %d, want 0 after reset", stats.Interactions)
	}
	if len(stats.Preferences) != 0 {
		t.Errorf("Preferences len = %d, want 0 after reset", len(stats.Preferences))
	}
}

func TestUpdateProfile(t *testing.T) {
	engine := New(DefaultProfile())
	engine.Update(Profile{
		Name:      "Friday",
		Formality: FormalityFormal,
		Humor:     HumorNone,
		Verbosity: VerbosityConcise,
	})

	p := engine.Profile()
	if p.Name != "Friday" {
		t.Errorf("Name = %q, want %q", p.Name, "Friday")
	}
	if p.Formality != FormalityFormal {
		t.Errorf("Formality = %v, want %v", p.Formality, FormalityFormal)
	}
}

func TestStats_CopyIsolation(t *testing.T) {
	engine := New(DefaultProfile())
	engine.Learn("ngắn gọn", "ok")

	stats := engine.Stats()
	stats.Preferences["hacked"] = 999 // mutate copy

	actual := engine.Stats()
	if actual.Preferences["hacked"] != 0 {
		t.Errorf("Stats returned shared map: hacked = %d", actual.Preferences["hacked"])
	}
}
