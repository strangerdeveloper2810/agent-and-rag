package tools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPTool(t *testing.T) {
	// Create a test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("X-Custom", "test")
		resp := map[string]any{
			"method": r.Method,
			"path":   r.URL.Path,
			"query":  r.URL.RawQuery,
			"body":   string(body),
			"auth":   r.Header.Get("Authorization"),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	tool := NewHTTPTool(nil)

	tests := []struct {
		name    string
		args    string
		wantErr bool
		check   func(t *testing.T, result Result)
	}{
		{
			name:    "GET request",
			args:    `{"method":"GET","url":"` + ts.URL + `/test?foo=bar"}`,
			wantErr: false,
			check: func(t *testing.T, result Result) {
				var out struct {
					Status int    `json:"status"`
					Body   string `json:"body"`
				}
				json.Unmarshal([]byte(result.Content), &out)
				if out.Status != 200 {
					t.Errorf("expected status 200, got %d", out.Status)
				}
			},
		},
		{
			name:    "POST with body",
			args:    `{"method":"POST","url":"` + ts.URL + `/api","body":"hello world","headers":{"Authorization":"Bearer xyz"}}`,
			wantErr: false,
			check: func(t *testing.T, result Result) {
				var out struct {
					Status int    `json:"status"`
					Body   string `json:"body"`
				}
				json.Unmarshal([]byte(result.Content), &out)
				if out.Status != 200 {
					t.Errorf("expected status 200, got %d", out.Status)
				}
				// Verify the body contains the sent data
				var resp map[string]any
				json.Unmarshal([]byte(out.Body), &resp)
				if resp["body"] != "hello world" {
					t.Errorf("expected body 'hello world', got %v", resp["body"])
				}
			},
		},
		{
			name:    "PUT request",
			args:    `{"method":"PUT","url":"` + ts.URL + `/item","body":"updated"}`,
			wantErr: false,
		},
		{
			name:    "DELETE request",
			args:    `{"method":"DELETE","url":"` + ts.URL + `/item"}`,
			wantErr: false,
		},
		{
			name:    "PATCH request",
			args:    `{"method":"PATCH","url":"` + ts.URL + `/item","body":"patched"}`,
			wantErr: false,
		},
		{
			name:    "invalid method",
			args:    `{"method":"INVALID","url":"` + ts.URL + `"}`,
			wantErr: true,
		},
		{
			name:    "missing URL",
			args:    `{"method":"GET"}`,
			wantErr: true,
		},
		{
			name:    "missing method",
			args:    `{"url":"` + ts.URL + `"}`,
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			args:    `{bad`,
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
			if err == nil && tt.check != nil {
				tt.check(t, result)
			}
		})
	}
}

func TestHTTPTool_Headers(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]string{
			"content-type": r.Header.Get("Content-Type"),
			"x-custom":     r.Header.Get("X-Custom"),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	tool := NewHTTPTool(nil)
	result, err := tool.Execute(context.Background(), json.RawMessage(
		`{"method":"POST","url":"`+ts.URL+`","headers":{"Content-Type":"application/json","X-Custom":"value123"},"body":"{}"}`,
	))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out struct {
		Body string `json:"body"`
	}
	json.Unmarshal([]byte(result.Content), &out)

	var resp struct {
		ContentType string `json:"content-type"`
		XCustom     string `json:"x-custom"`
	}
	json.Unmarshal([]byte(out.Body), &resp)

	if resp.XCustom != "value123" {
		t.Errorf("expected X-Custom=value123, got %q", resp.XCustom)
	}
	if resp.ContentType != "application/json" {
		t.Errorf("expected Content-Type=application/json, got %q", resp.ContentType)
	}
}

func TestHTTPTool_ResponseHeaders(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Response-ID", "abc-123")
		w.WriteHeader(201)
		w.Write([]byte("created"))
	}))
	defer ts.Close()

	tool := NewHTTPTool(nil)
	result, err := tool.Execute(context.Background(), json.RawMessage(
		`{"method":"POST","url":"`+ts.URL+`","body":"data"}`,
	))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out struct {
		Status  int               `json:"status"`
		Headers map[string]string `json:"headers"`
	}
	json.Unmarshal([]byte(result.Content), &out)

	if out.Status != 201 {
		t.Errorf("expected status 201, got %d", out.Status)
	}
	if out.Headers["X-Response-Id"] != "abc-123" {
		t.Errorf("expected X-Response-ID header, got %v", out.Headers)
	}
}
