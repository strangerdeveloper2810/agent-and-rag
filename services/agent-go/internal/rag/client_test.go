package rag

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// redirectTransport ép mọi request (kể cả URL Voyage hard-code) về httptest server.
type redirectTransport struct {
	base string
	seen []EmbedRequest
}

func (rt *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var body EmbedRequest
	if req.Body != nil {
		_ = json.NewDecoder(req.Body).Decode(&body)
		rt.seen = append(rt.seen, body)
	}

	// Giữ nguyên ctx của request gốc để test huỷ ctx vẫn có tác dụng.
	target, err := http.NewRequestWithContext(req.Context(), req.Method, rt.base,
		strings.NewReader(mustJSON(body)))
	if err != nil {
		return nil, err
	}
	target.Header = req.Header.Clone()
	return http.DefaultTransport.RoundTrip(target)
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *redirectTransport) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	rt := &redirectTransport{base: srv.URL}
	c := NewClient("test-key")
	c.httpClient = &http.Client{Transport: rt}
	return c, rt
}

func TestNewClient(t *testing.T) {
	c := NewClient("k")
	if c.apiKey != "k" {
		t.Errorf("apiKey = %q, want k", c.apiKey)
	}
	if c.httpClient == nil || c.httpClient.Timeout == 0 {
		t.Error("httpClient phải có timeout")
	}
}

func TestEmbed_SingleBatch(t *testing.T) {
	c, rt := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization = %q", got)
		}
		_, _ = w.Write([]byte(`{"data":[{"embedding":[0.1,0.2]},{"embedding":[0.3,0.4]}]}`))
	})

	vecs, err := c.Embed(context.Background(), []string{"a", "b"}, "document")
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(vecs) != 2 || vecs[1][1] != 0.4 {
		t.Errorf("vecs = %v", vecs)
	}
	if len(rt.seen) != 1 || rt.seen[0].InputType != "document" {
		t.Errorf("request gửi lên = %+v", rt.seen)
	}
}

// Nhiều hơn batchSize text → chia nhiều request, kết quả gộp theo đúng thứ tự.
func TestEmbed_SplitsIntoBatches(t *testing.T) {
	calls := 0
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		var body EmbedRequest
		_ = json.NewDecoder(r.Body).Decode(&body)

		resp := embedResponse{}
		for range body.Input {
			resp.Data = append(resp.Data, struct {
				Embedding []float64 `json:"embedding"`
			}{Embedding: []float64{float64(calls)}})
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	texts := make([]string, batchSize+10)
	for i := range texts {
		texts[i] = "t"
	}

	vecs, err := c.Embed(context.Background(), texts, "document")
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if calls != 2 {
		t.Errorf("gọi API %d lần, want 2", calls)
	}
	if len(vecs) != len(texts) {
		t.Errorf("vecs len = %d, want %d", len(vecs), len(texts))
	}
	// Batch 1 trả 1.0, batch 2 trả 2.0 → thứ tự gộp phải giữ nguyên.
	if vecs[0][0] != 1 || vecs[len(vecs)-1][0] != 2 {
		t.Errorf("thứ tự gộp sai: đầu %v cuối %v", vecs[0], vecs[len(vecs)-1])
	}
}

func TestEmbed_Empty(t *testing.T) {
	c, rt := newTestClient(t, func(http.ResponseWriter, *http.Request) {
		t.Error("không được gọi API khi không có text")
	})

	vecs, err := c.Embed(context.Background(), nil, "document")
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(vecs) != 0 || len(rt.seen) != 0 {
		t.Errorf("vecs = %v, seen = %v", vecs, rt.seen)
	}
}

func TestEmbed_HTTPError(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"detail":"rate limited"}`))
	})

	_, err := c.Embed(context.Background(), []string{"a"}, "query")
	if err == nil {
		t.Fatal("Embed = nil error, want lỗi 429")
	}
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("err = %q, want chứa 429", err)
	}
}

func TestEmbed_BadJSON(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("không-phải-json"))
	})

	if _, err := c.Embed(context.Background(), []string{"a"}, "query"); err == nil {
		t.Fatal("Embed = nil error, want lỗi decode")
	}
}

func TestEmbed_TransportError(t *testing.T) {
	c := NewClient("k")
	c.httpClient = &http.Client{Transport: &redirectTransport{base: "http://127.0.0.1:1"}}

	if _, err := c.Embed(context.Background(), []string{"a"}, "query"); err == nil {
		t.Fatal("Embed = nil error, want lỗi kết nối")
	}
}

func TestEmbed_ContextCancelled(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := c.Embed(ctx, []string{"a"}, "query"); err == nil {
		t.Fatal("Embed với ctx đã huỷ = nil error, want lỗi")
	}
}
