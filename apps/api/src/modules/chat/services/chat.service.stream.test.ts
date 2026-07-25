import { describe, it, expect, vi, beforeEach } from "vitest";
import type { AgentEvent } from "../../../agent/graph/index";
import type { AgentClient } from "../../../agent/client/index";

// Mock repository (DB). Agent được INJECT qua tham số (không cần mock module).
vi.mock("../repositories", () => ({
  createConversation: vi.fn(),
  listConversations: vi.fn(),
  getMessages: vi.fn(),
  addMessage: vi.fn(),
  deleteConversation: vi.fn(),
}));

import { streamReply } from "./chat.service";
import * as repo from "../repositories";

async function* fakeStream(events: AgentEvent[]): AsyncGenerator<AgentEvent> {
  for (const e of events) yield e;
}

// AgentClient giả: bỏ qua history, phát các event cấu hình sẵn.
const fakeAgent = (events: AgentEvent[]): AgentClient => ({
  stream: () => fakeStream(events),
});

const drain = async (result: Awaited<ReturnType<typeof streamReply>>) => {
  const out: AgentEvent[] = [];
  for await (const ev of result.events) out.push(ev);
  return { events: out, metadata: await result.metadata };
};

describe("streamReply", () => {
  beforeEach(() => vi.clearAllMocks());

  it("yield đúng chuỗi event và lưu assistant text khi stream xong", async () => {
    vi.mocked(repo.getMessages).mockResolvedValue([
      { role: "user", content: "hi" },
    ] as never);

    const result = await streamReply(
      "c1",
      undefined,
      fakeAgent([
        { type: "tool_start", name: "ragSearch" },
        { type: "tool_end", name: "ragSearch" },
        { type: "text", text: "Xin " },
        { type: "text", text: "chào" },
      ]),
    );

    const { events, metadata } = await drain(result);

    expect(events.map((e) => e.type)).toEqual([
      "tool_start",
      "tool_end",
      "text",
      "text",
    ]);

    // Text các mảnh được gộp lại rồi lưu 1 lần.
    expect(repo.addMessage).toHaveBeenCalledWith("c1", "assistant", "Xin chào");

    // Metadata phải có backend mặc định.
    expect(metadata.backend).toBeDefined();
  });

  it("không lưu khi output rỗng (tránh làm nhiễu lượt sau)", async () => {
    vi.mocked(repo.getMessages).mockResolvedValue([] as never);

    const result = await streamReply("c1", undefined, fakeAgent([]));
    await drain(result);

    expect(repo.addMessage).not.toHaveBeenCalled();
  });

  it("vẫn lưu phần đã sinh khi bị ngắt giữa chừng (finally)", async () => {
    vi.mocked(repo.getMessages).mockResolvedValue([] as never);
    const boomAgent: AgentClient = {
      stream: async function* () {
        yield { type: "text", text: "một phần" } as AgentEvent;
        throw new Error("aborted");
      },
    };

    const result = await streamReply("c1", undefined, boomAgent);

    const run = drain(result);
    await expect(run).rejects.toThrow();
    // Nhờ khối finally: phần đã sinh không bị mất.
    expect(repo.addMessage).toHaveBeenCalledWith("c1", "assistant", "một phần");
  });

  it("metadata ghi nhận agent name và tokens từ event done", async () => {
    vi.mocked(repo.getMessages).mockResolvedValue([
      { role: "user", content: "hi" },
    ] as never);

    const result = await streamReply(
      "c1",
      undefined,
      fakeAgent([
        { type: "text", text: "Hello" },
        { type: "done", agent: "general", tokens: 42 },
      ]),
    );

    const { metadata } = await drain(result);

    expect(metadata.backend).toBe("general");
    expect(metadata.tokensUsed).toBe(42);
  });
});
