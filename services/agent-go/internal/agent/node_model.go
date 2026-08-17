package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ai-agent-tut/agent-go/internal/provider"
	"github.com/ai-agent-tut/agent-go/internal/skills"
	"github.com/ai-agent-tut/agent-go/internal/tools"
)

// modelEngine là interface engine cung cấp cho node model (tránh import cycle).
// Engine thật và fakeEngine trong test đều implements interface này.
type modelEngine interface {
	getProvider() provider.Provider
	getRegistry() *tools.Registry
	getSystemPrompt() string
	getMaxContextTokens() int
	getDynamicThinking() DynamicThinkingConfig
	getSkillLoader() *skills.Loader
}

// nodeModel gọi LLM (qua Provider) với toàn bộ Messages + Tools,
// stream kết quả qua emit, append assistant message vào State, rồi gọi route.
//
// Flow:
//  1. Lấy Provider + Registry từ engine
//  2. provider.Generate(ctx, request) → stream channel
//  3. Loop qua stream: Text → emit + gom content; ToolCall → gom list; Usage → cộng dồn
//  4. Append assistant message (Content + ToolCalls) vào state.Messages
//  5. Tăng state.Step
//  6. return route(s), nil
func nodeModel(ctx context.Context, eng modelEngine, s *State, emit EmitFunc) (NodeID, error) {
	prov := eng.getProvider()
	reg := eng.getRegistry()

	// Token budget: trim context if over limit.
	if trimmed := trimContext(s, eng.getMaxContextTokens()); trimmed > 0 {
		s.TrimmedTokens += trimmed
		emit(MemoryEvent(fmt.Sprintf("trimmed %d tokens from context", trimmed)))
	}

	// Input gốc của người dùng — dùng chung cho dynamic thinking, skill
	// matching và lọc tool theo task.
	var userInput string
	for _, m := range s.Messages {
		if m.Role == provider.RoleUser {
			userInput = m.Content
			break
		}
	}

	// Dynamic thinking: choose OFF/LOW/MEDIUM based on task complexity.
	// Only applied when no explicit ThinkingLevel is configured.
	thinkingLevel := provider.ThinkingOff
	if dt := eng.getDynamicThinking(); dt.Enabled {
		hasToolCalls := s.LastAssistant() != nil && len(s.LastAssistant().ToolCalls) > 0
		thinkingLevel = ResolveThinking(dt, provider.ThinkingOff, userInput, hasToolCalls, s.Step)
	}

	// Progressive skill disclosure: match user input against skill triggers
	// and inject full SKILL.md content into the system prompt on first match.
	systemPrompt := eng.getSystemPrompt()
	if sl := eng.getSkillLoader(); sl != nil && s.activatedSkills == nil {
		s.activatedSkills = make(map[string]bool)
	}
	if sl := eng.getSkillLoader(); sl != nil {
		if matched := sl.MatchSkill(userInput); matched != nil && !s.activatedSkills[matched.Name] {
			s.activatedSkills[matched.Name] = true
			systemPrompt += "\n\n[KỸ NĂNG ĐANG KÍCH HOẠT: " + matched.Name + "]\n" + matched.Content
			slog.Info("model: skill activated", "skill", matched.Name)
			emit(MemoryEvent("Kích hoạt kỹ năng: " + matched.Name + " — " + matched.Description))
		}
	}

	// Register tool theo task: bước đầu (step 0) chỉ gửi tool liên quan
	// intent người dùng (3-8 tool thay vì toàn bộ registry) — giảm token +
	// latency + nhiễu tool-call. Từ bước 1 trở đi gửi toàn bộ để cho phép
	// tool chain phức tạp.
	toolDefs := reg.FilterToolDefs(userInput, s.Step)

	req := provider.GenerateRequest{
		System:   systemPrompt,
		Messages: s.Messages,
		Tools:    toolDefs,
		Options: provider.ProviderOptions{
			Cache:         true,
			ThinkingLevel: thinkingLevel,
		},
	}

	slog.Info("model: calling LLM", "provider", prov.Name(), "messages", len(s.Messages), "tools", len(req.Tools), "thinking", string(req.Options.ThinkingLevel))
	llmStart := time.Now()

	stream, err := prov.Generate(ctx, req)
	if err != nil {
		slog.Error("model: LLM call failed", "err", err, "provider", prov.Name())
		emit(ErrorEvent(err.Error()))
		return NodeEnd, fmt.Errorf("model: generate: %w", err)
	}

	var content strings.Builder
	var toolCalls []provider.ToolCall
	var stepInput, stepOutput int
	var finish provider.FinishReason

	for chunk := range stream {
		switch chunk.Kind {
		case provider.ChunkText:
			content.WriteString(chunk.Text)
			emit(TextEvent(chunk.Text))

		case provider.ChunkToolCall:
			if chunk.ToolCall != nil {
				toolCalls = append(toolCalls, *chunk.ToolCall)
			}

		case provider.ChunkUsage:
			if chunk.Usage != nil {
				s.Usage.InputTokens += chunk.Usage.InputTokens
				s.Usage.OutputTokens += chunk.Usage.OutputTokens
				stepInput += chunk.Usage.InputTokens
				stepOutput += chunk.Usage.OutputTokens
			}

		case provider.ChunkError:
			emit(ErrorEvent(chunk.Err.Error()))
			return NodeEnd, fmt.Errorf("model: provider error: %w", chunk.Err)

		case provider.ChunkDone:
			// done — channel sẽ đóng sau chunk này
			if chunk.FinishReason != "" {
				finish = chunk.FinishReason
			}
		}
	}

	// Model bị cắt vì chạm giới hạn output token → báo cho client để hiện
	// chỉ báo + nút "Tiếp tục". Không phải lỗi: phần text đã stream vẫn giữ.
	s.Truncated = finish == provider.FinishLength
	if s.Truncated {
		slog.Warn("model: response truncated by max output tokens", "provider", prov.Name(), "content_len", content.Len())
		emit(TruncatedEvent())
	}

	// Sync cumulative total and emit per-step usage event.
	s.TotalTokens = s.Usage.InputTokens + s.Usage.OutputTokens
	if stepInput > 0 || stepOutput > 0 {
		emit(UsageEvent(stepInput, stepOutput, s.Usage.InputTokens, s.Usage.OutputTokens))
	}

	// Append assistant message.
	s.Messages = append(s.Messages, provider.Message{
		Role:      provider.RoleAssistant,
		Content:   content.String(),
		ToolCalls: toolCalls,
	})

	s.Step++

	slog.Info("model: LLM response", "provider", prov.Name(), "content_len", content.Len(),
		"tool_calls", len(toolCalls), "tokens_in", s.Usage.InputTokens, "tokens_out", s.Usage.OutputTokens,
		"elapsed_ms", time.Since(llmStart).Milliseconds())

	return route(s), nil
}

