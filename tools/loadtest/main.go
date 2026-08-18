// Command loadtest bắn tải vào JARVIS production để xem stack chịu được bao nhiêu
// user prompting đồng thời trước khi "tèo".
//
// Nó mô phỏng đúng luồng của một user thật:
//
//	POST /api/conversations          -> tạo conversation (lấy _id)
//	POST /api/conversations/:id/chat -> SSE stream tới khi có event {"done":true}
//
// Điểm quan trọng: mỗi lỗi được PHÂN LOẠI THEO TẦNG, vì "chịu tải nổi không" là
// câu hỏi vô nghĩa nếu không biết ai gãy trước — nginx, Fastify rate-limit,
// agent-go, MongoDB, hay quota của LLM provider.
//
// Chạy:
//
//	go run ./tools/loadtest -mode=health -stages "50:500,200:1000"
//	go run ./tools/loadtest -mode=chat   -stages "10:20,50:100,200:400,1000:1000" \
//	    -secret "$JWT_SECRET" -users 200
//
// Nhớ nới file descriptor trước khi chạy concurrency cao (macOS default 256):
//
//	ulimit -n 10240
package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ── Cấu hình ────────────────────────────────────────────────────────────────

type options struct {
	base      string
	mode      string
	stages    string
	token     string
	secret    string
	users     int
	promptsIn string
	timeout   time.Duration
	newConv   bool
	canary    bool
	out       string
	rampDelay time.Duration
	pause     time.Duration
	verbose   bool
}

func parseFlags() options {
	var o options
	flag.StringVar(&o.base, "base", envOr("LOADTEST_BASE", "https://ai.ethansoftwaredeveloper.com"), "base URL của production")
	flag.StringVar(&o.mode, "mode", "chat", "health | chat — health chỉ đập /api/health (miễn phí), chat chạy full agent (tốn token LLM)")
	flag.StringVar(&o.stages, "stages", "10:20,50:100,200:400,1000:1000", "danh sách stage dạng concurrency:total, chạy tuần tự")
	flag.StringVar(&o.token, "token", os.Getenv("LOADTEST_TOKEN"), "access_token JWT có sẵn (bỏ qua -secret nếu set)")
	flag.StringVar(&o.secret, "secret", os.Getenv("LOADTEST_JWT_SECRET"), "JWT_SECRET của production để tự mint token test")
	flag.IntVar(&o.users, "users", 100, "số user ảo (số token riêng biệt) khi mint bằng -secret")
	flag.StringVar(&o.promptsIn, "prompts", "", "file prompt, mỗi dòng 1 prompt (rỗng = dùng bộ mặc định)")
	flag.DurationVar(&o.timeout, "timeout", 180*time.Second, "timeout mỗi request")
	flag.BoolVar(&o.newConv, "new-conv", true, "true = mỗi request tạo conversation mới (giống user mới vào); false = 1 conversation/worker")
	flag.BoolVar(&o.canary, "canary", true, "chạy canary /api/health 1s/lần trong lúc bắn để phát hiện service ngất")
	flag.StringVar(&o.out, "out", "", "ghi báo cáo JSON ra file")
	flag.DurationVar(&o.rampDelay, "ramp-delay", 0, "giãn thời điểm start của các worker trong 1 stage (0 = bắn đồng loạt)")
	flag.DurationVar(&o.pause, "pause", 15*time.Second, "nghỉ giữa các stage cho service hồi")
	flag.BoolVar(&o.verbose, "v", false, "in từng lỗi khi xảy ra")
	flag.Parse()
	o.base = strings.TrimRight(o.base, "/")
	return o
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// ── Phân loại kết quả ───────────────────────────────────────────────────────

// outcome là nhãn tầng-gãy của 1 request. Đây là phần giá trị nhất của bộ test:
// 1000 con 429 từ Fastify rate-limit và 1000 con 502 từ nginx nói hai câu chuyện
// hoàn toàn khác nhau.
type outcome string

const (
	outOK                = outcome("ok")
	outRateLimitGateway  = outcome("429_gateway_ratelimit")  // Fastify @fastify/rate-limit chặn
	outRateLimitUpstream = outcome("429_llm_provider_quota") // Gemini/DeepSeek/Anthropic hết quota
	outUnauthorized      = outcome("401_unauthorized")
	outBadRequest        = outcome("4xx_other")
	outProxyDown         = outcome("502_504_proxy")    // nginx không tới được upstream
	outServerError       = outcome("5xx_app_error")    // app tự ném 500
	outTimeout           = outcome("timeout")          // vượt -timeout
	outConnError         = outcome("connection_error") // dial/reset/EOF khi bắt tay
	outStreamTruncated   = outcome("stream_truncated") // SSE đứt giữa đường, không có done
	outSSEError          = outcome("sse_error_event")  // agent gửi event error
	outConvCreateFailed  = outcome("conv_create_failed")
)

type result struct {
	Stage     int           `json:"stage"`
	Outcome   outcome       `json:"outcome"`
	Status    int           `json:"status"`
	TTFB      time.Duration `json:"ttfb_ms"`
	Total     time.Duration `json:"total_ms"`
	Tokens    int           `json:"tokens"`
	Chars     int           `json:"chars"`
	Detail    string        `json:"detail,omitempty"`
	StartedAt time.Time     `json:"started_at"`
}

// ── JWT minting (HS256) ─────────────────────────────────────────────────────

// mintToken tạo access_token giống hệt jwt.strategy.ts: HS256, payload
// {sub, email, role}. Dùng để tạo N user ảo mà không phải đăng ký + verify OTP.
func mintToken(secret, sub, email string, ttl time.Duration) (string, error) {
	if secret == "" {
		return "", errors.New("thiếu JWT secret")
	}
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	now := time.Now()
	claims := map[string]any{
		"sub":   sub,
		"email": email,
		"role":  "user",
		"iat":   now.Unix(),
		"exp":   now.Add(ttl).Unix(),
	}
	hb, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	cb, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	enc := base64.RawURLEncoding.EncodeToString
	signing := enc(hb) + "." + enc(cb)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signing))
	return signing + "." + enc(mac.Sum(nil)), nil
}

