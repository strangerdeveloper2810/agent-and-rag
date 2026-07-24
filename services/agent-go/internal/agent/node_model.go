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

	req := provider.GenerateRequest{
		System:   eng.getSystemPrompt(),
		Messages: s.Messages,
		Tools:    reg.ToolDefs(),
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
