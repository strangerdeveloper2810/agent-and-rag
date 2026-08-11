import { describe, it, expect, beforeAll, afterAll, vi } from "vitest";
import type { FastifyInstance } from "fastify";
import jwt from "jsonwebtoken";

// buildApp() lấy PG pool ngay lúc register auth/users module. Các test dưới đây
// chỉ chạm route chat (ném lỗi validate trước khi tới DB), nên stub pool.
vi.mock("../../database/index.js", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../database/index.js")>()),
  getPgPool: () => ({}) as never,
}));

const { buildApp } = await import("../../app");
const { config } = await import("../../config");

// Chat routes nằm sau authGuard → mọi request phải kèm cookie access_token hợp lệ.
// Ký token bằng đúng JWT_SECRET của test env (khai báo trong vitest.config.ts).
const accessToken = jwt.sign(
  { sub: "64b7f0000000000000000001", email: "test@example.com", role: "user" },
  config.JWT_SECRET,
  { expiresIn: 900 },
);
const authCookie = { cookie: `access_token=${accessToken}` };

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
      headers: { "content-type": "application/json", ...authCookie },
      payload: { content: "   " },
    });
    expect(res.statusCode).toBe(400);
    expect(res.json().error).toContain("content");
  });

  it("POST /conversations/:id/chat thiếu content → 400", async () => {
    const res = await app.inject({
      method: "POST",
      url: `/api/conversations/${validId}/chat`,
      headers: { "content-type": "application/json", ...authCookie },
      payload: {},
    });
    expect(res.statusCode).toBe(400);
  });

  it("DELETE /conversations/:id với id sai định dạng → 400 (không phải 500)", async () => {
    const res = await app.inject({
      method: "DELETE",
      url: "/api/conversations/not-a-valid-id",
      headers: { ...authCookie },
    });
    expect(res.statusCode).toBe(400);
    expect(res.json().error).toContain("id");
    expect(res.json().code).toBe("BAD_REQUEST"); // response chuẩn hoá { error, code }
  });

  it("thiếu access_token → 401", async () => {
    const res = await app.inject({
      method: "DELETE",
      url: `/api/conversations/${validId}`,
    });
    expect(res.statusCode).toBe(401);
  });
});
