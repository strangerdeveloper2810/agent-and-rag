package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/ai-agent-tut/agent-go/internal/provider"
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

	// Dynamic thinking: choose OFF/LOW/MEDIUM based on task complexity
	thinkingLevel := provider.ThinkingOff
	if dt := eng.getDynamicThinking(); dt.Enabled {
		lastMsg := ""
		if len(s.Messages) > 0 {
			lastMsg = s.Messages[len(s.Messages)-1].Content
		}
		hasTools := len(s.Messages) > 2 // previous turns had tool calls
		thinkingLevel = ResolveThinking(dt, provider.ThinkingOff, lastMsg, hasTools, s.Step)
	}

	req := provider.GenerateRequest{
		System:   eng.getSystemPrompt(),
		Messages: s.Messages,
		Tools:    reg.ToolDefs(),
		Options: provider.ProviderOptions{
			Cache:         true,
			ThinkingLevel: thinkingLevel,
		},
	}

	stream, err := prov.Generate(ctx, req)
	if err != nil {
		emit(ErrorEvent(err.Error()))
		return NodeEnd, fmt.Errorf("model: generate: %w", err)
	}

	var content strings.Builder
	var toolCalls []provider.ToolCall

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
			}

		case provider.ChunkError:
			emit(ErrorEvent(chunk.Err.Error()))
			return NodeEnd, fmt.Errorf("model: provider error: %w", chunk.Err)

		case provider.ChunkDone:
			// done — channel sẽ đóng sau chunk này
		}
	}

	// Append assistant message.
	s.Messages = append(s.Messages, provider.Message{
		Role:      provider.RoleAssistant,
		Content:   content.String(),
		ToolCalls: toolCalls,
	})

	s.Step++

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
