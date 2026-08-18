import {
  describe,
  it,
  expect,
  beforeAll,
  afterAll,
  beforeEach,
  vi,
} from "vitest";
import type { FastifyInstance } from "fastify";
import jwt from "jsonwebtoken";

// buildApp() lấy PG pool ngay lúc register users module. Stub bằng 1 pool giả
// có `query` controllable per-test — cho phép ta vừa dựng kịch bản CRUD thật,
// vừa xác nhận CHÍNH XÁC câu SQL/tham số gửi đi (chứng minh scope theo user).
const pgQuery = vi.fn();
vi.mock("../../database/index.js", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../database/index.js")>()),
  getPgPool: () => ({ query: pgQuery }) as never,
}));

const { buildApp } = await import("../../app");
const { config } = await import("../../config");

// 2 user khác nhau — dùng để kiểm chứng KHÔNG user nào đọc/sửa/xoá được tài
// nguyên của user còn lại (IDOR — Insecure Direct Object Reference).
const userAId = "11111111-1111-1111-1111-111111111111";
const userBId = "22222222-2222-2222-2222-222222222222";

const tokenFor = (sub: string) =>
  jwt.sign(
    { sub, email: `${sub}@example.com`, role: "user" },
    config.JWT_SECRET,
    {
      expiresIn: 900,
    },
  );

const cookieFor = (sub: string) => ({
  cookie: `access_token=${tokenFor(sub)}`,
});

