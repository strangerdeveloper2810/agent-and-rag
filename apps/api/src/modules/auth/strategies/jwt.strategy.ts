import jwt from "jsonwebtoken";
import crypto from "crypto";
import type { FastifyReply } from "fastify";
import { config } from "../../../config";
import type { JwtPayload } from "../../../common/interfaces/auth-context";
import type { UserRow } from "../auth.repository";

/**
 * JWT Strategy — sign, verify, cookie setters.
 *
 * Access token: JWT, 15 phút, scope rộng (toàn bộ API).
 * Refresh token: random 48 bytes (base64url), SHA-256 hash lưu DB.
 */
export class JwtStrategy {
  private readonly secret: string;
  private readonly accessExpiry: number; // seconds
  private readonly refreshExpiry: number; // seconds

  constructor() {
    this.secret = config.JWT_SECRET;
    this.accessExpiry = 15 * 60; // 15 phút
    this.refreshExpiry = 7 * 24 * 60 * 60; // 7 ngày
  }

  // ── Token generation ──

  /** Tạo access token JWT (15 phút). */
  generateAccessToken(user: UserRow): string {
    return jwt.sign(
      { sub: user.id, email: user.email, role: user.role } satisfies JwtPayload,
      this.secret,
      { expiresIn: this.accessExpiry },
    );
  }

  /** Tạo refresh token (random) + SHA-256 hash để lưu DB. */
  generateRefreshToken(): { token: string; hash: string } {
    const token = crypto.randomBytes(48).toString("base64url");
    const hash = crypto.createHash("sha256").update(token).digest("hex");
    return { token, hash };
  }

  // ── Cookie setters ──

  setAccessTokenCookie(reply: FastifyReply, token: string): void {
    reply.setCookie("access_token", token, {
      httpOnly: true,
      secure: config.NODE_ENV === "production",
      sameSite: "lax",
      path: "/",
      maxAge: this.accessExpiry,
    });
  }

  setRefreshTokenCookie(reply: FastifyReply, token: string): void {
    reply.setCookie("refresh_token", token, {
      httpOnly: true,
      secure: config.NODE_ENV === "production",
      sameSite: "lax",
      path: "/api/auth", // chỉ gửi đến auth endpoints
      maxAge: this.refreshExpiry,
    });
  }

  clearTokens(reply: FastifyReply): void {
    reply.clearCookie("access_token", { path: "/" });
    reply.clearCookie("refresh_token", { path: "/api/auth" });
  }

  // ── Verify ──

  verify(token: string): JwtPayload {
    return jwt.verify(token, this.secret) as JwtPayload;
  }
}
