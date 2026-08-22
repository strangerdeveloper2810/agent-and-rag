import { config } from "../../config";
import { AgentUnavailableError, AgentTimeoutError } from "../../lib/errors";
import type { AgentEvent } from "../graph/index";
import type { AgentClient } from "./agent-client.interface";

// =============================================================================
// Go Agent (HTTP+SSE proxy) — P12+
// =============================================================================

/** Go agent server-sent event — khớp với Go struct Event. */
interface GoAgentEvent {
  type: string;
  node?: string;
  text?: string;
  name?: string;
  message?: string;
  usage?: { inputTokens: number; outputTokens: number };
  /** cumulative total tokens (Go gửi kèm event usage/done). */
  totalTokens?: number;
  /** true khi câu trả lời bị cắt vì chạm giới hạn output token. */
  truncated?: boolean;
  /** Kích thước ước tính (token) của context ở CUỐI lượt (Type=done). */
  contextTokens?: number;
  /** Ngân sách token context (Type=done). 0 = không giới hạn. */
  contextBudget?: number;
  /** Định danh run đang dừng (Type=interrupt) — dùng làm runId khi resume. */
  runId?: string;
}

// ----- Circuit Breaker -----

interface CircuitState {
  failures: number;
  lastFailureTime: number;
  open: boolean;
}

const circuit: CircuitState = {
  failures: 0,
  lastFailureTime: 0,
  open: false,
};

const CIRCUIT_THRESHOLD = 5; // 5 lỗi liên tiếp → mở mạch
const CIRCUIT_TIMEOUT_MS = 30_000; // 30s đóng lại

const checkCircuit = (): void => {
  if (!circuit.open) return;
  if (Date.now() - circuit.lastFailureTime >= CIRCUIT_TIMEOUT_MS) {
    circuit.open = false;
    circuit.failures = 0;
  } else {
    throw new AgentUnavailableError(
      "AI agent tạm thời không khả dụng (circuit breaker mở). Vui lòng thử lại sau.",
      Math.ceil(
        (CIRCUIT_TIMEOUT_MS - (Date.now() - circuit.lastFailureTime)) / 1000,
      ),
    );
  }
};

const recordFailure = (): void => {
  circuit.failures++;
  circuit.lastFailureTime = Date.now();
  if (circuit.failures >= CIRCUIT_THRESHOLD) {
    circuit.open = true;
  }
};

const recordSuccess = (): void => {
  circuit.failures = 0;
  circuit.open = false;
};

// ----- Retry -----

const MAX_RETRIES = 3;
const RETRY_BASE_DELAY_MS = 500;

const sleep = async (ms: number): Promise<void> => {
  return new Promise((resolve) => setTimeout(resolve, ms));
};

// ----- Health Check -----

/**
 * Gọi GET /healthz lên Go agent để kiểm tra trước khi proxy.
 * Không throw — trả về boolean. Dùng để:
 * - Circuit breaker (đánh dấu lỗi nếu health check thất bại).
 * - app.ts /healthz endpoint (báo trạng thái Go agent).
 */
export const checkGoAgentHealth = async (): Promise<boolean> => {
  const url = `${config.AGENT_GO_URL}/healthz`;
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), 5_000);

  try {
    const res = await fetch(url, { signal: controller.signal });
    return res.ok;
  } catch {
    return false;
  } finally {
    clearTimeout(timeoutId);
  }
};

// ----- MCP Test Connection -----

export interface McpTestConnectionResult {
  ok: boolean;
  toolCount?: number;
  error?: string;
}

/**
 * Gọi POST /mcp/test-connection lên Go agent để kiểm tra 1 MCP server remote
 * (handshake + list tools) mà không cần chờ hết 1 lượt chat thật. Dùng lại
 * đúng mcp.DiscoverSSE ở phía Go — không viết lại logic MCP client ở TS.
 *
 * ok=false là KẾT QUẢ HỢP LỆ (server cấu hình sai/không phản hồi), không
 * throw AgentUnavailableError — chỉ throw khi bản thân Go agent không gọi
 * được (mạng đứt, Go agent down), để phân biệt "server MCP của user lỗi"
 * với "hệ thống của mình lỗi".
 */
