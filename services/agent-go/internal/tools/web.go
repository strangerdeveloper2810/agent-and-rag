package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

var userAgents = []string{
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:133.0) Gecko/20100101 Firefox/133.0",
}

func randomUA() string {
	return userAgents[rand.Intn(len(userAgents))]
}

// ---------------------------------------------------------------------------
// WebSearchTool — Google scrape (không dùng DuckDuckGo)
// ---------------------------------------------------------------------------

const (
	// webSearchTimeout là ngân sách thời gian TỔNG cho một lần tìm kiếm.
	webSearchTimeout = 10 * time.Second
	// webSearchCacheTTL là thời gian sống của kết quả cache. Research thường
	// lặp lại query giữa các vòng — cache tránh gọi lại mạng.
	webSearchCacheTTL = 5 * time.Minute
	// webSearchCacheSize là số query tối đa được cache; khi đầy thì xoá sạch.
	webSearchCacheSize = 256
)

// webSearchProvider là một backend tìm kiếm.
// Trả về nil/empty khi không có kết quả hoặc lỗi — race sẽ thử backend khác.
type webSearchProvider func(context.Context, *http.Client, string) []map[string]string

// webSearchProviders là danh sách backend được chạy song song, lấy kết quả
// của backend đầu tiên trả về non-empty. Hiện chỉ có Google; giữ cấu trúc
// race để dễ thêm backend khác và để test inject được fake provider.
var webSearchProviders = []webSearchProvider{searchGoogleWeb}

type webSearchCacheEntry struct {
	expires time.Time
	results []map[string]string
}

type webSearchTool struct {
	httpClient *http.Client

	cacheMu sync.Mutex
	cache   map[string]webSearchCacheEntry // lazy init, key = query đã normalize
}

func NewWebSearchTool(client *http.Client) Tool {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &webSearchTool{httpClient: client}
}

func (t *webSearchTool) Name() string { return "web.search" }

func (t *webSearchTool) Description() string {
	return "Tìm kiếm web qua Google (cache 5 phút, bỏ qua Wikipedia). Trả về tiêu đề (title), trích dẫn (snippet), và URL trực tiếp. Dùng để tra cứu thông tin, báo cáo, tin tức thực tế."
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

	key := strings.ToLower(strings.TrimSpace(args.Query))
	if results, ok := t.cacheGet(key); ok {
		return Result{Content: buildWebSearchOutput(args.Query, results)}, nil
	}

	searchCtx, cancel := context.WithTimeout(ctx, webSearchTimeout)
	defer cancel()
	results := raceWebSearch(searchCtx, cancel, t.httpClient, args.Query)

	if len(results) == 0 {
		results = append(results, map[string]string{
			"title":   "⚠️ Search unavailable",
			"snippet": fmt.Sprintf("Google không trả kết quả cho '%s'. Thử query đơn giản hơn hoặc từ khoá khác.", args.Query),
			"url":     "",
		})
	} else {
		t.cacheSet(key, results)
	}

	return Result{Content: buildWebSearchOutput(args.Query, results)}, nil
}

// raceWebSearch chạy mọi backend SONG SONG, trả về kết quả của backend đầu
// tiên có kết quả (sau khi lọc Wikipedia) và cancel các backend còn lại
// (qua cancel của searchCtx).
func raceWebSearch(ctx context.Context, cancel context.CancelFunc, client *http.Client, query string) []map[string]string {
	out := make(chan []map[string]string, len(webSearchProviders))
	for _, provider := range webSearchProviders {
		provider := provider
		go func() {
			out <- filterWikipedia(provider(ctx, client, query))
		}()
	}

	first := []map[string]string(nil)
	remaining := len(webSearchProviders)
	for remaining > 0 {
		results := <-out
		remaining--
		if first == nil && len(results) > 0 {
			first = results
			cancel()
		}
	}
	return first
}

// filterWikipedia loại kết quả từ wikipedia.org — áp cho MỌI backend trước
// khi trả về.
func filterWikipedia(results []map[string]string) []map[string]string {
	out := make([]map[string]string, 0, len(results))
	for _, r := range results {
		if strings.Contains(r["url"], "wikipedia.org") {
			continue
		}
		out = append(out, r)
	}
	return out
}

// cacheGet trả về kết quả cache còn hạn cho query key; xoá entry đã hết hạn.
func (t *webSearchTool) cacheGet(key string) ([]map[string]string, bool) {
	t.cacheMu.Lock()
	defer t.cacheMu.Unlock()
	if t.cache == nil {
		return nil, false
	}
	entry, ok := t.cache[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expires) {
		delete(t.cache, key)
		return nil, false
	}
	return entry.results, true
}

