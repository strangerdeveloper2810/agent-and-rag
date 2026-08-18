import { describe, it, expect, vi, beforeEach } from "vitest";
import type { AgentEvent } from "../../../agent/graph/index";
import type { AgentClient } from "../../../agent/client/index";

// Mock repository (DB) — theo đúng pattern của chat.service.stream.test.ts:
// agent được INJECT qua tham số, không cần mock module agent.
vi.mock("../repositories", () => ({
  createConversation: vi.fn(),
  listConversations: vi.fn(),
  getMessages: vi.fn(),
  addMessage: vi.fn(),
  appendToLastAssistantMessage: vi.fn(),
  deleteConversation: vi.fn(),
}));

import { streamReply } from "./chat.service";
import * as repo from "../repositories";

async function* fakeStream(events: AgentEvent[]): AsyncGenerator<AgentEvent> {
  for (const e of events) yield e;
}

// AgentClient giả GHI LẠI opts nhận được — dùng để assert mcpServers/
// disabledSkills/customSkills forward đúng từ streamReply xuống agent.stream()
// (đây là "biên giới" giữa BFF apps/api và agent-go — sai ở đây thì user cấu
// hình MCP server/skill gì cũng bị BFF âm thầm nuốt mất, không tới agent-go).
const optsCapturingAgent = (
  events: AgentEvent[],
): {
  agent: AgentClient;
  getReceivedOpts: () => Parameters<AgentClient["stream"]>[1];
} => {
  let received: Parameters<AgentClient["stream"]>[1];
  return {
    agent: {
      stream: (_history, opts) => {
        received = opts;
        return fakeStream(events);
      },
    },
    getReceivedOpts: () => received,
  };
};

const drain = async (result: Awaited<ReturnType<typeof streamReply>>) => {
  const out: AgentEvent[] = [];
  for await (const ev of result.events) out.push(ev);
  return { events: out, metadata: await result.metadata };
};

describe("streamReply — forward MCP servers / disabled skills / custom skills", () => {
  beforeEach(() => vi.clearAllMocks());

  it("forward mcpServers xuống agent.stream() nguyên vẹn", async () => {
    vi.mocked(repo.getMessages).mockResolvedValue([
      { role: "user", content: "hi" },
    ] as never);

    const { agent, getReceivedOpts } = optsCapturingAgent([
      { type: "text", text: "hello" },
      { type: "done" },
    ]);

    const mcpServers = [
      { name: "weather", url: "https://mcp.example.com", apiKey: "k" },
    ];

    await drain(
      await streamReply(
        "c1",
        undefined,
        agent,
        undefined,
        undefined,
        undefined,
        undefined,
        mcpServers,
      ),
    );

    expect(getReceivedOpts()?.mcpServers).toEqual(mcpServers);
  });

  it("forward disabledSkills xuống agent.stream() nguyên vẹn", async () => {
    vi.mocked(repo.getMessages).mockResolvedValue([
      { role: "user", content: "hi" },
    ] as never);

    const { agent, getReceivedOpts } = optsCapturingAgent([
      { type: "text", text: "hello" },
      { type: "done" },
    ]);

    const disabledSkills = ["code-review", "debug"];

    await drain(
      await streamReply(
        "c1",
        undefined,
        agent,
        undefined,
        undefined,
        undefined,
        undefined,
        undefined,
        disabledSkills,
      ),
    );

    expect(getReceivedOpts()?.disabledSkills).toEqual(disabledSkills);
  });

  it("forward customSkills xuống agent.stream() nguyên vẹn", async () => {
    vi.mocked(repo.getMessages).mockResolvedValue([
      { role: "user", content: "hi" },
    ] as never);

    const { agent, getReceivedOpts } = optsCapturingAgent([
      { type: "text", text: "hello" },
      { type: "done" },
    ]);

    const customSkills = [
      {
        name: "invoice-formatter",
        description: "mô tả",
        whenToUse: "khi nào",
        content: "nội dung",
        triggers: ["hoá đơn"],
      },
    ];

    await drain(
      await streamReply(
        "c1",
        undefined,
        agent,
        undefined,
        undefined,
        undefined,
        undefined,
        undefined,
        undefined,
        customSkills,
      ),
    );

    expect(getReceivedOpts()?.customSkills).toEqual(customSkills);
  });

  it("mcpServers/disabledSkills/customSkills là undefined khi không truyền (không tự chế biến giá trị mặc định lạ)", async () => {
    vi.mocked(repo.getMessages).mockResolvedValue([
      { role: "user", content: "hi" },
    ] as never);

    const { agent, getReceivedOpts } = optsCapturingAgent([
      { type: "text", text: "hello" },
      { type: "done" },
    ]);

    await drain(await streamReply("c1", undefined, agent));

    expect(getReceivedOpts()?.mcpServers).toBeUndefined();
    expect(getReceivedOpts()?.disabledSkills).toBeUndefined();
    expect(getReceivedOpts()?.customSkills).toBeUndefined();
  });
});
