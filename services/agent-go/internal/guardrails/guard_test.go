package guardrails

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/tools"
)

// fakeGuardTool là Tool giả cho test CheckTool với Kind tuỳ chọn.
type fakeGuardTool struct {
	name string
	kind tools.Kind
}

func (f fakeGuardTool) Name() string            { return f.name }
func (f fakeGuardTool) Description() string     { return "fake guard tool" }
func (f fakeGuardTool) Schema() json.RawMessage { return nil }
func (f fakeGuardTool) Kind() tools.Kind        { return f.kind }
func (f fakeGuardTool) Execute(_ context.Context, _ json.RawMessage) (tools.Result, error) {
	return tools.Result{}, nil
}

func TestCheckTool(t *testing.T) {
	tests := []struct {
		name      string
		tool      tools.Tool
		wantErr   bool
		errIsConf bool
	}{
		{
			name:    "read cho phép",
			tool:    fakeGuardTool{name: "read_file", kind: tools.KindRead},
			wantErr: false,
		},
		{
			name:    "write cho phép + log info",
			tool:    fakeGuardTool{name: "save_note", kind: tools.KindWrite},
			wantErr: false,
		},
		{
			name:      "destructive cần xác nhận",
			tool:      fakeGuardTool{name: "delete_task", kind: tools.KindDestructive},
			wantErr:   true,
			errIsConf: true,
		},
		{
			name:    "kind không xác định",
			tool:    fakeGuardTool{name: "mystery", kind: tools.Kind(42)},
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckTool(tc.tool)
			if tc.wantErr && err == nil {
				t.Fatal("CheckTool = nil, want lỗi")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("CheckTool = %v, want nil", err)
			}
			if tc.errIsConf {
				conf, ok := err.(*NeedConfirmationError)
				if !ok {
					t.Fatalf("err = %T, want *NeedConfirmationError", err)
				}
				if conf.Tool != tc.tool.Name() {
					t.Fatalf("conf.Tool = %q, want %q", conf.Tool, tc.tool.Name())
				}
			} else if tc.wantErr && !strings.Contains(err.Error(), "unknown tool kind") {
				t.Fatalf("err = %v, want chứa unknown tool kind", err)
			}
		})
	}
}

func TestNeedConfirmationError_Error(t *testing.T) {
	err := &NeedConfirmationError{Tool: "delete_task"}
	msg := err.Error()
	if !strings.Contains(msg, "delete_task") || !strings.Contains(msg, "confirmation") {
		t.Fatalf("Error() = %q, want chứa tên tool và từ confirmation", msg)
	}
}
