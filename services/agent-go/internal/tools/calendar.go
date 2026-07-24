package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// CalendarTool -- manage a local .ics calendar
// ---------------------------------------------------------------------------

type calendarTool struct {
	icsPath string
}

// NewCalendarTool creates a calendar tool. icsPath defaults to ~/.jarvis/calendar.ics.
func NewCalendarTool(icsPath string) Tool {
	if icsPath == "" {
		home, _ := os.UserHomeDir()
		icsPath = home + "/.jarvis/calendar.ics"
	}
	return &calendarTool{icsPath: icsPath}
}

func (t *calendarTool) Name() string { return "calendar" }

func (t *calendarTool) Description() string {
	return "Manage a local .ics calendar: list today's events or add new events."
}

func (t *calendarTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"action":{"type":"string","enum":["today","add"],"description":"today (list) or add (create event)"},
			"title":{"type":"string","description":"event title (required for add)"},
			"date":{"type":"string","description":"event date YYYY-MM-DD (required for add)"},
			"time":{"type":"string","description":"event time HH:MM (required for add)"},
			"duration":{"type":"string","description":"duration e.g. 30m, 1h (default 1h)"}
		},
		"required":["action"],
		"additionalProperties":false
	}`)
}

func (t *calendarTool) Kind() Kind { return KindRead }

type icsEvent struct {
	Title    string
	Date     string
	Time     string
	Duration string
}

func (t *calendarTool) Execute(ctx context.Context, rawArgs json.RawMessage) (Result, error) {
	var args struct {
		Action   string `json:"action"`
		Title    string `json:"title"`
		Date     string `json:"date"`
		Time     string `json:"time"`
		Duration string `json:"duration"`
	}
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return Result{}, fmt.Errorf("calendar: invalid args: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	switch args.Action {
	case "today":
		return t.listToday(ctx)
	case "add":
		return t.addEvent(ctx, args.Title, args.Date, args.Time, args.Duration)
	default:
		return Result{}, fmt.Errorf("calendar: unknown action %q, use 'today' or 'add'", args.Action)
	}
}

func (t *calendarTool) listToday(ctx context.Context) (Result, error) {
	events := t.parseICS()
	today := time.Now().Format("2006-01-02")

	var todayEvents []map[string]string
	for _, e := range events {
		if e.Date == today {
			todayEvents = append(todayEvents, map[string]string{
				"title":    e.Title,
				"date":     e.Date,
				"time":     e.Time,
				"duration": e.Duration,
			})
		}
	}

	select {
	case <-ctx.Done():
		return Result{}, ctx.Err()
	default:
	}

	out, _ := json.Marshal(map[string]any{
		"date":   today,
		"count":  len(todayEvents),
		"events": todayEvents,
	})
	return Result{Content: string(out)}, nil
}

func (t *calendarTool) addEvent(ctx context.Context, title, date, timeVal, duration string) (Result, error) {
	if title == "" {
		return Result{}, fmt.Errorf("calendar.add: title is required")
	}
	if date == "" {
		return Result{}, fmt.Errorf("calendar.add: date is required (YYYY-MM-DD)")
	}
	if timeVal == "" {
		return Result{}, fmt.Errorf("calendar.add: time is required (HH:MM)")
	}
	if duration == "" {
		duration = "1h"
	}

	// Validate date format
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return Result{}, fmt.Errorf("calendar.add: invalid date format, use YYYY-MM-DD")
	}

	entry := fmt.Sprintf("BEGIN:VEVENT\nSUMMARY:%s\nDTSTART:%sT%s:00\nDURATION:%s\nEND:VEVENT\n",
		escapeICS(title), date, timeVal, duration)

	f, err := os.OpenFile(t.icsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return Result{}, fmt.Errorf("calendar.add: %w", err)
	}
	defer f.Close()

	// Write ICS header if new file
	stat, _ := f.Stat()
	if stat.Size() == 0 {
		f.WriteString("BEGIN:VCALENDAR\nVERSION:2.0\n")
	}
	f.WriteString(entry)

	select {
	case <-ctx.Done():
		return Result{}, ctx.Err()
	default:
	}

	out, _ := json.Marshal(map[string]any{
		"title":    title,
		"date":     date,
		"time":     timeVal,
		"duration": duration,
		"added":    true,
	})
	return Result{Content: string(out)}, nil
}

func (t *calendarTool) parseICS() []icsEvent {
	data, err := os.ReadFile(t.icsPath)
	if err != nil {
		return nil
	}
	content := string(data)
	var events []icsEvent
	var current icsEvent

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "SUMMARY:"):
			current.Title = unescapeICS(strings.TrimPrefix(line, "SUMMARY:"))
		case strings.HasPrefix(line, "DTSTART:"):
			dt := strings.TrimPrefix(line, "DTSTART:")
			if len(dt) >= 10 {
				current.Date = dt[:10]
			}
			if len(dt) >= 16 {
				current.Time = dt[11:16]
			}
		case strings.HasPrefix(line, "DURATION:"):
			current.Duration = strings.TrimPrefix(line, "DURATION:")
		case line == "END:VEVENT":
			if current.Title != "" {
				events = append(events, current)
			}
			current = icsEvent{}
		}
	}
	return events
}

func escapeICS(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, ";", "\\;")
	s = strings.ReplaceAll(s, ",", "\\,")
	return s
}

func unescapeICS(s string) string {
	s = strings.ReplaceAll(s, "\\;", ";")
	s = strings.ReplaceAll(s, "\\,", ",")
	s = strings.ReplaceAll(s, "\\\\", "\\")
	return s
}
