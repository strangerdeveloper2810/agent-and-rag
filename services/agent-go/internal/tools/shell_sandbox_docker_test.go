package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// assertArgsEqual so sánh 2 slice string theo thứ tự — dùng cho
// buildDockerArgs vì THỨ TỰ flags cũng quan trọng (vd -v/-w phải nằm sau
// flags an toàn nhưng trước image, image/command/args phải theo đúng thứ tự
// docker CLI mong đợi).
func assertArgsEqual(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("length mismatch: got %v (len=%d), want %v (len=%d)", got, len(got), want, len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("index %d: got %q, want %q\nfull got=%v\nfull want=%v", i, got[i], want[i], got, want)
		}
	}
}

func TestBuildDockerArgs_WithWorkDir(t *testing.T) {
	got := buildDockerArgs("alpine:3", "/work", "ls", []string{"-la"})
	want := []string{
		"run", "--rm",
		"--network=none",
		"--memory=256m",
		"--cpus=0.5",
		"--read-only",
		"--cap-drop=ALL",
		"--security-opt", "no-new-privileges",
		"--user", "1000:1000",
		"-v", "/work:/work:ro",
		"-w", "/work",
		"alpine:3", "ls", "-la",
	}
	assertArgsEqual(t, got, want)
}

func TestBuildDockerArgs_WithoutWorkDir(t *testing.T) {
	got := buildDockerArgs("alpine:3", "", "echo", []string{"hi"})
	want := []string{
		"run", "--rm",
		"--network=none",
		"--memory=256m",
		"--cpus=0.5",
		"--read-only",
		"--cap-drop=ALL",
		"--security-opt", "no-new-privileges",
		"--user", "1000:1000",
		"alpine:3", "echo", "hi",
	}
	assertArgsEqual(t, got, want)
}

func TestBuildDockerArgs_NoArgsCommand(t *testing.T) {
	got := buildDockerArgs("alpine:3", "", "pwd", nil)
	want := []string{
		"run", "--rm",
		"--network=none",
		"--memory=256m",
		"--cpus=0.5",
		"--read-only",
		"--cap-drop=ALL",
		"--security-opt", "no-new-privileges",
		"--user", "1000:1000",
		"alpine:3", "pwd",
	}
	assertArgsEqual(t, got, want)
}

// TestBuildDockerArgs_RequiredSafetyFlags khoá lại rằng KHÔNG ai vô tình xoá
// mất 1 trong các flag an toàn cốt lõi (network/memory/cpu/readonly/cap-drop/
// no-new-privileges/user non-root) khi sửa buildDockerArgs sau này.
func TestBuildDockerArgs_RequiredSafetyFlags(t *testing.T) {
	got := buildDockerArgs("alpine:3", "", "id", nil)
	required := []string{
		"--network=none",
		"--memory=256m",
		"--cpus=0.5",
		"--read-only",
		"--cap-drop=ALL",
		"--security-opt",
		"no-new-privileges",
		"--user",
		"1000:1000",
	}
	for _, flag := range required {
		found := false
		for _, a := range got {
			if a == flag {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing required safety flag %q in args=%v", flag, got)
		}
	}
}

func TestDockerSandboxImplementsExecBackend(t *testing.T) {
	var _ execBackend = (*dockerSandbox)(nil)
}

func TestDockerSocketPath(t *testing.T) {
	t.Run("no DOCKER_HOST uses default socket", func(t *testing.T) {
		t.Setenv("DOCKER_HOST", "")
		if got := dockerSocketPath(); got != defaultDockerSocket {
			t.Errorf("got %q, want %q", got, defaultDockerSocket)
		}
	})

	t.Run("unix:// DOCKER_HOST strips scheme", func(t *testing.T) {
		t.Setenv("DOCKER_HOST", "unix:///tmp/does-not-exist.sock")
		if got := dockerSocketPath(); got != "/tmp/does-not-exist.sock" {
			t.Errorf("got %q, want /tmp/does-not-exist.sock", got)
		}
	})

	t.Run("tcp:// DOCKER_HOST skips file check", func(t *testing.T) {
		t.Setenv("DOCKER_HOST", "tcp://127.0.0.1:2375")
		if got := dockerSocketPath(); got != "" {
			t.Errorf("got %q, want empty (not a unix socket path)", got)
		}
	})
}

// TestProbeDocker_FailsWhenSocketMissing giả lập "Docker không khả dụng"
// bằng cách trỏ DOCKER_HOST tới 1 socket path không tồn tại trên đĩa — KHÔNG
// phụ thuộc việc môi trường chạy test có cài Docker daemon thật hay không,
// vì os.Stat thất bại trước khi probeDocker kịp gọi lệnh `docker` CLI.
func TestProbeDocker_FailsWhenSocketMissing(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "nonexistent.sock")
	t.Setenv("DOCKER_HOST", "unix://"+sock)

	if err := probeDocker(); err == nil {
		t.Fatal("expected error khi docker socket không tồn tại, got nil")
	}
}

