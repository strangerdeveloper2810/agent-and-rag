# Security Model — J.A.R.V.I.S. Go Agent

> Tài liệu này mô tả mô hình tin cậy hiện tại của agent-go, và giải thích
> TRUNG THỰC về sandbox Docker opt-in cho `shell.exec` (`SHELL_SANDBOX=docker`)
> — nó bảo vệ được gì, KHÔNG bảo vệ được gì, và trade-off khi bật nó.

---

## 1. Mô hình tin cậy hiện tại

### 1.1 Tool đặc quyền (privileged tools) — chỉ tenant chủ hệ thống

`internal/tools/privileged.go` định nghĩa 1 nhóm tool tác động lên **máy chạy
agent** (không phải dữ liệu riêng của người dùng), nên KHÔNG được scope theo
tenant như các tool khác:

- `file.read` / `file.search` — đọc filesystem của server (`AllowedPaths`
  mặc định `[".", $HOME]`).
- `file.write` — ghi file lên server.
- `shell.exec` — chạy lệnh tuỳ ý trên server.
- `git` — đọc/thao tác repo trên server.

`IsOwnerTenant` quyết định tenant nào được coi là "chủ": **fail-closed có chủ
đích** — nếu `OWNER_TENANT_IDS` chưa cấu hình, CHỈ tenant `default` (chạy local
không qua auth) được coi là chủ; mọi tenant thật khác mặc định KHÔNG có đặc
quyền. `StripPrivilegedTools` loại hẳn các tool này khỏi danh sách gửi cho LLM
đối với tenant không phải chủ — model còn không biết các tool này tồn tại.

Lý do chọn fail-closed: hậu quả của việc quên cấu hình `OWNER_TENANT_IDS` là
cho người lạ quyền chạy shell trên server; hậu quả của fail-closed chỉ là chủ
máy phải thêm 1 dòng `.env`. Xem trade-off tương tự lặp lại ở phần sandbox bên
dưới — cùng triết lý "an toàn theo mặc định, tiện lợi phải opt-in".

### 1.2 Allowlist lệnh cho `shell.exec`

`SHELL_ALLOWED_COMMANDS` (mặc định trong `internal/config/config.go`, biến
`defaultShellAllowedCommands`):

```
git, ls, grep, cat, find, pwd, echo, wc, head, tail, diff
```

Cố ý **không có bất kỳ interpreter nào** (`node`, `python`, `go`, `npm`,
`bash`, `sh`...) — bất kỳ interpreter nào cũng cho phép thực thi mã tuỳ ý,
biến allowlist thành vô nghĩa (vd `python -c 'import os; os.system(...)'`).
Allowlist được áp dụng ở tầng `shellTool.Execute` (`internal/tools/shell.go`)
**trước khi** lệnh chạm tới bất kỳ backend nào (host hay Docker) — bật sandbox
Docker **không thay** allowlist này, chỉ đổi CÁCH lệnh đã được cho phép chạy.

### 1.3 Chặn resource leak khi timeout

`shell.exec` chạy qua `Registry.runOne` với deadline riêng (`TimeoutTool`
interface, `internal/tools/tool.go`). Backend host (`hostBackend`,
`internal/tools/shell.go`) dùng `Setpgid: true` + ghi đè `cmd.Cancel` để kill
CẢ process group (PID âm) khi timeout — không chỉ tiến trình con trực tiếp mà
cả tiến trình cháu nó tự fork (vd `sh -c 'sleep 100 &'`). Không có cơ chế này,
tiến trình cháu sống sót vô thời hạn sau khi tool đã báo timeout xong (DoS âm
thầm). **Không đụng vào cơ chế này khi thêm sandbox** — sandbox Docker là 1
backend độc lập, cắm cạnh backend host qua interface `execBackend`, không thay
thế nó.

### 1.4 Resume / interrupt store

Engine dừng ở `NodeInterrupt` và lưu `State` khi có `SetInterruptStore` (SQLite
`paused_runs`); nếu SQLite mở lỗi lúc khởi động, `main.go` log
`slog.Warn(...)` và tiếp tục chạy KHÔNG có `/chat/resume` (degrade, không
crash). Đây là ví dụ khác cho triết lý "fail-safe, không chặn đường chính"
lặp lại ở sandbox Docker bên dưới.

