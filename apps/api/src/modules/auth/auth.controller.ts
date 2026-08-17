import type { FastifyRequest, FastifyReply } from "fastify";
import type { AuthService } from "./auth.service";
import type { JwtStrategy } from "./strategies/jwt.strategy";
import { validate } from "../../common/pipes/validation.pipe";
import { registerSchema } from "./dto/register.dto";
import { loginSchema } from "./dto/login.dto";
import { verifyEmailSchema } from "./dto/verify-email.dto";
import { resendOtpSchema } from "./dto/resend-otp.dto";
import { config } from "../../config";

export class AuthController {
  constructor(
    private authService: AuthService,
    private jwt: JwtStrategy,
  ) {}

  /** POST /api/auth/register — tạo user (chưa verify) + gửi OTP, KHÔNG cấp token. */
  register = async (req: FastifyRequest, reply: FastifyReply) => {
    const input = validate(registerSchema, req.body);
    const result = await this.authService.register(input);
    return reply.status(201).send({ email: result.email });
  };

  /** POST /api/auth/verify-email — xác minh OTP, cấp token (đăng nhập lần đầu). */
  verifyEmail = async (req: FastifyRequest, reply: FastifyReply) => {
    const input = validate(verifyEmailSchema, req.body);
    const result = await this.authService.verifyEmail(input);

    this.jwt.setAccessTokenCookie(reply, result.accessToken);
    this.jwt.setRefreshTokenCookie(reply, result.refreshToken);

    return reply.status(200).send({ user: result.user });
  };

  /** POST /api/auth/resend-otp */
  resendOtp = async (req: FastifyRequest, reply: FastifyReply) => {
    const input = validate(resendOtpSchema, req.body);
    await this.authService.resendOtp(input.email);
    return reply.status(200).send({ message: "OTP đã được gửi lại." });
  };

  /** POST /api/auth/login */
  login = async (req: FastifyRequest, reply: FastifyReply) => {
    const input = validate(loginSchema, req.body);
    const result = await this.authService.login(input);

    this.jwt.setAccessTokenCookie(reply, result.accessToken);
    this.jwt.setRefreshTokenCookie(reply, result.refreshToken);

    return reply.status(200).send({ user: result.user });
  };

  /** GET /api/auth/google — redirect đến Google OAuth */
  googleRedirect = async (_req: FastifyRequest, reply: FastifyReply) => {
    const url = this.authService.getGoogleAuthUrl();
    return reply.redirect(url);
  };

  /** GET /api/auth/google/callback — Google gọi về sau khi user đồng ý */
  googleCallback = async (req: FastifyRequest, reply: FastifyReply) => {
    const { code } = req.query as { code?: string };
    if (!code) {
      return reply.status(400).send({ error: "Thiếu authorization code." });
    }

    const result = await this.authService.googleLogin(code);

    this.jwt.setAccessTokenCookie(reply, result.accessToken);
    this.jwt.setRefreshTokenCookie(reply, result.refreshToken);

    return reply.redirect(`${config.FRONTEND_URL}/`);
  };

  /** POST /api/auth/refresh — cấp access token mới từ refresh token */
  refresh = async (req: FastifyRequest, reply: FastifyReply) => {
    const token = req.cookies?.refresh_token;
    if (!token) {
      return reply.status(401).send({ error: "Thiếu refresh token." });
    }

    const result = await this.authService.refreshAccessToken(token);

    this.jwt.setAccessTokenCookie(reply, result.accessToken);
    this.jwt.setRefreshTokenCookie(reply, result.refreshToken);

    return reply.status(200).send({ user: result.user });
  };

  /** POST /api/auth/logout — xoá refresh token + clear cookies */
  logout = async (req: FastifyRequest, reply: FastifyReply) => {
    const token = req.cookies?.refresh_token;
    if (token) {
      await this.authService.logout(token);
    }
    this.jwt.clearTokens(reply);
    return reply.status(200).send({ message: "Đã đăng xuất." });
  };

  /** GET /api/auth/me — thông tin user hiện tại (cần authGuard) */
  me = async (req: FastifyRequest, reply: FastifyReply) => {
    return reply.status(200).send({ user: req.user });
  };
}
