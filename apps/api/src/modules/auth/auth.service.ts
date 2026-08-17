import bcrypt from "bcrypt";
import crypto from "crypto";
import type { AuthRepository, UserRow } from "./auth.repository";
import { TokenService } from "./strategies/token.service";
import { GoogleStrategy } from "./strategies/google.strategy";
import { OtpService } from "./otp.service";
import { sendOtpEmail } from "../../common/email/email.service";
import {
  ConflictError,
  UnauthorizedError,
  ForbiddenError,
  ValidationError,
  EmailNotVerifiedError,
} from "../../common/errors/app-errors";
import { RateLimitError } from "../../lib/errors";
import type { RegisterInput } from "./dto/register.dto";
import type { LoginInput } from "./dto/login.dto";
import type { VerifyEmailInput } from "./dto/verify-email.dto";

const BCRYPT_ROUNDS = 12;

export class AuthService {
  constructor(
    private repo: AuthRepository,
    private tokenService: TokenService,
    private google: GoogleStrategy,
    private otp: OtpService,
  ) {}

  // ── Email/Password Register (gửi OTP, KHÔNG cấp token ngay) ──

  async register(input: RegisterInput): Promise<{ email: string }> {
    const existing = await this.repo.findUserByEmail(input.email);
    const hash = await bcrypt.hash(input.password, BCRYPT_ROUNDS);

    let user: UserRow;
    if (existing) {
      if (existing.email_verified) {
        throw new ConflictError("Email đã được đăng ký.");
      }
      // Email tồn tại nhưng chưa verify → coi như đăng ký lại, ghi đè tên + mật khẩu.
      user = await this.repo.updateUserForReregister(existing.id, input.name);
      await this.repo.updateEmailCredential(existing.id, hash);
    } else {
      user = await this.repo.createUser(input.email, input.name);
      await this.repo.createEmailCredential(user.id, hash);
    }

    const otpCode = await this.otp.issue(user.email);
    await sendOtpEmail(user.email, user.name, otpCode);

    return { email: user.email };
  }

  // ── Xác minh email bằng OTP → cấp token (đăng nhập lần đầu) ──

  async verifyEmail(
    input: VerifyEmailInput,
  ): Promise<{ user: UserRow; accessToken: string; refreshToken: string }> {
    const user = await this.repo.findUserByEmail(input.email);
    if (!user) {
      throw new ValidationError({ otp: ["Email hoặc OTP không hợp lệ."] });
    }

    const result = await this.otp.verify(input.email, input.otp);
    if (result === "expired") {
      throw new ValidationError({ otp: ["OTP đã hết hạn, vui lòng gửi lại."] });
    }
    if (result === "locked") {
      throw new ValidationError({
        otp: ["OTP không hợp lệ, vui lòng gửi lại."],
      });
    }
    if (result === "invalid") {
      throw new ValidationError({ otp: ["OTP không đúng."] });
    }

    await this.repo.updateEmailVerified(user.id);
    user.email_verified = true;

    const tokens = await this.tokenService.issueTokens(user);
    return {
      user,
      accessToken: tokens.accessToken,
      refreshToken: tokens.refreshToken,
    };
  }

  // ── Gửi lại OTP (tôn trọng cooldown) ──

  async resendOtp(email: string): Promise<void> {
    const user = await this.repo.findUserByEmail(email);
    if (!user) {
      throw new ValidationError({ email: ["Email không tồn tại."] });
    }
    if (user.email_verified) {
      throw new ValidationError({ email: ["Email đã được xác minh."] });
    }

    const remaining = await this.otp.cooldownRemaining(email);
    if (remaining > 0) {
      throw new RateLimitError(
        `Vui lòng đợi ${remaining} giây trước khi gửi lại.`,
        remaining,
      );
    }

    const otpCode = await this.otp.issue(email);
    await sendOtpEmail(email, user.name, otpCode);
  }

  // ── Email/Password Login ──