---

## 2. Sandbox Docker cho `shell.exec` (opt-in: `SHELL_SANDBOX=docker`)

### 2.1 Nó LÀ gì

Khi `SHELL_SANDBOX=docker` VÀ Docker khả dụng, mỗi lệnh `shell.exec` (đã qua
allowlist ở §1.2) được chạy bên trong 1 container "anh em" (sibling container)
tạo bằng cách gọi CLI `docker run` (qua `os/exec`, không dùng Docker SDK) với
flags giới hạn:

| Flag | Mục đích |
|---|---|
| `--rm` | container tự dọn sau khi chạy xong |
| `--network=none` | **không có mạng** trong container |
| `--memory=256m` | trần bộ nhớ |
| `--cpus=0.5` | trần CPU |
| `--read-only` | filesystem gốc container read-only |
| `--cap-drop=ALL` | bỏ hết Linux capabilities |
| `--security-opt no-new-privileges` | chặn setuid/setgid leo quyền trong container |
| `--user 1000:1000` | không chạy root **bên trong** container |
| `-v <workDir>:<workDir>:ro -w <workDir>` | mount thư mục làm việc hiện tại, **CHỈ ĐỌC** |

Nói cách khác: nó giới hạn **CPU/memory/network/filesystem cho LỆNH ĐƯỢC
CHẠY**. Một lệnh ngốn tài nguyên, cố quét mạng nội bộ, hay cố ghi đè file hệ
thống trong container sẽ bị chặn/giới hạn ở tầng container đó.

Implementation: `internal/tools/shell_sandbox_docker.go`
(`dockerSandbox.Run` + `buildDockerArgs` — hàm dựng args THUẦN, test được
không cần Docker thật, xem `shell_sandbox_docker_test.go`).

### 2.2 Nó KHÔNG PHẢI là gì — ĐỌC PHẦN NÀY TRƯỚC KHI BẬT

**Sandbox này KHÔNG cô lập tuyệt đối agent-go khỏi host.**