// ── Prompt pool ─────────────────────────────────────────────────────────────

// defaultPrompts trộn prompt ngắn (chat thuần) với prompt cần suy luận/tool để
// tải giống traffic thật, không phải 1000 lần cùng một câu (dễ bị cache che mắt).
var defaultPrompts = []string{
	"Xin chào, bạn là ai và làm được gì?",
	"Giải thích ngắn gọn RAG là gì cho người mới.",
	"Tóm tắt giúp tôi ưu và nhược điểm của kiến trúc microservices.",
	"Viết cho tôi một hàm Go đọc file JSON và trả về map.",
	"Hôm nay tôi nên ưu tiên việc gì nếu có 3 task deadline cùng lúc?",
	"So sánh PostgreSQL và MongoDB cho ứng dụng chat.",
	"Cho tôi 5 ý tưởng đặt tên biến cho hàng đợi xử lý ảnh.",
	"Nêu 3 rủi ro khi deploy Docker Compose trên VPS dùng chung.",
	"Cách debug memory leak trong Node.js là gì?",
	"Viết commit message chuẩn conventional cho việc thêm rate limit.",
	"Giải thích khác nhau giữa p95 và p99 latency.",
	"Tôi nên chọn SSE hay WebSocket cho streaming câu trả lời LLM?",
}

