package agent

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/ai-agent-tut/agent-go/internal/guardrails"
	"github.com/ai-agent-tut/agent-go/internal/mcp"
	"github.com/ai-agent-tut/agent-go/internal/observability"
	"github.com/ai-agent-tut/agent-go/internal/provider"
	"github.com/ai-agent-tut/agent-go/internal/skills"
	"github.com/ai-agent-tut/agent-go/internal/tools"
)

// stepTracer là tracer OTel dùng riêng cho span quanh mỗi step của engine
// loop — xem observability.SetupTracer cho cách TracerProvider global được
// dựng thật (trước đây là noop, không phát telemetry gì).
var stepTracer = observability.Tracer("agent-go/engine")

// Engine là trái tim của agent runtime — chạy vòng lặp ReAct:
// recall → summarize → model → route → tools → model → route → ... → extract → end.
//
// Engine được inject Provider + Tool Registry qua constructor (DI).
// Memory nodes (recall, extract, summarize) được inject qua SetMemoryNodes
// để tránh import cycle giữa agent và memory package.
// Mọi I/O đều qua interface → test được với FakeProvider + Echo tool.
type Engine struct {
	prov     provider.Provider
	registry *tools.Registry

	// System prompt — injected before each LLM call
	systemPrompt string

	// Skill loader for progressive disclosure — skills are loaded on demand
	// when user input matches a skill's trigger keywords.
	skillLoader *skills.Loader

	// Memory node implementations — set via SetMemoryNodes.
	// nil = skip node (fallback: jump to next logical node).
	recallFn    Node
	extractFn   Node
	summarizeFn Node

	// Planning node implementations — set via SetPlanningNodes.
	// nil = skip node (fallback: jump to next logical node).
	planFn    Node
	reflectFn Node

	// Circuit breaker detects stuck loops (same tool+args called consecutively).
	// nil = disabled.
	circuitBreaker *guardrails.CircuitBreaker

	// Dynamic thinking: auto-adjust thinking level based on task complexity.
	dynamicThinking DynamicThinkingConfig

	// MaxContextTokens is the token budget before context trimming kicks in.
	// 0 = unlimited (no trimming). Default: 100000.
	maxContextTokens int

	// maxToolOutput giới hạn số KÝ TỰ output của mỗi tool được đưa vào
	// s.Messages. 0 = không giới hạn.
	//
	// Trước đây cfg.MaxToolOutput là CONFIG CHẾT (khai báo nhưng không nơi nào
	// đọc), và việc cắt output nằm rời rạc trong từng tool với các ngưỡng khác
	// nhau (8000 shell/git, 10000 http/json, 15000 web, 24000 files/rag) —
	// trong khi file.search và rag.search KHÔNG cắt gì cả. Một
	// file.search {"pattern":"*"} có thể đẩy hàng MB vào context, làm lượt LLM
	// sau đắt đột biến, lỗi provider, hoặc bị trimContext cắt mất ngữ cảnh cũ.
	// Đây là chốt an toàn TẬP TRUNG, áp cho mọi tool bất kể tool có tự cắt hay không.
	maxToolOutput int

	// maxTotalToolOutput là ngân sách TỔNG ký tự tool output cộng dồn qua CẢ
	// LƯỢT CHẠY (nhiều step), khác với maxToolOutput (trần từng tool call riêng
	// lẻ) — xem applyToolOutputBudget.
	maxTotalToolOutput int

	// allowDestructiveTools cho phép chạy tool KindDestructive không cần xác
	// nhận. false (mặc định) → guardrails chặn và agent giải thích cho user.
	allowDestructiveTools bool

	// maxOutputTokens là trần output token cho MỖI lần gọi LLM. 0 = không giới
	// hạn. Trước đây cfg.MaxTokens là config chết nên request luôn gửi
	// max_tokens=0 và không có trần nào — xem cfg.MaxTokens.
	maxOutputTokens int

	// ownerTenantIDs: tenant được dùng nhóm tool đặc quyền. Rỗng = chỉ tenant
	// "default" (local, không auth) — xem tools.IsOwnerTenant.
	ownerTenantIDs []string

	// fastModel là model rẻ/nhanh dùng cho các tác vụ phụ trợ tốn thêm 1 LLM
	// call (nén ngữ cảnh trong trimContext, reflection, HyDE, rerank) — KHÔNG
	// phải model chat chính. Rỗng → trimContext không gọi LLM, chỉ dùng
	// fallback trung thực (lược bỏ, không tóm tắt).
	fastModel string

	// interruptStore lưu SNAPSHOT MỚI NHẤT của State — không chỉ khi dừng ở
	// NodeInterrupt (HITL) mà SAU MỖI LẦN CHUYỂN NODE của cả lượt chạy (xem
	// checkpoint(), gọi từ runLoop). Tên field/interface giữ nguyên
	// "interruptStore"/"InterruptStore" (thay vì đổi thành "checkpointStore")
	// để không phải sửa cmd/server/main.go — nơi duy nhất gọi SetInterruptStore
	// — nhưng NGỮ NGHĨA đã tổng quát hơn tên gọi: đây giờ là "nơi lưu
	// checkpoint mới nhất của run", cho phép resume:
	//   (a) khi dừng CÓ CHỦ ĐÍCH ở NodeInterrupt (HITL, hành vi gốc), hoặc
	//   (b) khi tiến trình agent-go CRASH/RESTART giữa lúc đang chạy 1 node
	//       bất kỳ khác (NodeModel, NodeTools, NodeReflect...) — xem resume.go.
	// nil (mặc định) = resume TẮT hoàn toàn: engine chạy y hệt trước khi có
	// tính năng này, không lưu gì bất kể dừng ở đâu.
	interruptStore InterruptStore

	// name định danh agent này (vd "general"/"code"/"research" — khớp
	// orchestrator.AgentSpec.Name) — CHỈ dùng để gắn kèm khi lưu paused run
	// (saveInterruptedState), giúp /chat/resume tìm lại ĐÚNG Engine gốc
	// (registry tool khác nhau giữa các agent — vd chỉ codeRegistry có
	// shell.exec/git). Rỗng nếu không gọi SetName (vd cmd/jarvis dùng 1
	// engine duy nhất, không cần phân biệt).
	name string
}

