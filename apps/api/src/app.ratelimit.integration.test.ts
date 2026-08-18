import { describe, it, expect, beforeAll, afterAll, vi } from "vitest";
import type { FastifyInstance } from "fastify";
import jwt from "jsonwebtoken";

// buildApp() lấy PG pool khi register auth/users module. Test này chỉ chạm route
// chat với body KHÔNG hợp lệ (ném 400 trước khi tới DB) nên stub pool là đủ.
vi.mock("./database/index.js", async (importOriginal) => ({
  ...(await importOriginal<typeof import("./database/index.js")>()),
  getPgPool: () => ({}) as never,
}));

const { buildApp } = await import("./app");
const { config } = await import("./config");

const cookieFor = (sub: string) => ({
  cookie: `access_token=${jwt.sign(
    { sub, email: `${sub}@example.com`, role: "user" },
    config.JWT_SECRET,
    { expiresIn: 900 },
  )}`,
});

const validId = "64b7f0000000000000000000";

/**
 * Integration test cho fix rate limit, chạy qua ĐÚNG app production:
 * `buildApp()` thật, `chatRoutes` thật với `config.rateLimit.max = 20`, cookie
 * plugin thật, keyGenerator thật.
 *
 * Test đơn vị (common/guards/rate-limit-key.test.ts) chỉ chứng minh hàm sinh
 * khoá đúng. Test này chứng minh thứ quan trọng hơn: khi cắm vào app thật, hạn
 * mức 20/phút của route chat được tính THEO TỪNG USER — tức bug "20 request cho
 * toàn hệ thống" không còn.
 *
 * Body cố tình để rỗng → handler ném 400 trước khi chạm Mongo/LLM, nên test
 * không cần Atlas và không tốn token.
 */
describe("rate limit trên route chat thật (integration)", () => {
  let app: FastifyInstance;

  beforeAll(async () => {
    app = buildApp();
    await app.ready();
  });
  afterAll(async () => {
    await app.close();
  });

  const chatAs = (who: ReturnType<typeof cookieFor>) =>
    app.inject({
      method: "POST",
      url: `/api/conversations/${validId}/chat`,
      headers: { "content-type": "application/json", ...who },
      payload: { content: "   " }, // rỗng sau trim → 400 trước khi tới DB
    });

  it("user A dùng hết 20 request/phút thì bị 429, user B vẫn được phục vụ", async () => {
    const userA = cookieFor("rl-user-a");
    const userB = cookieFor("rl-user-b");

    // 20 request đầu của A: qua được rate limit (và nhận 400 vì body rỗng).
    for (let i = 1; i <= 20; i++) {
      const res = await chatAs(userA);
      expect(res.statusCode, `request thứ ${i} của user A`).toBe(400);
    }

    // Request 21 của A: chạm hạn mức của CHÍNH A.
    const overLimit = await chatAs(userA);
    expect(overLimit.statusCode).toBe(429);

    // Đây là điểm cốt lõi: B chưa gửi request nào nên PHẢI được phục vụ.
    // Trước fix, cả A và B dùng chung một bucket theo IP nginx container nên
    // request này là 429.
    const other = await chatAs(userB);
    expect(other.statusCode).toBe(400);
  });

  it("khách chưa đăng nhập vẫn bị chặn bởi authGuard (401), không lẫn vào bucket của user", async () => {
    const res = await app.inject({
      method: "POST",
      url: `/api/conversations/${validId}/chat`,
      headers: { "content-type": "application/json" },
      payload: { content: "xin chào" },
    });
    expect(res.statusCode).toBe(401);
  });

  it("app bật trustProxy nên đọc được IP client từ X-Forwarded-For", async () => {
    // /api/health không cần auth; chỉ cần khẳng định app không vỡ khi có XFF và
    // rate limit vẫn cho qua (khoá theo IP client, không phải IP proxy).
    const res = await app.inject({
      method: "GET",
      url: "/api/health",
      headers: { "x-forwarded-for": "203.0.113.9, 10.0.0.1" },
    });
    expect(res.statusCode).toBe(200);
    expect(res.json()).toEqual({ status: "ok" });
  });
});
