package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/ai-agent-tut/agent-go/internal/provider"
	"github.com/ai-agent-tut/agent-go/internal/tools"
)

// server.go implement JARVIS Ở VAI TRÒ MCP SERVER — chiều NGƯỢC LẠI với
// discovery.go/sse.go (JARVIS làm CLIENT, kết nối RA NGOÀI). File này expose
// tool CỦA JARVIS cho 1 MCP client khác (Claude Desktop, IDE, hay 1 JARVIS
// instance khác) gọi VÀO qua HTTP.
//
// Transport: "Streamable HTTP" (spec modelcontextprotocol.io/specification/
// 2025-06-18/basic/transports — bản thay thế chính thức cho "HTTP+SSE" cũ của
// 2024-11-05). Server này CHỈ cài đặt nhánh đơn giản nhất mà spec cho phép:
// mỗi JSON-RPC request POST tới endpoint nhận về ĐÚNG MỘT response JSON
// (Content-Type: application/json), KHÔNG mở SSE stream — spec nói rõ server
// "MUST either return Content-Type: text/event-stream ... or application/
// json" và client "MUST support both". JARVIS không cần gửi request/
// notification chủ động từ server → client (không có tiến trình nền nào cần
// đẩy dữ liệu), nên nhánh JSON đơn giản là đủ và không cần quản lý SSE
// session/resumability. GET/DELETE lên endpoint không được hỗ trợ (server
// không mở stream nhận từ client) — trả 405, đúng như spec cho phép
// ("... or else return HTTP 405 Method Not Allowed").
//
// BẢO MẬT — ĐỌC TRƯỚC KHI SỬA:
// Registry truyền vào đây là 1 ĐƯỜNG VÀO MỚI, hoàn toàn KHÔNG đi qua
// internal/agent/node_tools.go (nơi chặn tools.IsPrivilegedTool theo
// tools.IsOwnerTenant cho kênh /chat bình thường). Vì endpoint MCP KHÔNG có
// khái niệm "owner tenant" nào (bất kỳ ai gọi được HTTP tới server đều bình
// đẳng — hiện CHƯA có lớp auth riêng), nguyên tắc ở đây PHẢI nghiêm hơn kênh
// chat: hard-exclude TUYỆT ĐỐI tools.IsPrivilegedTool (shell.exec, file.*,
// git) khỏi CẢ tools/list lẫn tools/call — KHÔNG có ngoại lệ, KHÔNG có cấu
// hình nào bật lại được, kể cả filter caller truyền vào (xem allowed()).
// Nếu tương lai cần cho phép privileged tool qua MCP, đó là 1 quyết định bảo
// mật lớn cần thiết kế auth riêng — KHÔNG được nới lỏng ở đây.
const (
	// mcpServerProtocolVersion là version JARVIS server "thích" nói nhất khi
	// client không tự khai báo protocolVersion trong initialize. Server KHÔNG
	// làm gì khác nhau tuỳ version (không có logic gated theo version) nên khi
	// client CÓ khai báo, ta echo lại đúng version đó để tối đa tương thích
	// (xem handleInitialize) — tương tự tinh thần cố tình không strict-check
	// version phía CLIENT trong sse.go (mcpProtocolVersion comment).
	mcpServerProtocolVersion = "2025-06-18"
	mcpServerName            = "jarvis-go"
	mcpServerVersion         = "0.1.0"

	// maxMCPRequestBody giới hạn kích thước body JSON-RPC nhận vào — phòng
	// request khổng lồ làm tốn RAM trước khi kịp validate bất cứ gì. 1MB rộng
	// rãi cho mọi tool hiện có (không tool nào nhận input lớn hơn vài KB).
	maxMCPRequestBody = 1 << 20
)

// Mã lỗi JSON-RPC 2.0 CHUẨN (không riêng gì MCP) — xem
// https://www.jsonrpc.org/specification#error_object.
const (
	jsonRPCParseError     = -32700
	jsonRPCMethodNotFound = -32601
	jsonRPCInvalidParams  = -32602
)

// rpcServerRequest là request/notification JSON-RPC 2.0 SERVER nhận vào.
//
// CỐ TÌNH khác jsonRPCRequest (discovery.go) — struct đó ID kiểu int64 vì chỉ
// dùng khi JARVIS ĐÓNG VAI CLIENT tự sinh id tăng dần. Ở vai SERVER, id do
// client khác gửi tới có thể là string hoặc number (JSON-RPC 2.0 cho phép cả
// hai), và notification hợp lệ thì KHÔNG CÓ field "id" luôn — dùng
// json.RawMessage để giữ nguyên byte gốc, echo lại chính xác trong response
// thay vì ép kiểu sai làm hỏng correlation phía client.
type rpcServerRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

// isNotification: JSON-RPC notification không có field "id". Coi id=null
// cũng là notification (một số client gửi null cho message không cần phản
// hồi) — an toàn hơn là cố trả response cho 1 request client không đợi.
func (r rpcServerRequest) isNotification() bool {
	return len(r.ID) == 0 || string(r.ID) == "null"
}

