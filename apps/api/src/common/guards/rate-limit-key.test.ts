import { describe, it, expect } from "vitest";
import type { FastifyRequest } from "fastify";
import jwt from "jsonwebtoken";
import { rateLimitKeyGenerator } from "./rate-limit-key";
import { config } from "../../config";

// Dựng request giả tối thiểu — keyGenerator chỉ đọc cookies + ip.
const fakeReq = (cookies: Record<string, string>, ip = "203.0.113.9") =>
  ({ cookies, ip }) as unknown as FastifyRequest;

const tokenFor = (sub: string, secret = config.JWT_SECRET) =>
  jwt.sign({ sub, email: `${sub}@example.com`, role: "user" }, secret, {
    expiresIn: 900,
  });

describe("rateLimitKeyGenerator", () => {
  it("hai user khác nhau → hai khoá khác nhau (bucket riêng)", () => {
    const a = rateLimitKeyGenerator(
      fakeReq({ access_token: tokenFor("user-a") }),
    );
    const b = rateLimitKeyGenerator(
      fakeReq({ access_token: tokenFor("user-b") }),
    );

    expect(a).toBe("user:user-a");
    expect(b).toBe("user:user-b");
    expect(a).not.toBe(b);
  });

  it("cùng user nhưng khác IP → vẫn cùng một khoá", () => {
    const wifi = rateLimitKeyGenerator(
      fakeReq({ access_token: tokenFor("user-a") }, "203.0.113.9"),
    );
    const mobile = rateLimitKeyGenerator(
      fakeReq({ access_token: tokenFor("user-a") }, "198.51.100.7"),
    );

    // Đổi mạng không được cấp thêm hạn mức mới.
    expect(wifi).toBe(mobile);
  });

  it("chưa đăng nhập → khoá theo IP", () => {
    expect(rateLimitKeyGenerator(fakeReq({}, "198.51.100.7"))).toBe(
      "ip:198.51.100.7",
    );
  });

  it("token ký bằng secret khác → KHÔNG được cấp bucket riêng", () => {
    // Nếu chỉ decode mà không verify, ai cũng tự bơm `sub` để reset hạn mức.
    const forged = tokenFor("victim", "secret-cua-ke-tan-cong-khong-hop-le");
    expect(
      rateLimitKeyGenerator(fakeReq({ access_token: forged }, "198.51.100.7")),
    ).toBe("ip:198.51.100.7");
  });

  it("token hết hạn → rơi về IP, không ném lỗi", () => {
    const expired = jwt.sign({ sub: "user-a" }, config.JWT_SECRET, {
      expiresIn: -10,
    });
    expect(rateLimitKeyGenerator(fakeReq({ access_token: expired }))).toBe(
      "ip:203.0.113.9",
    );
  });

  it("cookie rỗng/không có cookies → khoá theo IP, không crash", () => {
    const noCookies = { ip: "203.0.113.9" } as unknown as FastifyRequest;
    expect(rateLimitKeyGenerator(noCookies)).toBe("ip:203.0.113.9");
  });
});
