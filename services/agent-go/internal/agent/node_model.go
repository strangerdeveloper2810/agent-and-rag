package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ai-agent-tut/agent-go/internal/middleware"
	"github.com/ai-agent-tut/agent-go/internal/observability"
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
	getMaxOutputTokens() int
	getOwnerTenants() []string
	getDynamicThinking() DynamicThinkingConfig
	getSkillLoader() *skills.Loader
	getFastModel() string
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
	if trimmed := trimContext(ctx, prov, eng.getFastModel(), s, eng.getMaxContextTokens()); trimmed > 0 {
		s.TrimmedTokens += trimmed
		emit(MemoryEvent(fmt.Sprintf("trimmed %d tokens from context", trimmed)))
	}

	// Câu hỏi HIỆN TẠI của người dùng — dùng chung cho dynamic thinking, skill
	// matching và lọc tool theo task. Phải là message user MỚI NHẤT: trước đây
	// vòng lặp ở đây duyệt xuôi + break nên lấy câu ĐẦU cuộc hội thoại, khiến
	// mọi lượt chat có history lọc tool/match skill theo câu hỏi đã cũ.
	userInput := s.LastUserContent()

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

	// Set tên builtin skill user đã TẮT (user_disabled_skills) — loại khỏi skill
	// matching cho lượt này. Việc lọc nằm ở đây (per-request) chứ KHÔNG đổi
	// BuildSystemPrompt tĩnh (dùng chung, cacheable) — đúng mô hình đã có cho
	// lang/persona/custom instructions.
	disabled := make(map[string]bool, len(s.DisabledSkills))
	for _, name := range s.DisabledSkills {
		disabled[name] = true
	}

	if sl := eng.getSkillLoader(); sl != nil && s.activatedSkills == nil {
		s.activatedSkills = make(map[string]bool)
	}
	// skillTools là tool mà skill đang kích hoạt khai báo cần dùng (frontmatter
	// `tools:` trong SKILL.md). Được BẢO ĐẢM có trong tool list gửi cho LLM.
	var skillTools []string
	if sl := eng.getSkillLoader(); sl != nil {
		if matched := sl.MatchSkill(userInput); matched != nil && !disabled[matched.Name] && !s.activatedSkills[matched.Name] {
			s.activatedSkills[matched.Name] = true
			// PromptBody: thân skill đã bỏ frontmatter và gọt vừa ngân sách token
			// (skills.MaxPromptBytes) — nội dung này được chèn lại ở MỖI lượt chat
			// có skill khớp, nên độ dài của nó là chi phí lặp.
			body := matched.PromptBody()
			systemPrompt += "\n\n[KỸ NĂNG ĐANG KÍCH HOẠT: " + matched.Name + "]\n" + body
			// Log kèm kích thước + có bị gọt hay không: skill bị gọt nhiều là tín
			// hiệu nên viết lại SKILL.md cho phần cốt lõi lên đầu, chứ không phải
			// cứ để nó âm thầm mất đuôi.
			slog.Info("model: skill activated",
				"skill", matched.Name,
				"skill_tools", matched.Tools,
				"body_bytes", len(body),
				"truncated", len(body) < len(matched.Content),
			)
			emit(MemoryEvent("Kích hoạt kỹ năng: " + matched.Name + " — " + matched.Description))
			skillTools = matched.Tools
		}
		// Skill đã kích hoạt ở bước trước trong CÙNG lượt chạy vẫn phải giữ
		// được tool của nó (activatedSkills chặn inject prompt lặp, nhưng không
		// nên chặn quyền dùng tool).
		if len(skillTools) == 0 {
			for name := range s.activatedSkills {
				if sk := sl.LoadSkill(name); sk != nil {
					skillTools = append(skillTools, sk.Tools...)
				}
			}
		}
	}

	// Weave memories recalled by the recall node (memory.RecallNode) for this
	// turn into the system prompt. The base systemPrompt returned by
	// getSystemPrompt() is built once at startup via BuildSystemPrompt(nil, ...)
	// so it can stay a stable, cacheable prefix (P6 prompt caching) — the
	// per-request recall results are appended dynamically here instead,
	// mirroring the "[BỘ NHỚ]" section BuildSystemPrompt would have produced
	// had real memories been available at startup.
	if len(s.RecalledMemories) > 0 {
		var mb strings.Builder
		mb.WriteString("\n\n[BỘ NHỚ] — Các quy ước, sở thích và kinh nghiệm kỹ thuật đã học từ người dùng (ưu tiên tuân thủ khi đưa ra giải pháp):\n")
		for _, m := range s.RecalledMemories {
			mb.WriteString("- " + m + "\n")
		}
		systemPrompt += mb.String()
	}

	// Per-request UI language override (i18n). getSystemPrompt() is the
	// STATIC prompt built once at wiring time (agent.BuildSystemPrompt), which
	// defaults to Vietnamese and is shared across every concurrent request —
	// it can't carry a per-user language choice. When the frontend user picked
	// English (s.Lang == "en", forwarded via RunInput.Lang for this turn
	// only), append an explicit override here instead, mirroring how
	// RecalledMemories above is woven in per-request without mutating the
	// cacheable prefix.
	if s.Lang == "en" {
		systemPrompt += "\n\n[NGÔN NGỮ TRẢ LỜI CHO LƯỢT NÀY]\nALWAYS respond in English for this turn (including all ask_user clarifying question prompts, headers, option labels, descriptions, and follow-up suggestions) — this overrides any earlier Vietnamese-default instruction above.\n"
	} else if s.Lang != "vi" && isEnglishInput(userInput) {
		systemPrompt += "\n\n[USER IS COMMUNICATING IN ENGLISH]\nThe user prompt is in English. You MUST respond completely in English for this turn (including all text responses, markdown tables, code explanations, ask_user question prompts/headers/options, and follow-up suggestions).\n"
	}

	// Per-user Custom Instructions
	if s.CustomInstructions != "" {
		systemPrompt += "\n\n[CHỈ THỊ RIÊNG TỪ NGƯỜI DÙNG]\n" + s.CustomInstructions + "\n"
	}

	// Per-user Persona & Style
	if s.PersonaPreset != "" || s.Formality != "" || s.Verbosity != "" {
		var pb strings.Builder
		pb.WriteString("\n\n[PHONG CÁCH & ĐỊNH HƯỚNG PERSONA CHO LƯỢT NÀY]\n")
		if s.PersonaPreset != "" && s.PersonaPreset != "default" {
			pb.WriteString(fmt.Sprintf("- Định hướng vai trò: %s\n", s.PersonaPreset))
		}
		if s.Formality != "" {
			pb.WriteString(fmt.Sprintf("- Giọng điệu giao tiếp: %s\n", s.Formality))
		}
		if s.Verbosity != "" {
			pb.WriteString(fmt.Sprintf("- Độ chi tiết câu trả lời: %s\n", s.Verbosity))
		}
		systemPrompt += pb.String()
	}

	// Custom skills của user (prompt instruction text lưu trong PostgreSQL) — nối
	// toàn bộ vào system prompt để agent "nhận và dùng khi phù hợp". Khác builtin
	// skill (progressive disclosure qua MatchSkill), custom skill là chỉ thị riêng
	// của user nên luôn sẵn, không cần trigger.
	if len(s.CustomSkills) > 0 {
		var csb strings.Builder
		csb.WriteString("\n\n[KỸ NĂNG TUỲ CHỈNH CỦA NGƯỜI DÙNG]\n")
		for _, cs := range s.CustomSkills {
			csb.WriteString("\n### " + cs.Name)
			if cs.Description != "" {
				csb.WriteString(" — " + cs.Description)
			}
			csb.WriteString("\n")
			if cs.WhenToUse != "" {
				csb.WriteString("Khi nào dùng: " + cs.WhenToUse + "\n")
			}
			if cs.Content != "" {
				csb.WriteString(cs.Content + "\n")
			}
		}
		systemPrompt += csb.String()
	}

	// Register tool theo task: bước đầu (step 0) chỉ gửi tool liên quan
	// intent người dùng (3-8 tool thay vì toàn bộ registry) — giảm token +
	// latency + nhiễu tool-call. Từ bước 1 trở đi gửi toàn bộ để cho phép
	// tool chain phức tạp.
	toolDefs := reg.FilterToolDefs(userInput, s.Step)

	// Bảo đảm tool mà skill khai báo luôn có mặt. Trước đây field `tools:` trong
	// SKILL.md được parse (skills/loader.go) rồi KHÔNG đọc ở đâu cả — dead code
	// cùng loại với AgentSpec.SystemPrompt: skill learning-tutor khai
	// `tools: [web.search, web.fetch]` nhưng FilterToolDefs chạy độc lập nên có
	// lượt tool list không hề chứa web.fetch, khiến hướng dẫn trong skill
	// ("web.fetch 2-3 nguồn tốt nhất") không thể thực hiện được.
	//
	// Cố tình dùng UNION (bổ sung) chứ KHÔNG phải INTERSECTION (giới hạn): lấy
	// giao sẽ cắt mất memory.save/memory.recall và mọi tool khác mà skill không
	// liệt kê, làm agent yếu đi mỗi khi có skill kích hoạt.
	if len(skillTools) > 0 {
		toolDefs = reg.UnionToolDefs(toolDefs, skillTools)
	}

	// MCP tools (SSE remote) do user cấu hình — LUÔN có mặt trong tool list cho
	// lượt này (không qua FilterToolDefs theo intent vì chúng là tool riêng của
	// user, thường ít). Registry riêng từng lượt chạy — xem Engine.Run.
	if s.mcpRegistry != nil {
		toolDefs = append(toolDefs, s.mcpRegistry.ToolDefs()...)
	}

	// Tenant không phải chủ hệ thống KHÔNG được thấy nhóm tool đặc quyền
	// (file.*, shell.exec, git) — chúng tác động lên máy chạy agent, không scope
	// theo tenant, nên với nhiều người dùng thì bất kỳ ai cũng đọc được .env
	// chứa API key của server. Ẩn khỏi tool list (thay vì chỉ chặn khi thực thi)
	// để model không cố gọi rồi nhận lỗi và làm rối câu trả lời; node_tools chặn
	// thêm một lớp nữa cho chắc.
	if !tools.IsOwnerTenant(middleware.GetTenantID(ctx), eng.getOwnerTenants()) {
		before := len(toolDefs)
		toolDefs = tools.StripPrivilegedTools(toolDefs)
		if removed := before - len(toolDefs); removed > 0 {
			slog.Debug("model: ẩn tool đặc quyền với tenant không phải chủ", "removed", removed)
		}
	}

	req := provider.GenerateRequest{
		System:   systemPrompt,
		Messages: s.Messages,
		Tools:    toolDefs,
		Options: provider.ProviderOptions{
			Cache:         true,
			ThinkingLevel: thinkingLevel,
			// Trần output token. Trước đây không set (cfg.MaxTokens là config
			// chết) nên request gửi max_tokens=0 → API không có trần nào, và
			// finish_reason=length không bao giờ xảy ra nên cờ s.Truncated +
			// nút "Tiếp tục" trên UI thực tế không có cơ hội kích hoạt.
			MaxTokens: eng.getMaxOutputTokens(),
		},
	}

	slog.Info("model: calling LLM", "provider", prov.Name(), "messages", len(s.Messages), "tools", len(req.Tools), "thinking", string(req.Options.ThinkingLevel))
	llmStart := time.Now()

	// LangSmith LLM Child Run
	llmRunID := observability.NewUUID()
	ls := observability.GetLangSmith()
	if ls != nil {
		lsMessages := make([]map[string]any, 0, len(s.Messages)+1)
		if systemPrompt != "" {
			lsMessages = append(lsMessages, map[string]any{
				"role":    "system",
				"content": systemPrompt,
			})
		}
		for _, m := range s.Messages {
			msgMap := map[string]any{
				"role":    m.Role,
				"content": m.Content,
			}
			if len(m.ToolCalls) > 0 {
				msgMap["tool_calls"] = m.ToolCalls
			}
			if m.ToolCallID != "" {
				msgMap["tool_call_id"] = m.ToolCallID
			}
			lsMessages = append(lsMessages, msgMap)
		}

		ls.StartChildRun(
			llmRunID,
			s.RunID,
			prov.Name(),
			observability.RunTypeLLM,
			map[string]any{
				"messages":       lsMessages,
				"tools":          req.Tools,
				"thinking_level": string(req.Options.ThinkingLevel),
			},
			map[string]any{
				"provider": prov.Name(),
				"step":     s.Step,
			},
		)
	}

	stream, err := prov.Generate(ctx, req)
	if err != nil {
		slog.Error("model: LLM call failed", "err", err, "provider", prov.Name())
		if ls != nil {
			ls.EndRun(llmRunID, nil, err)
		}
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
				// ChunkUsage là SNAPSHOT CỘNG DỒN của lượt gọi này, KHÔNG phải delta.
				//
				// Gemini gửi usageMetadata kèm promptTokenCount ĐẦY ĐỦ ở MỌI chunk
				// stream (gemini.go: emit ChunkUsage trong vòng lặp), nên cộng dồn
				// sẽ nhân input token với số chunk. Đo thực tế trên production:
				// một câu chat gửi ~5.200 token bị báo thành 41.400 (×8), câu dài
				// hơn thành 90.528 (×17). Anthropic/DeepSeek chỉ gửi một lần ở cuối
				// nên lấy max hoạt động đúng cho cả hai kiểu.
				//
				// Hệ quả của bug cũ không chỉ là log sai: totalTokens chảy ra tận
				// UI và vào contextTokens/contextBudget mà FE dùng để gợi ý "nên
				// bắt đầu chat mới", nên người dùng bị nhắc tạo chat mới quá sớm.
				stepInput = max(stepInput, chunk.Usage.InputTokens)
				stepOutput = max(stepOutput, chunk.Usage.OutputTokens)
			}

		case provider.ChunkError:
			if ls != nil {
				ls.EndRun(llmRunID, nil, chunk.Err)
			}
			emit(ErrorEvent(chunk.Err.Error()))
			return NodeEnd, fmt.Errorf("model: provider error: %w", chunk.Err)

		case provider.ChunkDone:
			// done — channel sẽ đóng sau chunk này
			if chunk.FinishReason != "" {
				finish = chunk.FinishReason
			}
		}
	}

	if ls != nil {
		ls.EndRun(llmRunID, map[string]any{
			"generations": []map[string]any{
				{
					"text": content.String(),
					"message": map[string]any{
						"role":       "assistant",
						"content":    content.String(),
						"tool_calls": toolCalls,
					},
				},
			},
			"content":    content.String(),
			"tool_calls": toolCalls,
			"llm_output": map[string]any{
				"token_usage": map[string]any{
					"prompt_tokens":     stepInput,
					"completion_tokens": stepOutput,
					"total_tokens":      stepInput + stepOutput,
				},
				"model_name": prov.Name(),
			},
			"duration_ms": time.Since(llmStart).Milliseconds(),
		}, nil)
	}

	// Model bị cắt vì chạm giới hạn output token → báo cho client để hiện
	// chỉ báo + nút "Tiếp tục". Không phải lỗi: phần text đã stream vẫn giữ.
	s.Truncated = finish == provider.FinishLength
	if s.Truncated {
		slog.Warn("model: response truncated by max output tokens", "provider", prov.Name(), "content_len", content.Len())
		emit(TruncatedEvent())
	}

	// Cộng usage của BƯỚC này vào tổng của cả lượt chạy. Cộng ở đây (một lần
	// cho mỗi lượt gọi provider) chứ không cộng trong vòng lặp stream — xem
	// giải thích ở case ChunkUsage.
	s.Usage.InputTokens += stepInput
	s.Usage.OutputTokens += stepOutput

	// Sync cumulative total and emit per-step usage event.
	s.TotalTokens = s.Usage.InputTokens + s.Usage.OutputTokens
	if stepInput > 0 || stepOutput > 0 {
		emit(UsageEvent(stepInput, stepOutput, s.Usage.InputTokens, s.Usage.OutputTokens))
	}

	// Completion rỗng (không text, không tool call) mà không phải do bị cắt vì
	// max_tokens là bất thường — một số API trả 200 kèm content rỗng khi gặp
	// lỗi nội bộ mà không báo qua ChunkError. Coi đây là lỗi thay vì âm thầm
	// cho qua như 1 lượt hợp lệ (route() sẽ đưa thẳng tới extract/end với câu
	// trả lời trống, người dùng không biết agent vừa thất bại).
	if content.Len() == 0 && len(toolCalls) == 0 && !s.Truncated {
		err := fmt.Errorf("model: empty response from provider %q (no text, no tool calls, finish=%q)",
			prov.Name(), finish)
		slog.Error("model: empty response", "provider", prov.Name(), "finish_reason", finish)
		emit(ErrorEvent(err.Error()))
		return NodeEnd, err
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

// trimContext estimates the token count of s.Messages and, if it exceeds
// maxTokens, drops the oldest messages — keeping only the tail — and replaces
// them with either a REAL LLM-generated summary (via SummarizeMessages, using
// model rẻ/nhanh) khi thành công, hoặc một placeholder TRUNG THỰC (nói rõ là
// đã lược bỏ, KHÔNG giả vờ đã tóm tắt) khi lượt gọi LLM thất bại/hết thời gian.
// Returns the number of estimated tokens trimmed (0 if no trimming was needed).
func trimContext(ctx context.Context, prov provider.Provider, model string, s *State, maxTokens int) int {
	if maxTokens <= 0 || len(s.Messages) <= keepLast {
		return 0
	}

	est := estimateTokens(s.Messages)
	if est <= maxTokens {
		return 0
	}

	// SafeDropBoundary tránh cắt giữa 1 cặp tool_call/tool_result — xem doc.
	// dropCount luôn > 0 tại đây: input len(s.Messages)-keepLast > 0 (đã check
	// ở trên) và SafeDropBoundary chỉ TĂNG dropCount, không bao giờ giảm.
	dropCount := SafeDropBoundary(s.Messages, len(s.Messages)-keepLast)

	dropped := s.Messages[:dropCount]
	var trimmedChars int
	for _, m := range dropped {
		trimmedChars += len(m.Content)
		trimmedChars += len(m.Role)
		trimmedChars += len(m.ToolCallID)
		for _, tc := range m.ToolCalls {
			trimmedChars += len(tc.Name) + len(tc.ID) + len(tc.Args)
		}
	}
	trimmedTokens := max(trimmedChars/4, 1)

	var noteContent string
	if summary, ok := SummarizeMessages(ctx, prov, model, dropped); ok {
		noteContent = fmt.Sprintf("[Tóm tắt %d tin nhắn cũ hơn]: %s", dropCount, summary)
	} else {
		noteContent = fmt.Sprintf("[%d tin nhắn cũ hơn đã bị lược bỏ do vượt giới hạn ngữ cảnh — không tóm tắt được]", dropCount)
	}

	newMsgs := make([]provider.Message, 1, len(s.Messages)-dropCount+1)
	newMsgs[0] = provider.Message{
		Role:    provider.RoleUser,
		Content: noteContent,
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

// isEnglishInput phát hiện câu hỏi người dùng có phải là tiếng Anh hay không.
func isEnglishInput(s string) bool {
	if s == "" {
		return false
	}
	// Nếu có dấu thanh tiếng Việt → tiếng Việt
	for _, r := range s {
		if (r >= 0x00C0 && r <= 0x00FF) || (r >= 0x0100 && r <= 0x024F) || (r >= 0x1EA0 && r <= 0x1EF9) {
			return false
		}
	}
	// Kiểm tra độ dài từ tiếng Anh
	words := strings.Fields(strings.ToLower(s))
	return len(words) >= 3
}