// rpcServerResponse là response JSON-RPC 2.0 SERVER trả về.
// ID không có "omitempty": json.RawMessage(nil) tự MarshalJSON thành "null"
// (theo encoding/json), và JSON-RPC 2.0 quy ước response luôn có field "id"
// (kể cả null khi server không xác định được id gốc, ví dụ lỗi parse).
type rpcServerResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *rpcServerError `json:"error,omitempty"`
}

type rpcServerError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// dispatchError là lỗi JSON-RPC "protocol-level" (khác lỗi thực thi tool —
// spec MCP phân biệt rõ 2 loại, xem handleToolsCall).
type dispatchError struct {
	Code    int
	Message string
}

// Server implement http.Handler, nói JSON-RPC 2.0 qua Streamable HTTP (nhánh
// JSON-only, xem comment đầu file) để expose tool CỦA JARVIS cho MCP client
// bên ngoài gọi vào.
type Server struct {
	registry *tools.Registry
	filter   func(name string) bool
}

// NewServer tạo MCP server bọc quanh registry.
//
// filter (tuỳ chọn, có thể nil) cho phép caller (cmd/server/main.go) tự thu
// hẹp whitelist hơn nữa — ví dụ chỉ muốn expose 3/8 tool của registry truyền
// vào. filter == nil nghĩa là "không lọc thêm, giữ nguyên mọi tool không đặc
// quyền trong registry".
//
// DÙ filter TRẢ VỀ GÌ, Server LUÔN tự áp thêm lớp lọc cứng loại bỏ
// tools.IsPrivilegedTool — đây là defense-in-depth CỐ Ý: không tin caller
// tuyệt đối, vì hậu quả của 1 chỗ quên chặn là lộ shell.exec/file.* ra ngoài
// Internet không auth.
func NewServer(registry *tools.Registry, filter func(name string) bool) *Server {
	return &Server{registry: registry, filter: filter}
}

// allowed quyết định 1 tool tên `name` có được liệt kê/gọi qua MCP server này
// hay không. Đây là ĐIỂM DUY NHẤT quyết định exposure — mọi đường (tools/list,
// tools/call) đều phải đi qua đây, không có đường tắt nào khác.
func (s *Server) allowed(name string) bool {
	if tools.IsPrivilegedTool(name) {
		return false
	}
	if s.filter != nil && !s.filter(name) {
		return false
	}
	return true
}

// allowedToolDefs trả ToolDef đã lọc, giữ nguyên thứ tự đăng ký gốc của registry.
func (s *Server) allowedToolDefs() []provider.ToolDef {
	if s.registry == nil {
		return nil
	}
	all := s.registry.ToolDefs()
	out := make([]provider.ToolDef, 0, len(all))
	for _, d := range all {
		if s.allowed(d.Name) {
			out = append(out, d)
		}
	}
	return out
}

// ServeHTTP implement http.Handler — điểm vào duy nhất của MCP endpoint.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		// Theo spec Streamable HTTP: GET/DELETE chỉ hợp lệ nếu server hỗ trợ
		// server-initiated SSE stream hoặc session termination — server này
		// không làm cả hai, nên trả 405 (chính là điều spec cho phép trong
		// trường hợp đó).
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "MCP endpoint chỉ hỗ trợ POST (server không mở SSE stream)", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxMCPRequestBody))
	if err != nil {
		s.writeError(w, http.StatusBadRequest, json.RawMessage("null"), jsonRPCParseError, "read request body: "+err.Error())
		return
	}

	var req rpcServerRequest
	if err := json.Unmarshal(body, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, json.RawMessage("null"), jsonRPCParseError, "parse error: "+err.Error())
		return
	}

	if req.isNotification() {
		// Spec: input là notification mà server chấp nhận được → 202
		// Accepted, KHÔNG có body (không có id để trả lời).
		w.WriteHeader(http.StatusAccepted)
		return
	}

	result, dErr := s.dispatch(r.Context(), req.Method, req.Params)
	if dErr != nil {
		s.writeError(w, http.StatusOK, req.ID, dErr.Code, dErr.Message)
		return
	}
	s.writeResult(w, req.ID, result)
}

// dispatch định tuyến theo method JSON-RPC. Method lạ → -32601 Method not
// found (đúng chuẩn JSON-RPC 2.0, không riêng MCP).
func (s *Server) dispatch(ctx context.Context, method string, params json.RawMessage) (interface{}, *dispatchError) {
	switch method {
	case "initialize":
		return s.handleInitialize(params), nil
	case "tools/list":
		return s.handleToolsList(), nil
	case "tools/call":
		return s.handleToolsCall(ctx, params)
	default:
		return nil, &dispatchError{Code: jsonRPCMethodNotFound, Message: "Method not found: " + method}
	}
}

type initializeParams struct {
	ProtocolVersion string `json:"protocolVersion"`
}

