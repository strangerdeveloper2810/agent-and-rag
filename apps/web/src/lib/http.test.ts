import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { api } from "./http";

// Bug: api.del() (xoá MCP server, xoá skill) luôn set Content-Type:
// application/json dù KHÔNG gửi body. Fastify mặc định reject request có
// Content-Type: application/json nhưng body rỗng bằng FST_ERR_CTP_EMPTY_JSON_BODY
// (400) — xảy ra TRƯỚC route handler, nên logic controller/service đúng vẫn
// vô dụng. Production log xác nhận: mọi DELETE /api/user/mcp-servers/:id đều
// trả 400 dù PATCH cùng id vẫn 200 OK bình thường (PATCH luôn có body thật).
describe("api client — Content-Type chỉ set khi có body", () => {
  const originalFetch = globalThis.fetch;

  beforeEach(() => {
    globalThis.fetch = vi
      .fn()
      .mockResolvedValue(new Response(null, { status: 204 }));
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
  });

  it("del() KHÔNG gửi Content-Type (không có body)", async () => {
    await api.del("/api/user/mcp-servers/abc");

    const [, init] = (globalThis.fetch as ReturnType<typeof vi.fn>).mock
      .calls[0];
    const headers = new Headers(init.headers);
    expect(headers.has("Content-Type")).toBe(false);
  });

  it("post() vẫn gửi Content-Type: application/json (có body)", async () => {
    globalThis.fetch = vi
      .fn()
      .mockResolvedValue(new Response(JSON.stringify({}), { status: 200 }));

    await api.post("/api/user/mcp-servers", { name: "x" });

    const [, init] = (globalThis.fetch as ReturnType<typeof vi.fn>).mock
      .calls[0];
    const headers = new Headers(init.headers);
    expect(headers.get("Content-Type")).toBe("application/json");
  });

  it("patch() vẫn gửi Content-Type: application/json (có body)", async () => {
    globalThis.fetch = vi
      .fn()
      .mockResolvedValue(new Response(JSON.stringify({}), { status: 200 }));

    await api.patch("/api/user/mcp-servers/abc", { enabled: false });

    const [, init] = (globalThis.fetch as ReturnType<typeof vi.fn>).mock
      .calls[0];
    const headers = new Headers(init.headers);
    expect(headers.get("Content-Type")).toBe("application/json");
  });
});
