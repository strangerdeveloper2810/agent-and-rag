package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// JSONTool -- parse, format, validate, query JSON
// ---------------------------------------------------------------------------

type jsonTool struct{}

// NewJSONTool creates a JSON utility tool.
func NewJSONTool() Tool {
	return &jsonTool{}
}

func (t *jsonTool) Name() string { return "json" }

func (t *jsonTool) Description() string {
	return "Parse, format, validate, or query JSON data. Operations: format (pretty-print), get (extract by jq-style path), validate (check if valid JSON)."
}

func (t *jsonTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"operation":{"type":"string","enum":["format","get","validate"],"description":"operation to perform"},
			"data":{"type":"string","description":"JSON string to operate on"},
			"path":{"type":"string","description":"dot-separated path for 'get' operation, e.g. 'user.name' or 'items.0.title'"}
		},
		"required":["operation","data"],
		"additionalProperties":false
	}`)
}

func (t *jsonTool) Kind() Kind { return KindRead }

func (t *jsonTool) Execute(ctx context.Context, rawArgs json.RawMessage) (Result, error) {
	var args struct {
		Operation string `json:"operation"`
		Data      string `json:"data"`
		Path      string `json:"path"`
	}
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return Result{}, fmt.Errorf("json: invalid args: %w", err)
	}
	if strings.TrimSpace(args.Data) == "" {
		return Result{}, fmt.Errorf("json: data is required")
	}

	// Validate data is valid JSON first
	var parsed any
	if err := json.Unmarshal([]byte(args.Data), &parsed); err != nil {
		return Result{}, fmt.Errorf("json: invalid JSON: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	switch args.Operation {
	case "format":
		return t.formatJSON(ctx, parsed)
	case "get":
		return t.getPath(ctx, parsed, args.Path)
	case "validate":
		return t.validateJSON(ctx, args.Data)
	default:
		return Result{}, fmt.Errorf("json: unknown operation %q, use 'format', 'get', or 'validate'", args.Operation)
	}
}

func (t *jsonTool) formatJSON(ctx context.Context, parsed any) (Result, error) {
	formatted, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		return Result{}, fmt.Errorf("json.format: %w", err)
	}

	select {
	case <-ctx.Done():
		return Result{}, ctx.Err()
	default:
	}

	result := string(formatted)
	if len(result) > 10_000 {
		result = result[:10_000] + "\n... [truncated]"
	}

	out, _ := json.Marshal(map[string]any{
		"operation": "format",
		"formatted": result,
	})
	return Result{Content: string(out)}, nil
}

func (t *jsonTool) getPath(ctx context.Context, parsed any, path string) (Result, error) {
	if path == "" {
		return Result{}, fmt.Errorf("json.get: path is required")
	}

	value, err := resolveJSONPath(parsed, path)
	if err != nil {
		return Result{}, fmt.Errorf("json.get: %w", err)
	}

	select {
	case <-ctx.Done():
		return Result{}, ctx.Err()
	default:
	}

	valueStr, _ := json.Marshal(value)
	if len(valueStr) > 10_000 {
		valueStr = valueStr[:10_000]
	}

	out, _ := json.Marshal(map[string]any{
		"operation": "get",
		"path":      path,
		"value":     json.RawMessage(valueStr),
	})
	return Result{Content: string(out)}, nil
}

func (t *jsonTool) validateJSON(ctx context.Context, data string) (Result, error) {
	// Already validated in Execute, so we know it's valid
	select {
	case <-ctx.Done():
		return Result{}, ctx.Err()
	default:
	}

	out, _ := json.Marshal(map[string]any{
		"operation": "validate",
		"valid":     true,
		"size":      len(data),
	})
	return Result{Content: string(out)}, nil
}

// resolveJSONPath resolves a dot-separated path against an arbitrary JSON value.
// Supports: "user.name", "items.0.title", "data.0" (array index).
func resolveJSONPath(data any, path string) (any, error) {
	parts := strings.Split(path, ".")
	current := data

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]any:
			val, ok := v[part]
			if !ok {
				return nil, fmt.Errorf("key %q not found", part)
			}
			current = val
		case []any:
			// Try to parse as array index
			var idx int
			if _, err := fmt.Sscanf(part, "%d", &idx); err != nil {
				return nil, fmt.Errorf("cannot access array with key %q", part)
			}
			if idx < 0 || idx >= len(v) {
				return nil, fmt.Errorf("array index %d out of bounds (len=%d)", idx, len(v))
			}
			current = v[idx]
		default:
			return nil, fmt.Errorf("cannot traverse into %T with key %q", current, part)
		}
	}

	return current, nil
}
