package tools

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ============================================================================
// Docker sandbox backend cho shell.exec — OPT-IN qua biến môi trường
// SHELL_SANDBOX=docker.
//
// ĐỌC docs/security-model.md (services/agent-go/docs/security-model.md)
// TRƯỚC KHI SỬA file này — tóm tắt trade-off: sandbox này giới hạn
// CPU/memory/network/filesystem cho LỆNH ĐƯỢC CHẠY, chứ KHÔNG cô lập tuyệt
// đối agent-go khỏi host. Để spawn được 1 container "anh em" (sibling
// container), agent-go (chính nó cũng đang chạy trong 1 container ở
// production) phải được mount /var/run/docker.sock từ host vào bên trong.
// Mount đó về bản chất trao quyền tương đương root trên HOST cho bất kỳ ai
// điều khiển được agent-go container (có thể tạo container khác mount "/"
// của host rồi thoát ra). Muốn cô lập THẬT (chống cả agent-go bị compromise)
// cần gVisor/Kata/Firecracker microVM — ngoài phạm vi sprint này.
//
// Vì vậy mặc định KHÔNG bật (backend cũ = hostBackend, chạy thẳng trên host,
// y hệt trước khi có sandbox). Chỉ bật khi operator chủ động set
// SHELL_SANDBOX=docker VÀ đã mount docker.sock theo hướng dẫn trong docs.
// ============================================================================

const (
	// defaultSandboxImage: alpine:3 — nhỏ (~7MB), có busybox nên đã sẵn hầu
	// hết lệnh POSIX cơ bản trong allowlist mặc định (ls, grep, cat, find,
	// pwd, echo, wc, head, tail, diff). RIÊNG "git" KHÔNG có sẵn trong
	// alpine:3 gốc (cần `apk add git` khi tự build image, hoặc dùng image
	// khác có cài sẵn git — nhưng image kiểu "alpine/git" thường CHỈ có git,
	// không có busybox nên các lệnh khác trong allowlist sẽ lỗi "not found").
	// Không cố chọn 1 image "vừa nhỏ vừa đủ mọi lệnh" trong phạm vi sprint
	// này — cho phép override qua SHELL_SANDBOX_IMAGE nếu cần image tự build
	// có đủ cả busybox lẫn git.
	defaultSandboxImage = "alpine:3"

	// defaultDockerSocket là đường dẫn socket mặc định khi không set
	// DOCKER_HOST. Dùng để feature-detect (os.Stat) trước khi gọi
	// `docker version` — tránh treo lâu nếu socket rõ ràng không tồn tại
	// (vd chưa mount /var/run/docker.sock vào container agent-go).
	defaultDockerSocket = "/var/run/docker.sock"

	dockerProbeTimeout = 3 * time.Second
)

// dockerSandbox chạy lệnh trong 1 container Docker "anh em" (sibling
// container), tạo qua gọi CLI `docker run` có sẵn trên host/socket (KHÔNG
// dùng Docker SDK — không cần thiết chỉ để gọi `docker run`) với flags giới
// hạn tối đa: không mạng, không quyền root, filesystem gốc read-only,
// resource limit CPU/memory. Xem buildDockerArgs để biết chi tiết từng flag.
type dockerSandbox struct {
	image   string
	workDir string // bind-mount source (read-only); rỗng = không mount gì
}

// newDockerSandbox tạo dockerSandbox nếu Docker khả dụng (probeDocker OK),
// đọc image override qua SHELL_SANDBOX_IMAGE. Trả lỗi nếu Docker chưa sẵn
// sàng — caller (resolveDefaultBackend) chịu trách nhiệm fallback, hàm này
// KHÔNG panic/crash.
func newDockerSandbox() (execBackend, error) {
	if err := probeDocker(); err != nil {
		return nil, err
	}
	image := os.Getenv("SHELL_SANDBOX_IMAGE")
	if image == "" {
		image = defaultSandboxImage
	}
	return &dockerSandbox{image: image, workDir: sandboxWorkDir()}, nil
}

