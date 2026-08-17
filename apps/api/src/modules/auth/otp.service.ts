import crypto from "crypto";
import { cacheSet, cacheGet, cacheDel, cacheKey } from "../../database/redis/redis.module";

const OTP_TTL_SECONDS = 10 * 60; // 10 phút
const COOLDOWN_SECONDS = 2 * 60; // 2 phút
const MAX_ATTEMPTS = 5;

interface OtpRecord {
  codeHash: string;
  attempts: number;
}

interface CooldownRecord {
  availableAt: number; // epoch ms
}

export type VerifyResult = "ok" | "invalid" | "expired" | "locked";

const otpKey = (email: string): string => cacheKey("otp", email.toLowerCase());
const cooldownKey = (email: string): string =>
  cacheKey("otp-cooldown", email.toLowerCase());

export function generateOtp(): string {
  return crypto.randomInt(100000, 1000000).toString();
}

export function hashOtp(otp: string): string {
  return crypto.createHash("sha256").update(otp).digest("hex");
}

/**
 * Quản lý OTP xác minh email trong Redis: sinh mã, kiểm tra, cooldown resend.
 * Không lưu Postgres — OTP là dữ liệu ngắn hạn (10 phút), Redis TTL tự dọn dẹp.
 */
export class OtpService {
  /** Sinh OTP mới, lưu Redis (TTL 10p), set cooldown (TTL 2p). Trả về mã OTP (plaintext) để gửi email. */
  async issue(email: string): Promise<string> {
    const otp = generateOtp();
    const record: OtpRecord = { codeHash: hashOtp(otp), attempts: 0 };
    await cacheSet(otpKey(email), record, OTP_TTL_SECONDS);
    await cacheSet(
      cooldownKey(email),
      { availableAt: Date.now() + COOLDOWN_SECONDS * 1000 } satisfies CooldownRecord,
      COOLDOWN_SECONDS + 10, // TTL dư 10s so với cooldown thật, tránh lệch giờ giữa client/server
    );
    return otp;
  }

  /** Số giây còn lại phải chờ trước khi được phép resend. 0 = có thể resend ngay. */
  async cooldownRemaining(email: string): Promise<number> {
    const record = await cacheGet<CooldownRecord>(cooldownKey(email));
    if (!record) return 0;
    const remainingMs = record.availableAt - Date.now();
    return remainingMs > 0 ? Math.ceil(remainingMs / 1000) : 0;
  }

  /** Kiểm tra OTP người dùng nhập. Tăng attempts nếu sai; xoá record nếu đúng hoặc chạm max attempts. */
  async verify(email: string, otp: string): Promise<VerifyResult> {
    const record = await cacheGet<OtpRecord>(otpKey(email));
    if (!record) return "expired";
    if (record.attempts >= MAX_ATTEMPTS) {
      await cacheDel(otpKey(email));
      return "locked";
    }

    if (record.codeHash !== hashOtp(otp)) {
      const attempts = record.attempts + 1;
      if (attempts >= MAX_ATTEMPTS) {
        await cacheDel(otpKey(email));
        return "locked";
      }
      await cacheSet(otpKey(email), { ...record, attempts }, OTP_TTL_SECONDS);
      return "invalid";
    }

    await cacheDel(otpKey(email));
    await cacheDel(cooldownKey(email));
    return "ok";
  }
}
