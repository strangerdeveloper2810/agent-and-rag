package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestDateTimeTool_Now(t *testing.T) {
	tool := NewDateTimeTool()

	tests := []struct {
		name    string
		args    string
		wantErr bool
	}{
		{
			name:    "now with no timezone",
			args:    `{"operation":"now"}`,
			wantErr: false,
		},
		{
			name:    "now with UTC",
			args:    `{"operation":"now","timezone":"UTC"}`,
			wantErr: false,
		},
		{
			name:    "now with timezone + format",
			args:    `{"operation":"now","timezone":"Asia/Ho_Chi_Minh","format":"2006-01-02 15:04:05"}`,
			wantErr: false,
		},
		{
			name:    "now with unknown timezone",
			args:    `{"operation":"now","timezone":"Mars/Olympus"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), json.RawMessage(tt.args))
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				var out struct {
					Operation string `json:"operation"`
					Datetime  string `json:"datetime"`
					Timezone  string `json:"timezone"`
					Unix      int64  `json:"unix"`
					DayOfWeek string `json:"dayOfWeek"`
				}
				json.Unmarshal([]byte(result.Content), &out)
				if out.Operation != "now" {
					t.Errorf("expected operation 'now', got %q", out.Operation)
				}
				if out.Unix == 0 {
					t.Error("expected non-zero unix timestamp")
				}
				if out.DayOfWeek == "" {
					t.Error("expected dayOfWeek to be set")
				}
			}
		})
	}
}

func TestDateTimeTool_Convert(t *testing.T) {
	tool := NewDateTimeTool()

	tests := []struct {
		name    string
		args    string
		wantErr bool
	}{
		{
			name:    "convert to UTC",
			args:    `{"operation":"convert","datetime":"2026-07-24T12:00:00Z","timezone":"UTC"}`,
			wantErr: false,
		},
		{
			name:    "convert to Asia/Ho_Chi_Minh",
			args:    `{"operation":"convert","datetime":"2026-07-24T12:00:00Z","timezone":"Asia/Ho_Chi_Minh"}`,
			wantErr: false,
		},
		{
			name:    "convert with format",
			args:    `{"operation":"convert","datetime":"2026-07-24T12:00:00Z","timezone":"America/New_York","format":"2006-01-02 15:04:05"}`,
			wantErr: false,
		},
		{
			name:    "missing datetime",
			args:    `{"operation":"convert","timezone":"UTC"}`,
			wantErr: true,
		},
		{
			name:    "invalid datetime",
			args:    `{"operation":"convert","datetime":"not-a-date","timezone":"UTC"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), json.RawMessage(tt.args))
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				var out struct {
					Operation      string `json:"operation"`
					Converted      string `json:"converted"`
					TargetTimezone string `json:"targetTimezone"`
				}
				json.Unmarshal([]byte(result.Content), &out)
				if out.Operation != "convert" {
					t.Errorf("expected operation 'convert', got %q", out.Operation)
				}
				if out.Converted == "" {
					t.Error("expected converted datetime to be set")
				}
			}
		})
	}
}