// InterruptStore là nơi lưu/đọc/xoá snapshot MỚI NHẤT của State cho một
// RunID — implement bởi *sqlite.Store (bảng paused_runs, xem
// internal/storage/sqlite/paused_runs.go). Khai báo interface NHỎ ở đây
// (thay vì Engine phụ thuộc thẳng *sqlite.Store) để package agent không phải
// import internal/storage/sqlite.
//
// SaveInterruptedState là UPSERT theo run_id (ghi đè bản cũ) — dùng được làm
// checkpoint gọi LẶP LẠI nhiều lần trong đời 1 run (mỗi lần chuyển node),
// không chỉ 1 lần duy nhất khi dừng ở NodeInterrupt như thiết kế ban đầu.
// DeleteInterruptedState được Engine gọi khi run kết thúc TỰ NHIÊN ở NodeEnd
// (dọn checkpoint không còn cần nữa) — trước đây chỉ được gọi bởi handler
// /chat/resume sau khi resolve xong 1 interrupt.
type InterruptStore interface {
	SaveInterruptedState(runID, agentName string, stateJSON []byte) error
	DeleteInterruptedState(runID string) error
}

// SetInterruptStore gán nơi lưu checkpoint của State — được gọi SAU MỖI LẦN
// CHUYỂN NODE trong runLoop (không chỉ khi dừng ở NodeInterrupt), cho phép
// /chat/resume tiếp tục MỘT lượt chạy đã dừng vì BẤT KỲ lý do gì (HITL, hoặc
// tiến trình agent-go crash/restart giữa chừng) — xem resume.go. Truyền nil
// để tắt tính năng resume hoàn toàn (mặc định).
func (e *Engine) SetInterruptStore(store InterruptStore) {
	e.interruptStore = store
}

// SetName gán tên agent (xem field name) — gọi cùng lúc với SetInterruptStore
// khi wiring nhiều Engine trong 1 Orchestrator.
func (e *Engine) SetName(name string) {
	e.name = name
}

// persistCheckpoint serialize s và lưu qua interruptStore. KHÔNG tự kiểm tra
// e.interruptStore == nil — caller (checkpoint/saveInterruptedState) phải
// check trước, vì 2 caller đó log lỗi khác nhau khi thất bại.
func (e *Engine) persistCheckpoint(s *State) error {
	data, err := s.SerializeForResume()
	if err != nil {
		return fmt.Errorf("serialize: %w", err)
	}
	if err := e.interruptStore.SaveInterruptedState(s.RunID, e.name, data); err != nil {
		return fmt.Errorf("save: %w", err)
	}
	return nil
}

