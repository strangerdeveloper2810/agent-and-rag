# agent-go — Guardrails, Checkpoint/Resume, Sandbox, Observability

← [Về mục lục](./README.md)

## Guardrails

- **Circuit breaker** (`guardrails.NewCircuitBreaker(3)`): chặn model tự lặp vô hạn (gọi cùng 1 tool y hệt liên tiếp quá N lần). Tách biệt với circuit breaker phía BFF (`go-agent.client.ts`, xem [`bff.md`](./bff.md)) — 2 tầng bảo vệ khác vấn đề khác nhau.
- **Tool Kind**: mỗi tool khai `Read`/`Write`/`Destructive` — destructive cần xác nhận (HITL) trừ khi `ALLOW_DESTRUCTIVE_TOOLS=true`.
- **Prompt-injection filter**: chặn sớm input dạng "ignore all previous instructions..." trước khi mở SSE.
- **Privileged tools** (`file.*`, `shell.exec`, `git`): chỉ tenant nằm trong `OWNER_TENANT_IDS` mới gọi được — chặn ở 2 lớp (ẩn khỏi tool list VÀ chặn cứng khi model cố gọi tên tool trực tiếp, kể cả tự bịa tên). Fail-closed: `OWNER_TENANT_IDS` rỗng → chỉ tenant `"default"` (local, không auth) được coi là chủ.
- **`shell.exec` allowlist thật**: `Config.ShellAllowedCommands` (env `SHELL_ALLOWED_COMMANDS`), default an toàn chỉ gồm lệnh đọc/inspect (`git, ls, grep, cat, find, pwd, echo, wc, head, tail, diff`) — KHÔNG có interpreter (`node`/`python`/`bash`) trong default vì chúng mở lại đường thực thi mã tuỳ ý.
- **`shell.exec` kill cả process group**: `SysProcAttr{Setpgid: true}` + `syscall.Kill(-pid, SIGKILL)` khi timeout — trước đây chỉ kill tiến trình con trực tiếp, tiến trình cháu (do lệnh tự fork thêm) sống sót sau kill.

## Sandbox Docker cho `shell.exec` (opt-in)

`SHELL_SANDBOX=docker` chạy `shell.exec` qua `docker run` (CLI, không SDK) với flags giới hạn tối đa: `--rm --network=none --memory=256m --cpus=0.5 --read-only --cap-drop=ALL --security-opt no-new-privileges --user 1000:1000`. Feature-detect Docker daemon lúc khởi tạo — không sẵn sàng thì tự fallback về chạy trực tiếp trên host, không chặn `shell.exec`.

**Trade-off cần hiểu trước khi bật**: agent-go tự nó chạy trong 1 container. Để sandbox spawn được container "anh em", container agent-go phải mount `/var/run/docker.sock` từ host — về bản chất trao quyền tương đương root trên HOST cho bất kỳ ai điều khiển được container đó (có thể tạo container mount `/` host rồi thoát ra). Đây KHÔNG PHẢI cô lập tuyệt đối — chỉ giới hạn resource/network/filesystem cho **lệnh được chạy**, không bảo vệ trước việc chính agent-go bị compromise. Chi tiết đầy đủ + hướng nâng cấp thật (gVisor/Kata/Firecracker) ở [`services/agent-go/docs/security-model.md`](../../../services/agent-go/docs/security-model.md).

---

## Checkpoint / Resume — crash-safe, không chỉ HITL

Ban đầu chỉ resume được khi agent chủ động dừng để hỏi user (`NodeInterrupt`, HITL). Đã tổng quát hoá: `Engine` giờ **checkpoint state vào SQLite (`paused_runs`) sau MỖI lần chuyển node** trong `runLoop`, không chỉ khi dừng ở interrupt — nghĩa là 1 run bị crash/restart giữa chừng (không chỉ khi cố ý hỏi user) cũng resume lại được đúng từ node vừa dừng.

- **`RunID`** sinh ngay khi bắt đầu 1 `Run()`, theo cùng suốt vòng đời.
- **Ghi checkpoint đồng bộ, fail-safe tuyệt đối**: lỗi ghi SQLite chỉ `slog.Warn`, KHÔNG BAO GIỜ chặn response user — nếu checkpoint fail, response vẫn trả về bình thường, chỉ là lần đó không resume được nếu crash ngay sau đó.
- **Xoá checkpoint khi `NodeEnd`** (kết thúc tự nhiên) — tránh tích luỹ rác cho đa số run không bao giờ cần resume.
- **`POST /chat/resume`**: nhận `{run_id, answer}` — `answer` giờ **optional**, chỉ bắt buộc khi state load ra có `Interrupt != nil` (đang chờ trả lời câu hỏi HITL thật). Nếu state dừng ở node khác (không phải interrupt), resume tiếp tục route bình thường từ đúng chỗ dừng, không cần `answer`.
- **Giới hạn còn biết**: nếu crash xảy ra NGAY GIỮA lúc nhiều tool chạy song song trong CÙNG 1 batch `NodeTools` (1 tool đã xong, tool khác chưa), checkpoint gần nhất vẫn là bản TRƯỚC batch đó — resume sẽ chạy lại TOÀN BỘ batch, có thể gọi lại tool destructive/write đã chạy thành công lần trước (side-effect kép). Cần idempotency key riêng cho từng tool call ở đợt sau để giải quyết triệt để — CHƯA làm.
- **Mất sau round-trip JSON**: field không exported của `State` (closure, `mcpRegistry`, `activatedSkills`) không serialize được — chấp nhận được vì đây là dữ liệu build-lại-được từ config, không phải dữ liệu hội thoại.

---

## Observability — OpenTelemetry thật

`internal/observability.SetupTracer` dựng `sdktrace.TracerProvider` THẬT (trước đây là noop, chưa từng được gọi). Exporter mặc định `stdouttrace` (in span JSON ra stdout); nếu có env `OTEL_EXPORTER_OTLP_ENDPOINT` thì dùng OTLP HTTP. Span thật được tạo quanh: mỗi node transition (`step.<node>`), mỗi lượt gọi LLM (`llm.generate`), mỗi tool call (`tool.<name>`). Lỗi setup tracer không chặn khởi động (tracing là phụ trợ).

`internal/metrics` (custom in-process counter, KHÔNG phải Prometheus) vẫn còn dead code — chưa wire gọi ở đâu, ưu tiên thấp vì OTel đã đủ cho nhu cầu hiện tại.
