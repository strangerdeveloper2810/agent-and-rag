import { describe, it, expect, beforeAll, afterAll } from "vitest";
import Fastify, { type FastifyInstance } from "fastify";
import cookie from "@fastify/cookie";
import rateLimit from "@fastify/rate-limit";
import jwt from "jsonwebtoken";
import { rateLimitKeyGenerator } from "./rate-limit-key";
import { config } from "../../config";

/**
 * Cả bản fix rate limit dựa trên một giả định về @fastify/rate-limit: hạn mức
 * RIÊNG của route (`config: { rateLimit: { max: 20 } }` ở chat.routes.ts) có kế
 * thừa `keyGenerator` khai báo ở cấp plugin hay không.
 *
 * Nếu KHÔNG kế thừa thì route chat vẫn khoá theo IP và mọi user vẫn dùng chung
 * một bucket — đúng cái bug đang sửa, chỉ là ẩn hơn. Nên phải test thật.
 *
 * Dựng app tối giản mô phỏng đúng thứ tự register của app.ts (cookie TRƯỚC
 * rateLimit — bắt buộc, vì keyGenerator đọc req.cookies).
 */
const buildTestApp = async () => {
  const app = Fastify({ logger: false, trustProxy: 2 });
  await app.register(cookie);
  // PHẢI await: hạn mức riêng của route được đọc qua hook `onRoute` của plugin,
  // nên route thêm ĐỒNG BỘ trước khi plugin register xong sẽ bị bỏ qua config
  // và chỉ chịu hạn mức toàn cục. (app.ts không gặp chuyện này vì route ở đó
  // được đăng ký qua plugin, tức xếp sau rate-limit trong hàng đợi.)
  await app.register(rateLimit, {
    max: 100,
    timeWindow: "1 minute",
    keyGenerator: rateLimitKeyGenerator,
  });
  // max: 1 để chỉ cần 2 request là thấy được hành vi.
  app.get(
    "/chat",
    { config: { rateLimit: { max: 1, timeWindow: "1 minute" } } },
    async () => ({
      ok: true,
    }),
  );
  await app.ready();
  return app;
};

const cookieFor = (sub: string) => ({
  cookie: `access_token=${jwt.sign(
    { sub, email: `${sub}@example.com`, role: "user" },
    config.JWT_SECRET,
    { expiresIn: 900 },
  )}`,
});

describe("rate limit theo route + keyGenerator", () => {
  let app: FastifyInstance;

  beforeAll(async () => {
    app = await buildTestApp();
  });
  afterAll(async () => {
    await app.close();
  });

  it("hạn mức riêng của route vẫn tính theo TỪNG user, không dùng chung bucket", async () => {
    const userA = cookieFor("user-a");
    const userB = cookieFor("user-b");

    const a1 = await app.inject({
      method: "GET",
      url: "/chat",
      headers: userA,
    });
    const a2 = await app.inject({
      method: "GET",
      url: "/chat",
      headers: userA,
    });

    expect(a1.statusCode).toBe(200);
    // User A đã dùng hết hạn mức của CHÍNH MÌNH.
    expect(a2.statusCode).toBe(429);

    // Điểm quan trọng: user B chưa gửi gì nên PHẢI được phục vụ. Trước fix,
    // request này là 429 vì cả hệ thống dùng chung một bucket.
    const b1 = await app.inject({
      method: "GET",
      url: "/chat",
      headers: userB,
    });
    expect(b1.statusCode).toBe(200);
  });

  it("khách chưa đăng nhập được tách bucket theo IP", async () => {
    const app2 = await buildTestApp();

    // Mô phỏng đúng chuỗi production: reverse proxy ngoài cùng ghi IP client vào
    // X-Forwarded-For, nginx container nối thêm IP của reverse proxy, và socket
    // mà Fastify thấy là nginx container (trong inject là 127.0.0.1).
    // Ba hop → trustProxy: 2 tin 2 hop gần server nhất và lấy IP client.
    const from = (clientIp: string) => ({
      "x-forwarded-for": `${clientIp}, 10.0.0.1`,
    });

    const first = await app2.inject({
      method: "GET",
      url: "/chat",
      headers: from("203.0.113.9"),
    });
    const second = await app2.inject({
      method: "GET",
      url: "/chat",
      headers: from("203.0.113.9"),
    });
    const otherIp = await app2.inject({
      method: "GET",
      url: "/chat",
      headers: from("198.51.100.7"),
    });

    expect(first.statusCode).toBe(200);
    expect(second.statusCode).toBe(429);
    expect(otherIp.statusCode).toBe(200);

    await app2.close();
  });
});
