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
// WebSearchTool — Brave Search (primary) or DuckDuckGo HTML (fallback)
// ---------------------------------------------------------------------------

type webSearchTool struct {
	httpClient *http.Client
}

func NewWebSearchTool(client *http.Client) Tool {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &webSearchTool{httpClient: client}
}

func (t *webSearchTool) Name() string { return "web.search" }

func (t *webSearchTool) Description() string {
	return "Tìm kiếm trực tiếp qua Google Web Search. Trả về danh sách kết quả mới nhất từ Google: tiêu đề (title), trích dẫn (snippet), và đường dẫn trực tiếp (URL). Dùng để tra cứu thông tin, báo cáo, tin tức thực tế."
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

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// 1. Primary: Google Web Search
	results := searchGoogleWeb(ctx, t.httpClient, args.Query)

	// 2. Secondary fallback: DuckDuckGo Lite HTML Search
	if len(results) == 0 {
		results = fetchDDGLite(ctx, t.httpClient, args.Query)
	}

	// 3. Tertiary fallback: DuckDuckGo JSON API
	if len(results) == 0 {
		results = searchDDGJSON(ctx, t.httpClient, args.Query)
	}

	if len(results) == 0 {
		results = append(results, map[string]string{
			"title":   "⚠️ Google Search unavailable",
			"snippet": fmt.Sprintf("Google Web Search and DuckDuckGo returned no results for '%s'. Try a simpler query or different keywords.", args.Query),
			"url":     "",
		})
	}

	out, _ := json.Marshal(map[string]any{
		"query":   args.Query,
		"count":   len(results),
		"results": results,
	})
	return Result{Content: string(out)}, nil
}

// fetchDDGLite fetches and parses DuckDuckGo Lite HTML results (fallback).
func fetchDDGLite(ctx context.Context, client *http.Client, query string) []map[string]string {
	reqURL := fmt.Sprintf("https://lite.duckduckgo.com/lite/?q=%s", url.QueryEscape(query))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", randomUA())

	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil
	}
	return parseDDGLite(string(body))
}

// parseDDGLite parses DuckDuckGo Lite HTML results.
func parseDDGLite(html string) []map[string]string {
	// DDG Lite returns results in this format:
	// <a rel="nofollow" href="URL">Title</a><span>Snippet</span>
	linkRe := regexp.MustCompile(`<a[^>]*href="([^"]*)"[^>]*>([^<]*)</a>`)
	snippetRe := regexp.MustCompile(`<span class="snippet">([^<]*)</span>`)

	// Simpler approach: find all result rows
	// Each row: <tr class="result-snippet"> or just <tr> with link + snippet
	rows := strings.Split(html, "<tr")
	results := make([]map[string]string, 0)

	for _, row := range rows {
		if !strings.Contains(row, "href=") {
			continue
		}

		// Extract link and title
		linkMatches := linkRe.FindStringSubmatch(row)
		if len(linkMatches) < 3 {
			continue
		}
		href := cleanURL(linkMatches[1])
		title := cleanHTML(linkMatches[2])

		// Skip DuckDuckGo internal links & Wikipedia
		if strings.Contains(href, "duckduckgo.com") || strings.Contains(href, "wikipedia.org") || title == "" {
			continue
		}

		// Extract snippet
		snippet := ""
		snipMatches := snippetRe.FindStringSubmatch(row)
		if len(snipMatches) >= 2 {
			snippet = cleanHTML(snipMatches[1])
		}
		if snippet == "" {
			// Fallback: extract any text between tags
			snippet = extractTextContent(row)
		}

		results = append(results, map[string]string{
			"title":   title,
			"snippet": truncateStr(snippet, 300),
			"url":     href,
		})

		if len(results) >= 10 {
			break
		}
	}

	return results
}

// DDGResponse là cấu trúc trả về từ DuckDuckGo API (exported for tests).
type DDGResponse struct {
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

// searchDDGJSON fallback: original DuckDuckGo JSON API for instant answers.
func searchDDGJSON(ctx context.Context, client *http.Client, query string) []map[string]string {
	reqURL := fmt.Sprintf("https://api.duckduckgo.com/?q=%s&format=json&no_html=1", url.QueryEscape(query))
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	req.Header.Set("User-Agent", randomUA())

	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	var ddg struct {
		AbstractText  string `json:"AbstractText"`
		AbstractURL   string `json:"AbstractURL"`
		Heading       string `json:"Heading"`
		Answer        string `json:"Answer"`
		Definition    string `json:"Definition"`
		RelatedTopics []struct {
			Text     string `json:"Text"`
			FirstURL string `json:"FirstURL"`
		} `json:"RelatedTopics"`
	}
	json.Unmarshal(body, &ddg)

	results := make([]map[string]string, 0)
	if ddg.AbstractText != "" {
		results = append(results, map[string]string{
			"title":   ddg.Heading,
			"snippet": ddg.AbstractText,
			"url":     ddg.AbstractURL,
		})
	}
	if ddg.Answer != "" {
		results = append(results, map[string]string{
			"title":   query,
			"snippet": ddg.Answer,
			"url":     "",
		})
	}
	for _, t := range ddg.RelatedTopics {
		results = append(results, map[string]string{
			"title":   cleanHTML(t.Text),
			"snippet": "",
			"url":     t.FirstURL,
		})
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
	u = strings.TrimPrefix(u, "/l/?kh=-1&uddg=")
	u = strings.TrimPrefix(u, "//")
	if !strings.HasPrefix(u, "http") && u != "" {
		u = "https://" + u
	}
	return u
}

func extractTextContent(html string) string {
	text := regexp.MustCompile(`<[^>]*>`).ReplaceAllString(html, " ")
	text = strings.Join(strings.Fields(text), " ")
	return truncateStr(text, 300)
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
