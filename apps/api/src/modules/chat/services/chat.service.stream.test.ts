import { describe, it, expect, vi, beforeEach } from "vitest";
import type { AgentEvent } from "../../../agent/graph/index";
import type { AgentClient, AgentMessage } from "../../../agent/client/index";

// Mock repository (DB). Agent được INJECT qua tham số (không cần mock module).
vi.mock("../repositories", () => ({
  createConversation: vi.fn(),
  listConversations: vi.fn(),
  getMessages: vi.fn(),
  addMessage: vi.fn(),
  appendToLastAssistantMessage: vi.fn(),
  deleteConversation: vi.fn(),
}));

import { streamReply, continueReply } from "./chat.service";
import { BadRequestError } from "../../../lib/errors";
import * as repo from "../repositories";

async function* fakeStream(events: AgentEvent[]): AsyncGenerator<AgentEvent> {
  for (const e of events) yield e;
}

// AgentClient giả: bỏ qua history, phát các event cấu hình sẵn.
const fakeAgent = (events: AgentEvent[]): AgentClient => ({
  stream: () => fakeStream(events),
});

// AgentClient giả CÓ GHI LẠI history nhận được — dùng để assert nội dung gửi
// cho agent (vd continueReply phải nối CONTINUE_INSTRUCTION vào cuối).
const capturingAgent = (
  events: AgentEvent[],
): { agent: AgentClient; getReceivedHistory: () => AgentMessage[] } => {
  let received: AgentMessage[] = [];
  return {
    agent: {
      stream: (history) => {
        received = history;
        return fakeStream(events);
      },
    },
    getReceivedHistory: () => received,
  };
};

// AgentClient giả CÓ GHI LẠI opts nhận được — dùng để assert lang được forward
// đúng từ streamReply xuống agent.stream (xem test "forward lang" bên dưới).
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

const drain = async (
  result: Awaited<ReturnType<typeof streamReply | typeof continueReply>>,
) => {
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
    expect(repo.addMessage).toHaveBeenCalledWith(
      "default",
      "c1",
      "assistant",
      "Xin chào",
    );

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
    expect(repo.addMessage).toHaveBeenCalledWith(
      "default",
      "c1",
      "assistant",
      "một phần",
    );
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
    expect(metadata.truncated).toBe(false);
  });

  it("forward event truncated và đánh dấu metadata.truncated", async () => {
    vi.mocked(repo.getMessages).mockResolvedValue([
      { role: "user", content: "viết dài vào" },
    ] as never);

    const result = await streamReply(
      "c1",
      undefined,
      fakeAgent([
        { type: "text", text: "một phần" },
        { type: "truncated", message: "Câu trả lời bị cắt" },
        { type: "done", agent: "go", tokens: 10, truncated: true },
      ]),
    );

    const { events, metadata } = await drain(result);

    expect(events.map((e) => e.type)).toContain("truncated");
    expect(metadata.truncated).toBe(true);
    // Phần text đã sinh vẫn được lưu.
    expect(repo.addMessage).toHaveBeenCalledWith(
      "default",
      "c1",
      "assistant",
      "một phần",
    );
  });

  // Tier 4: FE cần contextTokens/contextBudget (Go tính, gửi qua event done)
  // để tự gợi ý bắt đầu chat mới khi context lớn.
  it("metadata ghi nhận contextTokens + contextBudget từ event done", async () => {
    vi.mocked(repo.getMessages).mockResolvedValue([
      { role: "user", content: "hi" },
    ] as never);

    const result = await streamReply(
      "c1",
      undefined,
      fakeAgent([
        { type: "text", text: "Hello" },
        {
          type: "done",
          agent: "general",
          tokens: 42,
          contextTokens: 82000,
          contextBudget: 100000,
        },
      ]),
    );

    const { metadata } = await drain(result);

    expect(metadata.contextTokens).toBe(82000);
    expect(metadata.contextBudget).toBe(100000);
  });

  it("metadata.truncated = true khi chỉ có cờ trên event done", async () => {
    vi.mocked(repo.getMessages).mockResolvedValue([] as never);

    const result = await streamReply(
      "c1",
      undefined,
      fakeAgent([
        { type: "text", text: "cụt" },
        { type: "done", agent: "go", tokens: 1, truncated: true },
      ]),
    );

    const { metadata } = await drain(result);
    expect(metadata.truncated).toBe(true);
  });

  // FE cho phép chọn ngôn ngữ UI (vi/en) — streamReply phải forward lựa chọn
  // đó xuống agent.stream() để agent-go trả lời đúng ngôn ngữ.
  it("forward lang xuống agent.stream() khi được truyền", async () => {
    vi.mocked(repo.getMessages).mockResolvedValue([
      { role: "user", content: "hi" },
    ] as never);

    const { agent, getReceivedOpts } = optsCapturingAgent([
      { type: "text", text: "hello" },
      { type: "done" },
    ]);

    await drain(
      await streamReply("c1", undefined, agent, undefined, undefined, "en"),
    );

    expect(getReceivedOpts()?.lang).toBe("en");
  });

  it("lang là undefined khi không truyền (giữ hành vi mặc định)", async () => {
    vi.mocked(repo.getMessages).mockResolvedValue([
      { role: "user", content: "hi" },
    ] as never);

    const { agent, getReceivedOpts } = optsCapturingAgent([
      { type: "text", text: "hello" },
      { type: "done" },
    ]);

    await drain(await streamReply("c1", undefined, agent));

    expect(getReceivedOpts()?.lang).toBeUndefined();
  });
});