// probeDocker feature-detect Docker: kiểm tra socket tồn tại (nếu xác định
// được đường dẫn từ DOCKER_HOST/mặc định) rồi gọi `docker version` với
// timeout ngắn. Trả lỗi mô tả nguyên nhân — caller chỉ cần biết "dùng được
// hay không" để quyết định fallback.
func probeDocker() error {
	if sock := dockerSocketPath(); sock != "" {
		if _, err := os.Stat(sock); err != nil {
			return fmt.Errorf("docker socket không truy cập được tại %s: %w", sock, err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), dockerProbeTimeout)
	defer cancel()
	if err := exec.CommandContext(ctx, "docker", "version", "--format", "{{.Server.Version}}").Run(); err != nil {
		return fmt.Errorf("docker CLI không dùng được hoặc daemon không phản hồi: %w", err)
	}
	return nil
}

// dockerSocketPath trả đường dẫn socket cần kiểm tra bằng os.Stat trước khi
// gọi docker CLI. Tôn trọng DOCKER_HOST nếu có dạng unix:// (cho phép test
// trỏ tới 1 path không tồn tại để giả lập "Docker không khả dụng" mà không
// cần daemon thật đang chạy). DOCKER_HOST dạng tcp://... thì bỏ qua bước stat
// này (không phải file) — probeDocker tự phát hiện lỗi qua `docker version`.
func dockerSocketPath() string {
	if h := os.Getenv("DOCKER_HOST"); h != "" {
		if strings.HasPrefix(h, "unix://") {
			return strings.TrimPrefix(h, "unix://")
		}
		return ""
	}
	return defaultDockerSocket
}

// sandboxWorkDir xác định thư mục sẽ bind-mount (chỉ đọc) vào container
// sandbox.
//
// LƯU Ý QUAN TRỌNG (chi tiết trong docs/security-model.md): agent-go tự nó
// ĐANG chạy trong 1 container ở production. Khi gọi `docker run -v
// <path>:<path>` qua socket chia sẻ, <path> được HOST DAEMON diễn giải theo
// filesystem CỦA HOST, không phải filesystem bên trong container agent-go —
// os.Getwd() bên trong agent-go container (vd "/app") gần như chắc chắn
// KHÔNG map đúng sang cùng đường dẫn trên host. Set
// SHELL_SANDBOX_HOST_WORKDIR=<đường dẫn thật trên host> để bind-mount đúng
// chỗ khi chạy theo mô hình socket-sharing. Không set gì → dùng os.Getwd()
// trực tiếp, chỉ ĐÚNG khi agent-go chạy thẳng trên host (không qua container)
// hoặc khi 2 path trùng nhau do cấu hình volume cố ý.
func sandboxWorkDir() string {
	if v := os.Getenv("SHELL_SANDBOX_HOST_WORKDIR"); v != "" {
		return v
	}
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return wd
}

// buildDockerArgs dựng danh sách argument cho `docker run` chạy command bên
// trong sandbox. Hàm THUẦN (pure, không I/O) để test được không cần Docker
// thật — xem shell_sandbox_docker_test.go.
//
// Flags an toàn tối đa (xem docs/security-model.md để biết lý do & giới hạn
// từng flag):
//   - --rm: container tự dọn sau khi chạy xong, không rác lại.
//   - --network=none: KHÔNG mạng — lệnh cần mạng (vd "git fetch") sẽ lỗi, đây
//     là giới hạn CỐ Ý, không cố bật mạng có kiểm soát (ngoài phạm vi).
//   - --memory=256m, --cpus=0.5: chặn 1 lệnh ngốn hết tài nguyên host.
//   - --read-only: filesystem gốc container read-only.
//   - --cap-drop=ALL: bỏ hết Linux capabilities.
//   - --security-opt no-new-privileges: chặn setuid/setgid leo quyền.
//   - --user 1000:1000: không chạy root TRONG container.
//   - -v <workDir>:<workDir>:ro -w <workDir>: mount thư mục làm việc CHỈ
//     ĐỌC. Lệnh cần GHI sẽ thất bại — biết trước, ghi rõ trong docs; mở rộng
//     thêm write-mount phức tạp hơn nằm ngoài phạm vi sprint này.
func buildDockerArgs(image, workDir, command string, args []string) []string {
	dockerArgs := []string{
		"run", "--rm",
		"--network=none",
		"--memory=256m",
		"--cpus=0.5",
		"--read-only",
		"--cap-drop=ALL",
		"--security-opt", "no-new-privileges",
		"--user", "1000:1000",
	}
	if workDir != "" {
		dockerArgs = append(dockerArgs, "-v", workDir+":"+workDir+":ro", "-w", workDir)
	}
	dockerArgs = append(dockerArgs, image, command)
	dockerArgs = append(dockerArgs, args...)
	return dockerArgs
}

// Run thực thi command bên trong container sandbox. ctx bọc quanh chính tiến
// trình `docker run` (client CLI) qua exec.CommandContext — deadline/cancel
// tự động giết tiến trình client này, giống hostBackend.
//
// LƯU Ý: kill tiến trình client `docker run` không đảm bảo daemon dừng NGAY
// container tương ứng (client và daemon là 2 tiến trình tách biệt) — với
// --rm, daemon vẫn dọn container khi nó tự thoát, nhưng nếu lệnh bên trong
// "đứng" sau khi client đã bị kill, container có thể sống thêm 1 khoảng ngắn
// trước khi bị Docker dọn. Chấp nhận trade-off này trong phạm vi sprint —
// không tự `docker kill` theo container ID vì cần đặt tên/track ID, tăng
// đáng kể độ phức tạp so với lợi ích biên (đã có timeout ngắn, default 30s).
func (d *dockerSandbox) Run(ctx context.Context, command string, args []string) (string, string, int, error) {
	dockerArgs := buildDockerArgs(d.image, d.workDir, command, args)
	cmd := exec.CommandContext(ctx, "docker", dockerArgs...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.String(), stderr.String(), exitCode(err), err
}

// resolveDefaultBackend chọn execBackend mặc định cho shellTool. Gọi lại mỗi
// lần NewShellTool*/tạo tool — buildRegistries (cmd/server/main.go) gọi vài
// lần lúc khởi động (code registry + general registry); probe Docker khi đó
// chạy lại vài lần nhưng chỉ ở startup (không phải hot path per-request),
// nên không cache — giữ hàm đơn giản, dễ test (mỗi test set env riêng rồi
// gọi thẳng, không lo state global rò rỉ giữa các test).
//
// QUYẾT ĐỊNH THIẾT KẾ: đọc trực tiếp os.Getenv("SHELL_SANDBOX") ở đây thay vì
// thêm field vào config.Config: internal/config/config.go và
// cmd/server/main.go đang được 2 nhánh khác cùng sửa song song (Telegram
// transport + resume/state) trong cùng đợt làm việc này — sửa 2 file đó nằm
// ngoài phạm vi được giao cho nhánh này (tránh conflict/dẫm chân). Đọc env
// trực tiếp trong package tools giữ toàn bộ logic chọn backend tự chứa
// (self-contained): main.go tiếp tục gọi NewShellTool/NewShellToolWithTimeout
// y hệt trước giờ, không cần biết gì về sandbox, không cần đổi wiring.
//
// SHELL_SANDBOX=docker: thử dùng dockerSandbox — nếu probe thất bại (không
// có Docker CLI, socket chưa mount, daemon không phản hồi...), log rõ ràng
// rồi fallback về hostBackend — KHÔNG panic, KHÔNG chặn hẳn shell.exec. Bất
// kỳ giá trị nào khác (rỗng, "none", giá trị lạ...) đều dùng hostBackend —
// hành vi mặc định trước khi có sandbox, giữ nguyên để thay đổi này là
// opt-in thuần tuý, an toàn khi rollout.
func resolveDefaultBackend() execBackend {
	if os.Getenv("SHELL_SANDBOX") != "docker" {
		return hostBackend{}
	}
	sandbox, err := newDockerSandbox()
	if err != nil {
		slog.Warn("shell.exec: SHELL_SANDBOX=docker nhưng Docker không khả dụng, fallback chạy trực tiếp trên host", "err", err)
		return hostBackend{}
	}
	slog.Info("shell.exec: dùng Docker sandbox cho lệnh shell", "image", sandbox.(*dockerSandbox).image)
	return sandbox
}
