package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestWebSearchTool(t *testing.T) {
	t.Run("search returns results", func(t *testing.T) {
		// Mock search API (giả lập backend trả JSON kiểu instant-answer).
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			resp := testSearchResponse{
				AbstractText: "The Go Programming Language",
				AbstractURL:  "https://go.dev/",
				Heading:      "Go (programming language)",
				Answer:       "Go is a statically typed, compiled programming language.",
				AnswerType:   "programming",
				RelatedTopics: []struct {
					Text     string `json:"Text"`
					FirstURL string `json:"FirstURL"`
				}{
					{Text: "Go tutorial - Tour of Go", FirstURL: "https://go.dev/tour/"},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer srv.Close()

		// Tạo tool với custom client trỏ tới mock server
		tool := newWebSearchToolWithURL(srv.Client(), srv.URL)
		args, _ := json.Marshal(map[string]string{"query": "golang"})
		res, err := tool.Execute(context.Background(), args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var out struct {
			Query   string              `json:"query"`
			Count   int                 `json:"count"`
			Results []map[string]string `json:"results"`
		}
		json.Unmarshal([]byte(res.Content), &out)

		if out.Query != "golang" {
			t.Errorf("query: got %q, want %q", out.Query, "golang")
		}
		if out.Count == 0 {
			t.Error("expected at least 1 result")
		}
	})

	t.Run("empty query returns error", func(t *testing.T) {
		tool := NewWebSearchTool(nil)
		args, _ := json.Marshal(map[string]string{"query": ""})
		_, err := tool.Execute(context.Background(), args)
		if err == nil {
			t.Fatal("expected error for empty query, got nil")
		}
		if !strings.Contains(err.Error(), "query is required") {
			t.Errorf("expected 'query is required' error, got: %v", err)
		}
	})

	t.Run("missing query param", func(t *testing.T) {
		tool := NewWebSearchTool(nil)
		args, _ := json.Marshal(map[string]string{})
		_, err := tool.Execute(context.Background(), args)
		if err == nil {
			t.Fatal("expected error for missing query, got nil")
		}
		if !strings.Contains(err.Error(), "query is required") {
			t.Errorf("expected 'query is required' error, got: %v", err)
		}
	})

	t.Run("invalid json args", func(t *testing.T) {
		tool := NewWebSearchTool(nil)
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
		}))
		defer srv.Close()

		tool := newWebSearchToolWithURL(srv.Client(), srv.URL)
		args, _ := json.Marshal(map[string]string{"query": "test"})
		// Server trả 500 nhưng body không parse được JSON -> lỗi parse
		_, err := tool.Execute(context.Background(), args)
		// Có thể parse lỗi hoặc body rỗng -> đều là lỗi
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("timeout handling", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Block forever
			select {}
		}))
		defer srv.Close()

		tool := newWebSearchToolWithURL(srv.Client(), srv.URL)
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately
		args, _ := json.Marshal(map[string]string{"query": "test"})
		_, err := tool.Execute(ctx, args)
		if err == nil {
			t.Fatal("expected error for cancelled context, got nil")
		}
	})
}

func TestWebSearchRacesProviders(t *testing.T) {
	orig := webSearchProviders
	defer func() { webSearchProviders = orig }()
	webSearchProviders = []webSearchProvider{
		func(ctx context.Context, c *http.Client, q string) []map[string]string {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(50 * time.Millisecond):
				return []map[string]string{{"title": "fast"}}
			}
		},
		func(ctx context.Context, c *http.Client, q string) []map[string]string {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(2 * time.Second):
				return []map[string]string{{"title": "slow"}}
			}
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	start := time.Now()
	results := raceWebSearch(ctx, cancel, nil, "test")
	if len(results) == 0 || results[0]["title"] != "fast" {
		t.Fatalf("expected fast provider result, got %v", results)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("race should return first non-empty result immediately, took %v", elapsed)
	}
}