export const testMcpConnection = async (
  name: string,
  url: string,
  apiKey?: string,
): Promise<McpTestConnectionResult> => {
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), 12_000);

  try {
    const res = await fetch(`${config.AGENT_GO_URL}/mcp/test-connection`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name, url, apiKey }),
      signal: controller.signal,
    });
    if (!res.ok) {
      throw new AgentUnavailableError(
        `Go agent trả về lỗi ${res.status} khi test kết nối MCP.`,
      );
    }
    return (await res.json()) as McpTestConnectionResult;
  } catch (err) {
    if (err instanceof AgentUnavailableError) throw err;
    throw new AgentUnavailableError(
      "Không thể kết nối đến AI agent để test MCP server.",
    );
  } finally {
    clearTimeout(timeoutId);
  }
};

/** Một gợi ý mở đầu hội thoại, kèm category để FE lọc theo tab đang chọn. */
export interface SuggestionItem {
  text: string;
  category?: string;
}

/**
 * Gọi GET /suggestions lên Go agent, gửi kèm X-Tenant-ID để agent-go cá
 * nhân hoá theo ĐÚNG tenant đang đăng nhập (lịch sử hội thoại + facts đã
 * học) — trước đây FE gọi thẳng agent-go không qua BFF nên không có tenant
 * nào cả, mọi user đều rơi về "default".
 */
export const getSuggestions = async (
  tenantId: string,
): Promise<{ suggestions: SuggestionItem[] }> => {
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), 15_000);

  try {
    const res = await fetch(`${config.AGENT_GO_URL}/suggestions`, {
      headers: { "X-Tenant-ID": tenantId },
      signal: controller.signal,
    });
    if (!res.ok) {
      throw new AgentUnavailableError(
        `Go agent trả về lỗi ${res.status} khi lấy suggestions.`,
      );
    }
    return (await res.json()) as { suggestions: SuggestionItem[] };
  } catch (err) {
    if (err instanceof AgentUnavailableError) throw err;
    throw new AgentUnavailableError(
      "Không thể kết nối đến AI agent để lấy suggestions.",
    );
  } finally {
    clearTimeout(timeoutId);
  }
};

// ----- SSE Parser -----

/**
 * Parse ReadableStream<Uint8Array> từ Go agent response thành async generator
 * các GoAgentEvent. Mỗi line `data: {...}\n\n` → parse JSON → yield.
 */
async function* parseSSE(
  stream: ReadableStream<Uint8Array>,
): AsyncGenerator<GoAgentEvent> {
  const reader = stream.getReader();
  const decoder = new TextDecoder();
  let buffer = "";

  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split("\n");
      // Giữ lại phần dở (line cuối cùng) cho lần đọc tiếp theo.
      buffer = lines.pop() ?? "";

      for (const line of lines) {
        if (line.startsWith("data: ")) {
          try {
            yield JSON.parse(line.slice(6)) as GoAgentEvent;
          } catch {
            // Bỏ qua line không parse được (dòng trống, comment SSE).
          }
        }
      }
    }

    // Xử lý phần còn lại trong buffer.
    if (buffer.startsWith("data: ")) {
      try {
        yield JSON.parse(buffer.slice(6)) as GoAgentEvent;
      } catch {
        // Bỏ qua.
      }
    }
  } finally {
    reader.releaseLock();
  }
}

/**
 * mapGoAgentEvents — parse SSE body của Go agent (dùng chung cho cả
 * stream() và resume(), vì cả 2 endpoint /chat và /chat/resume trả về
 * CÙNG format event). Tách riêng để không lặp lại switch-case ~90 dòng.
 *
 * agentName/totalTokens là state CỤC BỘ trong 1 lần gọi (không escape ra
 * ngoài) — dùng để điền field `agent`/`totalTokens` vào event `done` cuối
 * cùng, xem case "agent"/"done" bên dưới.
 */
