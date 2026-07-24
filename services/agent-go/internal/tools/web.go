package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// ---------------------------------------------------------------------------
// WebSearchTool — tìm kiếm web qua DuckDuckGo Instant Answer API
// ---------------------------------------------------------------------------

// webSearchTool gọi DuckDuckGo Instant Answer API (miễn phí, không cần key).
// Kind=KindRead.
type webSearchTool struct {
	httpClient *http.Client
}

const ddgAPI = "https://api.duckduckgo.com/"

// DDGResponse là cấu trúc trả về từ DuckDuckGo API.
type DDGResponse struct {
	AbstractText string `json:"AbstractText"`
	AbstractURL  string `json:"AbstractURL"`
	Heading      string `json:"Heading"`
	Answer       string `json:"Answer"`
	AnswerType   string `json:"AnswerType"`
	Definition   string `json:"Definition"`
	RelatedTopics []struct {
		Text     string `json:"Text"`
		FirstURL string `json:"FirstURL"`
	} `json:"RelatedTopics"`
}

// NewWebSearchTool tạo web search tool với http.Client tuỳ chỉnh.
func NewWebSearchTool(client *http.Client) Tool {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &webSearchTool{httpClient: client}
}

func (t *webSearchTool) Name() string { return "web.search" }

func (t *webSearchTool) Description() string {
	return "Tìm kiếm web qua DuckDuckGo Instant Answer API. Trả về title + abstract + URL. Miễn phí, không cần API key."
}

func (t *webSearchTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"query":{"type":"string","description":"search query string"}
		},
		"required":["query"],
		"additionalProperties":false
	}`)
}

func (t *webSearchTool) Kind() Kind { return KindRead }

func (t *webSearchTool) Execute(ctx context.Context, rawArgs json.RawMessage) (Result, error) {
	var args struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return Result{}, fmt.Errorf("web.search: invalid args: %w", err)
	}
	if strings.TrimSpace(args.Query) == "" {
		return Result{}, fmt.Errorf("web.search: query is required")
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	reqURL := fmt.Sprintf("%s?q=%s&format=json&no_html=1&skip_disambig=1", ddgAPI, url.QueryEscape(args.Query))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return Result{}, fmt.Errorf("web.search: create request: %w", err)
	}
	req.Header.Set("User-Agent", "agent-go/1.0")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("web.search: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // max 1MB
	if err != nil {
		return Result{}, fmt.Errorf("web.search: read body: %w", err)
	}

	var ddg DDGResponse
	if err := json.Unmarshal(body, &ddg); err != nil {
		return Result{}, fmt.Errorf("web.search: parse response: %w", err)
	}

	// Tổng hợp kết quả
	results := make([]map[string]string, 0)

	// Abstract text
	if ddg.AbstractText != "" {
		results = append(results, map[string]string{
			"title":    ddg.Heading,
			"abstract": ddg.AbstractText,
			"url":      ddg.AbstractURL,
			"source":   "abstract",
		})
	}

	// Answer
	if ddg.Answer != "" {
		results = append(results, map[string]string{
			"title":  ddg.AnswerType,
			"answer": ddg.Answer,
			"source": "instant_answer",
		})
	}

	// Definition
	if ddg.Definition != "" {
		results = append(results, map[string]string{
			"title":      args.Query,
			"definition": ddg.Definition,
			"source":     "definition",
		})
	}

	// Related topics
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

// ---------------------------------------------------------------------------
// WebFetchTool — tải nội dung trang web
// ---------------------------------------------------------------------------

// webFetchTool tải và trả về text của một URL.
// Kind=KindRead. Giới hạn 10000 ký tự.
type webFetchTool struct {
	httpClient *http.Client
}

const webFetchMaxChars = 10_000

// NewWebFetchTool tạo web fetch tool với http.Client tuỳ chỉnh.
func NewWebFetchTool(client *http.Client) Tool {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &webFetchTool{httpClient: client}
}

func (t *webFetchTool) Name() string { return "web.fetch" }

func (t *webFetchTool) Description() string {
	return "Tải nội dung text của một URL (giới hạn 10000 ký tự)."
}

func (t *webFetchTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"url":{"type":"string","format":"uri","description":"URL to fetch"}
		},
		"required":["url"],
		"additionalProperties":false
	}`)
}

func (t *webFetchTool) Kind() Kind { return KindRead }

func (t *webFetchTool) Execute(ctx context.Context, rawArgs json.RawMessage) (Result, error) {
	var args struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return Result{}, fmt.Errorf("web.fetch: invalid args: %w", err)
	}
	if args.URL == "" {
		return Result{}, fmt.Errorf("web.fetch: url is required")
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, args.URL, nil)
	if err != nil {
		return Result{}, fmt.Errorf("web.fetch: create request: %w", err)
	}
	req.Header.Set("User-Agent", "agent-go/1.0")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("web.fetch: request failed: %w", err)
	}
	defer resp.Body.Close()

	// Chỉ đọc tối đa 1MB raw, sau đó parse và cắt
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Result{}, fmt.Errorf("web.fetch: read body: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	text := extractText(body, contentType)

	if len(text) > webFetchMaxChars {
		text = text[:webFetchMaxChars] + "\n... [truncated]"
	}

	out, _ := json.Marshal(map[string]any{
		"url":         args.URL,
		"status":      resp.StatusCode,
		"contentType": contentType,
		"content":     text,
	})
	return Result{Content: string(out)}, nil
}

// extractText trích xuất text từ HTML hoặc trả raw nếu là plain text.
func extractText(body []byte, contentType string) string {
	if strings.Contains(contentType, "text/html") || strings.HasPrefix(string(body), "<!") || strings.HasPrefix(strings.TrimSpace(string(body)), "<") {
		return htmlToText(body)
	}
	return string(body)
}

// htmlToText parse HTML và trích xuất text từ các node text (đơn giản, không dùng thư viện ngoài).
func htmlToText(body []byte) string {
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return string(body) // fallback: trả raw
	}

	var sb strings.Builder
	var extract func(n *html.Node)
	extract = func(n *html.Node) {
		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if text != "" {
				sb.WriteString(text)
				sb.WriteString(" ")
			}
		}
		// Bỏ qua script và style
		if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style" || n.Data == "noscript") {
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}
	}
	extract(doc)

	result := strings.TrimSpace(sb.String())
	// Gom nhiều whitespace
	result = strings.Join(strings.Fields(result), " ")
	return result
}
