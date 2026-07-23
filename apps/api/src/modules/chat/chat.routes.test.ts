import { describe, it, expect, beforeAll, afterAll } from "vitest";
import type { FastifyInstance } from "fastify";
import { buildApp } from "../../app";

// Test biên HTTP: các đường lỗi (validate/ObjectId) phải trả 400 và KHÔNG chạm
// DB (các handler này ném trước khi gọi Mongo) → chạy được không cần Atlas.
describe("chat routes - validation", () => {
  let app: FastifyInstance;

  beforeAll(async () => {
    app = buildApp();
    await app.ready();
  });
  afterAll(async () => {
    await app.close();
  });

  const validId = "64b7f0000000000000000000";

  it("POST /conversations/:id/chat với content rỗng → 400", async () => {
    const res = await app.inject({
      method: "POST",
      url: `/api/conversations/${validId}/chat`,
      headers: { "content-type": "application/json" },
      payload: { content: "   " },
    });
    expect(res.statusCode).toBe(400);
    expect(res.json().error).toContain("content");
  });

  it("POST /conversations/:id/chat thiếu content → 400", async () => {
    const res = await app.inject({
      method: "POST",
      url: `/api/conversations/${validId}/chat`,
      headers: { "content-type": "application/json" },
      payload: {},
    });
    expect(res.statusCode).toBe(400);
  });

  it("DELETE /conversations/:id với id sai định dạng → 400 (không phải 500)", async () => {
    const res = await app.inject({
      method: "DELETE",
      url: "/api/conversations/not-a-valid-id",
    });
    expect(res.statusCode).toBe(400);
    expect(res.json().error).toContain("id");
  });
});
