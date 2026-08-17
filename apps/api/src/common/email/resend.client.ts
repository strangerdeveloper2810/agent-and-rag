import { Resend } from "resend";
import { config } from "../../config";

let client: Resend | null = null;

/** Lazy singleton — trả về null nếu RESEND_API_KEY chưa cấu hình. */
export function getResendClient(): Resend | null {
  if (!config.RESEND_API_KEY) return null;
  if (!client) {
    client = new Resend(config.RESEND_API_KEY);
  }
  return client;
}
