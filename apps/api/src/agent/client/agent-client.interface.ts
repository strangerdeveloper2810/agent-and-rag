import type { AgentEvent } from "../graph/index";

// ----- AgentMessage & AgentStreamOptions -----

/** Một message trong lịch sử hội thoại gửi cho agent. */
export type AgentMessage = { role: string; content: string };

/** Tuỳ chọn cho agent stream (abort signal từ HTTP request). */
export type AgentStreamOptions = {
  signal?: AbortSignal;
  attachments?: Array<{
    type: string;
    name: string;
    data: string;
    mimeType: string;
  }>;
};

/**
 * AgentClient — BIÊN GIỚI giữa gateway (Fastify) và agent runtime.
 */
export interface AgentClient {
  stream(
    history: AgentMessage[],
    opts?: AgentStreamOptions,
  ): AsyncIterable<AgentEvent>;
}
