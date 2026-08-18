import type { FastifyInstance } from "fastify";
import type { Pool } from "pg";
import { AuthRepository } from "./auth.repository";
import { JwtStrategy } from "./strategies/jwt.strategy";
import { TokenService } from "./strategies/token.service";
import { GoogleStrategy } from "./strategies/google.strategy";
import { AuthService } from "./auth.service";
import { AuthController } from "./auth.controller";
import { OtpService } from "./otp.service";
import { authGuard } from "../../common/guards/auth.guard";

export interface AuthModuleOptions {
  pgPool: Pool;
}

/**
 * Auth module — Fastify plugin đóng gói toàn bộ auth routes.
 *
 * Yêu cầu: PostgreSQL phải được init trước khi buildApp().
 * server.ts đã đảm bảo thứ tự này (init DB → buildApp → listen).
 *
 * Register trong app.ts:
 *   app.register(authModule, { pgPool: getPgPool() });
 */
export const authModule = async (
  app: FastifyInstance,
  opts: AuthModuleOptions,
): Promise<void> => {
  // ── Wire dependencies ──
  const repo = new AuthRepository(opts.pgPool);
  const jwt = new JwtStrategy();
  const tokenService = new TokenService(repo, jwt);
  const google = new GoogleStrategy();
  const otp = new OtpService();
  const authService = new AuthService(repo, tokenService, google, otp);
  const controller = new AuthController(authService, jwt);

  // ── Public routes ──
  app.post("/api/auth/register", controller.register);
  app.post("/api/auth/verify-email", controller.verifyEmail);
  app.post("/api/auth/resend-otp", controller.resendOtp);
  app.post("/api/auth/forgot-password", controller.forgotPassword);
  app.post("/api/auth/reset-password", controller.resetPassword);
  app.post("/api/auth/login", controller.login);
  app.get("/api/auth/google", controller.googleRedirect);
  app.get("/api/auth/google/callback", controller.googleCallback);
  app.post("/api/auth/refresh", controller.refresh);
  app.post("/api/auth/logout", controller.logout);

  // ── Protected routes ──
  app.get("/api/auth/me", { preHandler: [authGuard] }, controller.me);
};
