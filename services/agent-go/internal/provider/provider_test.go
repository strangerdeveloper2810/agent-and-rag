package provider

import (
	"context"
	"encoding/json"
	"testing"
)

func TestBuildToolResultMessage(t *testing.T) {
	m := BuildToolResultMessage("call-1", "42")

	if m.Role != RoleTool {
		t.Errorf("Role = %q, want %q", m.Role, RoleTool)
	}
	if m.ToolCallID != "call-1" {
		t.Errorf("ToolCallID = %q, want call-1", m.ToolCallID)
	}
	if m.Content != "42" {
		t.Errorf("Content = %q, want 42", m.Content)
	}
	if len(m.ToolCalls) != 0 {
		t.Errorf("ToolCalls = %v, want rỗng", m.ToolCalls)
	}
}

func TestBuildToolResultMessage_EmptyResult(t *testing.T) {
	m := BuildToolResultMessage("", "")
	if m.Role != RoleTool || m.Content != "" || m.ToolCallID != "" {
		t.Errorf("BuildToolResultMessage(\"\",\"\") = %+v", m)
	}
}

func TestFakeProvider_Name(t *testing.T) {
	if got := NewFake().Name(); got != "fake" {
		t.Errorf("Name() = %q, want fake", got)
	}
}

func TestFakeProvider_ReplaysChunksInOrder(t *testing.T) {
	want := []StreamChunk{
		{Kind: ChunkText, Text: "a"},
		{Kind: ChunkText, Text: "b"},
		{Kind: ChunkToolCall, ToolCall: &ToolCall{ID: "1", Name: "echo"}},
		{Kind: ChunkUsage, Usage: &Usage{InputTokens: 3, OutputTokens: 4}},
		{Kind: ChunkDone, FinishReason: FinishStop},
	}

	f := NewFake(want...)
	stream, err := f.Generate(context.Background(), GenerateRequest{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var got []StreamChunk
	for c := range stream {
		got = append(got, c)
	}

	if len(got) != len(want) {
		t.Fatalf("nhận %d chunk, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Kind != want[i].Kind || got[i].Text != want[i].Text {
			t.Errorf("chunk %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if got[4].FinishReason != FinishStop {
		t.Errorf("FinishReason = %q, want %q", got[4].FinishReason, FinishStop)
	}
}

func TestFakeProvider_EmptyScript(t *testing.T) {
	stream, err := NewFake().Generate(context.Background(), GenerateRequest{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	n := 0
	for range stream {
		n++
	}
	if n != 0 {
		t.Errorf("nhận %d chunk từ fake rỗng, want 0", n)
	}
}

// Channel PHẢI được đóng sau khi phát hết — nếu không, node model treo mãi.
func TestFakeProvider_ClosesChannel(t *testing.T) {
	stream, err := NewFake(StreamChunk{Kind: ChunkDone}).Generate(context.Background(), GenerateRequest{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	<-stream // chunk done
	if _, open := <-stream; open {
		t.Error("channel vẫn mở sau khi hết chunk")
	}
}

func TestFakeProvider_RespectsContextCancel(t *testing.T) {
	// Buffer channel bằng len(chunks)+1 nên fake không block; huỷ ctx TRƯỚC
	// khi đọc vẫn phải cho channel đóng, không rò goroutine.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	chunks := make([]StreamChunk, 100)
	for i := range chunks {
		chunks[i] = StreamChunk{Kind: ChunkText, Text: "x"}
	}

	stream, err := NewFake(chunks...).Generate(ctx, GenerateRequest{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for range stream { //nolint:revive // chỉ cần drain tới khi đóng
	}
}

func TestFakeProvider_ImplementsProvider(t *testing.T) {
	var _ Provider = NewFake()
}

func TestChunkKindConstants_AreDistinct(t *testing.T) {
	kinds := []ChunkKind{ChunkText, ChunkToolCall, ChunkUsage, ChunkDone, ChunkError}
	seen := map[ChunkKind]bool{}
	for _, k := range kinds {
		if seen[k] {
			t.Errorf("ChunkKind %d bị trùng", k)
		}
		seen[k] = true
	}
}

func TestFinishReasonConstants(t *testing.T) {
	if FinishStop != "stop" || FinishToolCalls != "tool_calls" || FinishLength != "length" {
		t.Errorf("FinishReason constants = %q/%q/%q", FinishStop, FinishToolCalls, FinishLength)
	}
}

func TestToolDef_SchemaRoundTrip(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"q":{"type":"string"}}}`)
	td := ToolDef{Name: "web.search", Description: "search", Schema: schema}

	data, err := json.Marshal(td.Schema)
	if err != nil {
		t.Fatalf("marshal schema: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	if parsed["type"] != "object" {
		t.Errorf("schema type = %v, want object", parsed["type"])
	}
}

func TestRoleConstants(t *testing.T) {
	roles := map[Role]string{
		RoleSystem:    "system",
		RoleUser:      "user",
		RoleAssistant: "assistant",
		RoleTool:      "tool",
	}
	for r, want := range roles {
		if string(r) != want {
			t.Errorf("Role %v = %q, want %q", r, string(r), want)
		}
	}
}

func TestThinkingLevelConstants(t *testing.T) {
	levels := map[ThinkingLevel]string{
		ThinkingOff:    "OFF",
		ThinkingLow:    "LOW",
		ThinkingMedium: "MEDIUM",
		ThinkingHigh:   "HIGH",
	}
	for l, want := range levels {
		if string(l) != want {
			t.Errorf("ThinkingLevel %v = %q, want %q", l, string(l), want)
		}
	}
}