// checkpoint lưu snapshot MỚI NHẤT của s (nếu có cấu hình interruptStore) —
// gọi SAU MỖI LẦN CHUYỂN NODE trong runLoop (trừ khi dừng ở NodeInterrupt/
// NodeEnd, xem 2 hàm riêng bên dưới). Đây là phần MỞ RỘNG so với thiết kế
// gốc (chỉ lưu 1 lần khi dừng ở NodeInterrupt) — cho phép resume một run bị
// CRASH giữa chừng ở bất kỳ node nào, không chỉ khi cố ý hỏi user.
//
// FAIL-SAFE TUYỆT ĐỐI: đây là tính năng PHỤ TRỢ (checkpoint để phục hồi sau
// crash), KHÔNG PHẢI đường dẫn chính của response. Lỗi ở đây CHỈ log cảnh
// báo — không bao giờ được phép làm hỏng/chặn response cho user hiện tại,
// cùng triết lý với saveInterruptedState/SetInterruptStore fallback.
//
// Gọi ĐỒNG BỘ (không phải goroutine): serialize + 1 lần ghi SQLite mỗi bước
// tốn thêm vài ms, chấp nhận được cho use case này. Làm ASYNC sẽ nhanh hơn
// nhưng có nguy cơ 2 checkpoint ghi song song hoán đổi thứ tự (checkpoint cũ
// hơn ghi ĐÈ lên checkpoint mới hơn nếu goroutine sau hoàn thành trước) — để
// dành tối ưu sau khi thật sự cần, không làm sớm rồi tạo race condition.
func (e *Engine) checkpoint(s *State) {
	if e.interruptStore == nil {
		return
	}
	if err := e.persistCheckpoint(s); err != nil {
		slog.Warn("engine: checkpoint thất bại — bỏ qua, run vẫn tiếp tục bình thường (nếu crash NGAY BÂY GIỜ, run này sẽ không resume được từ đúng bước này)",
			"runID", s.RunID, "step", s.Step, "err", err)
		return
	}
	slog.Debug("engine: checkpoint saved", "runID", s.RunID, "step", s.Step)
}

// saveInterruptedState là bản checkpoint DÀNH RIÊNG cho lúc dừng ở
// NodeInterrupt (HITL) — logic lưu giống hệt checkpoint(), chỉ khác mức log
// (Info thay vì Warn/Debug) vì đây là điểm dừng CÓ CHỦ Ý, không phải checkpoint
// định kỳ âm thầm.
func (e *Engine) saveInterruptedState(s *State) {
	if e.interruptStore == nil {
		return
	}
	if err := e.persistCheckpoint(s); err != nil {
		slog.Error("engine: lưu paused run thất bại — /chat/resume sẽ không dùng được cho run này", "runID", s.RunID, "err", err)
		return
	}
	slog.Info("engine: run paused ở NodeInterrupt — state đã lưu, có thể resume", "runID", s.RunID, "agent", e.name)
}

// deleteCheckpoint xoá checkpoint của s (nếu có cấu hình interruptStore) khi
// run kết thúc TỰ NHIÊN ở NodeEnd — tránh tích luỹ rác trong paused_runs cho
// những run KHÔNG BAO GIỜ dừng ở NodeInterrupt (đa số run bình thường). Lỗi
// chỉ log cảnh báo — không chặn response, chỉ để lại 1 dòng rác vô hại trong
// SQLite (không ảnh hưởng correctness, /chat/resume chỉ đọc theo run_id cụ
// thể nên rác không tự nhiên bị dùng nhầm).
func (e *Engine) deleteCheckpoint(s *State) {
	if e.interruptStore == nil {
		return
	}
	if err := e.interruptStore.DeleteInterruptedState(s.RunID); err != nil {
		slog.Warn("engine: xoá checkpoint sau khi run kết thúc thất bại — bỏ qua, chỉ để lại rác vô hại trong SQLite",
			"runID", s.RunID, "err", err)
	}
}

