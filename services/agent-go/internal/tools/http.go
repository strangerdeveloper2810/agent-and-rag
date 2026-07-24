package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// HTTPTool -- make arbitrary HTTP requests
// ---------------------------------------------------------------------------

type httpTool struct {
	httpClient *http.Client
}

// NewHTTPTool creates an HTTP request tool. client defaults to 15s timeout.
func NewHTTPTool(client *http.Client) Tool {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &httpTool{httpClient: client}
}

func (t *httpTool) Name() string { return "http" }

func (t *httpTool) Description() string {
	return "Make HTTP requests. Supports GET, POST, PUT, DELETE, PATCH. Returns status code, response headers, and body (truncated at 10KB)."
}

func (t *httpTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"method":{"type":"string","enum":["GET","POST","PUT","DELETE","PATCH"],"description":"HTTP method"},
			"url":{"type":"string","format":"uri","description":"request URL"},
			"headers":{"type":"object","description":"optional request headers as key-value pairs"},
			"body":{"type":"string","description":"request body (for POST, PUT, PATCH)"}
		},
		"required":["method","url"],
		"additionalProperties":false
	}`)
}

func (t *httpTool) Kind() Kind { return KindWrite }

func (t *httpTool) Execute(ctx context.Context, rawArgs json.RawMessage) (Result, error) {
	var args struct {
		Method  string            `json:"method"`
		URL     string            `json:"url"`
		Headers map[string]string `json:"headers,omitempty"`
		Body    string            `json:"body,omitempty"`
	}
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return Result{}, fmt.Errorf("http: invalid args: %w", err)
	}

	args.Method = strings.ToUpper(args.Method)
	if args.Method == "" {
		return Result{}, fmt.Errorf("http: method is required")
	}
	if args.URL == "" {
		return Result{}, fmt.Errorf("http: url is required")
	}

	validMethods := map[string]bool{"GET": true, "POST": true, "PUT": true, "DELETE": true, "PATCH": true}
	if !validMethods[args.Method] {
		return Result{}, fmt.Errorf("http: unsupported method %q", args.Method)
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	var reqBody io.Reader
	if args.Body != "" {
		if len(args.Body) > 100_000 {
			return Result{}, fmt.Errorf("http: body too large (max 100KB)")
		}
		reqBody = bytes.NewReader([]byte(args.Body))
	}

	req, err := http.NewRequestWithContext(ctx, args.Method, args.URL, reqBody)
	if err != nil {
		return Result{}, fmt.Errorf("http: create request: %w", err)
	}

	for k, v := range args.Headers {
		req.Header.Set(k, v)
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "agent-go/1.0")
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("http: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // max 1MB
	if err != nil {
		return Result{}, fmt.Errorf("http: read response: %w", err)
	}

	bodyStr := string(respBody)
	if len(bodyStr) > 10_000 {
		bodyStr = bodyStr[:10_000] + "\n... [truncated]"
	}

	// Collect response headers
	respHeaders := make(map[string]string)
	for k, v := range resp.Header {
		respHeaders[k] = strings.Join(v, ", ")
	}

	out, _ := json.Marshal(map[string]any{
		"url":        args.URL,
		"method":     args.Method,
		"status":     resp.StatusCode,
		"statusText": resp.Status,
		"headers":    respHeaders,
		"body":       bodyStr,
	})
	return Result{Content: string(out)}, nil
}