  async login(
    input: LoginInput,
  ): Promise<{ user: UserRow; accessToken: string; refreshToken: string }> {
    const user = await this.repo.findUserByEmail(input.email);
    if (!user || user.status === "deleted") {
      throw new UnauthorizedError("Email hoặc mật khẩu không đúng.");
    }

    if (user.status === "disabled") {
      throw new ForbiddenError("Tài khoản đã bị vô hiệu hoá. Liên hệ hỗ trợ.");
    }

    const cred = await this.repo.findCredential(user.id, "email");
    if (!cred?.password_hash) {
      throw new UnauthorizedError("Email hoặc mật khẩu không đúng.");
    }

    const valid = await bcrypt.compare(input.password, cred.password_hash);
    if (!valid) {
      throw new UnauthorizedError("Email hoặc mật khẩu không đúng.");
    }

    if (!user.email_verified) {
      // Auto gửi OTP mới nếu không phạm cooldown, để user không cần tự bấm "gửi lại".
      const remaining = await this.otp.cooldownRemaining(user.email);
      if (remaining <= 0) {
        const otpCode = await this.otp.issue(user.email);
        await sendOtpEmail(user.email, user.name, otpCode);
      }
      throw new EmailNotVerifiedError(user.email);
    }

    const tokens = await this.tokenService.issueTokens(user);

    return {
      user,
      accessToken: tokens.accessToken,
      refreshToken: tokens.refreshToken,
    };
  }

  // ── Google OAuth ──

  getGoogleAuthUrl(): string {
    return this.google.getAuthUrl();
  }

  async googleLogin(code: string): Promise<{
    user: UserRow;
    accessToken: string;
    refreshToken: string;
    isNew: boolean;
  }> {
    const tokens = await this.google.exchangeCode(code);
    const googleUser = await this.google.getUserInfo(tokens.access_token);

    if (!googleUser.email_verified) {
      throw new UnauthorizedError("Email Google chưa được xác minh.");
    }

    const existingCred = await this.repo.findCredentialByGoogleId(
      googleUser.sub,
    );

    let user: UserRow;
    let isNew = false;

    if (existingCred) {
      const u = await this.repo.findUserById(existingCred.user_id);
      if (!u)
        throw new Error("Không tìm thấy user cho Google credential đã có.");
      user = u;
      if (user.avatar_url !== googleUser.picture) {
        await this.repo.updateAvatar(user.id, googleUser.picture);
        user = { ...user, avatar_url: googleUser.picture };
      }
    } else {
      const userByEmail = await this.repo.findUserByEmail(googleUser.email);

      if (userByEmail) {
        await this.repo.createGoogleCredential(
          userByEmail.id,
          googleUser.sub,
          googleUser.email,
        );
        user = userByEmail;
      } else {
        user = await this.repo.createUser(
          googleUser.email,
          googleUser.name,
          googleUser.picture,
        );
        await this.repo.createGoogleCredential(
          user.id,
          googleUser.sub,
          googleUser.email,
        );
        isNew = true;
      }
    }

    if (user.status === "disabled") {
      throw new ForbiddenError("Tài khoản đã bị vô hiệu hoá.");
    }

    const issued = await this.tokenService.issueTokens(user);

    return {
      user,
      accessToken: issued.accessToken,
      refreshToken: issued.refreshToken,
      isNew,
    };
  }

  // ── Token Refresh (rotation) ──

  async refreshAccessToken(
    refreshToken: string,
  ): Promise<{ user: UserRow; accessToken: string; refreshToken: string }> {
    const hash = crypto.createHash("sha256").update(refreshToken).digest("hex");
    const stored = await this.repo.findRefreshToken(hash);

    if (!stored) {
      throw new UnauthorizedError("Refresh token không hợp lệ.");
    }

    if (stored.expires_at < new Date()) {
      await this.repo.deleteRefreshToken(hash);
      throw new UnauthorizedError("Refresh token đã hết hạn.");
    }

    const user = await this.repo.findUserById(stored.user_id);
    if (!user || user.status === "disabled") {
      await this.repo.deleteRefreshTokenFamily(stored.family);
      throw new UnauthorizedError("User không tồn tại hoặc đã bị vô hiệu hoá.");
    }

    const tokens = await this.tokenService.rotateTokens(
      user,
      hash,
      stored.family,
    );

    return {
      user,
      accessToken: tokens.accessToken,
      refreshToken: tokens.refreshToken,
    };
  }

  // ── Logout ──

  async logout(refreshToken: string): Promise<void> {
    const hash = crypto.createHash("sha256").update(refreshToken).digest("hex");
    await this.repo.deleteRefreshToken(hash);
  }

  async logoutAll(userId: string): Promise<void> {
    await this.tokenService.revokeAll(userId);
  }
}