// SetMaxOutputTokens đặt trần output token cho mỗi lần gọi LLM. n <= 0 = không giới hạn.
func (e *Engine) SetMaxOutputTokens(n int) {
	e.maxOutputTokens = n
}

func (e *Engine) getMaxOutputTokens() int { return e.maxOutputTokens }

// SetOwnerTenants khai báo các tenant được dùng nhóm tool đặc quyền (file.*,
// shell.exec, git) — xem tools.IsOwnerTenant và cfg.OwnerTenantIDs.
func (e *Engine) SetOwnerTenants(ids []string) {
	e.ownerTenantIDs = ids
}

func (e *Engine) getOwnerTenants() []string { return e.ownerTenantIDs }

// SetMaxToolOutput đặt giới hạn ký tự output mỗi tool đưa vào context.
// n <= 0 → không giới hạn.
func (e *Engine) SetMaxToolOutput(n int) {
	e.maxToolOutput = n
}

func (e *Engine) getMaxToolOutput() int { return e.maxToolOutput }

// SetMaxTotalToolOutput đặt ngân sách TỔNG ký tự tool output cộng dồn qua cả
// lượt chạy. n <= 0 = không giới hạn.
func (e *Engine) SetMaxTotalToolOutput(n int) {
	e.maxTotalToolOutput = n
}

func (e *Engine) getMaxTotalToolOutput() int { return e.maxTotalToolOutput }

// SetAllowDestructiveTools cho phép agent tự chạy tool KindDestructive (shell.exec)
// mà không cần xác nhận. MẶC ĐỊNH false — xem cfg.AllowDestructiveTools.
func (e *Engine) SetAllowDestructiveTools(allow bool) {
	e.allowDestructiveTools = allow
}

func (e *Engine) getAllowDestructiveTools() bool { return e.allowDestructiveTools }

// SetDynamicThinking enables auto-adjusting thinking mode.
func (e *Engine) SetDynamicThinking(cfg DynamicThinkingConfig) {
	e.dynamicThinking = cfg
}

func (e *Engine) getDynamicThinking() DynamicThinkingConfig {
	return e.dynamicThinking
}

// SetFastModel đặt model rẻ/nhanh dùng cho các tác vụ phụ trợ (nén ngữ cảnh
// trong trimContext) — xem fastModel.
func (e *Engine) SetFastModel(model string) {
	e.fastModel = model
}

func (e *Engine) getFastModel() string { return e.fastModel }

// NewEngine tạo Engine với provider và tool registry cho trước.
func NewEngine(prov provider.Provider, registry *tools.Registry) *Engine {
	return &Engine{
		prov:               prov,
		registry:           registry,
		maxContextTokens:   100000,
		maxToolOutput:      defaultMaxToolOutput,
		maxTotalToolOutput: defaultMaxTotalToolOutput,
	}
}

// SetMemoryNodes gán các node memory (recall, extract, summarize).
// Dùng factory từ memory package: engine.SetMemoryNodes(memory.RecallNode(store), ...)
// nil node → node bị skip khi dispatch (fallback an toàn).
func (e *Engine) SetMemoryNodes(recall, extract, summarize Node) {
	e.recallFn = recall
	e.extractFn = extract
	e.summarizeFn = summarize
}

// SetPlanningNodes gán các node plan và reflect.
// nil node → node bị skip khi dispatch (fallback an toàn).
func (e *Engine) SetPlanningNodes(plan, reflect Node) {
	e.planFn = plan
	e.reflectFn = reflect
}

// EnablePlanning bật node plan/reflect nội bộ: request phức tạp tốn thêm
// 1 LLM call (plan) trước token đầu tiên. TẮT mặc định.
func (e *Engine) EnablePlanning() {
	e.planFn = func(ctx context.Context, s *State, emit EmitFunc) (NodeID, error) {
		return nodePlan(ctx, e, s, emit)
	}
	e.reflectFn = nodeReflect
}

// getProvider / getRegistry / getSystemPrompt / getSkillLoader — implements modelEngine & toolsEngine.
func (e *Engine) getProvider() provider.Provider { return e.prov }
func (e *Engine) getRegistry() *tools.Registry   { return e.registry }
func (e *Engine) getSystemPrompt() string        { return e.systemPrompt }
func (e *Engine) getSkillLoader() *skills.Loader { return e.skillLoader }

