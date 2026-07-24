package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestJSONTool_Validate(t *testing.T) {
	tool := NewJSONTool()

	tests := []struct {
		name    string
		data    string
		wantErr bool
	}{
		{
			name:    "valid JSON object",
			data:    `{"name":"John","age":30}`,
			wantErr: false,
		},
		{
			name:    "valid JSON array",
			data:    `[1,2,3]`,
			wantErr: false,
		},
		{
			name:    "invalid JSON",
			data:    `{bad json`,
			wantErr: true,
		},
		{
			name:    "empty data",
			data:    ``,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tool.Execute(context.Background(), json.RawMessage(
				`{"operation":"validate","data":`+jsonEscape(tt.data)+`}`,
			))
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestJSONTool_Format(t *testing.T) {
	tool := NewJSONTool()

	result, err := tool.Execute(context.Background(), json.RawMessage(
		`{"operation":"format","data":"{\"name\":\"John\",\"nested\":{\"key\":\"value\"}}"}`,
	))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out struct {
		Formatted string `json:"formatted"`
	}
	json.Unmarshal([]byte(result.Content), &out)

	if !strings.Contains(out.Formatted, "\n") {
		t.Error("formatted JSON should contain newlines (pretty-print)")
	}
	if !strings.Contains(out.Formatted, "John") {
		t.Error("formatted JSON missing expected content")
	}
}

func TestJSONTool_Get(t *testing.T) {
	tool := NewJSONTool()

	tests := []struct {
		name      string
		data      string
		path      string
		wantErr   bool
		wantValue string
	}{
		{
			name:      "simple key",
			data:      `{"name":"John","age":30}`,
			path:      "name",
			wantErr:   false,
			wantValue: `"John"`,
		},
		{
			name:      "nested key",
			data:      `{"user":{"profile":{"email":"a@b.com"}}}`,
			path:      "user.profile.email",
			wantErr:   false,
			wantValue: `"a@b.com"`,
		},
		{
			name:      "array index",
			data:      `{"items":[{"title":"First"},{"title":"Second"}]}`,
			path:      "items.0.title",
			wantErr:   false,
			wantValue: `"First"`,
		},
		{
			name:      "array second element",
			data:      `{"items":[{"title":"First"},{"title":"Second"}]}`,
			path:      "items.1.title",
			wantErr:   false,
			wantValue: `"Second"`,
		},
		{
			name:    "missing key",
			data:    `{"a":1}`,
			path:    "b",
			wantErr: true,
		},
		{
			name:    "array index out of bounds",
			data:    `[1,2,3]`,
			path:    "5",
			wantErr: true,
		},
		{
			name:    "empty path",
			data:    `{"a":1}`,
			path:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := `{"operation":"get","data":` + jsonEscape(tt.data) + `,"path":"` + tt.path + `"}`
			result, err := tool.Execute(context.Background(), json.RawMessage(args))
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && tt.wantValue != "" {
				var out struct {
					Value json.RawMessage `json:"value"`
				}
				json.Unmarshal([]byte(result.Content), &out)
				if string(out.Value) != tt.wantValue {
					t.Errorf("expected value %s, got %s", tt.wantValue, string(out.Value))
				}
			}
		})
	}
}

func TestJSONTool_UnknownOperation(t *testing.T) {
	tool := NewJSONTool()

	_, err := tool.Execute(context.Background(), json.RawMessage(
		`{"operation":"unknown","data":"{}"}`,
	))
	if err == nil {
		t.Fatal("expected error for unknown operation")
	}
}

func TestJSONTool_InvalidArgs(t *testing.T) {
	tool := NewJSONTool()

	_, err := tool.Execute(context.Background(), json.RawMessage(`{bad`))
	if err == nil {
		t.Fatal("expected error for invalid args")
	}
}

// jsonEscape escapes a string for embedding in a JSON value.
func jsonEscape(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
