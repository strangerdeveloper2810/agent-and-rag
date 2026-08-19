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

	// 1. Nested questions format
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

	// 2. Single question object (Parallel tool-call format from Gemini)
	singlePayload := `{
		"prompt": "Chọn Database chính",
		"header": "Database",
		"options": [
			{"label": "PostgreSQL", "recommended": true},
			{"label": "MySQL"}
		]
	}`
	resSingle, err := tool.Execute(context.Background(), json.RawMessage(singlePayload))
	if err != nil {
		t.Fatalf("unexpected error for single question: %v", err)
	}
	if len(resSingle.Content) == 0 {
		t.Error("expected non-empty content for single question")
	}

	// 3. String array options format
	stringOptsPayload := `{
		"question": "Chọn Go Web Framework",
		"options": ["Gin", "Fiber", "Echo"]
	}`
	resOpts, err := tool.Execute(context.Background(), json.RawMessage(stringOptsPayload))
	if err != nil {
		t.Fatalf("unexpected error for string options: %v", err)
	}
	if len(resOpts.Content) == 0 {
		t.Error("expected non-empty content for string options")
	}

	// 4. Array at top level
	arrayPayload := `[{"prompt": "Chọn Cloud", "options": [{"label": "AWS"}, {"label": "GCP"}]}]`
	resArr, err := tool.Execute(context.Background(), json.RawMessage(arrayPayload))
	if err != nil {
		t.Fatalf("unexpected error for array payload: %v", err)
	}
	if len(resArr.Content) == 0 {
		t.Error("expected non-empty content for array payload")
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