// SetSystemPrompt sets the system prompt used for every LLM call.
func (e *Engine) SetSystemPrompt(prompt string) {
	e.systemPrompt = prompt
}

// SetSkillLoader sets the skills loader for progressive disclosure.
// When nil, skill matching is disabled.
func (e *Engine) SetSkillLoader(l *skills.Loader) {
	e.skillLoader = l
}

// SetCircuitBreaker sets the circuit breaker for stuck-loop detection.
// Pass nil to disable.
func (e *Engine) SetCircuitBreaker(cb *guardrails.CircuitBreaker) {
	e.circuitBreaker = cb
}

// SetMaxContextTokens sets the token budget before context trimming kicks in.
// 0 = unlimited (no trimming).
func (e *Engine) SetMaxContextTokens(n int) {
	e.maxContextTokens = n
}

// getMaxContextTokens implements modelEngine.
func (e *Engine) getMaxContextTokens() int { return e.maxContextTokens }

// Run chạy agent loop cho một lượt chat.
//
// Flow:
//
//	for {
//	    ctx.Err()? → return
//	    dispatch(node, state, emit) → nextNode
//	    nextNode == END → break
//	    node = nextNode
//	}
//	emit(DoneEvent)
//
// Run nhận ctx từ HTTP handler → khi client disconnect, ctx bị cancel
// → loop dừng ở lần check tiếp theo.
func (e *Engine) Run(ctx context.Context, in RunInput, emit EmitFunc) (provider.Usage, error) {
	start := time.Now()
	s := newState(in)

	// Discovery MCP tools (SSE remote) cho LƯỢT CHẠY NÀY nếu user cấu hình. Tool
	// được đăng ký vào registry RIÊNG (s.mcpRegistry) — không ghi vào registry
	// dùng chung, tránh data race + rò rỉ tool giữa các user. Lỗi discovery không
	// chặn lượt chat: chỉ log và tiếp tục không có MCP tool.
	if len(in.McpServers) > 0 {
		cfg := make([]mcp.ServerConfig, 0, len(in.McpServers))
		for _, srv := range in.McpServers {
			// srv.Transport (http/sse) KHÔNG được truyền vào ServerConfig: cả 2
			// giá trị đều dùng chung mcp.SSEClient (Streamable HTTP với fallback
			// đọc response JSON hoặc SSE) — xem comment McpServer.Transport ở
			// state.go. APIKey là phần THỰC SỰ quan trọng ở đây: đây là token user
			// cấu hình cho server remote (Notion/GitHub/Linear/Sentry...), được
			// SSEClient gửi thành header Authorization: Bearer <APIKey>.
			cfg = append(cfg, mcp.ServerConfig{Name: srv.Name, URL: srv.URL, APIKey: srv.APIKey})
		}
		reg, clients, err := mcp.DiscoverSSE(ctx, cfg)
		if err != nil {
			slog.Warn("engine: MCP discovery thất bại", "err", err)
		} else {
			s.mcpRegistry = reg
			s.mcpClients = make([]io.Closer, 0, len(clients))
			for _, c := range clients {
				s.mcpClients = append(s.mcpClients, c)
			}
			slog.Info("engine: MCP tools đã discovery", "servers", len(clients), "tools", len(reg.ToolDefs()))
		}
	}
	if len(s.mcpClients) > 0 {
		defer func() {
			for _, c := range s.mcpClients {
				_ = c.Close()
			}
		}()
	}

	node := NodeRecall

	// Breaker RIÊNG cho lượt chạy này. Trước đây engine dùng thẳng
	// e.circuitBreaker — MỘT instance chia sẻ cho cả 3 agent và toàn bộ
	// process, và Reset() không được gọi ở đâu trong production. Hệ quả: 2
	// request khác nhau (khác user) gọi cùng tool + cùng args thì request thứ 3
	// bị chặn "stuck loop" oan, tool không chạy và câu trả lời rỗng; ngược lại
	// 2 run song song ghi đè state của nhau nên loop thật lại không bị phát hiện.
	if e.circuitBreaker != nil {
		s.loopBreaker = guardrails.NewCircuitBreaker(e.circuitBreaker.MaxRepeats())
	}

	slog.Info("engine: run started", "provider", e.prov.Name(), "maxSteps", s.MaxSteps, "runID", s.RunID)

	// LangSmith Root Chain Trace
	ls := observability.GetLangSmith()
	if ls != nil {
		ls.StartChainRun(
			s.RunID,
			"JARVIS Agent",
			map[string]any{
				"user_message": in.UserMessage,
				"history_len":  len(in.History),
			},
			[]string{"agent-go", e.prov.Name()},
			map[string]any{
				"conversation_id": in.ConversationID,
				"provider":        e.prov.Name(),
				"persona_preset":  in.PersonaPreset,
				"formality":       in.Formality,
				"verbosity":       in.Verbosity,
			},
		)
		defer func() {
			var finalAnswer string
			lastAss := s.LastAssistant()
			if lastAss != nil {
				finalAnswer = lastAss.Content
			}
			ls.EndRun(s.RunID, map[string]any{
				"output":        finalAnswer,
				"total_tokens":  s.TotalTokens,
				"input_tokens":  s.Usage.InputTokens,
				"output_tokens": s.Usage.OutputTokens,
				"steps":         s.Step,
			}, nil)
		}()
	}

	return e.runLoop(ctx, node, s, emit, start)
}

