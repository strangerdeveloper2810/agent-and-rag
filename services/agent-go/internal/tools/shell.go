package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// execBackend là nơi shell.exec THỰC SỰ chạy 1 lệnh. Mặc định (hostBackend)
// chạy trực tiếp trên host qua os/exec — hành vi này giữ NGUYÊN như trước khi
// có sandbox. Backend khác (dockerSandbox, xem shell_sandbox_docker.go) cắm
// vào được mà không đổi Execute()/allowlist/timeout logic bên dưới.
//
// Run trả err THÔ (chưa wrap) để Execute() tự phân biệt lỗi do ctx
// (timeout/cancel) hay lỗi thực thi bình thường (non-zero exit) — giữ đúng
// hành vi cũ hệt trước khi có interface này.
type execBackend interface {
	Run(ctx context.Context, command string, args []string) (stdout, stderr string, exitCode int, err error)
}

// hostBackend chạy lệnh trực tiếp trên host qua os/exec — hành vi MẶC ĐỊNH từ
// trước tới giờ, KHÔNG đổi: vẫn Setpgid + kill cả process group khi
// timeout/cancel (xem comment trong Run).
type hostBackend struct{}

func (hostBackend) Run(ctx context.Context, command string, args []string) (string, string, int, error) {
	cmd := exec.CommandContext(ctx, command, args...)

	// Setpgid gom cả tiến trình con LẪN mọi tiến trình cháu nó tự fork (vd:
	// "sh -c 'sleep 100 &'") vào 1 process group riêng, tách khỏi group của
	// agent. Nếu không có dòng này, exec.CommandContext khi timeout/cancel mặc
	// định chỉ kill đúng tiến trình con trực tiếp — tiến trình cháu SỐNG SÓT vô
	// thời hạn sau khi tool đã báo timeout (resource leak / DoS).
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// Ghi đè Cancel mặc định (chỉ gọi cmd.Process.Kill(), tức kill 1 tiến
	// trình) để kill CẢ process group bằng PID âm — cmd.Process.Pid ở đây
	// chính là pgid vì Setpgid=true ở trên khiến tiến trình con trở thành
	// process group leader (pgid == pid của chính nó).
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.String(), stderr.String(), exitCode(err), err
}

// shellTool executes shell commands via os/exec (hoặc qua backend khác, xem
// execBackend). Kind=KindDestructive because it can modify the system.
type shellTool struct {
	allowedCommands map[string]bool // empty = allow all
	timeout         time.Duration
	backend         execBackend
}

const (
	shellMaxOutput      = 8_000
	defaultShellTimeout = 30 * time.Second
)

// NewShellTool creates a shell execution tool with the default 30s timeout.
// allowedCommands: list of allowed command names. Empty or nil = allow all.
func NewShellTool(allowedCommands []string) Tool {
	return NewShellToolWithTimeout(allowedCommands, defaultShellTimeout)
}

// NewShellToolWithTimeout is like NewShellTool but with a configurable
// timeout (timeout <= 0 falls back to the 30s default). Registry.runOne
// applies this via the TimeoutTool interface — see tool.go.
//
// Backend mặc định LUÔN là hostBackend (chạy thẳng trên host, hành vi cũ) TRỪ
// KHI biến môi trường SHELL_SANDBOX=docker và Docker khả dụng — xem
// resolveDefaultBackend trong shell_sandbox_docker.go. Spawn container là
// thay đổi risk cao nên bắt buộc opt-in qua env, không bao giờ bật ngầm.
func NewShellToolWithTimeout(allowedCommands []string, timeout time.Duration) Tool {
	ac := make(map[string]bool, len(allowedCommands))
	for _, cmd := range allowedCommands {
		ac[cmd] = true
	}
	if timeout <= 0 {
		timeout = defaultShellTimeout
	}
	return &shellTool{allowedCommands: ac, timeout: timeout, backend: resolveDefaultBackend()}
}

// NewShellToolWithSandbox là NewShellToolWithTimeout nhưng cho phép caller ép
// 1 execBackend cụ thể (dùng để test, hoặc 1 call site tương lai muốn chọn
// sandbox mà không phụ thuộc biến môi trường). sandbox == nil rơi về
// hostBackend{}.
func NewShellToolWithSandbox(allowedCommands []string, timeout time.Duration, sandbox execBackend) Tool {
	tool := NewShellToolWithTimeout(allowedCommands, timeout).(*shellTool)
	if sandbox == nil {
		sandbox = hostBackend{}
	}
	tool.backend = sandbox
	return tool
}

func (t *shellTool) Name() string           { return "shell.exec" }
func (t *shellTool) Timeout() time.Duration { return t.timeout }

func (t *shellTool) Description() string {
	return fmt.Sprintf("Thực thi shell command qua os/exec. Timeout %s, output tối đa 8000 ký tự.", t.timeout)
}

func (t *shellTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"command":{"type":"string","description":"shell command to execute"},
			"args":{"type":"array","items":{"type":"string"},"description":"command arguments (optional)"}
		},
		"required":["command"],
		"additionalProperties":false
	}`)
}

func (t *shellTool) Kind() Kind { return KindDestructive }

func (t *shellTool) Execute(ctx context.Context, rawArgs json.RawMessage) (Result, error) {
	var args struct {
		Command string   `json:"command"`
		Args    []string `json:"args,omitempty"`
	}
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return Result{}, fmt.Errorf("shell.exec: invalid args: %w", err)
	}
	if strings.TrimSpace(args.Command) == "" {
		return Result{}, fmt.Errorf("shell.exec: command is required")
	}

	// Check allowed commands
	if len(t.allowedCommands) > 0 && !t.allowedCommands[args.Command] {
		return Result{}, fmt.Errorf("shell.exec: command %q is not allowed", args.Command)
	}

	// Deadline áp qua TimeoutTool ở Registry.runOne (xem tool.go) — ctx ở đây
	// đã mang deadline đó khi tool chạy qua registry. Backend (host hoặc
	// docker) tự chịu trách nhiệm tôn trọng ctx — cả 2 đều dùng
	// exec.CommandContext bên trong.
	backend := t.backend
	if backend == nil {
		backend = hostBackend{} // an toàn nếu shellTool bị khởi tạo tay, bỏ qua constructor
	}
	stdout, stderr, code, err := backend.Run(ctx, args.Command, args.Args)

	// Propagate context errors (deadline exceeded, cancelled)
	if err != nil {
		if ctx.Err() != nil {
			return Result{}, fmt.Errorf("shell.exec: %w", ctx.Err())
		}
	}

	output := stdout
	if len(stderr) > 0 {
		if output != "" {
			output += "\n"
		}
		output += "[stderr]\n" + stderr
	}

	// Truncate
	if len(output) > shellMaxOutput {
		output = output[:shellMaxOutput] + "\n... [truncated]"
	}

	out, _ := json.Marshal(map[string]any{
		"command":  args.Command,
		"args":     args.Args,
		"exitCode": code,
		"output":   output,
	})
	return Result{Content: string(out)}, nil
}

// exitCode extracts exit code from an exec error, returns -1 if not an ExitError.
func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return -1
}