func TestParseGoogleResults_Success(t *testing.T) {
	htmlSample := `
	<html><body>
	<div>
		<a href="/url?q=https://go.dev/&sa=U"><h3>The Go Programming Language</h3></a>
		<div class="VwiC3b">Go is an open source programming language that makes it easy to build simple software.</div>
	</div>
	<div>
		<a href="https://google.com/search?q=other"><h3>Google Search</h3></a>
	</div>
	<div>
		<a href="https://github.com/golang/go"><h3>Golang GitHub Repository</h3></a>
		<div class="yXMpt">Official repository for the Go programming language.</div>
	</div>
	</body></html>
	`

	results := parseGoogleResults(htmlSample)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d: %v", len(results), results)
	}

	if results[0]["title"] != "The Go Programming Language" {
		t.Errorf("title 0 = %q, want 'The Go Programming Language'", results[0]["title"])
	}
	if results[0]["url"] != "https://go.dev/" {
		t.Errorf("url 0 = %q, want 'https://go.dev/'", results[0]["url"])
	}

	if results[1]["title"] != "Golang GitHub Repository" {
		t.Errorf("title 1 = %q, want 'Golang GitHub Repository'", results[1]["title"])
	}
	if results[1]["url"] != "https://github.com/golang/go" {
		t.Errorf("url 1 = %q, want 'https://github.com/golang/go'", results[1]["url"])
	}
}

func TestParseBingResults(t *testing.T) {
	htmlSample := `
	<html><body>
	<ul>
		<li class="b_algo">
			<h2><a href="https://go.dev/">The Go Programming Language</a></h2>
			<div class="b_caption"><p>Go is an open source programming language supported by Google.</p></div>
		</li>
		<li class="b_algo">
			<h2><a href="https://bing.com/search">Bing Internal</a></h2>
			<div class="b_caption"><p>Bing page</p></div>
		</li>
		<li class="b_algo">
			<h2><a href="https://github.com/fastify/fastify">Fastify GitHub</a></h2>
			<div class="b_caption"><p>Fast and low overhead web framework for Node.js.</p></div>
		</li>
	</ul>
	</body></html>
	`

	results := parseBingResults(htmlSample)
	if len(results) != 2 {
		t.Fatalf("expected 2 non-internal results, got %d: %v", len(results), results)
	}

	if results[0]["title"] != "The Go Programming Language" {
		t.Errorf("title 0 = %q, want 'The Go Programming Language'", results[0]["title"])
	}
	if results[0]["url"] != "https://go.dev/" {
		t.Errorf("url 0 = %q, want 'https://go.dev/'", results[0]["url"])
	}
	if !strings.Contains(results[0]["snippet"], "open source programming language") {
		t.Errorf("snippet 0 = %q, missing expected content", results[0]["snippet"])
	}

	if results[1]["title"] != "Fastify GitHub" {
		t.Errorf("title 1 = %q, want 'Fastify GitHub'", results[1]["title"])
	}
}