func loadPrompts(path string) ([]string, error) {
	if path == "" {
		return defaultPrompts, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		if line := strings.TrimSpace(sc.Text()); line != "" {
			out = append(out, line)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, errors.New("file prompt rỗng")
	}
	return out, nil
}

// ── Stage parsing ───────────────────────────────────────────────────────────

type stage struct {
	Concurrency int `json:"concurrency"`
	Total       int `json:"total"`
}

func parseStages(s string) ([]stage, error) {
	var out []stage
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		bits := strings.SplitN(part, ":", 2)
		if len(bits) != 2 {
			return nil, fmt.Errorf("stage %q sai định dạng, cần concurrency:total", part)
		}
		c, err := strconv.Atoi(strings.TrimSpace(bits[0]))
		if err != nil || c <= 0 {
			return nil, fmt.Errorf("stage %q: concurrency không hợp lệ", part)
		}
		n, err := strconv.Atoi(strings.TrimSpace(bits[1]))
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("stage %q: total không hợp lệ", part)
		}
		out = append(out, stage{Concurrency: c, Total: n})
	}
	if len(out) == 0 {
		return nil, errors.New("không có stage nào")
	}
	return out, nil
}

// ── HTTP client ─────────────────────────────────────────────────────────────

func newClient(maxConns int, timeout time.Duration) *http.Client {
	// Keep-alive pool phải đủ rộng, nếu không chính client sẽ thành nút cổ chai
	// và ta đo sai — tưởng server chậm nhưng thực ra đang chờ connection.
	tr := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   15 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          maxConns + 64,
		MaxIdleConnsPerHost:   maxConns + 64,
		MaxConnsPerHost:       0, // không giới hạn: để server là phía nói "không"
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   20 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     true,
	}
	return &http.Client{Transport: tr, Timeout: timeout}
}

// ── Nhận diện lỗi ───────────────────────────────────────────────────────────

func classifyHTTP(status int, hdr http.Header, body string) outcome {
	switch {
	case status == 401 || status == 403:
		return outUnauthorized
	case status == 429:
		// Fastify rate-limit luôn gắn x-ratelimit-limit; 429 không có header đó
		// thường là do upstream (LLM provider) dội xuống.
		if hdr.Get("x-ratelimit-limit") != "" || strings.Contains(strings.ToLower(body), "too many requests") {
			return outRateLimitGateway
		}
		return outRateLimitUpstream
	case status == 502 || status == 503 || status == 504:
		return outProxyDown
	case status >= 500:
		return outServerError
	case status >= 400:
		return outBadRequest
	}
	return outOK
}

func classifyErr(err error) (outcome, string) {
	if err == nil {
		return outOK, ""
	}
	msg := err.Error()
	low := strings.ToLower(msg)
	switch {
	case errors.Is(err, context.DeadlineExceeded), strings.Contains(low, "timeout"), strings.Contains(low, "deadline exceeded"):
		return outTimeout, msg
	case strings.Contains(low, "connection refused"), strings.Contains(low, "connection reset"),
		strings.Contains(low, "no such host"), strings.Contains(low, "eof"),
		strings.Contains(low, "broken pipe"), strings.Contains(low, "can't assign requested address"),
		strings.Contains(low, "too many open files"), strings.Contains(low, "tls handshake"):
		return outConnError, msg
	}
	return outConnError, msg
}

// quotaHint bắt lỗi quota/rate-limit của LLM provider lọt xuống qua SSE error
// event — bên trong app nó là 200 OK nên chỉ đọc body mới thấy.
func quotaHint(s string) bool {
	low := strings.ToLower(s)
	for _, k := range []string{"quota", "rate limit", "ratelimit", "429", "resource_exhausted", "overloaded", "too many requests"} {
		if strings.Contains(low, k) {
			return true
		}
	}
	return false
}

// ── Một "user ảo" ───────────────────────────────────────────────────────────

type runner struct {
	opt     options
	client  *http.Client
	tokens  []string
	prompts []string
}

func (r *runner) tokenFor(i int) string {
	if len(r.tokens) == 0 {
		return ""
	}
	return r.tokens[i%len(r.tokens)]
}

func (r *runner) do(ctx context.Context, req *http.Request, token string) (*http.Response, error) {
	if token != "" {
		req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	}
	req.Header.Set("User-Agent", "jarvis-loadtest/1.0")
	return r.client.Do(req.WithContext(ctx))
}

