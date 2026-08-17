import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type { AgentEvent } from "../graph/index";

import { goAgentClient } from "./go-agent.client";

/** Dựng một Response giả có body SSE từ danh sách dòng data. */
function sseResponse(lines: string[]): Response {
  const payload = lines.map((l) => `data: ${l}\n\n`).join("");
  const stream = new ReadableStream<Uint8Array>({
    start(controller) {
      controller.enqueue(new TextEncoder().encode(payload));
      controller.close();
    },
  });
  return new Response(stream, {
    status: 200,
    headers: { "Content-Type": "text/event-stream" },
  });
}

const drain = async (gen: AsyncIterable<AgentEvent>): Promise<AgentEvent[]> => {
  const out: AgentEvent[] = [];
  for await (const e of gen) out.push(e);
  return out;
};

const history = [{ role: "user" as const, content: "hỏi gì đó" }];

describe("goAgentClient.stream — map SSE của Go sang AgentEvent", () => {
  beforeEach(() => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        sseResponse([
          JSON.stringify({ type: "agent", node: "code" }),
          JSON.stringify({ type: "text", text: "xin chào" }),
          JSON.stringify({
            type: "usage",
            usage: { inputTokens: 10, outputTokens: 5 },
            totalTokens: 15,
          }),
          JSON.stringify({
            type: "done",
            usage: { inputTokens: 10, outputTokens: 5 },
            totalTokens: 15,
          }),
        ]),
      ),
    );
  });

  afterEach(() => vi.unstubAllGlobals());

  // Orchestrator Go phát {type:"agent", node:"code"}. Trước fix, event này rơi
  // vào nhánh default → BỊ SKIP, nên BFF phải hardcode agent="go" và badge
  // agent trên UI không bao giờ hiện tên thật (general/code/research).
  it("forward event agent và chuẩn hoá node → name", async () => {
    const events = await drain(goAgentClient.stream(history));

    const agentEv = events.find((e) => e.type === "agent");
    expect(agentEv, "phải forward event agent").toBeDefined();
    expect((agentEv as { name?: string }).name).toBe("code");
  });

  // Per-step usage trước đây bị skip → đồng hồ token trên UI không có số dù
  // Go tính đúng.
  it("forward event usage kèm số token", async () => {
    const events = await drain(goAgentClient.stream(history));

    const usageEv = events.find((e) => e.type === "usage");
    expect(usageEv, "phải forward event usage").toBeDefined();
    expect((usageEv as { usage?: { inputTokens: number } }).usage).toEqual({
      inputTokens: 10,
      outputTokens: 5,
    });
  });

  // FE (packages/api-client normalizeEvent) đọc `usage`/`totalTokens`, không
  // đọc `tokens` → trước fix meta token luôn undefined.
  it("event done mang theo usage + totalTokens, và agent name thật", async () => {
    const events = await drain(goAgentClient.stream(history));

    const done = events.find((e) => e.type === "done") as
      | {
          type: "done";
          agent?: string;
          tokens?: number;
          usage?: { inputTokens: number; outputTokens: number };
          totalTokens?: number;
        }
      | undefined;

    expect(done).toBeDefined();
    expect(done!.usage).toEqual({ inputTokens: 10, outputTokens: 5 });
    expect(done!.totalTokens).toBe(15);
    expect(done!.tokens).toBe(15);
    expect(done!.agent, "agent phải là tên thật từ event agent").toBe("code");
  });

  it('agent mặc định là "go" khi Go không gửi event agent', async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        sseResponse([
          JSON.stringify({ type: "text", text: "hi" }),
          JSON.stringify({ type: "done" }),
        ]),
      ),
    );

    const events = await drain(goAgentClient.stream(history));
    const done = events.find((e) => e.type === "done") as
      { agent?: string } | undefined;
    expect(done?.agent).toBe("go");
  });
});
