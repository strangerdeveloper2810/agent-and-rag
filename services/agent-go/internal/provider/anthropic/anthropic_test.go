package anthropic

import (
	"encoding/json"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// marshalJSON marshal 1 giá trị có MarshalJSON rồi decode về map để soi wire shape.
func marshalToMap(t *testing.T, v any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal lỗi: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal về map lỗi: %v (raw=%s)", err, raw)
	}
	return m
}

func TestToAnthropicMessages_RolesAndShape(t *testing.T) {
	msgs := []provider.Message{
		{Role: provider.RoleSystem, Content: "bỏ qua tôi"}, // phải bị skip
		{Role: provider.RoleUser, Content: "thời tiết Hà Nội?"},
		{
			Role:    provider.RoleAssistant,
			Content: "để tôi tra cứu",
			ToolCalls: []provider.ToolCall{
				{ID: "call_1", Name: "get_weather", Args: json.RawMessage(`{"city":"Hanoi"}`)},
			},
		},
		{Role: provider.RoleTool, ToolCallID: "call_1", Content: "32°C, nắng"},
	}

	got := toAnthropicMessages(msgs)

	// System bị skip → còn 3 message.
	if len(got) != 3 {
		t.Fatalf("len(messages) = %d, muốn 3 (system phải bị skip)", len(got))
	}

	// [0] user
	user := marshalToMap(t, got[0])
	if user["role"] != "user" {
		t.Fatalf("msg[0].role = %v, muốn user", user["role"])
	}
	userContent, ok := user["content"].([]any)
	if !ok || len(userContent) != 1 {
		t.Fatalf("msg[0].content không phải mảng 1 phần tử: %#v", user["content"])
	}
	ub := userContent[0].(map[string]any)
	if ub["type"] != "text" || ub["text"] != "thời tiết Hà Nội?" {
		t.Fatalf("msg[0] text block sai: %#v", ub)
	}

	// [1] assistant: text + tool_use
	asst := marshalToMap(t, got[1])
	if asst["role"] != "assistant" {
		t.Fatalf("msg[1].role = %v, muốn assistant", asst["role"])
	}
	asstContent := asst["content"].([]any)
	if len(asstContent) != 2 {
		t.Fatalf("msg[1].content muốn 2 block (text+tool_use), có %d", len(asstContent))
	}
	txt := asstContent[0].(map[string]any)
	if txt["type"] != "text" || txt["text"] != "để tôi tra cứu" {
		t.Fatalf("msg[1] text block sai: %#v", txt)
	}
	tu := asstContent[1].(map[string]any)
	if tu["type"] != "tool_use" {
		t.Fatalf("msg[1] block[1].type = %v, muốn tool_use", tu["type"])
	}
	if tu["id"] != "call_1" || tu["name"] != "get_weather" {
		t.Fatalf("tool_use id/name sai: %#v", tu)
	}
	input, ok := tu["input"].(map[string]any)
	if !ok || input["city"] != "Hanoi" {
		t.Fatalf("tool_use input sai: %#v", tu["input"])
	}

	// [2] tool_result nằm trong user message
	tool := marshalToMap(t, got[2])
	if tool["role"] != "user" {
		t.Fatalf("msg[2].role = %v, muốn user (tool_result là user content)", tool["role"])
	}
	trContent := tool["content"].([]any)
	if len(trContent) != 1 {
		t.Fatalf("msg[2].content muốn 1 block, có %d", len(trContent))
	}
	tr := trContent[0].(map[string]any)
	if tr["type"] != "tool_result" || tr["tool_use_id"] != "call_1" {
		t.Fatalf("tool_result sai: %#v", tr)
	}
}

func TestToAnthropicMessages_AssistantOnlyToolCall(t *testing.T) {
	// Assistant không có text, chỉ có tool call → không được rỗng.
	msgs := []provider.Message{
		{
			Role: provider.RoleAssistant,
			ToolCalls: []provider.ToolCall{
				{ID: "c1", Name: "search", Args: json.RawMessage(`{"q":"go"}`)},
			},
		},
	}
	got := toAnthropicMessages(msgs)
	if len(got) != 1 {
		t.Fatalf("len = %d, muốn 1", len(got))
	}
	content := marshalToMap(t, got[0])["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("muốn 1 block tool_use, có %d", len(content))
	}
	if content[0].(map[string]any)["type"] != "tool_use" {
		t.Fatalf("block phải là tool_use: %#v", content[0])
	}
}

func TestToAnthropicMessages_EmptyToolArgsDefaultsToObject(t *testing.T) {
	// Args nil → phải marshal thành "{}" chứ không lỗi.
	msgs := []provider.Message{
		{
			Role:      provider.RoleAssistant,
			ToolCalls: []provider.ToolCall{{ID: "c1", Name: "noop"}},
		},
	}
	got := toAnthropicMessages(msgs)
	content := marshalToMap(t, got[0])["content"].([]any)
	tu := content[0].(map[string]any)
	input, ok := tu["input"].(map[string]any)
	if !ok {
		t.Fatalf("input phải là object, có %#v", tu["input"])
	}
	if len(input) != 0 {
		t.Fatalf("input muốn object rỗng, có %#v", input)
	}
}

func TestToAnthropicMessages_Empty(t *testing.T) {
	if got := toAnthropicMessages(nil); len(got) != 0 {
		t.Fatalf("nil messages → muốn 0, có %d", len(got))
	}
}

func TestToAnthropicTools_MapsSchema(t *testing.T) {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {"city": {"type": "string"}},
		"required": ["city"]
	}`)
	tools := []provider.ToolDef{
		{Name: "get_weather", Description: "Lấy thời tiết theo thành phố", Schema: schema},
	}

	got := toAnthropicTools(tools)
	if len(got) != 1 {
		t.Fatalf("len(tools) = %d, muốn 1", len(got))
	}

	m := marshalToMap(t, got[0])
	if m["name"] != "get_weather" {
		t.Fatalf("name = %v, muốn get_weather", m["name"])
	}
	if m["description"] != "Lấy thời tiết theo thành phố" {
		t.Fatalf("description sai: %v", m["description"])
	}
	in, ok := m["input_schema"].(map[string]any)
	if !ok {
		t.Fatalf("input_schema phải là object: %#v", m["input_schema"])
	}
	if in["type"] != "object" {
		t.Fatalf("input_schema.type = %v, muốn object", in["type"])
	}
	props, ok := in["properties"].(map[string]any)
	if !ok || props["city"] == nil {
		t.Fatalf("input_schema.properties sai: %#v", in["properties"])
	}
	req, ok := in["required"].([]any)
	if !ok || len(req) != 1 || req[0] != "city" {
		t.Fatalf("input_schema.required sai: %#v", in["required"])
	}
}

func TestToAnthropicTools_NoDescription(t *testing.T) {
	tools := []provider.ToolDef{
		{Name: "noop", Schema: json.RawMessage(`{"type":"object"}`)},
	}
	m := marshalToMap(t, toAnthropicTools(tools)[0])
	if _, has := m["description"]; has {
		t.Fatalf("không có Description → field description không nên xuất hiện: %#v", m)
	}
	if m["name"] != "noop" {
		t.Fatalf("name = %v, muốn noop", m["name"])
	}
}

func TestToAnthropicTools_Empty(t *testing.T) {
	if got := toAnthropicTools(nil); got != nil {
		t.Fatalf("nil tools → muốn nil, có %#v", got)
	}
}