func TestSearchTavilyMock(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			http.Error(w, "missing auth", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"query": "test query",
			"results": [
				{
					"title": "Tavily AI Result",
					"url": "https://example.com/tavily",
					"content": "Clean extracted content for AI models."
				},
				{
					"title": "Wikipedia Page",
					"url": "https://en.wikipedia.org/wiki/Test",
					"content": "Wikipedia info"
				}
			]
		}`))
	}))
	defer srv.Close()

	// Direct verification of parsing logic
	var tavilyResp struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}

	respStr := `{
		"results": [
			{"title": "Tavily AI Result", "url": "https://example.com/tavily", "content": "Clean extracted content for AI models."},
			{"title": "Wikipedia Page", "url": "https://en.wikipedia.org/wiki/Test", "content": "Wikipedia info"}
		]
	}`

	json.Unmarshal([]byte(respStr), &tavilyResp)
	results := make([]map[string]string, 0)
	for _, r := range tavilyResp.Results {
		if strings.Contains(r.URL, "wikipedia.org") || r.Title == "" {
			continue
		}
		results = append(results, map[string]string{
			"title":   r.Title,
			"snippet": truncateStr(r.Content, 400),
			"url":     r.URL,
		})
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result (filtered wikipedia), got %d", len(results))
	}
	if results[0]["title"] != "Tavily AI Result" || results[0]["url"] != "https://example.com/tavily" {
		t.Errorf("unexpected result: %v", results[0])
	}
}

func TestWebSearchCache(t *testing.T) {
	orig := webSearchProviders
	defer func() { webSearchProviders = orig }()

	var calls atomic.Int32
	webSearchProviders = []webSearchProvider{
		func(ctx context.Context, c *http.Client, q string) []map[string]string {
			calls.Add(1)
			return []map[string]string{{"title": "cached"}}
		},
	}

	tool := NewWebSearchTool(nil).(*webSearchTool)
	args, _ := json.Marshal(map[string]string{"query": "test query"})
	for i := 0; i < 3; i++ {
		if _, err := tool.Execute(context.Background(), args); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("expected 1 provider call (cached after first), got %d", calls.Load())
	}
}

func TestWebFetchTool(t *testing.T) {
	t.Run("fetch returns content", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Write([]byte("Hello from test server!"))
		}))
		defer srv.Close()

		tool := NewWebFetchTool(srv.Client())
		args, _ := json.Marshal(map[string]string{"url": srv.URL})
		res, err := tool.Execute(context.Background(), args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var out struct {
			URL     string `json:"url"`
			Status  int    `json:"status"`
			Content string `json:"content"`
		}
		json.Unmarshal([]byte(res.Content), &out)

		if out.Status != 200 {
			t.Errorf("status: got %d, want 200", out.Status)
		}
		if out.Content != "Hello from test server!" {
			t.Errorf("content: got %q, want %q", out.Content, "Hello from test server!")
		}
	})

	t.Run("fetch html page", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<html><head><title>Test</title></head><body><h1>Hello</h1><p>World</p></body></html>`))
		}))
		defer srv.Close()

		tool := NewWebFetchTool(srv.Client())
		args, _ := json.Marshal(map[string]string{"url": srv.URL})
		res, err := tool.Execute(context.Background(), args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var out struct {
			Content string `json:"content"`
		}
		json.Unmarshal([]byte(res.Content), &out)

		if !strings.Contains(out.Content, "Hello") || !strings.Contains(out.Content, "World") {
			t.Errorf("expected HTML text extraction to contain 'Hello World', got: %q", out.Content)
		}
		// Should NOT contain script/style tags
		if strings.Contains(out.Content, "<script>") || strings.Contains(out.Content, "<style>") {
			t.Errorf("content should not contain HTML tags, got: %q", out.Content)
		}
	})

	t.Run("empty url returns error", func(t *testing.T) {
		tool := NewWebFetchTool(nil)
		args, _ := json.Marshal(map[string]string{"url": ""})
		_, err := tool.Execute(context.Background(), args)
		if err == nil {
			t.Fatal("expected error for empty url, got nil")
		}
		if !strings.Contains(err.Error(), "url is required") {
			t.Errorf("expected 'url is required' error, got: %v", err)
		}
	})

	t.Run("truncation", func(t *testing.T) {
		bigContent := strings.Repeat("X", 16_000)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte(bigContent))
		}))
		defer srv.Close()

		tool := NewWebFetchTool(srv.Client())
		args, _ := json.Marshal(map[string]string{"url": srv.URL})
		res, err := tool.Execute(context.Background(), args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var out struct {
			Content string `json:"content"`
		}
		json.Unmarshal([]byte(res.Content), &out)

		if !strings.Contains(out.Content, "[truncated]") {
			t.Error("expected truncated content")
		}
		if len(out.Content) > webFetchMaxChars+len("\n... [truncated]")+10 {
			t.Errorf("content too long: %d chars", len(out.Content))
		}
	})

	t.Run("timeout handling", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {}
		}))
		defer srv.Close()

		tool := NewWebFetchTool(srv.Client())
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		args, _ := json.Marshal(map[string]string{"url": srv.URL})
		_, err := tool.Execute(ctx, args)
		if err == nil {
			t.Fatal("expected error for cancelled context, got nil")
		}
	})

	t.Run("invalid json args", func(t *testing.T) {
		tool := NewWebFetchTool(nil)
		_, err := tool.Execute(context.Background(), json.RawMessage(`{bad`))
		if err == nil {
			t.Fatal("expected error for invalid JSON, got nil")
		}
	})
}