// keepLast is the number of most recent messages to preserve when trimming.
const keepLast = 10

// trimContext estimates the token count of s.Messages and, if it exceeds maxTokens,
// drops middle messages keeping only the last keepLast messages plus a summary placeholder.
// Returns the number of estimated tokens trimmed (0 if no trimming was needed).
func trimContext(s *State, maxTokens int) int {
	if maxTokens <= 0 || len(s.Messages) <= keepLast {
		return 0
	}

	est := estimateTokens(s.Messages)
	if est <= maxTokens {
		return 0
	}

	// Calculate trimmed tokens from messages being dropped.
	dropCount := len(s.Messages) - keepLast
	var trimmedChars int
	for _, m := range s.Messages[:dropCount] {
		trimmedChars += len(m.Content)
		trimmedChars += len(m.Role)
		trimmedChars += len(m.ToolCallID)
		for _, tc := range m.ToolCalls {
			trimmedChars += len(tc.Name) + len(tc.ID) + len(tc.Args)
		}
	}
	trimmedTokens := max(trimmedChars/4, 1)

	// Build new messages: summary placeholder + last keepLast messages.
	newMsgs := make([]provider.Message, 1, keepLast+1)
	newMsgs[0] = provider.Message{
		Role:    provider.RoleUser,
		Content: "[...các tin nhắn cũ hơn đã được tóm tắt...]",
	}
	newMsgs = append(newMsgs, s.Messages[dropCount:]...)
	s.Messages = newMsgs

	return trimmedTokens
}

// estimateTokens estimates token count from messages using the rough heuristic
// of 1 token ≈ 4 characters (works for most Latin and CJK text).
func estimateTokens(messages []provider.Message) int {
	total := 0
	for _, m := range messages {
		total += len(m.Content)
		total += len(string(m.Role))
		total += len(m.ToolCallID)
		for _, tc := range m.ToolCalls {
			total += len(tc.Name) + len(tc.ID) + len(tc.Args)
		}
	}
	return total / 4
}