// TestNewDockerSandbox_FallbackWhenUnavailable: newDockerSandbox phải trả về
// lỗi (không panic) khi Docker không khả dụng, để caller fallback được.
func TestNewDockerSandbox_FallbackWhenUnavailable(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "nonexistent.sock")
	t.Setenv("DOCKER_HOST", "unix://"+sock)

	sandbox, err := newDockerSandbox()
	if err == nil {
		t.Fatal("expected error khi docker socket không tồn tại, got nil")
	}
	if sandbox != nil {
		t.Errorf("expected nil sandbox on error, got %v", sandbox)
	}
}

// TestResolveDefaultBackend_DefaultIsHost: SHELL_SANDBOX không set (hoặc
// rỗng) → luôn hostBackend, không đụng gì tới Docker cả (không probe).
func TestResolveDefaultBackend_DefaultIsHost(t *testing.T) {
	t.Setenv("SHELL_SANDBOX", "")
	backend := resolveDefaultBackend()
	if _, ok := backend.(hostBackend); !ok {
		t.Fatalf("expected hostBackend khi SHELL_SANDBOX unset, got %T", backend)
	}
}

func TestResolveDefaultBackend_UnknownValueIsHost(t *testing.T) {
	t.Setenv("SHELL_SANDBOX", "some-typo-value")
	backend := resolveDefaultBackend()
	if _, ok := backend.(hostBackend); !ok {
		t.Fatalf("expected hostBackend cho giá trị lạ, got %T", backend)
	}
}

// TestResolveDefaultBackend_FallsBackToHostWhenDockerUnavailable là test
// fallback CHÍNH yêu cầu bởi task: SHELL_SANDBOX=docker nhưng Docker không
// khả dụng (giả lập bằng DOCKER_HOST trỏ tới socket không tồn tại) →
// resolveDefaultBackend KHÔNG lỗi/panic, rơi về hostBackend, và backend đó
// vẫn thực thi lệnh bình thường (không chặn đường chính shell.exec).
func TestResolveDefaultBackend_FallsBackToHostWhenDockerUnavailable(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "nonexistent.sock")
	t.Setenv("SHELL_SANDBOX", "docker")
	t.Setenv("DOCKER_HOST", "unix://"+sock)

	backend := resolveDefaultBackend()
	if _, ok := backend.(hostBackend); !ok {
		t.Fatalf("expected fallback hostBackend, got %T", backend)
	}

	stdout, _, code, err := backend.Run(context.Background(), "echo", []string{"ok"})
	if err != nil {
		t.Fatalf("unexpected error chạy lệnh qua fallback backend: %v", err)
	}
	if code != 0 {
		t.Fatalf("exitCode = %d, want 0", code)
	}
	if !strings.Contains(stdout, "ok") {
		t.Fatalf("stdout = %q, want chứa 'ok'", stdout)
	}
}

// TestNewShellToolWithTimeout_RespectsShellSandboxFallback là test tích hợp
// nhẹ: dựng shellTool thật qua constructor production dùng khi
// SHELL_SANDBOX=docker + Docker không khả dụng, xác nhận tool vẫn chạy được
// lệnh bình thường (không lỗi ở tầng Execute do sandbox chặn).
func TestNewShellToolWithTimeout_RespectsShellSandboxFallback(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "nonexistent.sock")
	t.Setenv("SHELL_SANDBOX", "docker")
	t.Setenv("DOCKER_HOST", "unix://"+sock)

	tool := NewShellToolWithTimeout([]string{"echo"}, 0)
	args := []byte(`{"command":"echo","args":["from-fallback"]}`)
	res, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.Content, "from-fallback") {
		t.Errorf("expected output to contain 'from-fallback', got %q", res.Content)
	}
}

func TestSandboxWorkDir_EnvOverride(t *testing.T) {
	t.Setenv("SHELL_SANDBOX_HOST_WORKDIR", "/custom/host/path")
	if got := sandboxWorkDir(); got != "/custom/host/path" {
		t.Errorf("got %q, want /custom/host/path", got)
	}
}

func TestSandboxWorkDir_DefaultsToGetwd(t *testing.T) {
	t.Setenv("SHELL_SANDBOX_HOST_WORKDIR", "")
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	if got := sandboxWorkDir(); got != wd {
		t.Errorf("got %q, want os.Getwd() %q", got, wd)
	}
}

// TestNewShellToolWithSandbox_InjectedBackendUsed xác nhận
// NewShellToolWithSandbox thực sự dùng backend được truyền vào (không âm
// thầm rơi về hostBackend) — dùng 1 fake backend đếm số lần gọi.
func TestNewShellToolWithSandbox_InjectedBackendUsed(t *testing.T) {
	fake := &countingBackend{}
	tool := NewShellToolWithSandbox(nil, 0, fake)

	args := []byte(`{"command":"whatever"}`)
	if _, err := tool.Execute(context.Background(), args); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.calls != 1 {
		t.Fatalf("expected injected backend to be called once, got %d calls", fake.calls)
	}
}

func TestNewShellToolWithSandbox_NilFallsBackToHost(t *testing.T) {
	tool := NewShellToolWithSandbox([]string{"echo"}, 0, nil).(*shellTool)
	if _, ok := tool.backend.(hostBackend); !ok {
		t.Fatalf("expected hostBackend khi sandbox=nil, got %T", tool.backend)
	}
}

type countingBackend struct {
	calls int
}

func (c *countingBackend) Run(ctx context.Context, command string, args []string) (string, string, int, error) {
	c.calls++
	return "fake-output", "", 0, nil
}
