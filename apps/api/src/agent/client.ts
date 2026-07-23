import { config } from "../config";
import { runGraph, type AgentEvent } from "./graph-runner";

export type AgentMessage = { role: string; content: string };
export type AgentStreamOptions = { signal?: AbortSignal };

/**
 * AgentClient — BIÊN GIỚI giữa gateway (Fastify) và agent runtime.
 *
 * Hôm nay: LangGraph (TS, in-process). Sau (P12): thêm impl proxy sang service
 * `agent-go` (HTTP+SSE). `chat.service` chỉ phụ thuộc interface này → khi chuyển
 * agent sang Go chỉ cần thêm 1 impl + đổi `AGENT_BACKEND`, không đụng chat.service.
 */
export interface AgentClient {
  stream(
    history: AgentMessage[],
    opts?: AgentStreamOptions,
  ): AsyncIterable<AgentEvent>;
}

// Impl chạy LangGraph engine hiện có (in-process).
export const langGraphAgentClient: AgentClient = {
  stream(history, opts) {
    return runGraph(history, opts?.signal);
  },
};

/**
 * Chọn agent backend theo `config.AGENT_BACKEND`:
 * - "langgraph" (mặc định): LangGraph in-process.
 * - "go" (P12): proxy sang agent-go — chưa implement.
 */
export function createAgentClient(): AgentClient {
  switch (config.AGENT_BACKEND) {
    case "go":
      throw new Error(
        "AGENT_BACKEND=go chưa hỗ trợ (thêm goAgentClient ở P12)",
      );
    case "langgraph":
    default:
      return langGraphAgentClient;
  }
}
