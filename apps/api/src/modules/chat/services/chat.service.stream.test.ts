import { describe, it, expect, vi, beforeEach } from "vitest";
import type { AgentEvent } from "../../../agent/graph-runner";
import type { AgentClient } from "../../../agent/client";

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

const drain = async (gen: AsyncGenerator<AgentEvent>) => {
  const out: AgentEvent[] = [];
  for await (const ev of gen) out.push(ev);
  return out;
};

describe("streamReply", () => {
  beforeEach(() => vi.clearAllMocks());

  it("yield đúng chuỗi event và lưu assistant text khi stream xong", async () => {
    vi.mocked(repo.getMessages).mockResolvedValue([
      { role: "user", content: "hi" },
    ] as never);

    const out = await drain(
      streamReply(
        "c1",
        undefined,
        fakeAgent([
          { type: "tool_start", name: "ragSearch" },
          { type: "tool_end", name: "ragSearch" },
          { type: "text", text: "Xin " },
          { type: "text", text: "chào" },
        ]),
      ) as AsyncGenerator<AgentEvent>,
    );

    expect(out.map((e) => e.type)).toEqual([
      "tool_start",
      "tool_end",
      "text",
      "text",
    ]);
    // Text các mảnh được gộp lại rồi lưu 1 lần.
    expect(repo.addMessage).toHaveBeenCalledWith("c1", "assistant", "Xin chào");
  });

  it("không lưu khi output rỗng (tránh làm nhiễu lượt sau)", async () => {
    vi.mocked(repo.getMessages).mockResolvedValue([] as never);

    await drain(
      streamReply("c1", undefined, fakeAgent([])) as AsyncGenerator<AgentEvent>,
    );

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

    const run = drain(
      streamReply("c1", undefined, boomAgent) as AsyncGenerator<AgentEvent>,
    );
    await expect(run).rejects.toThrow();
    // Nhờ khối finally: phần đã sinh không bị mất.
    expect(repo.addMessage).toHaveBeenCalledWith("c1", "assistant", "một phần");
  });
});