func TestDateTimeTool_Add(t *testing.T) {
	tool := NewDateTimeTool()

	tests := []struct {
		name     string
		args     string
		wantErr  bool
		checkVal func(t *testing.T, resultStr string)
	}{
		{
			name:    "add 1 hour",
			args:    `{"operation":"add","datetime":"2026-07-24T12:00:00Z","duration":"1h"}`,
			wantErr: false,
			checkVal: func(t *testing.T, s string) {
				if !strings.Contains(s, "13:00:00") {
					t.Errorf("expected 13:00:00 after adding 1h, got %s", s)
				}
			},
		},
		{
			name:    "add 30 minutes",
			args:    `{"operation":"add","datetime":"2026-07-24T12:00:00Z","duration":"30m"}`,
			wantErr: false,
			checkVal: func(t *testing.T, s string) {
				if !strings.Contains(s, "12:30:00") {
					t.Errorf("expected 12:30:00 after adding 30m, got %s", s)
				}
			},
		},
		{
			name:    "subtract 1 day",
			args:    `{"operation":"add","datetime":"2026-07-24T12:00:00Z","duration":"-24h"}`,
			wantErr: false,
			checkVal: func(t *testing.T, s string) {
				if !strings.Contains(s, "2026-07-23") {
					t.Errorf("expected 2026-07-23 after subtracting 24h, got %s", s)
				}
			},
		},
		{
			name:    "invalid duration",
			args:    `{"operation":"add","datetime":"2026-07-24T12:00:00Z","duration":"xyz"}`,
			wantErr: true,
		},
		{
			name:    "missing datetime",
			args:    `{"operation":"add","duration":"1h"}`,
			wantErr: true,
		},
		{
			name:    "missing duration",
			args:    `{"operation":"add","datetime":"2026-07-24T12:00:00Z"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), json.RawMessage(tt.args))
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && tt.checkVal != nil {
				tt.checkVal(t, result.Content)
			}
		})
	}
}

func TestDateTimeTool_Diff(t *testing.T) {
	tool := NewDateTimeTool()

	result, err := tool.Execute(context.Background(), json.RawMessage(
		`{"operation":"diff","datetime":"2026-07-24T12:00:00Z","datetime2":"2026-07-24T14:30:00Z"}`,
	))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out struct {
		Difference string  `json:"difference"`
		Hours      float64 `json:"hours"`
		Minutes    float64 `json:"minutes"`
	}
	json.Unmarshal([]byte(result.Content), &out)

	if out.Hours < 2.49 || out.Hours > 2.51 {
		t.Errorf("expected ~2.5 hours, got %f", out.Hours)
	}
	if out.Minutes < 149 || out.Minutes > 151 {
		t.Errorf("expected ~150 minutes, got %f", out.Minutes)
	}
}

func TestDateTimeTool_DiffErrors(t *testing.T) {
	tool := NewDateTimeTool()

	tests := []struct {
		name    string
		args    string
		wantErr bool
	}{
		{
			name:    "missing first datetime",
			args:    `{"operation":"diff","datetime2":"2026-07-24T12:00:00Z"}`,
			wantErr: true,
		},
		{
			name:    "missing second datetime",
			args:    `{"operation":"diff","datetime":"2026-07-24T12:00:00Z"}`,
			wantErr: true,
		},
		{
			name:    "invalid first datetime",
			args:    `{"operation":"diff","datetime":"invalid","datetime2":"2026-07-24T12:00:00Z"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tool.Execute(context.Background(), json.RawMessage(tt.args))
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDateTimeTool_UnknownOperation(t *testing.T) {
	tool := NewDateTimeTool()
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"operation":"unknown"}`))
	if err == nil {
		t.Fatal("expected error for unknown operation")
	}
}

func TestDateTimeTool_InvalidArgs(t *testing.T) {
	tool := NewDateTimeTool()
	_, err := tool.Execute(context.Background(), json.RawMessage(`{bad`))
	if err == nil {
		t.Fatal("expected error for invalid args")
	}
}

func TestParseTime(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"RFC3339", "2026-07-24T12:00:00Z", false},
		{"date only", "2026-07-24", false},
		{"datetime with space", "2026-07-24 15:04:05", false},
		{"RFC3339 nano", "2026-07-24T12:00:00.123456789Z", false},
		{"invalid", "not-a-date", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseTime(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTime(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestResolveLocation(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		wantLoc string
	}{
		{"UTC", "UTC", false, "UTC"},
		{"empty defaults to Local", "", false, "Local"},
		{"Asia/Ho_Chi_Minh", "Asia/Ho_Chi_Minh", false, "Asia/Ho_Chi_Minh"},
		{"alias est", "est", false, "America/New_York"},
		{"alias pst", "pst", false, "America/Los_Angeles"},
		{"unknown", "Narnia/Wardrobe", true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, name, err := resolveLocation(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("resolveLocation(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if err == nil && tt.wantLoc != "" && name != tt.wantLoc {
				t.Errorf("resolveLocation(%q) name = %q, want %q", tt.input, name, tt.wantLoc)
			}
		})
	}
}

func TestWeekNumber(t *testing.T) {
	// July 24, 2026 is a Friday — should be in week 30 or so
	d := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	week := weekNumber(d)
	if week < 1 || week > 53 {
		t.Errorf("week number %d is out of range", week)
	}
}