// runLoop chạy vòng lặp dispatch bắt đầu từ startNode — logic dùng chung giữa
// Run() (bắt đầu từ NodeRecall) và Resume() (bắt đầu từ node do route() quyết
// định sau khi caller đã xử lý xong Interrupt — xem resume.go). Tách ra khỏi
// Run() để Resume() không phải chạy lại discovery MCP / LangSmith root run,
// chỉ tiếp tục ĐÚNG state machine từ giữa.
func (e *Engine) runLoop(ctx context.Context, node NodeID, s *State, emit EmitFunc, start time.Time) (provider.Usage, error) {
	for {
		select {
		case <-ctx.Done():
			slog.Warn("engine: cancelled", "step", s.Step)
			return s.Usage, ctx.Err()
		default:
		}

		emit(StepEvent(node))
		stepStart := time.Now()

		// Span OTel THẬT quanh 1 step của engine loop (dispatch 1 node) — con
		// của span run tổng (nếu caller đã mở span cha qua ctx). stepCtx được
		// truyền xuống dispatch nên tool/LLM span bên trong (node_tools.go,
		// node_model.go) trở thành CON của step span này.
		stepCtx, stepSpan := stepTracer.Start(ctx, "step."+string(node), trace.WithAttributes(
			attribute.String("node", string(node)),
			attribute.Int("step", s.Step),
		))

		next, err := e.dispatch(stepCtx, node, s, emit)
		elapsed := time.Since(stepStart)
		if err != nil {
			stepSpan.RecordError(err)
			stepSpan.SetStatus(codes.Error, err.Error())
			stepSpan.End()
			slog.Error("engine: dispatch failed", "node", node, "step", s.Step, "err", err)
			emit(ErrorEvent(err.Error()))
			return s.Usage, fmt.Errorf("engine: dispatch %s: %w", node, err)
		}
		stepSpan.SetAttributes(attribute.String("next_node", string(next)))
		stepSpan.End()

		slog.Info("engine: step done", "node", node, "next", next, "step", s.Step, "elapsed", elapsed.Round(time.Millisecond))

		if next == NodeInterrupt {
			// Lưu State lại (nếu có nơi lưu cấu hình) để client resume sau
			// qua POST /chat/resume — xem SetInterruptStore và resume.go.
			// Trước đây (và vẫn là hành vi mặc định khi KHÔNG cấu hình
			// interruptStore) engine chỉ emit done rồi QUÊN LUÔN state, không
			// có cách nào tiếp tục lượt chạy đã dừng.
			e.saveInterruptedState(s)
			break
		}
		if next == NodeEnd {
			// Run kết thúc TỰ NHIÊN (không qua interrupt) — xoá checkpoint
			// (nếu có) đã tích luỹ qua các bước trước đó, tránh rác trong
			// paused_runs cho những run KHÔNG BAO GIỜ dừng ở NodeInterrupt.
			e.deleteCheckpoint(s)
			break
		}
		// Checkpoint SAU MỖI LẦN CHUYỂN NODE (không chỉ khi dừng ở
		// NodeInterrupt) — cho phép /chat/resume tiếp tục run này nếu tiến
		// trình agent-go crash/restart NGAY SAU bước vừa dispatch xong. state
		// ở đây LUÔN Ở RANH GIỚI SẠCH giữa 2 node (mọi side-effect của node
		// vừa chạy — kể cả AppendObservation của tool call — đã ghi xong vào
		// s trước khi dispatch() trả về), nên route(s) khi resume từ checkpoint
		// này luôn tính đúng node kế tiếp, không bao giờ chạy lại 1 tool đã
		// thực thi xong — xem resume.go và resume_test.go. Rủi ro CÒN LẠI (đã
		// đánh giá, chấp nhận có tài liệu): nếu crash xảy ra NGAY GIỮA lúc
		// dispatch(NodeTools) đang chạy (vd 1 trong nhiều tool call song song
		// đã thực thi xong, tool khác chưa), checkpoint gần nhất vẫn là bản
		// TRƯỚC khi NodeTools bắt đầu (tool call chưa trả lời) — resume sẽ
		// chạy lại NodeTools từ đầu, có thể gọi lại tool ĐÃ chạy thành công ở
		// lần trước đó. Việc này cần idempotency key riêng cho từng tool call
		// để giải quyết triệt để — CHƯA làm trong sprint này (xem báo cáo).
		e.checkpoint(s)
		node = next
	}

	slog.Info("engine: run done", "steps", s.Step, "total_ms", time.Since(start).Milliseconds(),
		"tokens_in", s.Usage.InputTokens, "tokens_out", s.Usage.OutputTokens, "total_tokens", s.TotalTokens)

	// contextTokens: kích thước s.Messages ở CUỐI lượt chạy — đúng bằng những
	// gì client sẽ gửi lại làm history ở lượt kế tiếp (xem comment
	// Event.ContextTokens). Tính riêng ở đây thay vì trong DoneEvent() vì cần
	// s.Messages (đã qua trimContext) SAU KHI dispatch loop kết thúc.
	done := DoneEvent(s.Usage, s.TotalTokens, s.Truncated)
	done.ContextTokens = estimateTokens(s.Messages)
	done.ContextBudget = e.maxContextTokens
	emit(done)
	return s.Usage, nil
}