// createConversation trả về _id của conversation mới.
func (r *runner) createConversation(ctx context.Context, token, first string) (string, int, error) {
	body, _ := json.Marshal(map[string]string{"firstMessage": first})
	req, err := http.NewRequest(http.MethodPost, r.opt.base+"/api/conversations", bytes.NewReader(body))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.do(ctx, req, token)
	if err != nil {
		return "", 0, err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode >= 400 {
		return "", resp.StatusCode, fmt.Errorf("status %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}
	var out struct {
		ID  string `json:"_id"`
		Id2 string `json:"id"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", resp.StatusCode, fmt.Errorf("parse conversation: %w", err)
	}
	id := out.ID
	if id == "" {
		id = out.Id2
	}
	if id == "" {
		return "", resp.StatusCode, fmt.Errorf("không tìm thấy _id trong %s", truncate(string(raw), 200))
	}
	return id, resp.StatusCode, nil
}

// chat gửi 1 prompt và đọc hết SSE stream, đo TTFB (token đầu tiên) + tổng thời gian.
func (r *runner) chat(ctx context.Context, token, convID, prompt string) result {
	res := result{StartedAt: time.Now()}
	start := time.Now()

	body, _ := json.Marshal(map[string]string{"content": prompt})
	req, err := http.NewRequest(http.MethodPost, r.opt.base+"/api/conversations/"+convID+"/chat", bytes.NewReader(body))
	if err != nil {
		res.Outcome, res.Detail = outConnError, err.Error()
		return res
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := r.do(ctx, req, token)
	if err != nil {
		res.Outcome, res.Detail = classifyErr(err)
		res.Total = time.Since(start)
		return res
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
	res.Status = resp.StatusCode

	if resp.StatusCode != 200 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		res.Outcome = classifyHTTP(resp.StatusCode, resp.Header, string(raw))
		res.Detail = truncate(string(raw), 200)
		res.Total = time.Since(start)
		return res
	}

	// SSE: đọc từng dòng "data: {...}".
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 1<<20), 8<<20)
	var (
		sawFirst bool
		sawDone  bool
	)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" {
			continue
		}
		var ev map[string]any
		if err := json.Unmarshal([]byte(payload), &ev); err != nil {
			continue
		}
		if !sawFirst {
			// TTFB "hữu ích" = lúc user thấy chữ/step đầu tiên, không phải lúc
			// nhận HTTP header (SSE trả header ngay lập tức).
			res.TTFB = time.Since(start)
			sawFirst = true
		}
		if t, ok := ev["token"].(string); ok {
			res.Chars += len(t)
		}
		if s, ok := ev["type"].(string); ok && s == "error" {
			msg, _ := ev["message"].(string)
			res.Outcome = outSSEError
			if quotaHint(msg) || quotaHint(payload) {
				res.Outcome = outRateLimitUpstream
			}
			res.Detail = truncate(msg, 200)
		}
		if d, ok := ev["done"].(bool); ok && d {
			sawDone = true
			if v, ok := ev["tokens"].(float64); ok {
				res.Tokens = int(v)
			}
		}
	}
	res.Total = time.Since(start)

	if err := sc.Err(); err != nil {
		o, detail := classifyErr(err)
		res.Outcome, res.Detail = o, detail
		return res
	}
	if res.Outcome != "" {
		return res
	}
	if !sawDone {
		res.Outcome = outStreamTruncated
		res.Detail = fmt.Sprintf("stream đóng sau %d ký tự mà không có done", res.Chars)
		return res
	}
	res.Outcome = outOK
	return res
}

// health đập endpoint liveness — dùng cho baseline và cho canary.
func (r *runner) health(ctx context.Context) result {
	res := result{StartedAt: time.Now()}
	start := time.Now()
	req, err := http.NewRequest(http.MethodGet, r.opt.base+"/api/health", nil)
	if err != nil {
		res.Outcome, res.Detail = outConnError, err.Error()
		return res
	}
	resp, err := r.do(ctx, req, "")
	if err != nil {
		res.Outcome, res.Detail = classifyErr(err)
		res.Total = time.Since(start)
		return res
	}
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	_ = resp.Body.Close()
	res.Status = resp.StatusCode
	res.Total = time.Since(start)
	res.TTFB = res.Total
	res.Outcome = classifyHTTP(resp.StatusCode, resp.Header, string(raw))
	if res.Outcome != outOK {
		res.Detail = truncate(string(raw), 200)
	}
	return res
}

// ── Chạy 1 stage ────────────────────────────────────────────────────────────

func (r *runner) runStage(ctx context.Context, idx int, st stage) []result {
	results := make([]result, st.Total)
	var next int64 = -1
	var done int64

	// Progress ticker: bắn 1000 request có thể mất vài phút, im lặng là khó chịu.
	stop := make(chan struct{})
	go func() {
		t := time.NewTicker(5 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				fmt.Printf("    ... %d/%d xong\n", atomic.LoadInt64(&done), st.Total)
			}
		}
	}()

	var wg sync.WaitGroup
	for w := 0; w < st.Concurrency; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			if r.opt.rampDelay > 0 {
				time.Sleep(time.Duration(worker) * r.opt.rampDelay)
			}

			// Ở chế độ -new-conv=false, mỗi worker giữ 1 conversation cho cả
			// stage (giống 1 user chat nhiều lượt) -> history dài dần, tốn
			// token hơn nhưng đúng với hành vi hội thoại thật.
			var convID string
			token := r.tokenFor(worker)

			for {
				i := atomic.AddInt64(&next, 1)
				if int(i) >= st.Total {
					return
				}
				reqCtx, cancel := context.WithTimeout(ctx, r.opt.timeout)

				if r.opt.mode == "health" {
					results[i] = r.health(reqCtx)
					results[i].Stage = idx
					cancel()
					atomic.AddInt64(&done, 1)
					continue
				}

				prompt := r.prompts[int(i)%len(r.prompts)]

				needConv := r.opt.newConv || convID == ""
				if needConv {
					id, status, err := r.createConversation(reqCtx, token, prompt)
					if err != nil {
						res := result{Stage: idx, Status: status, StartedAt: time.Now()}
						if status > 0 {
							res.Outcome = classifyHTTP(status, http.Header{}, err.Error())
							if res.Outcome == outOK {
								res.Outcome = outConvCreateFailed
							}
						} else {
							res.Outcome, res.Detail = classifyErr(err)
						}
						if res.Detail == "" {
							res.Detail = truncate(err.Error(), 200)
						}
						results[i] = res
						cancel()
						atomic.AddInt64(&done, 1)
						if r.opt.verbose {
							fmt.Printf("      [w%d] conv fail: %s\n", worker, res.Detail)
						}
						continue
					}
					convID = id
				}

				res := r.chat(reqCtx, token, convID, prompt)
				res.Stage = idx
				results[i] = res
				cancel()
				atomic.AddInt64(&done, 1)
				if r.opt.verbose && res.Outcome != outOK {
					fmt.Printf("      [w%d] %s (%d): %s\n", worker, res.Outcome, res.Status, res.Detail)
				}
			}
		}(w)
	}
	wg.Wait()
	close(stop)
	return results
}

// ── Canary ──────────────────────────────────────────────────────────────────

// canary hỏi /api/health mỗi giây trong suốt bài test. Đây là thước đo trực tiếp
// cho câu "có tèo không": nếu canary cũng fail thì service đã ngất với MỌI user,
// không chỉ với đám request đang bắn.
type canaryReport struct {
	Samples     int            `json:"samples"`
	Failures    int            `json:"failures"`
	P50Ms       float64        `json:"p50_ms"`
	P99Ms       float64        `json:"p99_ms"`
	MaxMs       float64        `json:"max_ms"`
	FirstFailAt string         `json:"first_fail_at,omitempty"`
	LastFailAt  string         `json:"last_fail_at,omitempty"`
	FailKinds   map[string]int `json:"fail_kinds,omitempty"`
}

func (r *runner) startCanary(ctx context.Context) func() canaryReport {
	var mu sync.Mutex
	var lat []float64
	rep := canaryReport{FailKinds: map[string]int{}}
	client := newClient(4, 10*time.Second)
	cr := &runner{opt: r.opt, client: client}

	done := make(chan struct{})
	go func() {
		defer close(done)
		t := time.NewTicker(1 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				res := cr.health(c)
				cancel()
				mu.Lock()
				rep.Samples++
				if res.Outcome == outOK {
					lat = append(lat, float64(res.Total.Milliseconds()))
				} else {
					rep.Failures++
					rep.FailKinds[string(res.Outcome)]++
					ts := res.StartedAt.Format("15:04:05")
					if rep.FirstFailAt == "" {
						rep.FirstFailAt = ts
					}
					rep.LastFailAt = ts
				}
				mu.Unlock()
			}
		}
	}()

	return func() canaryReport {
		<-done
		mu.Lock()
		defer mu.Unlock()
		rep.P50Ms = percentile(lat, 50)
		rep.P99Ms = percentile(lat, 99)
		if len(lat) > 0 {
			sort.Float64s(lat)
			rep.MaxMs = lat[len(lat)-1]
		}
		return rep
	}
}

// ── Thống kê ────────────────────────────────────────────────────────────────

type stageReport struct {
	Stage       int            `json:"stage"`
	Concurrency int            `json:"concurrency"`
	Total       int            `json:"total"`
	WallMs      int64          `json:"wall_ms"`
	Throughput  float64        `json:"req_per_sec"`
	OK          int            `json:"ok"`
	SuccessRate float64        `json:"success_rate"`
	Outcomes    map[string]int `json:"outcomes"`
	TTFB        latencyStats   `json:"ttfb_ms"`
	Latency     latencyStats   `json:"latency_ms"`
	TokensSum   int            `json:"tokens_sum"`
	Samples     []string       `json:"error_samples,omitempty"`
}

type latencyStats struct {
	P50 float64 `json:"p50"`
	P90 float64 `json:"p90"`
	P95 float64 `json:"p95"`
	P99 float64 `json:"p99"`
	Max float64 `json:"max"`
	Avg float64 `json:"avg"`
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	s := make([]float64, len(sorted))
	copy(s, sorted)
	sort.Float64s(s)
	if len(s) == 1 {
		return s[0]
	}
	rank := (p / 100) * float64(len(s)-1)
	lo := int(math.Floor(rank))
	hi := int(math.Ceil(rank))
	if hi >= len(s) {
		hi = len(s) - 1
	}
	if lo == hi {
		return s[lo]
	}
	frac := rank - float64(lo)
	return s[lo] + (s[hi]-s[lo])*frac
}

func statsOf(vals []float64) latencyStats {
	if len(vals) == 0 {
		return latencyStats{}
	}
	s := make([]float64, len(vals))
	copy(s, vals)
	sort.Float64s(s)
	var sum float64
	for _, v := range s {
		sum += v
	}
	return latencyStats{
		P50: percentile(s, 50),
		P90: percentile(s, 90),
		P95: percentile(s, 95),
		P99: percentile(s, 99),
		Max: s[len(s)-1],
		Avg: sum / float64(len(s)),
	}
}

func summarize(idx int, st stage, wall time.Duration, results []result) stageReport {
	rep := stageReport{
		Stage:       idx,
		Concurrency: st.Concurrency,
		Total:       st.Total,
		WallMs:      wall.Milliseconds(),
		Outcomes:    map[string]int{},
	}
	var ttfb, total []float64
	seen := map[string]bool{}
	for _, r := range results {
		o := r.Outcome
		if o == "" {
			o = outConnError
		}
		rep.Outcomes[string(o)]++
		if o == outOK {
			rep.OK++
			rep.TokensSum += r.Tokens
			if r.TTFB > 0 {
				ttfb = append(ttfb, float64(r.TTFB.Milliseconds()))
			}
			total = append(total, float64(r.Total.Milliseconds()))
		} else if r.Detail != "" && !seen[string(o)] {
			seen[string(o)] = true
			rep.Samples = append(rep.Samples, fmt.Sprintf("%s → %s", o, r.Detail))
		}
	}
	if st.Total > 0 {
		rep.SuccessRate = float64(rep.OK) / float64(st.Total) * 100
	}
	if wall > 0 {
		rep.Throughput = float64(st.Total) / wall.Seconds()
	}
	rep.TTFB = statsOf(ttfb)
	rep.Latency = statsOf(total)
	return rep
}

func printStage(rep stageReport) {
	fmt.Printf("\n  ── Stage %d: %d concurrent × %d request ──\n", rep.Stage, rep.Concurrency, rep.Total)
	fmt.Printf("     wall=%.1fs  throughput=%.1f req/s  success=%.1f%% (%d/%d)\n",
		float64(rep.WallMs)/1000, rep.Throughput, rep.SuccessRate, rep.OK, rep.Total)
	if rep.Latency.P50 > 0 {
		fmt.Printf("     TTFB   p50=%.0fms p95=%.0fms p99=%.0fms max=%.0fms\n",
			rep.TTFB.P50, rep.TTFB.P95, rep.TTFB.P99, rep.TTFB.Max)
		fmt.Printf("     total  p50=%.0fms p95=%.0fms p99=%.0fms max=%.0fms\n",
			rep.Latency.P50, rep.Latency.P95, rep.Latency.P99, rep.Latency.Max)
	}
	keys := make([]string, 0, len(rep.Outcomes))
	for k := range rep.Outcomes {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return rep.Outcomes[keys[i]] > rep.Outcomes[keys[j]] })
	fmt.Printf("     kết quả:")
	for _, k := range keys {
		fmt.Printf("  %s=%d", k, rep.Outcomes[k])
	}
	fmt.Println()
	if rep.TokensSum > 0 {
		fmt.Printf("     token LLM đã dùng: %d\n", rep.TokensSum)
	}
	for _, s := range rep.Samples {
		fmt.Printf("     ví dụ lỗi: %s\n", s)
	}
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(strings.TrimSpace(s), "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// ── main ────────────────────────────────────────────────────────────────────

type fullReport struct {
	Base      string        `json:"base"`
	Mode      string        `json:"mode"`
	StartedAt string        `json:"started_at"`
	Stages    []stageReport `json:"stages"`
	Canary    canaryReport  `json:"canary"`
}

func main() {
	opt := parseFlags()

	stages, err := parseStages(opt.stages)
	if err != nil {
		fmt.Fprintln(os.Stderr, "❌", err)
		os.Exit(2)
	}
	prompts, err := loadPrompts(opt.promptsIn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "❌ prompt:", err)
		os.Exit(2)
	}

	maxC := 0
	for _, s := range stages {
		if s.Concurrency > maxC {
			maxC = s.Concurrency
		}
	}

	r := &runner{opt: opt, client: newClient(maxC, opt.timeout), prompts: prompts}

	// Token: ưu tiên -token, không thì mint N user ảo từ -secret.
	if opt.mode == "chat" {
		switch {
		case opt.token != "":
			r.tokens = []string{opt.token}
		case opt.secret != "":
			for i := 0; i < opt.users; i++ {
				tok, err := mintToken(opt.secret, fmt.Sprintf("loadtest-user-%04d", i),
					fmt.Sprintf("loadtest+%04d@example.com", i), 3*time.Hour)
				if err != nil {
					fmt.Fprintln(os.Stderr, "❌ mint token:", err)
					os.Exit(2)
				}
				r.tokens = append(r.tokens, tok)
			}
		default:
			fmt.Fprintln(os.Stderr, "❌ mode=chat cần -token hoặc -secret (xem -h)")
			os.Exit(2)
		}
	}

	fmt.Printf("🎯 Target: %s  |  mode=%s  |  stages=%s\n", opt.base, opt.mode, opt.stages)
	if opt.mode == "chat" {
		fmt.Printf("   %d user ảo, %d prompt mẫu, timeout=%s, new-conv=%v\n",
			len(r.tokens), len(prompts), opt.timeout, opt.newConv)
	}

	// Smoke: 1 request trước khi bắn thật — sai auth/URL thì biết ngay,
	// không đốt 1000 request để phát hiện typo.
	fmt.Print("\n🔎 Smoke test 1 request... ")
	smokeCtx, cancelSmoke := context.WithTimeout(context.Background(), opt.timeout)
	var smoke result
	if opt.mode == "health" {
		smoke = r.health(smokeCtx)
	} else {
		id, status, err := r.createConversation(smokeCtx, r.tokenFor(0), "smoke test")
		if err != nil {
			cancelSmoke()
			fmt.Printf("THẤT BẠI\n   tạo conversation lỗi (status %d): %v\n", status, err)
			os.Exit(1)
		}
		smoke = r.chat(smokeCtx, r.tokenFor(0), id, prompts[0])
	}
	cancelSmoke()
	if smoke.Outcome != outOK {
		fmt.Printf("THẤT BẠI\n   %s (status %d): %s\n", smoke.Outcome, smoke.Status, smoke.Detail)
		fmt.Println("   → Dừng lại. Sửa cấu hình trước khi bắn tải.")
		os.Exit(1)
	}
	fmt.Printf("OK (ttfb=%dms, total=%dms, %d ký tự, %d token)\n",
		smoke.TTFB.Milliseconds(), smoke.Total.Milliseconds(), smoke.Chars, smoke.Tokens)

	ctx, cancel := context.WithCancel(context.Background())
	var stopCanary func() canaryReport
	if opt.canary {
		stopCanary = r.startCanary(ctx)
	}

	report := fullReport{Base: opt.base, Mode: opt.mode, StartedAt: time.Now().Format(time.RFC3339)}
	for i, st := range stages {
		fmt.Printf("\n🔥 Stage %d/%d — %d concurrent, %d request\n", i+1, len(stages), st.Concurrency, st.Total)
		begin := time.Now()
		results := r.runStage(ctx, i+1, st)
		rep := summarize(i+1, st, time.Since(begin), results)
		printStage(rep)
		report.Stages = append(report.Stages, rep)

		// Dừng sớm khi stack đã gãy hẳn: bắn tiếp chỉ tốn tiền và tốn downtime.
		if rep.SuccessRate < 10 && i < len(stages)-1 {
			fmt.Printf("\n⛔ Success rate %.1f%% < 10%% — dừng leo thang, stack đã gãy ở mức này.\n", rep.SuccessRate)
			break
		}
		if i < len(stages)-1 && opt.pause > 0 {
			fmt.Printf("\n   ⏸  nghỉ %s cho service hồi...\n", opt.pause)
			time.Sleep(opt.pause)
		}
	}

	cancel()
	if stopCanary != nil {
		report.Canary = stopCanary()
		c := report.Canary
		fmt.Printf("\n🩺 Canary /api/health trong lúc test: %d mẫu, %d fail (%.1f%%), p50=%.0fms p99=%.0fms max=%.0fms\n",
			c.Samples, c.Failures, pct(c.Failures, c.Samples), c.P50Ms, c.P99Ms, c.MaxMs)
		if c.Failures > 0 {
			fmt.Printf("   ⚠️  service KHÔNG phản hồi được từ %s đến %s — kiểu lỗi: %v\n", c.FirstFailAt, c.LastFailAt, c.FailKinds)
		} else {
			fmt.Println("   ✅ service vẫn trả lời health check suốt bài test — không sập.")
		}
	}

	if opt.out != "" {
		b, err := json.MarshalIndent(report, "", "  ")
		if err == nil {
			if err := os.WriteFile(opt.out, b, 0o644); err == nil {
				fmt.Printf("\n📄 Báo cáo JSON: %s\n", opt.out)
			} else {
				fmt.Fprintln(os.Stderr, "⚠️  không ghi được báo cáo:", err)
			}
		}
	}
}

func pct(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b) * 100
}
