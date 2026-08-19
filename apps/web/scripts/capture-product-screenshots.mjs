// Chụp ảnh màn hình thật của giao diện sản phẩm (chat + tài liệu/RAG) để dùng cho
// section ProductPreview ở landing page (public/screenshots/{chat,documents}-preview.png).
//
// Tài khoản demo hard-code trong LoginPage.tsx (demo@javis.ai) KHÔNG tồn tại trong
// DB dev local này, nên script tự tạo 1 tài khoản throwaway qua form /register thật,
// rồi bypass bước xác thực email (chỉ update cột email_verified — KHÔNG đụng gì khác)
// bằng docker exec vào container Postgres dev local (jarvis-development-postgres).
//
// KHÔNG gửi tin nhắn chat mới, KHÔNG upload tài liệu thật — chỉ đăng nhập rồi chụp
// trạng thái có sẵn (empty state), tránh phát sinh gọi LLM thật tốn phí.
//
// QUAN TRỌNG: mọi screenshot đều CROP chỉ vùng nội dung chính (header + panel),
// KHÔNG chụp sidebar — tài khoản throwaway từng hiển thị lịch sử hội thoại có vẻ
// không thuộc về nó (nghi vấn lỗi cô lập dữ liệu giữa user, xem
// docs/plans hoặc memory "project-suspected-conversation-isolation-bug"). Luôn tự
// kiểm tra lại ảnh xuất ra trước khi commit/dùng public.
//
// Yêu cầu: dev server web (:3000), backend api (:3001), Postgres dev container đã chạy sẵn.
// Chạy: node scripts/capture-product-screenshots.mjs

import { chromium } from "playwright";
import { execSync } from "node:child_process";
import path from "node:path";
import fs from "node:fs";

const baseUrl = process.argv[2] || "http://localhost:3000";
const outputDir = process.argv[3] || path.resolve("public/screenshots");

const THROWAWAY = {
  name: "Landing Preview",
  email: "landing-preview@jarvis.local",
  password: "Password123!",
};

const PG_CONTAINER = "jarvis-development-postgres";
const CONTENT_AREA_SELECTOR = "div.min-w-0.flex-1.flex-col";

function markEmailVerified(email) {
  execSync(
    `docker exec ${PG_CONTAINER} psql -U jarvis -d ai_agent_tut -c "UPDATE users SET email_verified = true WHERE email = '${email}';"`,
    { stdio: "inherit" },
  );
}

async function captureContentArea(page, outputPath) {
  const contentArea = page.locator(CONTENT_AREA_SELECTOR).first();
  await contentArea.screenshot({ path: outputPath });
  console.log(`[screenshot] saved ${outputPath} (main content only, sidebar excluded)`);
}

async function main() {
  fs.mkdirSync(outputDir, { recursive: true });

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

    // --- Chat (empty state) ---
    await page.waitForSelector("textarea, [contenteditable='true']", {
      timeout: 15_000,
    });
    await page.waitForTimeout(4500); // toast "Signed in successfully" tự biến mất
    await captureContentArea(page, path.join(outputDir, "chat-preview.png"));

    // --- Documents / RAG upload ---
    console.log("[screenshot] navigating to /documents...");
    await page.goto(`${baseUrl}/documents`, { waitUntil: "networkidle" });
    await page.waitForTimeout(1500); // để list/empty-state render xong
    await captureContentArea(page, path.join(outputDir, "documents-preview.png"));
  } finally {
    await browser.close();
  }
}

main().catch((err) => {
  console.error("[screenshot] failed:", err);
  process.exit(1);
});