async function* mapGoAgentEvents(
  body: ReadableStream<Uint8Array>,
): AsyncGenerator<AgentEvent> {
  // agentName mặc định "go" (phân biệt với "langgraph"), được ghi đè bằng
  // tên agent thật ngay khi nhận event {type:"agent"} từ orchestrator Go
  // (general/code/research) — xem case "agent" bên dưới.
  let agentName = "go";
  let totalTokens = 0;

  for await (const raw of parseSSE(body)) {
    switch (raw.type) {
      case "text":
        yield { type: "text", text: raw.text ?? "" } as AgentEvent;
        break;
      case "tool_start":
        yield {
          type: "tool_start",
          name: raw.name ?? "unknown",
        } as AgentEvent;
        break;
      case "tool_end":
        yield {
          type: "tool_end",
          name: raw.name ?? "unknown",
          message: raw.message,
          text: raw.text,
        } as AgentEvent;
        break;
      case "error":
        yield {
          type: "error",
          message: raw.message ?? "Agent error",
        } as AgentEvent;
        break;
      case "truncated":
        // Model bị cắt vì chạm max output tokens → UI hiện chỉ báo +
        // nút "Tiếp tục". Không phải lỗi: text đã stream vẫn giữ.
        yield {
          type: "truncated",
          message: raw.message,
        } as AgentEvent;
        break;
      case "done":
        if (raw.usage) {
          totalTokens = raw.usage.inputTokens + raw.usage.outputTokens;
        }
        yield {
          type: "done",
          agent: agentName,
          tokens: totalTokens,
          // Forward cả usage + totalTokens: FE đọc `usage`/`totalTokens`
          // (packages/api-client normalizeEvent) chứ không đọc `tokens`,
          // nên trước đây meta token luôn undefined.
          usage: raw.usage,
          totalTokens: raw.totalTokens ?? totalTokens,
          truncated: raw.truncated === true,
          // contextTokens/contextBudget: FE tự tính tỉ lệ để gợi ý bắt
          // đầu chat mới khi context lớn (Tier 4) — không forward thì
          // gợi ý này không bao giờ có dữ liệu dù Go đã tính đúng.
          contextTokens: raw.contextTokens,
          contextBudget: raw.contextBudget,
        } as AgentEvent;
        break;
      case "step":
        // Step event: forward nguyên bản (node info).
        yield {
          type: "step",
          node: raw.node,
        } as AgentEvent;
        break;
      case "agent":
        // Orchestrator Go phát {type:"agent", node:"<tên agent>"} khi
        // route xong. Trước đây event này rơi vào default → bị skip, nên
        // BFF phải hardcode agentName="go" và badge agent trên UI không
        // bao giờ hiện tên thật (general/code/research).
        // Go dùng field `node`, còn api-client đọc `name` → chuẩn hoá ở đây.
        if (raw.node) agentName = raw.node;
        yield { type: "agent", name: raw.node } as unknown as AgentEvent;
        break;
      case "usage":
        // Per-step token usage. Trước đây bị skip nên đồng hồ token trên
        // UI không có số dù Go tính đúng.
        yield {
          type: "usage",
          usage: raw.usage,
          totalTokens: raw.totalTokens,
        } as AgentEvent;
        break;
      case "ask_user":
      case "suggestions":
      case "citation":
      case "memory":
      case "interrupt":
        // Forward các event type khác (đã gồm runId nếu Go gửi kèm, xem
        // GoAgentEvent.runId — "as unknown" nên field lạ vẫn đi qua nguyên
        // trạng dù interface AgentEvent không khai báo hết mọi field).
        yield raw as unknown as AgentEvent;
        break;
      default:
        // Unknown event type → skip.
        break;
    }
  }
}

