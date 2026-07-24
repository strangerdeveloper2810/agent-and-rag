package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCalendarTool_Today(t *testing.T) {
	dir := t.TempDir()
	icsPath := filepath.Join(dir, "test.ics")

	// Pre-populate with today's event
	today := time.Now().Format("2006-01-02")
	ics := "BEGIN:VCALENDAR\nVERSION:2.0\n" +
		"BEGIN:VEVENT\nSUMMARY:Morning Standup\nDTSTART:" + today + "T09:00:00\nDURATION:30m\nEND:VEVENT\n"
	os.WriteFile(icsPath, []byte(ics), 0644)

	tool := NewCalendarTool(icsPath)
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"action":"today"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out struct {
		Date   string `json:"date"`
		Count  int    `json:"count"`
		Events []struct {
			Title string `json:"title"`
		} `json:"events"`
	}
	if err := json.Unmarshal([]byte(result.Content), &out); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if out.Date != today {
		t.Errorf("expected date %s, got %s", today, out.Date)
	}
	if out.Count < 1 {
		t.Error("expected at least 1 today event")
	}
}

func TestCalendarTool_Add(t *testing.T) {
	dir := t.TempDir()
	icsPath := filepath.Join(dir, "test.ics")

	tool := NewCalendarTool(icsPath)

	tests := []struct {
		name    string
		args    string
		wantErr bool
	}{
		{
			name:    "valid add",
			args:    `{"action":"add","title":"Lunch","date":"2026-07-24","time":"12:00","duration":"1h"}`,
			wantErr: false,
		},
		{
			name:    "missing title",
			args:    `{"action":"add","date":"2026-07-24","time":"12:00"}`,
			wantErr: true,
		},
		{
			name:    "missing date",
			args:    `{"action":"add","title":"Meeting","time":"12:00"}`,
			wantErr: true,
		},
		{
			name:    "invalid date",
			args:    `{"action":"add","title":"Bad","date":"not-a-date","time":"12:00"}`,
			wantErr: true,
		},
		{
			name:    "unknown action",
			args:    `{"action":"delete"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), json.RawMessage(tt.args))
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v, result = %s", err, tt.wantErr, result.Content)
			}
		})
	}
}

func TestCalendarTool_AddCreatesFile(t *testing.T) {
	dir := t.TempDir()
	icsPath := filepath.Join(dir, "test.ics")

	tool := NewCalendarTool(icsPath)
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"action":"add","title":"Test","date":"2026-07-24","time":"10:00"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(icsPath)
	if err != nil {
		t.Fatalf("failed to read ics: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "Test") || !strings.Contains(content, "VEVENT") {
		t.Errorf("ics file missing expected content: %s", content)
	}
}

func TestCalendarTool_EmptyCalendar(t *testing.T) {
	dir := t.TempDir()
	icsPath := filepath.Join(dir, "nonexistent.ics")

	tool := NewCalendarTool(icsPath)
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"action":"today"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out struct {
		Count int `json:"count"`
	}
	json.Unmarshal([]byte(result.Content), &out)
	if out.Count != 0 {
		t.Errorf("expected 0 events for empty calendar, got %d", out.Count)
	}
}

func TestICSFunctions(t *testing.T) {
	input := "Hello; World, Test\\Done"
	escaped := escapeICS(input)
	unescaped := unescapeICS(escaped)

	if unescaped != input {
		t.Errorf("roundtrip failed: input=%q unescaped=%q", input, unescaped)
	}
}