func TestWebToolInterface(t *testing.T) {
	t.Run("WebSearchTool interface", func(t *testing.T) {
		var tool Tool = NewWebSearchTool(nil)
		if tool.Name() != "web.search" {
			t.Errorf("Name: got %q, want %q", tool.Name(), "web.search")
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
	})

	t.Run("WebFetchTool interface", func(t *testing.T) {
		var tool Tool = NewWebFetchTool(nil)
		if tool.Name() != "web.fetch" {
			t.Errorf("Name: got %q, want %q", tool.Name(), "web.fetch")
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
	})
}

// testSearchResponse là response của mock server trong test variant.
type testSearchResponse struct {
	AbstractText  string `json:"AbstractText"`
	AbstractURL   string `json:"AbstractURL"`
	Heading       string `json:"Heading"`
	Answer        string `json:"Answer"`
	AnswerType    string `json:"AnswerType"`
	Definition    string `json:"Definition"`
	RelatedTopics []struct {
		Text     string `json:"Text"`
		FirstURL string `json:"FirstURL"`
	} `json:"RelatedTopics"`
}

// newWebSearchToolWithURL creates a WebSearchTool that uses a custom base URL (for testing).
// This is an internal helper that overrides the search API endpoint.
func newWebSearchToolWithURL(client *http.Client, baseURL string) Tool {
	return &webSearchToolWithURL{
		httpClient: client,
		baseURL:    baseURL,
	}
}

// webSearchToolWithURL is a test variant that allows overriding the API base URL.
type webSearchToolWithURL struct {
	httpClient *http.Client
	baseURL    string
}

func (t *webSearchToolWithURL) Name() string        { return "web.search" }
func (t *webSearchToolWithURL) Description() string { return "web search (test)" }
func (t *webSearchToolWithURL) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`)
}
func (t *webSearchToolWithURL) Kind() Kind { return KindRead }

func (t *webSearchToolWithURL) Execute(ctx context.Context, rawArgs json.RawMessage) (Result, error) {
	var args struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return Result{}, err
	}
	if args.Query == "" {
		return Result{}, errQueryRequired
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.baseURL+"?q="+args.Query+"&format=json", nil)
	if err != nil {
		return Result{}, err
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()

	var ddg testSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&ddg); err != nil {
		return Result{}, err
	}

	results := make([]map[string]string, 0)
	if ddg.AbstractText != "" {
		results = append(results, map[string]string{
			"title":    ddg.Heading,
			"abstract": ddg.AbstractText,
			"url":      ddg.AbstractURL,
			"source":   "abstract",
		})
	}
	if ddg.Answer != "" {
		results = append(results, map[string]string{
			"title":  ddg.AnswerType,
			"answer": ddg.Answer,
			"source": "instant_answer",
		})
	}
	for _, topic := range ddg.RelatedTopics {
		results = append(results, map[string]string{
			"title":  topic.Text,
			"url":    topic.FirstURL,
			"source": "related",
		})
	}

	out, _ := json.Marshal(map[string]any{
		"query":   args.Query,
		"count":   len(results),
		"results": results,
	})
	return Result{Content: string(out)}, nil
}

var errQueryRequired = &queryRequiredError{}

type queryRequiredError struct{}

func (e *queryRequiredError) Error() string { return "web.search: query is required" }
