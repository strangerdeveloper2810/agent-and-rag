package agent

// Test cho tính năng MỚI (user skills management): builtin skill bị user tắt
// (DisabledSkills) KHÔNG được kích hoạt/xuất hiện trong system prompt, và
// custom skill của user (lưu PostgreSQL) LUÔN được nối vào system prompt bất
// kể có khớp trigger hay không — xem nodeModel trong node_model.go.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/provider"
	"github.com/ai-agent-tut/agent-go/internal/skills"
	"github.com/ai-agent-tut/agent-go/internal/tools"
)

// fakeEngineWithSkills mở rộng fakeEngine (node_model_test.go): fakeEngine gốc
// luôn trả getSkillLoader() = nil nên không dùng được để test progressive
// disclosure builtin skill (matching/disable). Đây là kiểu RIÊNG cho file này,
// không sửa fakeEngine gốc.
type fakeEngineWithSkills struct {
	prov        provider.Provider
	registry    *tools.Registry
	skillLoader *skills.Loader
}

func (e *fakeEngineWithSkills) getProvider() provider.Provider { return e.prov }
func (e *fakeEngineWithSkills) getRegistry() *tools.Registry   { return e.registry }
func (e *fakeEngineWithSkills) getSystemPrompt() string        { return "" }
func (e *fakeEngineWithSkills) getMaxContextTokens() int       { return 0 }
func (e *fakeEngineWithSkills) getMaxOutputTokens() int        { return 0 }
func (e *fakeEngineWithSkills) getOwnerTenants() []string      { return nil }
func (e *fakeEngineWithSkills) getDynamicThinking() DynamicThinkingConfig {
	return DynamicThinkingConfig{}
}
func (e *fakeEngineWithSkills) getSkillLoader() *skills.Loader { return e.skillLoader }
func (e *fakeEngineWithSkills) getFastModel() string           { return "" }

// buildTestSkillLoaderForDisable dựng 1 Loader với đúng 1 skill "debug" có
// trigger tiếng Việt MẠNH (khớp chắc chắn, tránh flaky do ngưỡng điểm).
func buildTestSkillLoaderForDisable(t *testing.T) *skills.Loader {
	t.Helper()
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "debug")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := `---
name: debug
description: Systematic debugging reproduce isolate identify fix verify
when_to_use: When user reports a bug, error, crash, or unexpected behavior
triggers: [debug lỗi này]
---

# Debugging Skill
Follow reproduce-isolate-fix steps.`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	loader, err := skills.NewLoader(dir)
	if err != nil {
		t.Fatalf("NewLoader: %v", err)
	}
	return loader
}

// TestNodeModel_DisabledSkillNotInjectedIntoSystemPrompt khoá đúng tính năng
// "user tắt 1 builtin skill" (POST /api/user/skills/:name/toggle với
// enabled=false, forward qua RunInput.DisabledSkills): dù input khớp trigger
// RẤT MẠNH, skill bị tắt KHÔNG được kích hoạt và KHÔNG được nối vào system
// prompt gửi cho LLM.
func TestNodeModel_DisabledSkillNotInjectedIntoSystemPrompt(t *testing.T) {
	loader := buildTestSkillLoaderForDisable(t)

	fake := newCapturingProvider(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "OK"},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)
	eng := &fakeEngineWithSkills{prov: fake, registry: tools.NewRegistry(), skillLoader: loader}

	s := newState(RunInput{
		UserMessage:    "debug lỗi này giúp tôi với",
		MaxSteps:       12,
		DisabledSkills: []string{"debug"},
	})

	if _, err := nodeModel(context.Background(), eng, s, nilEmit); err != nil {
		t.Fatalf("nodeModel error: %v", err)
	}

	sys := fake.LastRequest.System
	if strings.Contains(sys, "KỸ NĂNG ĐANG KÍCH HOẠT") {
		t.Errorf("system prompt = %q, KHÔNG được chứa skill đã bị user tắt", sys)
	}
	if strings.Contains(sys, "Follow reproduce-isolate-fix") {
		t.Errorf("system prompt vẫn lộ nội dung skill 'debug' dù đã bị tắt: %q", sys)
	}
}

