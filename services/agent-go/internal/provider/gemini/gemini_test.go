package gemini

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/provider"
	"google.golang.org/genai"
)

func TestToGeminiContents_RoleMapping(t *testing.T) {
	msgs := []provider.Message{
		{Role: provider.RoleSystem, Content: "you are helpful"},
		{Role: provider.RoleUser, Content: "hi"},
		{Role: provider.RoleAssistant, Content: "hello"},
	}

	got := toGeminiContents(msgs)
	if len(got) != 3 {
		t.Fatalf("want 3 contents, got %d", len(got))
	}

	// system → role "user", text giữ nguyên
	if got[0].Role != genai.RoleUser {
		t.Errorf("system: want role %q, got %q", genai.RoleUser, got[0].Role)
	}
	if got[0].Parts[0].Text != "you are helpful" {
		t.Errorf("system: text mismatch: %q", got[0].Parts[0].Text)
	}

	// user → role "user"
	if got[1].Role != genai.RoleUser {
		t.Errorf("user: want role %q, got %q", genai.RoleUser, got[1].Role)
	}
	if got[1].Parts[0].Text != "hi" {
		t.Errorf("user: text mismatch: %q", got[1].Parts[0].Text)
	}

	// assistant → role "model"
	if got[2].Role != genai.RoleModel {
		t.Errorf("assistant: want role %q, got %q", genai.RoleModel, got[2].Role)
	}
	if got[2].Parts[0].Text != "hello" {
		t.Errorf("assistant: text mismatch: %q", got[2].Parts[0].Text)
	}
}

func TestToGeminiContents_AssistantToolCalls(t *testing.T) {
	msgs := []provider.Message{
		{
			Role:    provider.RoleAssistant,
			Content: "let me check",
			ToolCalls: []provider.ToolCall{
				{ID: "call_1", Name: "ragSearch", Args: json.RawMessage(`{"query":"go"}`)},
			},
		},
	}

	got := toGeminiContents(msgs)
	if len(got) != 1 {
		t.Fatalf("want 1 content, got %d", len(got))
	}
	c := got[0]
	if c.Role != genai.RoleModel {
		t.Fatalf("want role %q, got %q", genai.RoleModel, c.Role)
	}
	if len(c.Parts) != 2 {
		t.Fatalf("want 2 parts (text + functionCall), got %d", len(c.Parts))
	}
	if c.Parts[0].Text != "let me check" {
		t.Errorf("part0 text mismatch: %q", c.Parts[0].Text)
	}

	fc := c.Parts[1].FunctionCall
	if fc == nil {
		t.Fatalf("part1 FunctionCall is nil")
	}
	if fc.ID != "call_1" || fc.Name != "ragSearch" {
		t.Errorf("functionCall id/name mismatch: %q/%q", fc.ID, fc.Name)
	}
	if got, ok := fc.Args["query"].(string); !ok || got != "go" {
		t.Errorf("functionCall args mismatch: %#v", fc.Args)
	}
}

func TestToGeminiContents_ToolResult(t *testing.T) {
	msgs := []provider.Message{
		{Role: provider.RoleTool, ToolCallID: "call_1", Content: "found 3 docs"},
	}

	got := toGeminiContents(msgs)
	if len(got) != 1 {
		t.Fatalf("want 1 content, got %d", len(got))
	}
	c := got[0]
	if c.Role != genai.RoleUser {
		t.Errorf("tool result: want role %q, got %q", genai.RoleUser, c.Role)
	}
	fr := c.Parts[0].FunctionResponse
	if fr == nil {
		t.Fatalf("FunctionResponse is nil")
	}
	if fr.ID != "call_1" {
		t.Errorf("FunctionResponse ID mismatch: %q", fr.ID)
	}
	if out, ok := fr.Response["output"].(string); !ok || out != "found 3 docs" {
		t.Errorf("FunctionResponse output mismatch: %#v", fr.Response)
	}
}

func TestToGeminiContents_Empty(t *testing.T) {
	if got := toGeminiContents(nil); got != nil {
		t.Errorf("want nil for empty input, got %#v", got)
	}
}

