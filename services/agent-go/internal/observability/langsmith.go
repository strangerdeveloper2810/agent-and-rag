// Package observability cung cấp LangSmith Tracing client cho agent-go.
// Gửi trace bất đồng bộ (non-blocking) qua background worker queue lên LangSmith REST API.
package observability

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/ai-agent-tut/agent-go/internal/config"
)

// RunType định nghĩa loại run trên LangSmith.
type RunType string

const (
	RunTypeChain RunType = "chain"
	RunTypeLLM   RunType = "llm"
	RunTypeTool  RunType = "tool"
)

// LangSmithRun đại diện cho một trace run payload gửi tới LangSmith API.
type LangSmithRun struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	RunType     RunType        `json:"run_type"`
	StartTime   string         `json:"start_time"`
	EndTime     *string        `json:"end_time,omitempty"`
	Inputs      map[string]any `json:"inputs,omitempty"`
	Outputs     map[string]any `json:"outputs,omitempty"`
	Error       *string        `json:"error,omitempty"`
	ParentRunID *string        `json:"parent_run_id,omitempty"`
	ProjectName string         `json:"project_name,omitempty"`
	SessionName string         `json:"session_name,omitempty"`
	Extra       map[string]any `json:"extra,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
}

// LangSmithClient quản lý gửi traces lên LangSmith.
type LangSmithClient struct {
	apiKey   string
	project  string
	endpoint string
	enabled  bool
	client   *http.Client
	queue    chan runAction
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
}

type actionType int

const (
	actionCreate actionType = iota
	actionUpdate
)

type runAction struct {
	kind actionType
	run  LangSmithRun
}

var (
	globalClient *LangSmithClient
	globalMu     sync.RWMutex
)

// InitLangSmith khởi tạo LangSmith tracing client toàn cục từ config.
func InitLangSmith(cfg config.Config) *LangSmithClient {
	globalMu.Lock()
	defer globalMu.Unlock()

	if globalClient != nil {
		globalClient.Close()
	}

	enabled := cfg.LangSmithTracing && cfg.LangSmithAPIKey != ""
	ctx, cancel := context.WithCancel(context.Background())

	c := &LangSmithClient{
		apiKey:   cfg.LangSmithAPIKey,
		project:  cfg.LangSmithProject,
		endpoint: cfg.LangSmithEndpoint,
		enabled:  enabled,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		queue:  make(chan runAction, 500),
		ctx:    ctx,
		cancel: cancel,
	}

	if enabled {
		c.wg.Add(1)
		go c.worker()
		slog.Info("langsmith: tracing enabled",
			"project", c.project,
			"endpoint", c.endpoint,
		)
	} else {
		slog.Info("langsmith: tracing disabled (no API key or tracing=false)")
	}

	globalClient = c
	return c
}

// GetLangSmith trả về global LangSmith client.
func GetLangSmith() *LangSmithClient {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalClient
}

// Close đợi worker xử lý hết queue và dừng.
func (c *LangSmithClient) Close() {
	if c == nil || !c.enabled {
		return
	}
	c.cancel()
	c.wg.Wait()
}

// NewUUID sinh UUID v4 chuẩn dạng hex string.
func NewUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant RFC 4122
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func isoTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

// StartChainRun tạo một Root Chain Run (đại diện cho toàn bộ lượt chat của Agent).
func (c *LangSmithClient) StartChainRun(runID, name string, inputs map[string]any, tags []string, extra map[string]any) {
	if c == nil || !c.enabled {
		return
	}
	run := LangSmithRun{
		ID:          runID,
		Name:        name,
		RunType:     RunTypeChain,
		StartTime:   isoTime(time.Now()),
		Inputs:      inputs,
		ProjectName: c.project,
		SessionName: c.project,
		Tags:        tags,
		Extra:       extra,
	}
	c.enqueue(actionCreate, run)
}

// StartChildRun tạo một Child Run (LLM call hoặc Tool call) thuộc về parentRunID.
func (c *LangSmithClient) StartChildRun(runID, parentRunID, name string, runType RunType, inputs map[string]any, extra map[string]any) {
	if c == nil || !c.enabled {
		return
	}
	run := LangSmithRun{
		ID:          runID,
		Name:        name,
		RunType:     runType,
		StartTime:   isoTime(time.Now()),
		ParentRunID: &parentRunID,
		Inputs:      inputs,
		ProjectName: c.project,
		SessionName: c.project,
		Extra:       extra,
	}
	c.enqueue(actionCreate, run)
}

// EndRun kết thúc một Run với outputs hoặc error.
func (c *LangSmithClient) EndRun(runID string, outputs map[string]any, err error) {
	if c == nil || !c.enabled {
		return
	}
	now := isoTime(time.Now())
	run := LangSmithRun{
		ID:      runID,
		EndTime: &now,
		Outputs: outputs,
	}
	if err != nil {
		errStr := err.Error()
		run.Error = &errStr
	}
	c.enqueue(actionUpdate, run)
}

func (c *LangSmithClient) enqueue(kind actionType, run LangSmithRun) {
	select {
	case c.queue <- runAction{kind: kind, run: run}:
	default:
		slog.Warn("langsmith: queue full, dropping run", "runID", run.ID, "name", run.Name)
	}
}

func (c *LangSmithClient) worker() {
	defer c.wg.Done()

	for {
		select {
		case <-c.ctx.Done():
			// Flush remaining
			for len(c.queue) > 0 {
				act := <-c.queue
				c.sendHTTP(act)
			}
			return
		case act := <-c.queue:
			c.sendHTTP(act)
		}
	}
}

func (c *LangSmithClient) sendHTTP(act runAction) {
	var url string
	var method string

	if act.kind == actionCreate {
		url = fmt.Sprintf("%s/runs", c.endpoint)
		method = "POST"
	} else {
		url = fmt.Sprintf("%s/runs/%s", c.endpoint, act.run.ID)
		method = "PATCH"
	}

	body, err := json.Marshal(act.run)
	if err != nil {
		return
	}

	req, err := http.NewRequestWithContext(context.Background(), method, url, bytes.NewReader(body))
	if err != nil {
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		slog.Debug("langsmith: send failed", "err", err)
		return
	}
	_ = resp.Body.Close()
}
