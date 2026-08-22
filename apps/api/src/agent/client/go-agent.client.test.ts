import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type { AgentEvent } from "../graph/index";

import { goAgentClient } from "./go-agent.client";
import { AgentUnavailableError } from "../../lib/errors";

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

  // Tier 4: FE cần contextTokens/contextBudget để tự gợi ý bắt đầu chat mới
  // khi context lớn — trước fix, 2 field này không nằm trong danh sách được
  // forward ở nhánh case "done" nên luôn undefined dù Go gửi đúng.
  it("event done forward contextTokens + contextBudget", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        sseResponse([
          JSON.stringify({ type: "text", text: "hi" }),
          JSON.stringify({
            type: "done",
            contextTokens: 82000,
            contextBudget: 100000,
          }),
        ]),
      ),
    );

    const events = await drain(goAgentClient.stream(history));
    const done = events.find((e) => e.type === "done") as
      { contextTokens?: number; contextBudget?: number } | undefined;

    expect(done?.contextTokens).toBe(82000);
    expect(done?.contextBudget).toBe(100000);
  });

  // FE cho phép chọn ngôn ngữ UI (vi/en) — goAgentClient phải forward lựa chọn
  // đó sang agent-go trong body JSON để JARVIS trả lời đúng ngôn ngữ.
  it("forward opts.lang trong body JSON gửi lên agent-go", async () => {
    const fetchMock = vi.fn(async () =>
      sseResponse([JSON.stringify({ type: "done" })]),
    );
    vi.stubGlobal("fetch", fetchMock);

    await drain(goAgentClient.stream(history, { lang: "en" }));

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [, init] = fetchMock.mock.calls[0] as unknown as [
      string,
      RequestInit,
    ];
    const sentBody = JSON.parse(init.body as string);
    expect(sentBody.lang).toBe("en");
  });

  // Khi opts.lang không được truyền, KHÔNG được gửi field lang méo (undefined)
  // — giữ hành vi mặc định tiếng Việt phía agent-go (json tag omitempty).
  it("không gửi lang khi opts.lang không được truyền", async () => {
    const fetchMock = vi.fn(async () =>
      sseResponse([JSON.stringify({ type: "done" })]),
    );
    vi.stubGlobal("fetch", fetchMock);

    await drain(goAgentClient.stream(history));

    const [, init] = fetchMock.mock.calls[0] as unknown as [
      string,
      RequestInit,
    ];
    const sentBody = JSON.parse(init.body as string);
    expect(sentBody.lang).toBeUndefined();
  });

  // Sửa lỗi "user tự thêm MCP server không dùng được với server thật": token
  // (auth_token) + transport phải THỰC SỰ có mặt trong body JSON gửi lên
  // agent-go — chứ không chỉ được lưu trong DB rồi không bao giờ tới nơi cần.
  it("forward mcpServers (kèm apiKey + transport) trong body JSON gửi lên agent-go", async () => {
    const fetchMock = vi.fn(async () =>
      sseResponse([JSON.stringify({ type: "done" })]),
    );
    vi.stubGlobal("fetch", fetchMock);

    const mcpServers = [
      {
        name: "notion",
        url: "https://mcp.notion.com",
        apiKey: "secret-notion-token",
        transport: "http" as const,
      },
    ];

    await drain(goAgentClient.stream(history, { mcpServers }));

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [, init] = fetchMock.mock.calls[0] as unknown as [
      string,
      RequestInit,
    ];
    const sentBody = JSON.parse(init.body as string);
    expect(sentBody.mcpServers).toEqual(mcpServers);
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

describe("goAgentClient.resume — proxy POST /chat/resume", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("gọi đúng URL /chat/resume với body {run_id, answer} và header X-Tenant-ID", async () => {
    const fetchMock = vi.fn(async () =>
      sseResponse([JSON.stringify({ type: "text", text: "tiếp tục" })]),
    );
    vi.stubGlobal("fetch", fetchMock);

    await drain(
      goAgentClient.resume!("run-123", "yes", { tenantId: "tenant-a" }),
    );

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0] as unknown as [
      string,
      RequestInit,
    ];
    expect(url).toMatch(/\/chat\/resume$/);
    expect((init.headers as Record<string, string>)["X-Tenant-ID"]).toBe(
      "tenant-a",
    );
    expect(JSON.parse(init.body as string)).toEqual({
      run_id: "run-123",
      answer: "yes",
    });
  });

  it("answer là undefined khi không truyền (resume không cần trả lời interrupt)", async () => {
    const fetchMock = vi.fn(async () =>
      sseResponse([JSON.stringify({ type: "done" })]),
    );
    vi.stubGlobal("fetch", fetchMock);

    await drain(goAgentClient.resume!("run-456", undefined));

    const [, init] = fetchMock.mock.calls[0] as unknown as [
      string,
      RequestInit,
    ];
    const body = JSON.parse(init.body as string);
    expect(body.run_id).toBe("run-456");
    expect(body.answer).toBeUndefined();
  });

  it("dùng CHUNG logic map event với stream() — forward event interrupt kèm runId", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        sseResponse([
          JSON.stringify({ type: "text", text: "phần còn lại" }),
          JSON.stringify({
            type: "done",
            usage: { inputTokens: 3, outputTokens: 7 },
          }),
        ]),
      ),
    );

    const events = await drain(goAgentClient.resume!("run-789", "ok"));
    const done = events.find((e) => e.type === "done") as
      { totalTokens?: number } | undefined;
    expect(done?.totalTokens).toBe(10);
  });

  it("ném AgentUnavailableError khi Go trả lỗi (vd 404 — run_id không tồn tại/đã resume rồi), KHÔNG retry", async () => {
    const fetchMock = vi.fn(async () => new Response(null, { status: 404 }));
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      drain(goAgentClient.resume!("run-not-found", undefined)),
    ).rejects.toBeInstanceOf(AgentUnavailableError);

    // Khác stream(): resume KHÔNG retry khi lỗi trước response, vì agent-go
    // đã xoá checkpoint sau lần load đầu — gọi lại chỉ nhận lại đúng lỗi này.
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });
});
