package tools_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/tools"
)

func TestAskUserTool(t *testing.T) {
	tool := tools.NewAskUserTool()
	if tool.Name() != "ask_user" {
		t.Errorf("name = %q, want 'ask_user'", tool.Name())
	}
	if tool.Kind() != tools.KindRead {
		t.Errorf("kind = %v, want KindRead", tool.Kind())
	}

	payload := `{
		"questions": [
			{
				"prompt": "Chọn hệ điều hành mục tiêu",
				"header": "Nền tảng",
				"options": [
					{"label": "Linux", "recommended": true},
					{"label": "Windows"}
				],
				"multi_select": false
			}
		]
	}`

	res, err := tool.Execute(context.Background(), json.RawMessage(payload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.Content) == 0 {
		t.Error("expected non-empty content")
	}

	// Empty questions error test
	_, err = tool.Execute(context.Background(), json.RawMessage(`{"questions":[]}`))
	if err == nil {
		t.Error("expected error on empty questions")
	}

	// Invalid json error test
	_, err = tool.Execute(context.Background(), json.RawMessage(`invalid-json`))
	if err == nil {
		t.Error("expected error on invalid JSON")
	}
}