// ----- Go Agent Client -----

/**
 * GoAgentClient — proxy HTTP+SSE sang service agent-go (cổng mặc định 3002).
 *
 * Flow mỗi request:
 *   1. Check circuit breaker (5 lỗi liên tiếp → từ chối trong 30s).
 *   2. POST /chat với history + userMessage (tách từ history input).
 *      Timeout AGENT_GO_TIMEOUT chỉ áp cho thời gian chờ response headers
 *      (TTFB) — sau khi stream bắt đầu, không abort giữa chừng để tránh
 *      retry chạy LẠI task LLM từ đầu (nhân đôi chi phí + latency).
 *   3. Parse SSE → convert GoAgentEvent → AgentEvent, yield từng event.
 *   4. Record success/failure để cập nhật circuit breaker.
 */
export const goAgentClient: AgentClient = {
  async *stream(history, opts) {
    // 1. Circuit breaker
    checkCircuit();

    // 2. Tách user message cuối khỏi history
    const userMessage = history[history.length - 1]?.content ?? "";
    const chatHistory = history.slice(0, -1).map((m) => ({
      role: m.role,
      content: m.content,
    }));

    const body = JSON.stringify({
      history: chatHistory,
      userMessage,
      attachments: opts?.attachments ?? [],
      lang: opts?.lang,
      personaPreset: opts?.personaPreset,
      formality: opts?.formality,
      verbosity: opts?.verbosity,
      customInstructions: opts?.customInstructions,
      mcpServers: opts?.mcpServers ?? [],
      disabledSkills: opts?.disabledSkills ?? [],
      customSkills: opts?.customSkills ?? [],
    });

    // 3. Gọi POST /chat với retry (chỉ retry khi lỗi TRƯỚC khi có response).
    const url = `${config.AGENT_GO_URL}/chat`;

    for (let attempt = 0; attempt <= MAX_RETRIES; attempt++) {
      const controller = new AbortController();
      let headersReceived = false;
      const timeoutId = setTimeout(
        () => controller.abort(),
        config.AGENT_GO_TIMEOUT,
      );

      // Nếu client gốc có signal → propagate (user cancel giữa chừng vẫn
      // được phép — client tự ngắt, không phải retry).
      const onAbort = () => controller.abort();
      opts?.signal?.addEventListener("abort", onAbort, { once: true });

      try {
        const headers: Record<string, string> = {
          "Content-Type": "application/json",
        };
        if (opts?.tenantId) {
          headers["X-Tenant-ID"] = opts.tenantId;
        }

        const res = await fetch(url, {
          method: "POST",
          headers,
          body,
          signal: controller.signal,
        });

        // Đã nhận response headers → hết giai đoạn TTFB, clear timeout để
        // không abort task đang chạy (lỗi 4xx/5xx vẫn giữ nguyên logic retry).
        clearTimeout(timeoutId);

        if (!res.ok) {
          // 4xx = lỗi client, không retry.
          if (res.status >= 400 && res.status < 500) {
            recordFailure();
            throw new AgentUnavailableError(
              `Go agent trả về lỗi ${res.status}: ${res.statusText}`,
            );
          }
          // 5xx = retry.
          throw new Error(`Go agent lỗi ${res.status}`);
        }

        // Từ đây stream đã bắt đầu → lỗi giữa chừng KHÔNG retry nữa.
        headersReceived = true;

        // Parse SSE stream → convert to AgentEvent.
        if (!res.body) {
          recordFailure();
          throw new AgentUnavailableError("Go agent trả về response rỗng.");
        }

        yield* mapGoAgentEvents(res.body);

        recordSuccess();
        return; // stream thành công → thoát.
      } catch (err) {
        clearTimeout(timeoutId);
        opts?.signal?.removeEventListener("abort", onAbort);

        if (headersReceived) {
          // Stream đã bắt đầu: lỗi giữa chừng (mạng đứt, Go crash) KHÔNG
          // retry — Go đã chạy LLM work, retry sẽ duplicate toàn bộ task.
          recordFailure();
          throw err;
        }

        // Không retry nếu client abort hoặc circuit breaker đã mở.
        if (opts?.signal?.aborted) {
          return;
        }
        if (err instanceof AgentUnavailableError) {
          throw err;
        }

        if (controller.signal.aborted && !opts?.signal?.aborted) {
          // Đây là AGENT_GO_TIMEOUT (TTFB quá lâu).
          if (attempt === MAX_RETRIES) {
            recordFailure();
            throw new AgentTimeoutError(
              `AI agent phản hồi quá chậm sau ${MAX_RETRIES + 1} lần thử (${config.AGENT_GO_TIMEOUT}ms mỗi lần).`,
              config.AGENT_GO_TIMEOUT,
            );
          }
          recordFailure();
        } else {
          recordFailure();
        }

        if (attempt === MAX_RETRIES) {
          throw new AgentUnavailableError(
            `Không thể kết nối đến AI agent sau ${MAX_RETRIES + 1} lần thử.`,
          );
        }

        // Exponential backoff: 500ms, 1000ms, 2000ms.
        await sleep(RETRY_BASE_DELAY_MS * Math.pow(2, attempt));
      } finally {
        clearTimeout(timeoutId);
        opts?.signal?.removeEventListener("abort", onAbort);
      }
    }
  },

  /**
   * Gọi POST /chat/resume lên Go agent để tiếp tục 1 run đã dừng giữa
   * chừng (interrupt HITL hoặc crash) — xem
   * services/agent-go/internal/transport/http/chat_resume.go.
   *
   * KHÁC stream(): KHÔNG retry khi lỗi TRƯỚC response. agent-go XOÁ
   * checkpoint (paused_runs) ngay sau khi load để resume thành công, nên
   * gọi lại lần 2 với cùng runId chỉ nhận lỗi "not found" — retry vô nghĩa
   * (ngược với /chat, nơi agent-go chưa động tới gì trước khi có response
   * nên retry TTFB timeout là an toàn).
   */
  async *resume(runId, answer, opts) {
    checkCircuit();

    const controller = new AbortController();
    const timeoutId = setTimeout(
      () => controller.abort(),
      config.AGENT_GO_TIMEOUT,
    );
    const onAbort = () => controller.abort();
    opts?.signal?.addEventListener("abort", onAbort, { once: true });

    try {
      const headers: Record<string, string> = {
        "Content-Type": "application/json",
      };
      if (opts?.tenantId) {
        headers["X-Tenant-ID"] = opts.tenantId;
      }

      const res = await fetch(`${config.AGENT_GO_URL}/chat/resume`, {
        method: "POST",
        headers,
        body: JSON.stringify({ run_id: runId, answer }),
        signal: controller.signal,
      });

      clearTimeout(timeoutId);

      if (!res.ok) {
        recordFailure();
        // 404 (run_id không tồn tại/đã resume rồi) hay 400 (thiếu answer khi
        // đang chờ interrupt) đều là lỗi client rõ ràng — không đoán thêm,
        // để caller (chat.controller) tự quyết định thông báo gì cho user.
        throw new AgentUnavailableError(
          `Go agent trả về lỗi ${res.status} khi resume run "${runId}".`,
        );
      }

      if (!res.body) {
        recordFailure();
        throw new AgentUnavailableError(
          "Go agent trả về response rỗng khi resume.",
        );
      }

      yield* mapGoAgentEvents(res.body);
      recordSuccess();
    } catch (err) {
      if (opts?.signal?.aborted) return;
      if (err instanceof AgentUnavailableError) throw err;
      recordFailure();
      throw new AgentUnavailableError(
        `Không thể kết nối đến AI agent để resume run "${runId}".`,
      );
    } finally {
      clearTimeout(timeoutId);
      opts?.signal?.removeEventListener("abort", onAbort);
    }
  },
};
