package http

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/ai-agent-tut/agent-go/internal/agent"
	"github.com/ai-agent-tut/agent-go/internal/guardrails"
	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// ConversationLearner extracts and persists knowledge in the background.
//
// ctx MUST be the original request context (r.Context()) — it carries the
// tenant ID set by middleware.TenantMiddleware, which the learner needs to
// scope every write (in-memory Store + Mongo) to the right tenant. See
// memory.Learner.LearnFromConversation for how it snapshots the tenant ID
// before detaching into a background goroutine.
type ConversationLearner interface {
	LearnFromConversation(ctx context.Context, messages []provider.Message, conversationID string)
}

// ChatHandler xử lý POST /chat — nhận JSON, chạy engine hoặc orchestrator,
// stream SSE events. Accepts agent.Runner interface so both Engine and
// Orchestrator work interchangeably.
type ChatHandler struct {
	runner  agent.Runner
	learner ConversationLearner
}

// NewChatHandler tạo ChatHandler với runner (Engine hoặc Orchestrator).
func NewChatHandler(runner agent.Runner) *ChatHandler {
	return &ChatHandler{runner: runner}
}

// SetLearner configures the autonomous background learner.
func (h *ChatHandler) SetLearner(l ConversationLearner) *ChatHandler {
	h.learner = l
	return h
}

// ChatRequest là body JSON client gửi lên.
type ChatRequest struct {
	ConversationID string        `json:"conversationId,omitempty"`
	History        []chatMessage `json:"history,omitempty"`
	UserMessage    string        `json:"userMessage"`
	MaxSteps       int           `json:"maxSteps,omitempty"`
	Attachments    []Attachment  `json:"attachments,omitempty"`
	// Lang là ngôn ngữ UI người dùng đang chọn ở FE (vd "en", "vi"). Optional —
	// rỗng giữ nguyên hành vi mặc định (tiếng Việt). Forward nguyên văn vào
	// agent.RunInput.Lang, xem node_model.go để biết cách nó ghi đè chỉ dẫn
	// ngôn ngữ trong system prompt cho riêng lượt chạy này.
	Lang string `json:"lang,omitempty"`
}

// Attachment represents a file or image attached to a user message.
type Attachment struct {
	Type     string `json:"type"` // "image" or "file"
	Name     string `json:"name"`
	Data     string `json:"data"` // base64 for images, text for files
	MimeType string `json:"mimeType"`
}

// chatMessage là message trong history (JSON-friendly).
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

const (
	maxImagesPerRequest = 7
	maxFilesPerRequest  = 7
	maxAttachmentBytes  = 10 * 1024 * 1024 // 10 MB
)

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

	// Validate input chống prompt injection + XSS (defense in depth).
	if err := guardrails.ValidateUserInput(req.UserMessage); err != nil {
		slog.Warn("chat input rejected by guardrails", "error", err, "len", len(req.UserMessage))
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusBadRequest)
		return
	}

	// Validate attachments: count + size limits.
	if err := validateAttachments(req.Attachments); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusBadRequest)
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

	// Map HTTP attachments to provider attachments.
	atts := make([]provider.Attachment, len(req.Attachments))
	for i, a := range req.Attachments {
		atts[i] = provider.Attachment{
			Type:     a.Type,
			Name:     a.Name,
			Data:     a.Data,
			MimeType: a.MimeType,
		}
	}

	input := agent.RunInput{
		ConversationID: req.ConversationID,
		History:        history,
		UserMessage:    req.UserMessage,
		Attachments:    atts,
		MaxSteps:       req.MaxSteps,
		Lang:           req.Lang,
	}

	var assistantContent strings.Builder
	emit := func(e agent.Event) {
		if e.Type == "text" {
			assistantContent.WriteString(e.Text)
		}
		data, _ := json.Marshal(e)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	_, _ = h.runner.Run(r.Context(), input, emit)

	if h.learner != nil && assistantContent.Len() > 0 {
		fullMsgs := make([]provider.Message, 0, len(history)+2)
		fullMsgs = append(fullMsgs, history...)
		fullMsgs = append(fullMsgs, provider.Message{Role: provider.RoleUser, Content: req.UserMessage})
		fullMsgs = append(fullMsgs, provider.Message{Role: provider.RoleAssistant, Content: assistantContent.String()})
		h.learner.LearnFromConversation(r.Context(), fullMsgs, req.ConversationID)
	}
}

// validateAttachments checks count limits (7 images + 7 files max) and
// size limits (10 MB per attachment).
func validateAttachments(atts []Attachment) error {
	var imageCount, fileCount int
	for _, a := range atts {
		switch a.Type {
		case "image":
			imageCount++
			// Decode base64 to check raw byte size.
			decoded, err := base64.StdEncoding.DecodeString(a.Data)
			if err != nil {
				return fmt.Errorf("invalid base64 for image %q: %w", a.Name, err)
			}
			if len(decoded) > maxAttachmentBytes {
				return fmt.Errorf("image %q exceeds 10 MB limit", a.Name)
			}
		case "file":
			fileCount++
			if len(a.Data) > maxAttachmentBytes {
				return fmt.Errorf("file %q exceeds 10 MB limit", a.Name)
			}
		default:
			return fmt.Errorf("unknown attachment type %q for %q (expected \"image\" or \"file\")", a.Type, a.Name)
		}
	}
	if imageCount > maxImagesPerRequest {
		return fmt.Errorf("too many images: %d (max %d)", imageCount, maxImagesPerRequest)
	}
	if fileCount > maxFilesPerRequest {
		return fmt.Errorf("too many files: %d (max %d)", fileCount, maxFilesPerRequest)
	}
	return nil
}
