import bcrypt from "bcrypt";
import crypto from "crypto";
import type { AuthRepository, UserRow } from "./auth.repository";
import { TokenService } from "./strategies/token.service";
import { GoogleStrategy } from "./strategies/google.strategy";
import {
  ConflictError,
  UnauthorizedError,
  ForbiddenError,
} from "../../common/errors/app-errors";
import type { RegisterInput } from "./dto/register.dto";
import type { LoginInput } from "./dto/login.dto";

const BCRYPT_ROUNDS = 12;

export class AuthService {
  constructor(
    private repo: AuthRepository,
    private tokenService: TokenService,
    private google: GoogleStrategy,
  ) {}

  // ── Email/Password Register ──

  async register(
    input: RegisterInput,
  ): Promise<{ user: UserRow; accessToken: string; refreshToken: string }> {
    const existing = await this.repo.findUserByEmail(input.email);
    if (existing) {
      throw new ConflictError("Email đã được đăng ký.");
    }

    const user = await this.repo.createUser(input.email, input.name);
    const hash = await bcrypt.hash(input.password, BCRYPT_ROUNDS);
    await this.repo.createEmailCredential(user.id, hash);

    const tokens = await this.tokenService.issueTokens(user);

    return {
      user,
      accessToken: tokens.accessToken,
      refreshToken: tokens.refreshToken,
    };
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
