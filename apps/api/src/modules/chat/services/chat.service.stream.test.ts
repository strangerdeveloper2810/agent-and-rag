import { describe, it, expect, vi, beforeEach } from "vitest";
import type { AgentEvent } from "../../../agent/graph-runner";

// Mock repository (DB) + agent để test riêng logic orchestration của streamReply.
vi.mock("../repositories", () => ({
  createCoversation: vi.fn(),
  listConversations: vi.fn(),
  getMessages: vi.fn(),
  addMessage: vi.fn(),
  deleteConversation: vi.fn(),
}));
vi.mock("../../../agent/graph-runner", () => ({ runGraph: vi.fn() }));

import { streamReply } from "./chat.service";
import * as repo from "../repositories";
import { runGraph } from "../../../agent/graph-runner";

// Dựng async generator giả lập luồng event của agent.
// TReturn = string để khớp chữ ký runGraph (): AsyncGenerator<AgentEvent, string>.
async function* fakeStream(
  events: AgentEvent[],
): AsyncGenerator<AgentEvent, string> {
  for (const e of events) yield e;
  return "";
}

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
    vi.mocked(runGraph).mockReturnValue(
      fakeStream([
        { type: "tool_start", name: "ragSearch" },
        { type: "tool_end", name: "ragSearch" },
        { type: "text", text: "Xin " },
        { type: "text", text: "chào" },
      ]),
    );

    const out = await drain(streamReply("c1") as AsyncGenerator<AgentEvent>);

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
    vi.mocked(runGraph).mockReturnValue(fakeStream([]));

    await drain(streamReply("c1") as AsyncGenerator<AgentEvent>);

    expect(repo.addMessage).not.toHaveBeenCalled();
  });

  it("vẫn lưu phần đã sinh khi bị ngắt giữa chừng (finally)", async () => {
    vi.mocked(repo.getMessages).mockResolvedValue([] as never);
    async function* boom(): AsyncGenerator<AgentEvent, string> {
      yield { type: "text", text: "một phần" };
      throw new Error("aborted");
    }
    vi.mocked(runGraph).mockReturnValue(boom());

    const run = drain(streamReply("c1") as AsyncGenerator<AgentEvent>);
    await expect(run).rejects.toThrow();
    // Nhờ khối finally: phần đã sinh không bị mất.
    expect(repo.addMessage).toHaveBeenCalledWith("c1", "assistant", "một phần");
  });
});