// cacheSet lưu kết quả vào cache với TTL; khi đầy thì xoá sạch (đủ tốt cho
// 256 entries — không cần LRU).
func (t *webSearchTool) cacheSet(key string, results []map[string]string) {
	t.cacheMu.Lock()
	defer t.cacheMu.Unlock()
	if t.cache == nil {
		t.cache = make(map[string]webSearchCacheEntry)
	}
	if len(t.cache) >= webSearchCacheSize {
		for k := range t.cache {
			delete(t.cache, k)
		}
	}
	t.cache[key] = webSearchCacheEntry{expires: time.Now().Add(webSearchCacheTTL), results: results}
}

// buildWebSearchOutput encode kết quả search thành JSON trả về cho LLM.
func buildWebSearchOutput(query string, results []map[string]string) string {
	out, _ := json.Marshal(map[string]any{
		"query":   query,
		"count":   len(results),
		"results": results,
	})
	return string(out)
}

// searchGoogleWeb tries Google search via HTML scraping.
func searchGoogleWeb(ctx context.Context, client *http.Client, query string) []map[string]string {
	reqURL := fmt.Sprintf("https://www.google.com/search?q=%s&hl=vi", url.QueryEscape(query))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", randomUA())
	req.Header.Set("Accept-Language", "vi-VN,vi;q=0.9,en-US;q=0.8,en;q=0.7")

	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return parseGoogleResults(string(body))
}

// parseGoogleResults extracts search results from Google HTML (best-effort).
func parseGoogleResults(html string) []map[string]string {
	// Extract Google HTML result links and titles
	linkTitleRe := regexp.MustCompile(`<a[^>]*href="(/url\?q=)?(https?://[^"&]+)"[^>]*>[\s\S]*?<h3[^>]*>([\s\S]*?)</h3>`)
	snippetRe := regexp.MustCompile(`<div[^>]*class="[^"]*(?:VwiC3b|yXMpt|s3tnU|BNeawe|r05eec)[^"]*"[^>]*>([\s\S]*?)</div>`)

	matches := linkTitleRe.FindAllStringSubmatch(html, 12)
	snippets := snippetRe.FindAllStringSubmatch(html, 12)

	results := make([]map[string]string, 0)
	for i, m := range matches {
		if len(m) < 4 {
			continue
		}
		rawURL := cleanURL(m[2])
		rawTitle := cleanHTML(m[3])

		// Skip internal google links and wikipedia
		if strings.Contains(rawURL, "google.com") || strings.Contains(rawURL, "wikipedia.org") || rawTitle == "" {
			continue
		}

		snip := ""
		if i < len(snippets) && len(snippets[i]) >= 2 {
			snip = cleanHTML(snippets[i][1])
		}

		results = append(results, map[string]string{
			"title":   rawTitle,
			"snippet": truncateStr(snip, 300),
			"url":     rawURL,
		})

		if len(results) >= 8 {
			break
		}
	}

	return results
}

func cleanHTML(s string) string {
	s = regexp.MustCompile(`<[^>]*>`).ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&#39;", "'")
	return strings.TrimSpace(s)
}

func cleanURL(u string) string {
	u = strings.TrimPrefix(u, "//")
	if !strings.HasPrefix(u, "http") && u != "" {
		u = "https://" + u
	}
	return u
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ---------------------------------------------------------------------------
// WebFetchTool — tải nội dung trang web
// ---------------------------------------------------------------------------

type webFetchTool struct {
	httpClient *http.Client
}

const webFetchMaxChars = 15_000

func NewWebFetchTool(client *http.Client) Tool {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &webFetchTool{httpClient: client}
}

func (t *webFetchTool) Name() string { return "web.fetch" }

func (t *webFetchTool) Description() string {
	return "Tải nội dung text của một URL (giới hạn 15000 ký tự). Dùng để đọc nội dung trang web sau khi search."
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

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, args.URL, nil)
	if err != nil {
		return Result{}, fmt.Errorf("web.fetch: create request: %w", err)
	}
	req.Header.Set("User-Agent", randomUA())
	req.Header.Set("Accept", "text/html,text/plain,*/*")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("web.fetch: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20)) // 2MB max
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
		"length":      len(text),
		"content":     text,
	})
	return Result{Content: string(out)}, nil
}

func extractText(body []byte, contentType string) string {
	bodyStr := string(body)
	isHTML := strings.Contains(contentType, "text/html") ||
		strings.HasPrefix(strings.TrimSpace(bodyStr), "<!") ||
		strings.HasPrefix(strings.TrimSpace(bodyStr), "<")
	if isHTML {
		return htmlToText(body)
	}
	return bodyStr
}

func htmlToText(body []byte) string {
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return string(body)
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
		if n.Type == html.ElementNode {
			switch n.Data {
			case "script", "style", "noscript", "nav", "footer", "header":
				return
			case "br", "p", "div", "li", "h1", "h2", "h3", "h4", "h5", "h6":
				sb.WriteString("\n")
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}
	}
	extract(doc)

	result := strings.TrimSpace(sb.String())
	result = strings.Join(strings.Fields(result), " ")
	return result
}