describe("users routes — MCP servers + Skills (auth, validate DTO, CRUD, scope theo user)", () => {
  let app: FastifyInstance;

  beforeAll(async () => {
    app = buildApp();
    await app.ready();
  });
  afterAll(async () => {
    await app.close();
  });
  beforeEach(() => {
    pgQuery.mockReset();
  });

  // ── Auth guard ──

  it("thiếu access_token → 401 cho mọi route MCP/skill", async () => {
    const res = await app.inject({
      method: "GET",
      url: "/api/user/mcp-servers",
    });
    expect(res.statusCode).toBe(401);
    expect(pgQuery).not.toHaveBeenCalled();
  });

  it("thiếu access_token → 401 cho route skills", async () => {
    const res = await app.inject({ method: "GET", url: "/api/user/skills" });
    expect(res.statusCode).toBe(401);
  });

  // ── Validate DTO: MCP server ──

  it("POST /api/user/mcp-servers với url không hợp lệ → 422, KHÔNG chạm DB", async () => {
    const res = await app.inject({
      method: "POST",
      url: "/api/user/mcp-servers",
      headers: { "content-type": "application/json", ...cookieFor(userAId) },
      payload: { name: "srv", url: "không-phải-url" },
    });
    expect(res.statusCode).toBe(422);
    expect(res.json().code).toBe("VALIDATION_ERROR");
    expect(pgQuery).not.toHaveBeenCalled();
  });

  it("POST /api/user/mcp-servers với name có ký tự đặc biệt → 422", async () => {
    const res = await app.inject({
      method: "POST",
      url: "/api/user/mcp-servers",
      headers: { "content-type": "application/json", ...cookieFor(userAId) },
      payload: { name: "srv với dấu cách!!", url: "https://mcp.example.com" },
    });
    expect(res.statusCode).toBe(422);
  });

  it("POST /api/user/mcp-servers thiếu url → 422", async () => {
    const res = await app.inject({
      method: "POST",
      url: "/api/user/mcp-servers",
      headers: { "content-type": "application/json", ...cookieFor(userAId) },
      payload: { name: "srv" },
    });
    expect(res.statusCode).toBe(422);
  });

  // ── CRUD MCP server (happy path) ──

  it("GET /api/user/mcp-servers → 200, query lọc theo user_id của người gọi", async () => {
    pgQuery.mockResolvedValueOnce({
      rows: [{ id: "s1", user_id: userAId, name: "weather", url: "https://x" }],
    });

    const res = await app.inject({
      method: "GET",
      url: "/api/user/mcp-servers",
      headers: cookieFor(userAId),
    });

    expect(res.statusCode).toBe(200);
    expect(res.json().servers).toHaveLength(1);
    const [sql, params] = pgQuery.mock.calls[0];
    expect(sql).toContain("WHERE user_id = $1");
    expect(params).toEqual([userAId]);
  });

  it("POST /api/user/mcp-servers hợp lệ → 201, INSERT gắn user_id của người gọi", async () => {
    pgQuery.mockResolvedValueOnce({
      rows: [{ id: "s1", user_id: userAId, name: "weather", url: "https://x" }],
    });

    const res = await app.inject({
      method: "POST",
      url: "/api/user/mcp-servers",
      headers: { "content-type": "application/json", ...cookieFor(userAId) },
      payload: { name: "weather", url: "https://x" },
    });

    expect(res.statusCode).toBe(201);
    const [sql, params] = pgQuery.mock.calls[0];
    expect(sql).toContain("INSERT INTO user_mcp_servers");
    expect(params[0]).toBe(userAId);
  });

  // ── SCOPE: user A KHÔNG được sửa/xoá MCP server của user B ──

  it("PATCH /api/user/mcp-servers/:id với id thuộc user KHÁC → 404, và query dùng userId của người GỌI (không phải id trong URL)", async () => {
    // DB trả rows rỗng vì WHERE id=... AND user_id=<userA> không khớp record
    // thực sự thuộc userB — đúng hành vi Postgres thật khi scope đúng.
    pgQuery.mockResolvedValueOnce({ rows: [] });

    const res = await app.inject({
      method: "PATCH",
      url: "/api/user/mcp-servers/server-belongs-to-user-b",
      headers: { "content-type": "application/json", ...cookieFor(userAId) },
      payload: { name: "hacked-name" },
    });

    expect(res.statusCode).toBe(404);
    const [sql, params] = pgQuery.mock.calls[0];
    expect(sql).toMatch(/WHERE id = \$\d+ AND user_id = \$\d+/);
    // 2 tham số cuối PHẢI là [id trong URL, userA — người đang đăng nhập],
    // KHÔNG có cách nào để userA giả mạo user_id khác qua request.
    expect(params.slice(-2)).toEqual(["server-belongs-to-user-b", userAId]);
  });

  it("DELETE /api/user/mcp-servers/:id với id thuộc user KHÁC → 404, không xoá được gì", async () => {
    pgQuery.mockResolvedValueOnce({ rowCount: 0 });

    const res = await app.inject({
      method: "DELETE",
      url: "/api/user/mcp-servers/server-belongs-to-user-b",
      headers: cookieFor(userAId),
    });

    expect(res.statusCode).toBe(404);
    const [sql, params] = pgQuery.mock.calls[0];
    expect(sql).toContain("DELETE FROM user_mcp_servers");
    expect(params).toEqual(["server-belongs-to-user-b", userAId]);
  });

  it("PATCH /api/user/mcp-servers/:id với id thuộc CHÍNH user gọi → 200 (đối chứng — chặn cross-user không phải chặn luôn mọi update)", async () => {
    pgQuery.mockResolvedValueOnce({
      rows: [{ id: "own-server", user_id: userAId, name: "renamed" }],
    });

    const res = await app.inject({
      method: "PATCH",
      url: "/api/user/mcp-servers/own-server",
      headers: { "content-type": "application/json", ...cookieFor(userAId) },
      payload: { name: "renamed" },
    });

    expect(res.statusCode).toBe(200);
  });

  // ── Validate DTO: custom skill ──

  it("POST /api/user/skills với content rỗng → 422", async () => {
    const res = await app.inject({
      method: "POST",
      url: "/api/user/skills",
      headers: { "content-type": "application/json", ...cookieFor(userAId) },
      payload: { name: "my-skill", content: "" },
    });
    expect(res.statusCode).toBe(422);
  });

  it("POST /api/user/skills với content vượt 10.000 ký tự → 422", async () => {
    const res = await app.inject({
      method: "POST",
      url: "/api/user/skills",
      headers: { "content-type": "application/json", ...cookieFor(userAId) },
      payload: { name: "my-skill", content: "a".repeat(10001) },
    });
    expect(res.statusCode).toBe(422);
  });

  it("POST /api/user/skills hợp lệ → 201, INSERT gắn user_id người gọi", async () => {
    pgQuery.mockResolvedValueOnce({
      rows: [{ id: "sk1", user_id: userAId, name: "my-skill" }],
    });

    const res = await app.inject({
      method: "POST",
      url: "/api/user/skills",
      headers: { "content-type": "application/json", ...cookieFor(userAId) },
      payload: { name: "my-skill", content: "nội dung" },
    });

    expect(res.statusCode).toBe(201);
    const [sql, params] = pgQuery.mock.calls[0];
    expect(sql).toContain("INSERT INTO user_skills");
    expect(params[0]).toBe(userAId);
  });

  // ── SCOPE: user A KHÔNG được sửa/xoá custom skill của user B ──

  it("PATCH /api/user/skills/:id với id thuộc user KHÁC → 404, dùng userId người GỌI để scope", async () => {
    pgQuery.mockResolvedValueOnce({ rows: [] });

    const res = await app.inject({
      method: "PATCH",
      url: "/api/user/skills/skill-belongs-to-user-b",
      headers: { "content-type": "application/json", ...cookieFor(userAId) },
      payload: { content: "hacked" },
    });

    expect(res.statusCode).toBe(404);
    const [sql, params] = pgQuery.mock.calls[0];
    expect(sql).toMatch(/WHERE id = \$\d+ AND user_id = \$\d+/);
    expect(params.slice(-2)).toEqual(["skill-belongs-to-user-b", userAId]);
  });

  it("DELETE /api/user/skills/:id với id thuộc user KHÁC → 404", async () => {
    pgQuery.mockResolvedValueOnce({ rowCount: 0 });

    const res = await app.inject({
      method: "DELETE",
      url: "/api/user/skills/skill-belongs-to-user-b",
      headers: cookieFor(userAId),
    });

    expect(res.statusCode).toBe(404);
    const [, params] = pgQuery.mock.calls[0];
    expect(params).toEqual(["skill-belongs-to-user-b", userAId]);
  });

  it("GET /api/user/skills → 200, trả customSkills + disabledBuiltinSkills, cả 2 query đều scoped theo user_id người gọi", async () => {
    pgQuery.mockImplementation(async (sql: string) => {
      if (sql.includes("user_skills")) {
        return { rows: [{ id: "sk1", user_id: userAId, name: "custom" }] };
      }
      if (sql.includes("user_disabled_skills")) {
        return { rows: [{ skill_name: "debug" }] };
      }
      return { rows: [] };
    });

    const res = await app.inject({
      method: "GET",
      url: "/api/user/skills",
      headers: cookieFor(userAId),
    });

    expect(res.statusCode).toBe(200);
    const body = res.json();
    expect(body.customSkills).toHaveLength(1);
    expect(body.disabledBuiltinSkills).toEqual(["debug"]);

    // Cả 2 lệnh query song song đều phải scoped đúng theo userA.
    for (const call of pgQuery.mock.calls) {
      expect(call[1]).toEqual([userAId]);
    }
  });

  // ── Toggle builtin skill ──

  it("POST /api/user/skills/:name/toggle với enabled không phải boolean → 422", async () => {
    const res = await app.inject({
      method: "POST",
      url: "/api/user/skills/code-review/toggle",
      headers: { "content-type": "application/json", ...cookieFor(userAId) },
      payload: { enabled: "yes" },
    });
    expect(res.statusCode).toBe(422);
    expect(pgQuery).not.toHaveBeenCalled();
  });

  it("POST /api/user/skills/:name/toggle enabled=false → 200, INSERT vào user_disabled_skills với user_id người gọi", async () => {
    pgQuery.mockResolvedValueOnce({});

    const res = await app.inject({
      method: "POST",
      url: "/api/user/skills/code-review/toggle",
      headers: { "content-type": "application/json", ...cookieFor(userAId) },
      payload: { enabled: false },
    });

    expect(res.statusCode).toBe(200);
    expect(res.json()).toEqual({ name: "code-review", enabled: false });
    const [sql, params] = pgQuery.mock.calls[0];
    expect(sql).toContain("INSERT INTO user_disabled_skills");
    expect(params).toEqual([userAId, "code-review"]);
  });

  it("POST /api/user/skills/:name/toggle enabled=true → 200, DELETE khỏi user_disabled_skills chỉ của user_id gọi", async () => {
    pgQuery.mockResolvedValueOnce({});

    const res = await app.inject({
      method: "POST",
      url: "/api/user/skills/code-review/toggle",
      headers: { "content-type": "application/json", ...cookieFor(userBId) },
      payload: { enabled: true },
    });

    expect(res.statusCode).toBe(200);
    const [sql, params] = pgQuery.mock.calls[0];
    expect(sql).toContain("DELETE FROM user_disabled_skills");
    expect(params).toEqual([userBId, "code-review"]);
  });
});
