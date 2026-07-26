import { v4 as uuid } from "uuid";
import type { AuthRepository, UserRow } from "../auth.repository";
import type { JwtStrategy } from "./jwt.strategy";

/**
 * Token Service — issue, rotate, revoke refresh tokens.
 *
 * Refresh token rotation: mỗi lần refresh, token cũ bị xoá và cấp token mới.
 * Nếu attacker dùng token cũ (đã bị xoá) → toàn bộ family bị revoke.
 * Family dùng để nhóm các token cùng 1 phiên đăng nhập.
 */
export class TokenService {
  private readonly refreshExpiryMs = 7 * 24 * 60 * 60 * 1000; // 7 ngày

  constructor(
    private repo: AuthRepository,
    private jwtStrat: JwtStrategy,
  ) {}

  /** Cấp access + refresh token, lưu refresh hash vào DB. */
  async issueTokens(user: UserRow): Promise<{
    accessToken: string;
    refreshToken: string;
    refreshHash: string;
    family: string;
  }> {
    const accessToken = this.jwtStrat.generateAccessToken(user);
    const { token: refreshToken, hash: refreshHash } =
      this.jwtStrat.generateRefreshToken();
    const family = uuid();

    await this.repo.saveRefreshToken(
      refreshHash,
      user.id,
      family,
      new Date(Date.now() + this.refreshExpiryMs),
    );

    return { accessToken, refreshToken, refreshHash, family };
  }

  /**
   * Rotate token: xoá token cũ, cấp cặp mới cùng family.
   * Nếu token cũ không tồn tại → có thể bị replay attack → revoke cả family.
   */
  async rotateTokens(
    user: UserRow,
    oldTokenHash: string,
    family: string,
  ): Promise<{ accessToken: string; refreshToken: string }> {
    await this.repo.deleteRefreshToken(oldTokenHash);

    const { token: newRefreshToken, hash: newHash } =
      this.jwtStrat.generateRefreshToken();
    await this.repo.saveRefreshToken(
      newHash,
      user.id,
      family,
      new Date(Date.now() + this.refreshExpiryMs),
    );

    return {
      accessToken: this.jwtStrat.generateAccessToken(user),
      refreshToken: newRefreshToken,
    };
  }

  /** Revoke tất cả token của user (logout tất cả thiết bị). */
  async revokeAll(userId: string): Promise<void> {
    await this.repo.deleteAllUserRefreshTokens(userId);
  }
}
