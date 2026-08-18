import { describe, expect, it } from "vitest";
import { createQueryClient, shouldRetry, STALE_TIME } from "./query-client";

describe("shouldRetry", () => {
  it("thử lại một lần khi lỗi không có HTTP status (mạng/timeout)", () => {
    expect(shouldRetry(0, new Error("network down"))).toBe(true);
  });

  it("thử lại khi server lỗi 5xx", () => {
    expect(shouldRetry(0, { status: 503 })).toBe(true);
  });

  it("KHÔNG thử lại với 4xx — request sai thì gọi lại vẫn sai", () => {
    // 401 xảy ra mỗi lần user chưa đăng nhập; mặc định của TanStack là retry
    // 3 lần, tức 3 request chắc chắn thất bại cho mỗi lần vào trang.
    expect(shouldRetry(0, { status: 401 })).toBe(false);
    expect(shouldRetry(0, { status: 400 })).toBe(false);
    // 429 thì retry còn làm tình hình rate limit tệ hơn.
    expect(shouldRetry(0, { status: 429 })).toBe(false);
  });

  it("chỉ thử lại tối đa một lần", () => {
    expect(shouldRetry(1, { status: 500 })).toBe(false);
  });
});

describe("STALE_TIME", () => {
  it("gợi ý (một lượt gọi LLM) được cache lâu nhất", () => {
    const others = [
      STALE_TIME.session,
      STALE_TIME.settings,
      STALE_TIME.userResources,
      STALE_TIME.conversations,
      STALE_TIME.messages,
    ];
    expect(Math.max(...others)).toBeLessThan(STALE_TIME.suggestions);
  });

  it("không có mốc nào bằng 0 — 0 nghĩa là mount lại là gọi API lại", () => {
    for (const value of Object.values(STALE_TIME)) {
      expect(value).toBeGreaterThan(0);
    }
  });
});

describe("createQueryClient", () => {
  it("tắt refetchOnWindowFocus để alt-tab không phát thêm request", () => {
    const defaults = createQueryClient().getDefaultOptions();
    expect(defaults.queries?.refetchOnWindowFocus).toBe(false);
  });

  it("mặc định staleTime khác 0", () => {
    const defaults = createQueryClient().getDefaultOptions();
    expect(defaults.queries?.staleTime).toBeGreaterThan(0);
  });

  it("không retry mutation — tạo hội thoại/MCP server không idempotent", () => {
    const defaults = createQueryClient().getDefaultOptions();
    expect(defaults.mutations?.retry).toBe(0);
  });
});
