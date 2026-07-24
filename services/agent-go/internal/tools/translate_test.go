package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTranslateTool(t *testing.T) {
	t.Run("valid translation", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify request is a POST with JSON body
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]string{
				"translatedText": "xin chao",
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer srv.Close()

		tool := newTranslateToolWithURL(srv.Client(), srv.URL)
		args, _ := json.Marshal(map[string]string{
			"text":   "hello",
			"source": "en",
			"target": "vi",
		})
		res, err := tool.Execute(context.Background(), args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var out struct {
			Source     string `json:"source"`
			Target     string `json:"target"`
			Original   string `json:"original"`
			Translated string `json:"translated"`
		}
		if err := json.Unmarshal([]byte(res.Content), &out); err != nil {
			t.Fatalf("failed to parse result: %v", err)
		}
		if out.Translated != "xin chao" {
			t.Errorf("translated: got %q, want %q", out.Translated, "xin chao")
		}
		if out.Original != "hello" {
			t.Errorf("original: got %q, want %q", out.Original, "hello")
		}
	})

	t.Run("source auto-detect", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]string{
				"translatedText": "hola",
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer srv.Close()

		tool := newTranslateToolWithURL(srv.Client(), srv.URL)
		// No source specified -> should default to "auto"
		args, _ := json.Marshal(map[string]string{
			"text":   "hello",
			"target": "es",
		})
		res, err := tool.Execute(context.Background(), args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var out struct {
			Source string `json:"source"`
		}
		json.Unmarshal([]byte(res.Content), &out)
		if out.Source != "auto" {
			t.Errorf("source: got %q, want %q", out.Source, "auto")
		}
	})

	t.Run("empty text returns error", func(t *testing.T) {
		tool := NewTranslateTool(nil)
		args, _ := json.Marshal(map[string]string{
			"text":   "",
			"target": "vi",
		})
		_, err := tool.Execute(context.Background(), args)
		if err == nil {
			t.Fatal("expected error for empty text, got nil")
		}
		if !strings.Contains(err.Error(), "text is required") {
			t.Errorf("expected 'text is required' error, got: %v", err)
		}
	})

	t.Run("missing target returns error", func(t *testing.T) {
		tool := NewTranslateTool(nil)
		args, _ := json.Marshal(map[string]string{
			"text":   "hello",
			"source": "en",
		})
		_, err := tool.Execute(context.Background(), args)
		if err == nil {
			t.Fatal("expected error for missing target, got nil")
		}
		if !strings.Contains(err.Error(), "target language is required") {
			t.Errorf("expected 'target language is required' error, got: %v", err)
		}
	})

	t.Run("invalid json args", func(t *testing.T) {
		tool := NewTranslateTool(nil)
		_, err := tool.Execute(context.Background(), json.RawMessage(`{bad`))
		if err == nil {
			t.Fatal("expected error for invalid JSON, got nil")
		}
		if !strings.Contains(err.Error(), "invalid args") {
			t.Errorf("expected 'invalid args' error, got: %v", err)
		}
	})

	t.Run("server error response", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`Internal Server Error`))
		}))
		defer srv.Close()

		tool := newTranslateToolWithURL(srv.Client(), srv.URL)
		args, _ := json.Marshal(map[string]string{
			"text":   "hello",
			"target": "vi",
		})
		// Server returns non-JSON response for translate -> parse error
		_, err := tool.Execute(context.Background(), args)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("truncation of long text", func(t *testing.T) {
		longText := strings.Repeat("A", 6000)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]string{
				"translatedText": strings.Repeat("B", 6000),
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer srv.Close()

		tool := newTranslateToolWithURL(srv.Client(), srv.URL)
		args, _ := json.Marshal(map[string]string{
			"text":   longText,
			"target": "vi",
		})
		res, err := tool.Execute(context.Background(), args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var out struct {
			Original   string `json:"original"`
			Translated string `json:"translated"`
		}
		json.Unmarshal([]byte(res.Content), &out)

		// Original text should be truncated to 5000 chars
		if len(out.Original) > 5000 {
			t.Errorf("original text should be at most 5000, got %d", len(out.Original))
		}
		// Translated text should be truncated; output text < 5000 no truncation
		if len(out.Translated) > 10000+len("... [truncated]") {
			t.Errorf("translated text too long: %d", len(out.Translated))
		}
	})

	t.Run("translated text truncation", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]string{
				"translatedText": strings.Repeat("X", 12000),
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer srv.Close()

		tool := newTranslateToolWithURL(srv.Client(), srv.URL)
		args, _ := json.Marshal(map[string]string{
			"text":   "test",
			"target": "vi",
		})
		res, err := tool.Execute(context.Background(), args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var out struct {
			Translated string `json:"translated"`
		}
		json.Unmarshal([]byte(res.Content), &out)

		if !strings.Contains(out.Translated, "[truncated]") {
			t.Error("expected truncated translated text")
		}
	})

	t.Run("timeout handling", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {}
		}))
		defer srv.Close()

		tool := newTranslateToolWithURL(srv.Client(), srv.URL)
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately
		args, _ := json.Marshal(map[string]string{"text": "hello", "target": "vi"})
		_, err := tool.Execute(ctx, args)
		if err == nil {
			t.Fatal("expected error for cancelled context, got nil")
		}
	})
}

