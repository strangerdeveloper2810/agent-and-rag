import { describe, it, expect, vi } from "vitest";

// buildApp() lấy PG pool ngay lúc register auth/users module. Test này chỉ kiểm
// tra route health (không chạm PG), nên stub pool để không cần Postgres thật.
vi.mock("./database/index.js", async (importOriginal) => ({
  ...(await importOriginal<typeof import("./database/index.js")>()),
  getPgPool: () => ({}) as never,
}));

const { buildApp } = await import("./app.js");

describe("health route", () => {
  it("returns status ok", async () => {
    const app = buildApp();
    const res = await app.inject({ method: "GET", url: "/api/health" });
    expect(res.statusCode).toBe(200);
    expect(res.json()).toEqual({ status: "ok" });
    await app.close();
  });
});
