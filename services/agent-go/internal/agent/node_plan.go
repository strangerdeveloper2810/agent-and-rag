package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ai-agent-tut/agent-go/internal/provider"
)

var complexKeywords = []string{
	"plan", "timeline", "steps", "step by step", "first do",
	"break down", "outline", "roadmap", "phases", "milestones",
	"multiple steps", "several steps", "sequence of",
}

func isComplexRequest(input string) bool {
	lower := strings.ToLower(input)
	for _, kw := range complexKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// nodePlan phân tích user request và tạo plan nếu request phức tạp.
// Plan được lưu vào State để các node sau tham chiếu.
// Chạy giữa summarize và model.
func nodePlan(ctx context.Context, eng modelEngine, s *State, emit EmitFunc) (NodeID, error) {
	if len(s.Plan) > 0 {
		return NodeModel, nil
	}

	var userInput string
	for _, m := range s.Messages {
		if m.Role == provider.RoleUser {
			userInput = m.Content
			break
		}
	}

	if userInput == "" || !isComplexRequest(userInput) {
		return NodeModel, nil
	}

	slog.Info("plan: detecting complex request, generating plan")
	prov := eng.getProvider()

	planPrompt := fmt.Sprintf(`You are a task planner. Given the user's request below, break it into a numbered list of discrete, actionable steps. Be concise.

Return ONLY a JSON array of strings. Example: ["Step 1", "Step 2"].

User request:
%s`, userInput)

	req := provider.GenerateRequest{
		System:   "You are a task planner. Return ONLY a JSON array of strings, one per step. No other text.",
		Messages: []provider.Message{{Role: provider.RoleUser, Content: planPrompt}},
		Options:  provider.ProviderOptions{ThinkingLevel: provider.ThinkingOff},
	}

	stream, err := prov.Generate(ctx, req)
	if err != nil {
		slog.Warn("plan: LLM call failed, skipping", "err", err)
		return NodeModel, nil
	}

	var response strings.Builder
	for chunk := range stream {
		switch chunk.Kind {
		case provider.ChunkText:
			response.WriteString(chunk.Text)
		case provider.ChunkUsage:
			if chunk.Usage != nil {
				s.Usage.InputTokens += chunk.Usage.InputTokens
				s.Usage.OutputTokens += chunk.Usage.OutputTokens
			}
		case provider.ChunkError:
			slog.Warn("plan: LLM stream error, skipping", "err", chunk.Err)
			return NodeModel, nil
		}
	}

	planText := strings.TrimSpace(response.String())
	if planText == "" {
		return NodeModel, nil
	}

	planText = extractJSONArray(planText)
	var steps []string
	if err := json.Unmarshal([]byte(planText), &steps); err != nil {
		slog.Warn("plan: failed to parse plan JSON, skipping", "text", planText[:min(100, len(planText))], "err", err)
		return NodeModel, nil
	}

	if len(steps) == 0 {
		return NodeModel, nil
	}

	s.Plan = steps
	s.PlanStep = 0
	s.TotalTokens = s.Usage.InputTokens + s.Usage.OutputTokens

	slog.Info("plan: generated", "steps", len(steps))
	emit(PlanEvent(steps))
	return NodeModel, nil
}

func extractJSONArray(s string) string {
	start := strings.Index(s, "[")
	end := strings.LastIndex(s, "]")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}