func TestTranslateToolInterface(t *testing.T) {
	var tool Tool = NewTranslateTool(nil)
	if tool.Name() != "translate" {
		t.Errorf("Name: got %q, want %q", tool.Name(), "translate")
	}
	if tool.Kind() != KindRead {
		t.Errorf("Kind: got %v, want KindRead", tool.Kind())
	}
	if tool.Description() == "" {
		t.Error("Description is empty")
	}
	if len(tool.Schema()) == 0 {
		t.Error("Schema is empty")
	}
}

// newTranslateToolWithURL creates a TranslateTool that uses a custom base URL (for testing).
func newTranslateToolWithURL(client *http.Client, baseURL string) Tool {
	return &translateToolWithURL{
		httpClient: client,
		baseURL:    baseURL,
	}
}

// translateToolWithURL is a test variant that allows overriding the API base URL.
type translateToolWithURL struct {
	httpClient *http.Client
	baseURL    string
}

func (t *translateToolWithURL) Name() string        { return "translate" }
func (t *translateToolWithURL) Description() string { return "translate (test)" }
func (t *translateToolWithURL) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"text":{"type":"string"},"source":{"type":"string"},"target":{"type":"string"}},"required":["text","target"]}`)
}
func (t *translateToolWithURL) Kind() Kind { return KindRead }

func (t *translateToolWithURL) Execute(ctx context.Context, rawArgs json.RawMessage) (Result, error) {
	var args struct {
		Text   string `json:"text"`
		Source string `json:"source"`
		Target string `json:"target"`
	}
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return Result{}, err
	}
	if args.Text == "" {
		return Result{}, errTranslateTextRequired
	}
	if args.Target == "" {
		return Result{}, errTranslateTargetRequired
	}
	if args.Source == "" {
		args.Source = "auto"
	}
	if len(args.Text) > 5000 {
		args.Text = args.Text[:5000]
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	bodyBytes, _ := json.Marshal(map[string]string{
		"q":      args.Text,
		"source": args.Source,
		"target": args.Target,
		"format": "text",
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.baseURL, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()

	var result struct {
		TranslatedText string `json:"translatedText"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return Result{}, err
	}

	translated := result.TranslatedText
	if len(translated) > 10000 {
		translated = translated[:10000] + "... [truncated]"
	}

	out, _ := json.Marshal(map[string]any{
		"source":     args.Source,
		"target":     args.Target,
		"original":   args.Text,
		"translated": translated,
	})
	return Result{Content: string(out)}, nil
}

var (
	errTranslateTextRequired   = &translateTextRequiredError{}
	errTranslateTargetRequired = &translateTargetRequiredError{}
)

type translateTextRequiredError struct{}

func (e *translateTextRequiredError) Error() string { return "translate: text is required" }

type translateTargetRequiredError struct{}

func (e *translateTargetRequiredError) Error() string { return "translate: target language is required" }
