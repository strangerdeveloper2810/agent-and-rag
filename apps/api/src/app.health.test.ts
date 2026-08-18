import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import type { FastifyInstance } from "fastify";

// Stub PG pool: buildApp() lấy pool khi register auth/users module.
vi.mock("./database/index.js", async (importOriginal) => ({
  ...(await importOriginal<typeof import("./database/index.js")>()),
  getPgPool: () => ({}) as never,
}));

// Mongo + Go agent được mock để điều khiển từng nhánh của healthz/ready.
const mongoCommand = vi.fn();
vi.mock("./lib/mongo", () => ({
  getDb: () => ({ command: mongoCommand }),
}));

const goHealthy = vi.fn();
vi.mock("./agent/client", () => ({
  checkGoAgentHealth: () => goHealthy(),
  createAgentClient: () => ({ stream: () => [] }),
}));

const { buildApp } = await import("./app");

/**
 * Health endpoint là thứ CD dựa vào để quyết định deploy thành công hay rollback
 * (xem .github/workflows/cd.yml gọi /api/healthz), nhưng trước đây không có test
 * nào chạm tới. Nếu healthz trả 200 khi Mongo đã chết thì deploy lỗi vẫn được
 * coi là thành công — im lặng và nguy hiểm.
 */
describe("health endpoints", () => {
  let app: FastifyInstance;

  beforeEach(async () => {
    mongoCommand.mockReset();
    goHealthy.mockReset();
    app = buildApp();
    await app.ready();
  });
  afterEach(async () => {
    await app.close();
  });

  it("GET /api/health — liveness, không phụ thuộc service ngoài", async () => {
    const res = await app.inject({ method: "GET", url: "/api/health" });
    expect(res.statusCode).toBe(200);
    expect(res.json()).toEqual({ status: "ok" });
    // Liveness KHÔNG được ping DB — nếu ping, process sẽ bị coi là chết khi DB
    // chỉ tạm gián đoạn, và orchestrator sẽ restart container vô ích.
    expect(mongoCommand).not.toHaveBeenCalled();
  });

  it("GET /api/healthz — Mongo sống → 200 healthy", async () => {
    mongoCommand.mockResolvedValue({ ok: 1 });
    goHealthy.mockResolvedValue(true);

    const res = await app.inject({ method: "GET", url: "/api/healthz" });
    expect(res.statusCode).toBe(200);

    const body = res.json();
    expect(body.status).toBe("healthy");
    expect(body.checks.mongo).toBe("ok");
  });

  it("GET /api/healthz — Mongo chết → 503 degraded, chỉ rõ thành phần lỗi", async () => {
    mongoCommand.mockRejectedValue(new Error("mongo down"));
    goHealthy.mockResolvedValue(true);

    const res = await app.inject({ method: "GET", url: "/api/healthz" });
    expect(res.statusCode).toBe(503);

    const body = res.json();
    expect(body.status).toBe("degraded");
    expect(body.checks.mongo).toBe("error");
  });

  it("GET /api/ready — Mongo sống → 200 ready", async () => {
    mongoCommand.mockResolvedValue({ ok: 1 });

    const res = await app.inject({ method: "GET", url: "/api/ready" });
    expect(res.statusCode).toBe(200);
    expect(res.json()).toEqual({ status: "ready" });
  });

  it("GET /api/ready — Mongo chết → 503 not_ready", async () => {
    mongoCommand.mockRejectedValue(new Error("mongo down"));

    const res = await app.inject({ method: "GET", url: "/api/ready" });
    expect(res.statusCode).toBe(503);
    expect(res.json()).toEqual({ status: "not_ready" });
  });
});
