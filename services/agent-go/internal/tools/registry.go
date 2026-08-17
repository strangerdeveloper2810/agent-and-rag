package tools

import (
	"context"
	"fmt"
	"sync"

	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// Registry là nơi đăng ký & tra cứu Tool theo tên. Nó cũng sinh []provider.ToolDef
// để nạp cho LLM, và fan-out nhiều tool_call chạy song song.
type Registry struct {
	tools map[string]Tool
	order []string // giữ thứ tự đăng ký cho All()/ToolDefs() ổn định
}

// NewRegistry tạo một Registry rỗng.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register thêm (hoặc ghi đè) một tool theo Name(). Ghi đè không thêm lại vào order.
func (r *Registry) Register(t Tool) {
	name := t.Name()
	if _, exists := r.tools[name]; !exists {
		r.order = append(r.order, name)
	}
	r.tools[name] = t
}

// Get trả về tool theo tên; ok=false nếu không có.
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// All trả về mọi tool đã đăng ký theo thứ tự đăng ký.
func (r *Registry) All() []Tool {
	out := make([]Tool, 0, len(r.order))
	for _, name := range r.order {
		out = append(out, r.tools[name])
	}
	return out
}

// ToolDefs map mỗi tool sang provider.ToolDef để nạp cho LLM.
func (r *Registry) ToolDefs() []provider.ToolDef {
	defs := make([]provider.ToolDef, 0, len(r.order))
	for _, name := range r.order {
		t := r.tools[name]
		defs = append(defs, provider.ToolDef{
			Name:        t.Name(),
			Description: t.Description(),
			Schema:      t.Schema(),
		})
	}
	return defs
}

// CallResult là kết quả của một tool_call. Err != nil khi tool không tồn tại
// hoặc Execute lỗi; khi đó Result là zero-value.
type CallResult struct {
	Call   provider.ToolCall
	Result Result
	Err    error
}

// RunParallel chạy các tool_call SONG SONG và trả về kết quả ĐÚNG THỨ TỰ đầu vào.
// Tool không tìm thấy hoặc Execute lỗi được gán vào CallResult.Err (KHÔNG panic).
// ctx được truyền vào từng Execute để tôn trọng cancel/timeout.
func (r *Registry) RunParallel(ctx context.Context, calls []provider.ToolCall) []CallResult {
	return r.RunParallelStreaming(ctx, calls, nil)
}

// RunParallelStreaming chạy các tool_call SONG SONG như RunParallel, nhưng gọi
// onResult(index, result) NGAY KHI từng tool hoàn thành (không theo thứ tự)
// để caller có thể stream kết quả sớm thay vì chờ tất cả. Trả về kết quả
// ĐÚNG THỨ TỰ đầu vào.
//
// onResult chạy tuần tự trên goroutine gọi hàm này (an toàn cho emit SSE).
func (r *Registry) RunParallelStreaming(ctx context.Context, calls []provider.ToolCall, onResult func(index int, res CallResult)) []CallResult {
	results := make([]CallResult, len(calls))
	for i, call := range calls {
		results[i].Call = call
	}

	done := make(chan int, len(calls))
	var wg sync.WaitGroup
	for i, call := range calls {
		wg.Add(1)
		go func(i int, call provider.ToolCall) {
			defer wg.Done()
			res, err := r.runOne(ctx, call)
			results[i].Result = res
			results[i].Err = err
			done <- i
		}(i, call)
	}

	// Đóng done khi mọi goroutine đã gửi xong → vòng range dưới kết thúc.
	go func() {
		wg.Wait()
		close(done)
	}()

	for i := range done {
		if onResult != nil {
			onResult(i, results[i])
		}
	}
	return results
}

// runOne tra cứu & thực thi một tool_call, gói mọi lỗi vào giá trị trả về.
// Nếu tool thoả TimeoutTool, ctx được bọc deadline riêng (cooperative — xem
// doc TimeoutTool) trước khi gọi Execute.
func (r *Registry) runOne(ctx context.Context, call provider.ToolCall) (Result, error) {
	t, ok := r.Get(call.Name)
	if !ok {
		return Result{}, &NotFoundError{Name: call.Name}
	}

	var ownDeadline bool
	if tt, ok := t.(TimeoutTool); ok {
		if d := tt.Timeout(); d > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, d)
			defer cancel()
			ownDeadline = true
		}
	}

	res, err := t.Execute(ctx, call.Args)
	// Chỉ gói TimeoutError khi CHÍNH deadline vừa tạo ở trên là nguyên nhân —
	// nếu ctx gốc (caller) tự huỷ, giữ nguyên lỗi gốc, đừng gán nhầm cho
	// TimeoutTool của tool này.
	if err != nil && ownDeadline && ctx.Err() == context.DeadlineExceeded {
		return Result{}, &TimeoutError{Name: call.Name, Cause: err}
	}
	return res, err
}

// TimeoutError báo tool_call bị huỷ vì chạm deadline khai báo qua TimeoutTool.
type TimeoutError struct {
	Name  string
	Cause error
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("tools: %q timed out: %v", e.Name, e.Cause)
}

func (e *TimeoutError) Unwrap() error { return e.Cause }

// NotFoundError báo tool_call trỏ tới tool chưa được đăng ký.
type NotFoundError struct {
	Name string
}

func (e *NotFoundError) Error() string {
	return "tools: tool not found: " + e.Name
}