Lý do gốc rễ: bản thân agent-go **đã** chạy trong 1 container ở production
(service `agent-go` trong `docker-compose.yml`, image ~12MB). Để 1 tiến trình
BÊN TRONG container đó (agent-go) có thể ra lệnh cho Docker daemon TRÊN HOST
tạo ra container "anh em" khác, container agent-go phải được mount
`/var/run/docker.sock` từ host vào bên trong (mô hình "Docker-in-Docker kiểu
socket sharing").

**Hệ quả:** bất kỳ ai/tiến trình nào điều khiển được container agent-go (vd
qua 1 lỗ hổng RCE trong chính code Go, 1 dependency độc hại, hay chiếm được
quyền gọi `shell.exec` một cách không giới hạn) đều có thể dùng chính
`docker.sock` đó để:

```
docker run --rm -v /:/host -it alpine chroot /host sh
```

— tức là tạo 1 container MỚI (không đi qua bất kỳ giới hạn nào ở §2.1, vì đây
là lệnh `docker` gọi trực tiếp, không phải qua `shellTool`), mount TOÀN BỘ
filesystem của HOST vào trong, rồi `chroot` để có shell root **trên host**.

Nói thẳng: **mount `/var/run/docker.sock` vào 1 container về bản chất trao
quyền tương đương root trên HOST cho bất kỳ ai điều khiển được container đó.**
Đây không phải lỗ hổng riêng của cách implement ở đây — đây là tính chất cố
hữu của kiến trúc "socket sharing" nói chung, được biết rộng rãi trong cộng
đồng Docker.

Vì vậy: sandbox Docker này **hữu ích để giới hạn 1 LỆNH SHELL đơn lẻ có thể
làm gì** (đặc biệt nếu lệnh đó gọi ra từ input không hoàn toàn tin cậy, hoặc
để giảm thiệt hại nếu 1 lệnh trong allowlist bị lạm dụng ngoài ý muốn — vd
`git` với `-c core.fsmonitor=<script độc>`), nhưng **KHÔNG bảo vệ được nếu
CHÍNH TIẾN TRÌNH agent-go bị compromise** — lúc đó kẻ tấn công bỏ qua
`shellTool` hoàn toàn và nói chuyện thẳng với `docker.sock`.

> **Tóm tắt 1 câu:** đây là "tốt hơn chạy thẳng trên host", KHÔNG PHẢI "an
> toàn tuyệt đối trước code độc hại của chính agent-go".

### 2.3 Giới hạn khác cần biết trước khi dùng

- **Không có mạng** (`--network=none`): lệnh cần mạng (vd `git fetch`,
  `git clone` từ remote, `git pull`) sẽ LỖI trong sandbox. Đây là giới hạn CỐ
  Ý — không cố gắng bật mạng có kiểm soát (network namespace riêng, proxy...)
  vì phức tạp và nằm ngoài phạm vi sprint này. Nếu cần lệnh có mạng, tắt
  sandbox cho command đó (không có cơ chế per-command hiện tại) hoặc chấp
  nhận chạy qua host backend.
- **Filesystem chỉ đọc**: `--read-only` (root fs) + bind-mount thư mục làm
  việc dạng `:ro`. Lệnh cần GHI (kể cả ghi file tạm vào `/tmp` bên trong
  container, vì `/tmp` cũng nằm trên root fs read-only) sẽ lỗi. Mở rộng thêm
  write-mount (chọn thư mục nào được ghi, quyền gì, dọn dẹp ra sao) là việc
  có chủ đích để SAU, không làm trong phạm vi này.
- **Đường dẫn bind-mount có thể SAI khi agent-go tự chạy trong container**:
  vì agent-go dùng chung `docker.sock` với HOST daemon, đường dẫn truyền cho
  `docker run -v <path>:<path>` được HOST diễn giải theo filesystem CỦA HOST —
  KHÔNG phải filesystem bên trong container agent-go. `os.Getwd()` bên trong
  agent-go container (vd `/app`) gần như chắc chắn không map đúng sang cùng
  đường dẫn trên host VPS thật. Set `SHELL_SANDBOX_HOST_WORKDIR=<đường dẫn
  thật trên host>` để bind-mount đúng chỗ trong mô hình socket-sharing (xem
  §3). Không set gì → dùng thẳng `os.Getwd()`, chỉ đúng khi agent-go chạy trực
  tiếp trên host (không qua container) hoặc khi 2 đường dẫn trùng nhau do cố
  ý cấu hình volume giống hệt nhau ở cả 2 nơi.
- **Image mặc định `alpine:3` không có sẵn `git`**: alpine gốc chỉ có busybox
  (đủ cho `ls, grep, cat, find, pwd, echo, wc, head, tail, diff`). Muốn chạy
  `git` trong sandbox cần tự build 1 image có cả busybox lẫn git rồi trỏ
  `SHELL_SANDBOX_IMAGE` tới image đó — không có image "vừa nhỏ vừa đủ mọi
  lệnh trong allowlist" nào được chọn sẵn trong phạm vi sprint này.
- **Kill tiến trình `docker run` không đảm bảo container dừng ngay lập tức**:
  timeout/cancel của `shell.exec` giết tiến trình CLI client (`docker run`)
  qua `exec.CommandContext`, nhưng client và daemon là 2 tiến trình tách biệt
  — với `--rm`, daemon vẫn dọn container khi nó tự thoát, nhưng có thể trễ 1
  chút so với host backend (vốn kill thẳng process group). Chấp nhận
  trade-off này vì timeout mặc định đã ngắn (30s) và việc `docker kill` chủ
  động theo container ID cần thêm cơ chế đặt tên/track ID, không đáng độ phức
  tạp tăng thêm trong phạm vi này.

### 2.4 Khi nào nên bật `SHELL_SANDBOX=docker`

Cân nhắc bật khi:
- Có nhiều tenant/nguồn input ít tin cậy hơn có thể tới được `shellTool` (dù
  hiện tại `shell.exec` vẫn là tool đặc quyền chỉ owner tenant — xem §1.1), và
  muốn thêm 1 lớp giới hạn resource/filesystem/network cho MỖI lệnh, phòng
  trường hợp 1 lệnh trong allowlist bị dùng theo cách không lường trước.
- Chấp nhận đánh đổi: lệnh cần mạng hoặc cần ghi file sẽ không chạy được
  trong sandbox (xem §2.3) — nếu allowlist thực tế của bạn phụ thuộc nhiều
  vào các thao tác đó, cân nhắc kỹ trước khi bật mặc định.
- Đã đọc và chấp nhận rủi ro mount `docker.sock` ở §2.2 — nếu mục tiêu là cô
  lập agent-go khỏi chính rủi ro trong code của nó (vd RCE qua dependency),
  sandbox này KHÔNG giải quyết được vấn đề đó; xem §2.5.

**Mặc định KHÔNG bật** (`SHELL_SANDBOX` không set, hoặc bất kỳ giá trị nào
khác `"docker"`) — hành vi giữ nguyên y hệt trước khi có sandbox: chạy thẳng
trên host qua `hostBackend`. Đây là quyết định risk cao nên bắt buộc opt-in.

### 2.5 Hướng nâng cấp thật sự nếu cần cô lập mạnh hơn

Nếu sau này cần cô lập chống được cả trường hợp chính agent-go bị compromise
(không chỉ giới hạn 1 lệnh), giải pháp đúng là container runtime có **kernel
riêng hoặc syscall filtering sâu hơn namespace thường**, KHÔNG PHẢI thêm flag
cho `docker run`:

- **gVisor** (`runsc`) — user-space kernel chặn syscall trước khi chạm kernel
  thật của host; tương thích tốt với Docker (`--runtime=runsc`), overhead
  thấp, KHÔNG cần mount `docker.sock` theo kiểu sibling-container (có thể
  chạy như 1 runtime cho chính container thực thi, quản lý qua API riêng thay
  vì CLI `docker run` từ bên trong).
- **Kata Containers** — mỗi container chạy trong 1 microVM riêng (dùng
  QEMU/Firecracker/Cloud Hypervisor bên dưới), cô lập ở mức phần cứng ảo hoá
  thật, không chỉ namespace.
- **Firecracker** (trực tiếp, không qua Kata) — microVM cực nhẹ (thiết kế cho
  AWS Lambda), khởi động ~125ms, phù hợp nếu cần spawn rất nhiều sandbox
  ngắn hạn.

Cả 3 hướng này đều **ngoài phạm vi sprint hiện tại** — cần hạ tầng riêng
(kernel module, runtime class trong Docker/containerd hoặc Kubernetes
RuntimeClass), không thể làm bằng cách thêm flag vào `docker run` gọi qua
socket chia sẻ như cách hiện tại.

---

## 3. Hướng dẫn set up (thủ công — KHÔNG tự động sửa hạ tầng deploy)

> File này CHỈ ghi hướng dẫn dạng text. Việc sửa `docker-compose.yml` thật
> (root `docker/deployment/docker-compose.yml` hoặc
> `docker/development/docker-compose.yml`) cần người vận hành tự cân nhắc —
> không phải việc code tự động, vì đây là thay đổi hạ tầng có rủi ro bảo mật
> đã nêu ở §2.2.

### 3.1 Mount `docker.sock` vào container agent-go

Trong file `docker-compose.yml` triển khai (vd `docker/deployment/docker-compose.yml`,
service `agent-go`), thêm volume mount:

```yaml
  agent-go:
    # ... cấu hình hiện có giữ nguyên ...
    volumes:
      - jarvis_data:/data
      - /var/run/docker.sock:/var/run/docker.sock   # THÊM DÒNG NÀY
    environment:
      - SHELL_SANDBOX=docker                         # bật sandbox
      # - SHELL_SANDBOX_IMAGE=my-registry/shell-sandbox:latest  # tuỳ chọn
      # - SHELL_SANDBOX_HOST_WORKDIR=/path/that/host/sees        # xem §2.3
```

Lưu ý so với service `dozzle`/`vector` đã có sẵn trong compose (mount
`docker.sock:ro` — CHỈ ĐỌC, dùng để đọc log container): mount cho `agent-go`
ở đây **không thể** chỉ đọc (`:ro`) vì `docker run` cần quyền TẠO container
mới qua socket, không chỉ đọc trạng thái — đây chính xác là phần rủi ro mô tả
ở §2.2, không có cách giảm nhẹ bằng flag mount.

### 3.2 Đảm bảo `docker` CLI có trong image agent-go

`internal/tools/shell_sandbox_docker.go` gọi CLI `docker` qua `os/exec` (không
dùng Docker SDK) — image agent-go (build từ `services/agent-go/Dockerfile`,
hiện ~12MB) cần cài thêm gói `docker-cli` (KHÔNG cần cài docker daemon, chỉ
cần binary client) để lệnh `docker run ...` gọi được. Nếu thiếu, `probeDocker`
sẽ fail và tự fallback về host backend (không crash) — nhưng sandbox sẽ không
bao giờ thực sự bật được cho tới khi thêm CLI vào image.

### 3.3 Biến môi trường liên quan

| Biến | Mặc định | Ý nghĩa |
|---|---|---|
| `SHELL_SANDBOX` | (rỗng, = host trực tiếp) | Set `docker` để bật sandbox (opt-in) |
| `SHELL_SANDBOX_IMAGE` | `alpine:3` | Image chạy sandbox — xem giới hạn "không có git" ở §2.3 |
| `SHELL_SANDBOX_HOST_WORKDIR` | (rỗng, = `os.Getwd()`) | Đường dẫn THẬT trên host để bind-mount, cần khi agent-go tự chạy trong container (xem §2.3) |
| `DOCKER_HOST` | (rỗng, = `/var/run/docker.sock`) | Chuẩn Docker CLI — dùng để trỏ tới socket khác nếu cần, cũng dùng để test fallback |

Tất cả đọc TRỰC TIẾP qua `os.Getenv` trong package `internal/tools` (không đi
qua `config.Config`) — xem giải thích quyết định thiết kế này trong comment
tại `resolveDefaultBackend` (`internal/tools/shell_sandbox_docker.go`): tránh
sửa `internal/config/config.go` / `cmd/server/main.go` đang được nhánh khác
giữ trong cùng đợt làm việc.

### 3.4 Feature-detect & fallback

Lúc khởi tạo `shellTool` (constructor `NewShellTool`/`NewShellToolWithTimeout`
trong `internal/tools/shell.go`), nếu `SHELL_SANDBOX=docker`:

1. Kiểm tra socket (`os.Stat`) — nếu không tồn tại/không truy cập được, coi
   như Docker không khả dụng ngay, không cần gọi CLI.
2. Nếu socket OK, gọi `docker version` với timeout 3s để xác nhận daemon thật
   sự phản hồi.
3. Nếu bước nào thất bại: `slog.Warn(...)` mô tả lỗi cụ thể, rồi **fallback về
   hostBackend** — `shell.exec` vẫn hoạt động bình thường (chạy thẳng trên
   host), không có gì bị chặn hay crash.

Đây là cùng triết lý "fail-safe, không chặn đường chính" đã thấy ở
`SetInterruptStore`/SQLite (§1.4) — tính năng mới KHÔNG BAO GIỜ được phép làm
sập tính năng cũ khi nó không sẵn sàng.

## 4. MCP Server (JARVIS as MCP server)

JARVIS expose 1 tập tool tối giản qua `POST /mcp` (JSON-RPC 2.0, Streamable
HTTP, xem `internal/mcp/server.go`): `calculator, datetime, echo, version,
web.search, web.fetch, notes.search, notes.create`.

Tool đặc quyền (`shell.exec`, `file.*`, `git` — xem §1.1) bị hard-exclude
tuyệt đối khỏi cả `tools/list` lẫn `tools/call`, không có ngoại lệ, không có
cấu hình bật lại — đây là 1 đường vào mới không đi qua `node_tools.go`/
owner-tenant gate (§1.2) nên phải nghiêm hơn kênh chat thường, không được
phép tin tưởng bất kỳ filter nào caller truyền vào (defense-in-depth, xem
test `TestServer_CallerFilter_NarrowsFurtherButNeverWidens`).

**Giới hạn biết trước:** endpoint này CHƯA có auth/rate-limit — ai gọi HTTP
tới được server đều dùng được các tool non-privileged này bình đẳng. Nếu
deploy public, cần thêm auth (API key/JWT) và rate-limit ở tầng ngoài
(reverse proxy/API gateway) trước khi mở `/mcp` ra Internet.