// Fix: nút "Tiếp tục" trước đây gọi thẳng streamReply với 1 user message mới
// ("Tiếp tục câu trả lời từ chỗ bị cắt.") — tạo ra 1 cặp user+assistant message
// HOÀN TOÀN MỚI trong DB, khiến code/văn bản dài bị TÁCH ĐÔI vĩnh viễn (thấy
// lại y hệt sau khi F5 trang, vì DB lưu tách rời). continueReply khác:
// (1) KHÔNG persist user turn mới (không gọi appendUserMessage/addMessage cho
// role=user), (2) response được NỐI vào cuối assistant message CŨ qua
// appendToLastAssistantMessage thay vì tạo message mới.
describe("continueReply", () => {
  beforeEach(() => vi.clearAllMocks());

  it("nối text mới vào assistant message CŨ qua appendToLastAssistantMessage, KHÔNG tạo message mới", async () => {
    vi.mocked(repo.getMessages).mockResolvedValue([
      { role: "user", content: "build landing page" },
      { role: "assistant", content: "<html>...(bị cắt)" },
    ] as never);

    const result = await continueReply(
      "c1",
      undefined,
      fakeAgent([
        { type: "text", text: "phần còn lại</html>" },
        { type: "done", agent: "code", tokens: 99 },
      ]),
    );
    await drain(result);

    expect(repo.appendToLastAssistantMessage).toHaveBeenCalledWith(
      "default",
      "c1",
      "phần còn lại</html>",
    );
    expect(repo.addMessage).not.toHaveBeenCalled();
  });

  it("gửi agent history kết thúc bằng instruction tiếp tục (không phải user message thật)", async () => {
    vi.mocked(repo.getMessages).mockResolvedValue([
      { role: "user", content: "build landing page" },
      { role: "assistant", content: "<html>...(bị cắt)" },
    ] as never);

    const { agent, getReceivedHistory } = capturingAgent([
      { type: "text", text: "tiếp" },
      { type: "done" },
    ]);

    await drain(await continueReply("c1", undefined, agent));

    const history = getReceivedHistory();
    // 2 message gốc PHẢI còn nguyên (kể cả assistant message bị cắt — model
    // cần thấy để biết tiếp tục từ đâu), cộng thêm đúng 1 instruction cuối.
    expect(history).toHaveLength(3);
    expect(history[0]).toEqual({
      role: "user",
      content: "build landing page",
    });
    expect(history[1]).toEqual({
      role: "assistant",
      content: "<html>...(bị cắt)",
    });
    expect(history[2].role).toBe("user");
    // Instruction không được đơn giản lặp lại text hiển thị cho user cũ
    // ("Tiếp tục câu trả lời từ chỗ bị cắt.") — đây là văn bản NỘI BỘ mới,
    // không bao giờ hiển thị/lưu, nên không cần khớp chuỗi cũ, chỉ cần khác
    // rỗng và có nhắc "tiếp tục"/"cắt" để model hiểu đúng ý.
    expect(history[2].content.length).toBeGreaterThan(0);
    expect(history[2].content.toLowerCase()).toContain("tiếp tục");
  });

  it("ném BadRequestError khi hội thoại trống (không có gì để tiếp tục)", async () => {
    vi.mocked(repo.getMessages).mockResolvedValue([] as never);

    await expect(
      continueReply("c1", undefined, fakeAgent([])),
    ).rejects.toBeInstanceOf(BadRequestError);
    expect(repo.appendToLastAssistantMessage).not.toHaveBeenCalled();
  });

  it("ném BadRequestError khi tin nhắn cuối không phải của assistant", async () => {
    vi.mocked(repo.getMessages).mockResolvedValue([
      { role: "user", content: "hi" },
      { role: "assistant", content: "chào bạn" },
      { role: "user", content: "còn nữa không" },
    ] as never);

    await expect(
      continueReply("c1", undefined, fakeAgent([])),
    ).rejects.toBeInstanceOf(BadRequestError);
  });

  it("không nối gì khi text sinh ra rỗng (agent lỗi/không trả chữ nào)", async () => {
    vi.mocked(repo.getMessages).mockResolvedValue([
      { role: "user", content: "hi" },
      { role: "assistant", content: "chào" },
    ] as never);

    await drain(
      await continueReply("c1", undefined, fakeAgent([{ type: "done" }])),
    );

    expect(repo.appendToLastAssistantMessage).not.toHaveBeenCalled();
  });

  it("metadata ghi nhận contextTokens/contextBudget/truncated giống streamReply", async () => {
    vi.mocked(repo.getMessages).mockResolvedValue([
      { role: "user", content: "hi" },
      { role: "assistant", content: "chào" },
    ] as never);

    const result = await continueReply(
      "c1",
      undefined,
      fakeAgent([
        { type: "text", text: " tiếp" },
        {
          type: "done",
          agent: "code",
          tokens: 5,
          truncated: true,
          contextTokens: 91000,
          contextBudget: 100000,
        },
      ]),
    );

    const { metadata } = await drain(result);
    expect(metadata.backend).toBe("code");
    expect(metadata.truncated).toBe(true);
    expect(metadata.contextTokens).toBe(91000);
    expect(metadata.contextBudget).toBe(100000);
  });
});
