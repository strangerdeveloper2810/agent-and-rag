// Chụp ảnh màn hình thật của giao diện chat để dùng cho section ProductPreview
// ở landing page (public/screenshots/chat-preview.png).
//
// Tài khoản demo hard-code trong LoginPage.tsx (demo@javis.ai) KHÔNG tồn tại trong
// DB dev local này, nên script tự tạo 1 tài khoản throwaway qua form /register thật,
// rồi bypass bước xác thực email (chỉ update cột email_verified — KHÔNG đụng gì khác)
// bằng docker exec vào container Postgres dev local (jarvis-development-postgres).
//
// KHÔNG gửi tin nhắn chat mới — chỉ đăng nhập rồi chụp trạng thái có sẵn (empty state),
// tránh phát sinh gọi LLM thật tốn phí.
//
// Yêu cầu: dev server web (:3000), backend api (:3001), Postgres dev container đã chạy sẵn.
// Chạy: node scripts/screenshot-chat.mjs

import { chromium } from "playwright";
import { execSync } from "node:child_process";
import path from "node:path";
import fs from "node:fs";

const baseUrl = process.argv[2] || "http://localhost:3000";
const outputPath =
  process.argv[3] || path.resolve("public/screenshots/chat-preview.png");

const THROWAWAY = {
  name: "Landing Preview",
  email: "landing-preview@jarvis.local",
  password: "Password123!",
};

const PG_CONTAINER = "jarvis-development-postgres";

function markEmailVerified(email) {
  execSync(
    `docker exec ${PG_CONTAINER} psql -U jarvis -d ai_agent_tut -c "UPDATE users SET email_verified = true WHERE email = '${email}';"`,
    { stdio: "inherit" },
  );
}

async function main() {
  fs.mkdirSync(path.dirname(outputPath), { recursive: true });

  const browser = await chromium.launch();
  const page = await browser.newPage({
    viewport: { width: 1600, height: 1000 },
  });

  try {
    console.log(`[screenshot] registering throwaway account (idempotent)...`);
    await page.goto(`${baseUrl}/register`, { waitUntil: "networkidle" });
    await page.locator("#register-name").fill(THROWAWAY.name);
    await page.locator("#register-email").fill(THROWAWAY.email);
    await page.locator("#register-password").fill(THROWAWAY.password);
    await page.locator('form button[type="submit"]').click();
    // Backend register (bcrypt hash + có thể gửi email OTP) khá chậm (~5-10s).
    // Không quan tâm kết quả cuối (thành công -> /verify-email, hoặc lỗi "đã tồn tại"
    // -> ở lại /register) — bước sau luôn tự bypass verify bằng DB update.
    await page
      .waitForURL((url) => !url.pathname.startsWith("/register"), {
        timeout: 20_000,
      })
      .catch(() => {});

    console.log("[screenshot] marking email_verified=true in dev DB...");
    markEmailVerified(THROWAWAY.email);

    console.log("[screenshot] logging in...");
    await page.goto(`${baseUrl}/login`, { waitUntil: "networkidle" });
    await page.locator("#login-email").fill(THROWAWAY.email);
    await page.locator("#login-password").fill(THROWAWAY.password);
    await page.locator('form button[type="submit"]').click();

    await page.waitForURL((url) => !url.pathname.startsWith("/login"), {
      timeout: 15_000,
    });

    await page.waitForSelector("textarea, [contenteditable='true']", {
      timeout: 15_000,
    });
    // Chờ toast "Signed in successfully" tự biến mất.
    await page.waitForTimeout(4500);

    // QUAN TRỌNG: chỉ chụp vùng nội dung chính (header + chat panel), KHÔNG chụp
    // sidebar — tài khoản throwaway vừa tạo vẫn hiển thị lịch sử hội thoại có vẻ
    // không thuộc về nó (nghi vấn lỗi cô lập dữ liệu giữa user, xem báo cáo riêng).
    // Tránh rủi ro lộ nội dung hội thoại thật của người khác lên ảnh public.
    const contentArea = page.locator("div.min-w-0.flex-1.flex-col").first();
    await contentArea.screenshot({ path: outputPath });
    console.log(`[screenshot] saved to ${outputPath} (main content only, sidebar excluded)`);
  } finally {
    await browser.close();
  }
}

main().catch((err) => {
  console.error("[screenshot] failed:", err);
  process.exit(1);
});
