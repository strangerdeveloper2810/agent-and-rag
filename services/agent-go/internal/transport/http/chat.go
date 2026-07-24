package http

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ai-agent-tut/agent-go/internal/agent"
	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// ChatHandler xử lý POST /chat — nhận JSON, chạy engine, stream SSE events.
type ChatHandler struct {
	engine *agent.Engine
}

// NewChatHandler tạo ChatHandler với engine đã được inject.
func NewChatHandler(engine *agent.Engine) *ChatHandler {
	return &ChatHandler{engine: engine}
}

// ChatRequest là body JSON client gửi lên.
type ChatRequest struct {
	ConversationID string        `json:"conversationId,omitempty"`
	History        []chatMessage `json:"history,omitempty"`
	UserMessage    string        `json:"userMessage"`
	MaxSteps       int           `json:"maxSteps,omitempty"`
}

// chatMessage là message trong history (JSON-friendly).
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ServeHTTP implements http.Handler.
func (h *ChatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"invalid json: %v"}`, err), http.StatusBadRequest)
		return
	}
	if req.UserMessage == "" {
		http.Error(w, `{"error":"userMessage is required"}`, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	history := make([]provider.Message, len(req.History))
	for i, m := range req.History {
		history[i] = provider.Message{
			Role:    provider.Role(m.Role),
			Content: m.Content,
		}
	}

	input := agent.RunInput{
		ConversationID: req.ConversationID,
		History:        history,
		UserMessage:    req.UserMessage,
		MaxSteps:       req.MaxSteps,
	}

	emit := func(e agent.Event) {
		data, _ := json.Marshal(e)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	_, _ = h.engine.Run(r.Context(), input, emit)
}