// handleInitialize trả InitializeResult theo spec lifecycle "initialize":
// protocolVersion + capabilities + serverInfo. JARVIS chỉ khai báo capability
// "tools" (listChanged=false — registry cố định lúc khởi động, không đổi
// giữa 2 lần tools/list của cùng 1 tiến trình server), KHÔNG khai báo
// resources/prompts/logging vì không cài đặt.
func (s *Server) handleInitialize(params json.RawMessage) rpcInitializeResult {
	version := mcpServerProtocolVersion
	var p initializeParams
	if len(params) > 0 {
		if err := json.Unmarshal(params, &p); err == nil && p.ProtocolVersion != "" {
			// Echo lại version client yêu cầu: server không có logic gì khác
			// nhau tuỳ version nên chấp nhận bất kỳ version nào client đưa ra
			// là an toàn, và tối đa hoá khả năng tương thích với client thật
			// (cùng tinh thần comment mcpProtocolVersion trong sse.go).
			version = p.ProtocolVersion
		}
	}
	return rpcInitializeResult{
		ProtocolVersion: version,
		Capabilities: rpcServerCapabilities{
			Tools: &rpcToolsCapability{ListChanged: false},
		},
		ServerInfo: rpcServerInfo{Name: mcpServerName, Version: mcpServerVersion},
		Instructions: "JARVIS MCP server chỉ expose tool KHÔNG đặc quyền " +
			"(không có shell.exec/file.*/git) — xem docs/security-model.md.",
	}
}

type rpcInitializeResult struct {
	ProtocolVersion string                `json:"protocolVersion"`
	Capabilities    rpcServerCapabilities `json:"capabilities"`
	ServerInfo      rpcServerInfo         `json:"serverInfo"`
	Instructions    string                `json:"instructions,omitempty"`
}

type rpcServerCapabilities struct {
	Tools *rpcToolsCapability `json:"tools,omitempty"`
}

type rpcToolsCapability struct {
	ListChanged bool `json:"listChanged"`
}

type rpcServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// handleToolsList map ToolDef (đã lọc qua allowed()) sang định dạng MCP tool
// schema (name/description/inputSchema) — tái dùng type mcpToolDef/
// listToolsResult đã có sẵn trong discovery.go (đúng shape spec, JARVIS đã
// dùng để PARSE response khi làm client, giờ dùng lại để SINH response khi
// làm server).
func (s *Server) handleToolsList() listToolsResult {
	defs := s.allowedToolDefs()
	list := make([]mcpToolDef, 0, len(defs))
	for _, d := range defs {
		schema := d.Schema
		if len(schema) == 0 {
			schema = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		list = append(list, mcpToolDef{Name: d.Name, Description: d.Description, InputSchema: schema})
	}
	return listToolsResult{Tools: list}
}

// handleToolsCall gọi tool qua registry.RunParallel (đường công khai duy nhất
// Registry cho phép chạy 1 tool_call — không có "RunOne" export riêng, dùng
// slice 1 phần tử là đủ và không cần thêm API mới lên Registry).
//
// LƯU Ý BẢO MẬT: allowed(p.Name) được kiểm tra TRƯỚC KHI tra registry — dù
// registry.Get thực ra không tìm thấy tool bị lọc (nó vẫn NẰM TRONG registry,
// chỉ không được liệt kê/gọi qua Server), nên guard này là bắt buộc, không
// phải optimization. Không phân biệt "tool không tồn tại" với "tool bị chặn
// vì đặc quyền" trong message lỗi — cùng chung "Unknown tool" như ví dụ spec
// dùng cho tools/call lỗi, tránh lộ registry nội bộ có gì.
func (s *Server) handleToolsCall(ctx context.Context, params json.RawMessage) (interface{}, *dispatchError) {
	var p callToolParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &dispatchError{Code: jsonRPCInvalidParams, Message: "Invalid params: " + err.Error()}
	}

	if !s.allowed(p.Name) || s.registry == nil {
		return nil, &dispatchError{Code: jsonRPCInvalidParams, Message: "Unknown tool: " + p.Name}
	}
	if _, ok := s.registry.Get(p.Name); !ok {
		return nil, &dispatchError{Code: jsonRPCInvalidParams, Message: "Unknown tool: " + p.Name}
	}

	results := s.registry.RunParallel(ctx, []provider.ToolCall{{Name: p.Name, Args: p.Arguments}})
	res := results[0]
	if res.Err != nil {
		// "Tool Execution Error" theo spec (khác protocol error): trả result
		// bình thường với isError=true, KHÔNG phải JSON-RPC error — để phía
		// client/LLM tự đọc message lỗi như 1 tool result.
		return callToolResult{
			Content: []mcpContent{{Type: "text", Text: res.Err.Error()}},
			IsError: true,
		}, nil
	}

	return callToolResult{
		Content: []mcpContent{{Type: "text", Text: res.Result.Content}},
		IsError: false,
	}, nil
}

func (s *Server) writeResult(w http.ResponseWriter, id json.RawMessage, result interface{}) {
	s.writeJSON(w, http.StatusOK, rpcServerResponse{JSONRPC: "2.0", ID: id, Result: result})
}

func (s *Server) writeError(w http.ResponseWriter, status int, id json.RawMessage, code int, message string) {
	s.writeJSON(w, status, rpcServerResponse{JSONRPC: "2.0", ID: id, Error: &rpcServerError{Code: code, Message: message}})
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