func TestToGeminiTools(t *testing.T) {
	tools := []provider.ToolDef{
		{
			Name:        "ragSearch",
			Description: "search the knowledge base",
			Schema:      json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`),
		},
		{Name: "noArgs", Description: "no schema tool"},
	}

	got := toGeminiTools(tools)
	if len(got) != 1 {
		t.Fatalf("want 1 Tool wrapping declarations, got %d", len(got))
	}
	decls := got[0].FunctionDeclarations
	if len(decls) != 2 {
		t.Fatalf("want 2 function declarations, got %d", len(decls))
	}

	if decls[0].Name != "ragSearch" || decls[0].Description != "search the knowledge base" {
		t.Errorf("decl0 name/desc mismatch: %q/%q", decls[0].Name, decls[0].Description)
	}
	schema, ok := decls[0].ParametersJsonSchema.(map[string]any)
	if !ok {
		t.Fatalf("decl0 ParametersJsonSchema type: %T", decls[0].ParametersJsonSchema)
	}
	if schema["type"] != "object" {
		t.Errorf("decl0 schema type mismatch: %#v", schema["type"])
	}

	// tool không có schema → ParametersJsonSchema để nil
	if decls[1].ParametersJsonSchema != nil {
		t.Errorf("decl1 want nil ParametersJsonSchema, got %#v", decls[1].ParametersJsonSchema)
	}
}

func TestToGeminiTools_Empty(t *testing.T) {
	if got := toGeminiTools(nil); got != nil {
		t.Errorf("want nil for empty tools, got %#v", got)
	}
}

func TestMapThinkingLevel(t *testing.T) {
	tests := []struct {
		name  string
		level provider.ThinkingLevel
		want  genai.ThinkingLevel // "" nghĩa là kỳ vọng nil config
		isNil bool
	}{
		{"off", provider.ThinkingOff, "", true},
		{"empty", provider.ThinkingLevel(""), "", true},
		{"low", provider.ThinkingLow, genai.ThinkingLevelLow, false},
		{"medium", provider.ThinkingMedium, genai.ThinkingLevelMedium, false},
		{"high", provider.ThinkingHigh, genai.ThinkingLevelHigh, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapThinkingLevel(tt.level)
			if tt.isNil {
				if got != nil {
					t.Fatalf("want nil, got %#v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("want non-nil config for %q", tt.level)
			}
			if got.ThinkingLevel != tt.want {
				t.Errorf("want ThinkingLevel %q, got %q", tt.want, got.ThinkingLevel)
			}
		})
	}
}

func TestRawToMap(t *testing.T) {
	if m := rawToMap(nil); len(m) != 0 || m == nil {
		t.Errorf("nil input: want empty non-nil map, got %#v", m)
	}
	if m := rawToMap(json.RawMessage(`not json`)); len(m) != 0 || m == nil {
		t.Errorf("invalid input: want empty non-nil map, got %#v", m)
	}
	m := rawToMap(json.RawMessage(`{"a":1}`))
	if v, ok := m["a"].(float64); !ok || v != 1 {
		t.Errorf("valid input: want a=1, got %#v", m)
	}
}

func TestToGeminiContents_UserWithImageAttachment(t *testing.T) {
	imgData := base64.StdEncoding.EncodeToString([]byte("fake-png-bytes"))
	msgs := []provider.Message{
		{
			Role:    provider.RoleUser,
			Content: "what is this image?",
			Attachments: []provider.Attachment{
				{Type: "image", Name: "photo.png", Data: imgData, MimeType: "image/png"},
			},
		},
	}

	got := toGeminiContents(msgs)
	if len(got) != 1 {
		t.Fatalf("want 1 content, got %d", len(got))
	}
	c := got[0]
	if c.Role != genai.RoleUser {
		t.Fatalf("want role %q, got %q", genai.RoleUser, c.Role)
	}
	// Should have 2 parts: text + inline_data
	if len(c.Parts) != 2 {
		t.Fatalf("want 2 parts (text + inline_data), got %d: %+v", len(c.Parts), c.Parts)
	}
	// First part is text
	if c.Parts[0].Text != "what is this image?" {
		t.Errorf("part[0].Text = %q, want %q", c.Parts[0].Text, "what is this image?")
	}
	// Second part is inline data
	blob := c.Parts[1].InlineData
	if blob == nil {
		t.Fatal("part[1].InlineData is nil")
	}
	if blob.MIMEType != "image/png" {
		t.Errorf("InlineData.MIMEType = %q, want image/png", blob.MIMEType)
	}
	if string(blob.Data) != "fake-png-bytes" {
		t.Errorf("InlineData.Data = %q, want fake-png-bytes", string(blob.Data))
	}
}

func TestToGeminiContents_UserWithFileAttachment(t *testing.T) {
	// File attachments are pre-processed into Content by newState(),
	// so by the time we reach toGeminiContents they are already text.
	msgs := []provider.Message{
		{
			Role:    provider.RoleUser,
			Content: "check this\n\n[File: log.txt]\nerror line",
		},
	}

	got := toGeminiContents(msgs)
	if len(got) != 1 {
		t.Fatalf("want 1 content, got %d", len(got))
	}
	if got[0].Role != genai.RoleUser {
		t.Fatalf("want role %q, got %q", genai.RoleUser, got[0].Role)
	}
	if len(got[0].Parts) != 1 {
		t.Fatalf("want 1 part (text only), got %d", len(got[0].Parts))
	}
}

func TestToGeminiContents_UserWithMultipleImages(t *testing.T) {
	img1 := base64.StdEncoding.EncodeToString([]byte("bytes1"))
	img2 := base64.StdEncoding.EncodeToString([]byte("bytes2"))
	msgs := []provider.Message{
		{
			Role:    provider.RoleUser,
			Content: "compare these",
			Attachments: []provider.Attachment{
				{Type: "image", Name: "a.png", Data: img1, MimeType: "image/png"},
				{Type: "image", Name: "b.jpg", Data: img2, MimeType: "image/jpeg"},
			},
		},
	}

	got := toGeminiContents(msgs)
	c := got[0]
	if len(c.Parts) != 3 {
		t.Fatalf("want 3 parts (text + 2 images), got %d", len(c.Parts))
	}
	if c.Parts[1].InlineData.MIMEType != "image/png" {
		t.Errorf("image1 MIMEType: %q", c.Parts[1].InlineData.MIMEType)
	}
	if c.Parts[2].InlineData.MIMEType != "image/jpeg" {
		t.Errorf("image2 MIMEType: %q", c.Parts[2].InlineData.MIMEType)
	}
}

func TestToGeminiContents_UserWithInvalidBase64(t *testing.T) {
	// Invalid base64 in attachment — should skip that image gracefully.
	msgs := []provider.Message{
		{
			Role:    provider.RoleUser,
			Content: "is this valid?",
			Attachments: []provider.Attachment{
				{Type: "image", Name: "broken.png", Data: "!!!bad!!!", MimeType: "image/png"},
			},
		},
	}

	got := toGeminiContents(msgs)
	c := got[0]
	// Should only have the text part; the malformed image should be skipped.
	if len(c.Parts) != 1 {
		t.Fatalf("want 1 part (text only, bad image skipped), got %d", len(c.Parts))
	}
	if c.Parts[0].Text != "is this valid?" {
		t.Errorf("text mismatch: %q", c.Parts[0].Text)
	}
}