// dispatch gọi đúng node function dựa trên NodeID.
// Thêm node mới (recall, plan, reflect...) chỉ cần thêm case.
func (e *Engine) dispatch(ctx context.Context, node NodeID, s *State, emit EmitFunc) (NodeID, error) {
	switch node {
	case NodeRecall:
		if e.recallFn != nil {
			return e.recallFn(ctx, s, emit)
		}
		// Fallback: không có recall → vào summarize → plan → model
		return NodeSummarize, nil
	case NodeSummarize:
		if e.summarizeFn != nil {
			return e.summarizeFn(ctx, s, emit)
		}
		return NodePlan, nil
	case NodePlan:
		if e.planFn != nil {
			return e.planFn(ctx, s, emit)
		}
		return NodeModel, nil
	case NodeModel:
		return nodeModel(ctx, e, s, emit)
	case NodeTools:
		// Circuit breaker: detect stuck loops (same tool+args called consecutively).
		// Dùng breaker của LƯỢT CHẠY NÀY (s.loopBreaker), không phải instance
		// chia sẻ toàn process — xem comment trong Engine.Run.
		if s.loopBreaker != nil {
			last := s.LastAssistant()
			if last != nil {
				for _, tc := range last.ToolCalls {
					if err := s.loopBreaker.Record(tc.Name, tc.Args); err != nil {
						emit(ErrorEvent(err.Error()))
						return NodeEnd, nil
					}
				}
			}
		}
		return nodeTools(ctx, e, s, emit)
	case NodeReflect:
		if e.reflectFn != nil {
			return e.reflectFn(ctx, s, emit)
		}
		return NodeExtract, nil
	case NodeExtract:
		if e.extractFn != nil {
			return e.extractFn(ctx, s, emit)
		}
		return NodeEnd, nil
	case NodeInterrupt:
		return NodeEnd, nil
	default:
		return NodeEnd, fmt.Errorf("engine: unknown node %q", node)
	}
}

// Provider returns the engine's LLM provider (for health checks, etc.)
func (e *Engine) Provider() provider.Provider { return e.prov }

// Registry returns the engine's tool registry (for health checks, etc.)
func (e *Engine) Registry() *tools.Registry { return e.registry }
