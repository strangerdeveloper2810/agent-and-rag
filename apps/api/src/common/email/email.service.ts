import { getResendClient } from "./resend.client";
import { config } from "../../config";

/**
 * Gửi email OTP xác minh. KHÔNG throw khi lỗi (thiếu API key, Resend down, ...)
 * — register/resend-otp vẫn phải trả về thành công, user có thể bấm "gửi lại".
 */
export async function sendOtpEmail(
  to: string,
  name: string,
  otp: string,
): Promise<void> {
  const resend = getResendClient();
  if (!resend) {
    console.warn(
      `[email] RESEND_API_KEY chưa cấu hình — bỏ qua gửi mail. OTP cho ${to}: ${otp}`,
    );
    return;
  }

  try {
    const result = await resend.emails.send({
      from: config.EMAIL_FROM,
      to,
      subject: "Mã xác minh JARVIS của bạn",
      html: `
        <div style="font-family: 'DM Sans', sans-serif; background:#0b0b0f; color:#f5f0e6; padding:32px; border-radius:12px;">
          <h1 style="color:#d4a94a; font-size:20px; margin:0 0 16px;">Xin chào ${name},</h1>
          <p style="margin:0 0 12px;">Mã xác minh email của bạn là:</p>
          <p style="font-family:'JetBrains Mono', monospace; font-size:32px; letter-spacing:8px; color:#d4a94a; margin:0 0 16px;">${otp}</p>
          <p style="color:#9a9a9a; font-size:13px; margin:0;">Mã có hiệu lực trong 10 phút. Nếu bạn không yêu cầu mã này, hãy bỏ qua email.</p>
        </div>
      `,
    });

    if (result.error) {
      console.error(`[email] Resend API trả về lỗi (to=${to}):`, result.error);
      console.info(`[email] [FALLBACK_LOG] OTP cho ${to}: ${otp}`);
    } else {
      console.log(
        `[email] Đã gửi OTP thành công tới ${to} (id=${result.data?.id})`,
      );
    }
  } catch (err) {
    console.error(`[email] sendOtpEmail thất bại (to=${to})`, err);
    console.info(`[email] [FALLBACK_LOG] OTP cho ${to}: ${otp}`);
  }
}

/**
 * Gửi email OTP đặt lại mật khẩu.
 */
export async function sendPasswordResetOtpEmail(
  to: string,
  name: string,
  otp: string,
): Promise<void> {
  const resend = getResendClient();
  if (!resend) {
    console.warn(
      `[email] RESEND_API_KEY chưa cấu hình — bỏ qua gửi mail. Reset OTP cho ${to}: ${otp}`,
    );
    return;
  }

  try {
    const result = await resend.emails.send({
      from: config.EMAIL_FROM,
      to,
      subject: "Yêu cầu đặt lại mật khẩu J.A.R.V.I.S.",
      html: `
        <div style="font-family: 'DM Sans', sans-serif; background:#0b0b0f; color:#f5f0e6; padding:32px; border-radius:12px;">
          <h1 style="color:#6366f1; font-size:20px; margin:0 0 16px;">Xin chào ${name},</h1>
          <p style="margin:0 0 12px;">Chúng tôi nhận được yêu cầu đặt lại mật khẩu cho tài khoản J.A.R.V.I.S. của bạn.</p>
          <p style="margin:0 0 12px;">Mã xác thực OTP của bạn là:</p>
          <p style="font-family:'JetBrains Mono', monospace; font-size:32px; letter-spacing:8px; color:#6366f1; margin:0 0 16px; font-weight:bold;">${otp}</p>
          <p style="color:#9a9a9a; font-size:13px; margin:0;">Mã OTP có hiệu lực trong 10 phút. Nếu bạn không yêu cầu đặt lại mật khẩu, xin vui lòng bỏ qua email này để bảo vệ tài khoản.</p>
        </div>
      `,
    });

    if (result.error) {
      console.error(
        `[email] Resend API trả về lỗi khi gửi reset OTP (to=${to}):`,
        result.error,
      );
      console.info(`[email] [FALLBACK_LOG] Reset OTP cho ${to}: ${otp}`);
    } else {
      console.log(
        `[email] Đã gửi Reset OTP thành công tới ${to} (id=${result.data?.id})`,
      );
    }
  } catch (err) {
    console.error(`[email] sendPasswordResetOtpEmail thất bại (to=${to})`, err);
    console.info(`[email] [FALLBACK_LOG] Reset OTP cho ${to}: ${otp}`);
  }
}