// TestNodeModel_EnabledSkillInjectedIntoSystemPrompt là ĐỐI CHỨNG bắt buộc:
// CÙNG input, KHÔNG disable → skill PHẢI được kích hoạt. Không có test này thì
// test "disabled" ở trên có thể pass giả (vd input không hề khớp trigger từ
// đầu) mà không thực sự chứng minh cờ DisabledSkills có tác dụng gì.
func TestNodeModel_EnabledSkillInjectedIntoSystemPrompt(t *testing.T) {
	loader := buildTestSkillLoaderForDisable(t)

	fake := newCapturingProvider(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "OK"},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)
	eng := &fakeEngineWithSkills{prov: fake, registry: tools.NewRegistry(), skillLoader: loader}

	s := newState(RunInput{
		UserMessage: "debug lỗi này giúp tôi với",
		MaxSteps:    12,
		// DisabledSkills rỗng — không tắt gì.
	})

	if _, err := nodeModel(context.Background(), eng, s, nilEmit); err != nil {
		t.Fatalf("nodeModel error: %v", err)
	}

	sys := fake.LastRequest.System
	if !strings.Contains(sys, "KỸ NĂNG ĐANG KÍCH HOẠT: debug") {
		t.Errorf("system prompt = %q, thiếu skill 'debug' khi KHÔNG bị tắt", sys)
	}
}

// TestNodeModel_CustomSkillInjectedIntoSystemPrompt khoá tính năng custom
// skill của user (lưu PostgreSQL, forward qua RunInput.CustomSkills): PHẢI
// luôn được nối vào system prompt — KHÔNG cần trigger khớp, khác hẳn builtin
// skill (progressive disclosure qua MatchSkill).
func TestNodeModel_CustomSkillInjectedIntoSystemPrompt(t *testing.T) {
	fake := newCapturingProvider(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "OK"},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)
	eng := &fakeEngine{prov: fake, registry: tools.NewRegistry()}

	s := newState(RunInput{
		UserMessage: "chào bạn, hôm nay khoẻ không",
		MaxSteps:    12,
		CustomSkills: []CustomSkill{
			{
				Name:        "invoice-formatter",
				Description: "Format hoá đơn theo chuẩn công ty",
				WhenToUse:   "Khi user yêu cầu xuất hoá đơn",
				Content:     "Luôn dùng VNĐ và ghi rõ thuế VAT.",
			},
		},
	})

	if _, err := nodeModel(context.Background(), eng, s, nilEmit); err != nil {
		t.Fatalf("nodeModel error: %v", err)
	}

	sys := fake.LastRequest.System
	if !strings.Contains(sys, "[KỸ NĂNG TUỲ CHỈNH CỦA NGƯỜI DÙNG]") {
		t.Errorf("system prompt thiếu section custom skill: %q", sys)
	}
	if !strings.Contains(sys, "invoice-formatter") {
		t.Errorf("system prompt thiếu tên custom skill: %q", sys)
	}
	if !strings.Contains(sys, "Luôn dùng VNĐ và ghi rõ thuế VAT.") {
		t.Errorf("system prompt thiếu nội dung custom skill: %q", sys)
	}
	if !strings.Contains(sys, "Khi user yêu cầu xuất hoá đơn") {
		t.Errorf("system prompt thiếu when_to_use của custom skill: %q", sys)
	}
}

// Đối chứng: không có custom skill nào (rỗng) → KHÔNG được thêm section thừa
// (tránh nhiễu token mỗi request không cấu hình gì).
func TestNodeModel_NoCustomSkills_NoCustomSkillSection(t *testing.T) {
	fake := newCapturingProvider(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "OK"},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)
	eng := &fakeEngine{prov: fake, registry: tools.NewRegistry()}

	s := newState(RunInput{UserMessage: "chào bạn", MaxSteps: 12})

	if _, err := nodeModel(context.Background(), eng, s, nilEmit); err != nil {
		t.Fatalf("nodeModel error: %v", err)
	}

	if strings.Contains(fake.LastRequest.System, "KỸ NĂNG TUỲ CHỈNH") {
		t.Errorf("system prompt không được có section custom skill khi rỗng: %q", fake.LastRequest.System)
	}
}

// Nhiều custom skill cùng lúc đều phải xuất hiện đầy đủ (không bị ghi đè nhau).
func TestNodeModel_MultipleCustomSkillsAllInjected(t *testing.T) {
	fake := newCapturingProvider(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "OK"},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)
	eng := &fakeEngine{prov: fake, registry: tools.NewRegistry()}

	s := newState(RunInput{
		UserMessage: "chào",
		MaxSteps:    12,
		CustomSkills: []CustomSkill{
			{Name: "skill-a", Content: "nội dung A"},
			{Name: "skill-b", Content: "nội dung B"},
		},
	})

	if _, err := nodeModel(context.Background(), eng, s, nilEmit); err != nil {
		t.Fatalf("nodeModel error: %v", err)
	}

	sys := fake.LastRequest.System
	if !strings.Contains(sys, "skill-a") || !strings.Contains(sys, "nội dung A") {
		t.Errorf("thiếu skill-a: %q", sys)
	}
	if !strings.Contains(sys, "skill-b") || !strings.Contains(sys, "nội dung B") {
		t.Errorf("thiếu skill-b: %q", sys)
	}
}
