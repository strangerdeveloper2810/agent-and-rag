package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ---------------------------------------------------------------------------
// DateTimeTool -- current time, timezone conversion, date arithmetic
// ---------------------------------------------------------------------------

type dateTimeTool struct{}

// NewDateTimeTool creates a date/time utility tool.
func NewDateTimeTool() Tool {
	return &dateTimeTool{}
}

func (t *dateTimeTool) Name() string { return "datetime" }

func (t *dateTimeTool) Description() string {
	return "Get current time, convert between timezones, add/subtract durations, or calculate the difference between two datetimes."
}

func (t *dateTimeTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"operation":{"type":"string","enum":["now","convert","add","diff"],"description":"operation to perform"},
			"datetime":{"type":"string","description":"ISO 8601 datetime string (required for convert, add, diff)"},
			"timezone":{"type":"string","description":"IANA timezone, e.g. 'Asia/Ho_Chi_Minh', 'America/New_York', 'UTC' (for now, convert)"},
			"duration":{"type":"string","description":"duration to add, e.g. '1h', '30m', '-1d' (for add)"},
			"datetime2":{"type":"string","description":"second ISO 8601 datetime (required for diff)"},
			"format":{"type":"string","description":"output format, e.g. '2006-01-02 15:04:05' (optional)"}
		},
		"required":["operation"],
		"additionalProperties":false
	}`)
}

func (t *dateTimeTool) Kind() Kind { return KindRead }

func (t *dateTimeTool) Execute(ctx context.Context, rawArgs json.RawMessage) (Result, error) {
	var args struct {
		Operation string `json:"operation"`
		Datetime  string `json:"datetime"`
		Timezone  string `json:"timezone"`
		Duration  string `json:"duration"`
		Datetime2 string `json:"datetime2"`
		Format    string `json:"format"`
	}
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return Result{}, fmt.Errorf("datetime: invalid args: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	switch args.Operation {
	case "now":
		return t.now(ctx, args.Timezone, args.Format)
	case "convert":
		return t.convert(ctx, args.Datetime, args.Timezone, args.Format)
	case "add":
		return t.addDuration(ctx, args.Datetime, args.Duration, args.Format)
	case "diff":
		return t.diff(ctx, args.Datetime, args.Datetime2)
	default:
		return Result{}, fmt.Errorf("datetime: unknown operation %q, use 'now', 'convert', 'add', or 'diff'", args.Operation)
	}
}

func (t *dateTimeTool) now(ctx context.Context, timezone, format string) (Result, error) {
	now := time.Now()
	loc, locName, err := resolveLocation(timezone)
	if err != nil {
		return Result{}, fmt.Errorf("datetime.now: %w", err)
	}

	nowInLoc := now.In(loc)
	formatted := formatTime(nowInLoc, format)

	select {
	case <-ctx.Done():
		return Result{}, ctx.Err()
	default:
	}

	out, _ := json.Marshal(map[string]any{
		"operation":  "now",
		"datetime":   nowInLoc.Format(time.RFC3339),
		"formatted":  formatted,
		"timezone":   locName,
		"unix":       now.Unix(),
		"unixMilli":  now.UnixMilli(),
		"dayOfWeek":  nowInLoc.Weekday().String(),
		"weekNumber": weekNumber(nowInLoc),
	})
	return Result{Content: string(out)}, nil
}

func (t *dateTimeTool) convert(ctx context.Context, datetime, timezone, format string) (Result, error) {
	if datetime == "" {
		return Result{}, fmt.Errorf("datetime.convert: datetime is required")
	}

	parsed, err := parseTime(datetime)
	if err != nil {
		return Result{}, fmt.Errorf("datetime.convert: %w", err)
	}

	srcLoc, srcName, err := resolveLocation(timezone)
	if err != nil {
		return Result{}, fmt.Errorf("datetime.convert: %w", err)
	}

	converted := parsed.In(srcLoc)
	formatted := formatTime(converted, format)

	select {
	case <-ctx.Done():
		return Result{}, ctx.Err()
	default:
	}

	out, _ := json.Marshal(map[string]any{
		"operation":      "convert",
		"original":       datetime,
		"converted":      converted.Format(time.RFC3339),
		"formatted":      formatted,
		"targetTimezone": srcName,
	})
	return Result{Content: string(out)}, nil
}

func (t *dateTimeTool) addDuration(ctx context.Context, datetime, duration, format string) (Result, error) {
	if datetime == "" {
		return Result{}, fmt.Errorf("datetime.add: datetime is required")
	}
	if duration == "" {
		return Result{}, fmt.Errorf("datetime.add: duration is required (e.g. 1h, 30m, -1d)")
	}

	parsed, err := parseTime(datetime)
	if err != nil {
		return Result{}, fmt.Errorf("datetime.add: %w", err)
	}

	d, err := time.ParseDuration(duration)
	if err != nil {
		return Result{}, fmt.Errorf("datetime.add: invalid duration %q: %w", duration, err)
	}

	result := parsed.Add(d)
	formatted := formatTime(result, format)

	select {
	case <-ctx.Done():
		return Result{}, ctx.Err()
	default:
	}

	out, _ := json.Marshal(map[string]any{
		"operation": "add",
		"original":  parsed.Format(time.RFC3339),
		"duration":  duration,
		"result":    result.Format(time.RFC3339),
		"formatted": formatted,
	})
	return Result{Content: string(out)}, nil
}

func (t *dateTimeTool) diff(ctx context.Context, datetime1, datetime2 string) (Result, error) {
	if datetime1 == "" {
		return Result{}, fmt.Errorf("datetime.diff: first datetime is required")
	}
	if datetime2 == "" {
		return Result{}, fmt.Errorf("datetime.diff: second datetime is required")
	}

	t1, err := parseTime(datetime1)
	if err != nil {
		return Result{}, fmt.Errorf("datetime.diff: first datetime: %w", err)
	}
	t2, err := parseTime(datetime2)
	if err != nil {
		return Result{}, fmt.Errorf("datetime.diff: second datetime: %w", err)
	}

	diff := t2.Sub(t1)
	absDiff := diff
	if absDiff < 0 {
		absDiff = -absDiff
	}

	hours := diff.Hours()
	minutes := diff.Minutes()
	seconds := diff.Seconds()

	select {
	case <-ctx.Done():
		return Result{}, ctx.Err()
	default:
	}

	out, _ := json.Marshal(map[string]any{
		"operation":  "diff",
		"datetime1":  t1.Format(time.RFC3339),
		"datetime2":  t2.Format(time.RFC3339),
		"difference": diff.String(),
		"hours":      hours,
		"minutes":    minutes,
		"seconds":    seconds,
		"days":       hours / 24,
	})
	return Result{Content: string(out)}, nil
}

// parseTime tries multiple common datetime formats.
func parseTime(s string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
		time.DateOnly,
		time.DateTime,
		"2006-01-02T15:04:05Z07:00",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse datetime %q", s)
}

// resolveLocation resolves an IANA timezone name to *time.Location.
func resolveLocation(tz string) (*time.Location, string, error) {
	if tz == "" {
		return time.Local, "Local", nil
	}

	// Map common short aliases to canonical IANA names first
	aliases := map[string]string{
		"UTC":  "UTC",
		"utc":  "UTC",
		"est":  "America/New_York",
		"pst":  "America/Los_Angeles",
		"cst":  "America/Chicago",
		"ist":  "Asia/Kolkata",
		"jst":  "Asia/Tokyo",
		"cet":  "Europe/Berlin",
		"gmt":  "Europe/London",
		"aest": "Australia/Sydney",
	}
	lookup := tz
	if mapped, ok := aliases[tz]; ok {
		lookup = mapped
	}

	loc, err := time.LoadLocation(lookup)
	if err != nil {
		return nil, "", fmt.Errorf("unknown timezone %q", tz)
	}
	return loc, lookup, nil
}

// formatTime formats a time.Time, using the specified layout or default RFC3339.
func formatTime(t time.Time, layout string) string {
	if layout != "" {
		return t.Format(layout)
	}
	return t.Format(time.RFC3339)
}

// weekNumber returns the ISO week number.
func weekNumber(t time.Time) int {
	_, week := t.ISOWeek()
	return week
}
