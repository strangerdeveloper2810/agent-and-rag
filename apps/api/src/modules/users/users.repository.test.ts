import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Pool } from "pg";
import { UsersRepository } from "./users.repository";

// ── Fake Pool ──
// Không dùng Postgres thật: mock `query` để bắt CHÍNH XÁC câu SQL + tham số đã
// gửi. Đây là cách kiểm chứng SCOPE THEO USER mạnh nhất — nếu repository lỡ bỏ
// mất điều kiện `AND user_id = $2` (vd refactor nhầm), test phải đỏ ngay cả khi
// mock trả về rows rỗng, vì ta assert trên CÂU QUERY THẬT gửi đi, không chỉ kết
// quả trả về.
const makeFakePool = () => {
  const query = vi.fn();
  return { query } as unknown as Pool & { query: ReturnType<typeof vi.fn> };
};

const userA = "11111111-1111-1111-1111-111111111111";
const userB = "22222222-2222-2222-2222-222222222222";

describe("UsersRepository — MCP servers", () => {
  let pool: ReturnType<typeof makeFakePool>;
  let repo: UsersRepository;

  beforeEach(() => {
    pool = makeFakePool();
    repo = new UsersRepository(pool);
  });

  it("findMcpServers: query lọc theo user_id (không trả toàn bộ bảng)", async () => {
    pool.query.mockResolvedValue({ rows: [] });
    await repo.findMcpServers(userA);

    expect(pool.query).toHaveBeenCalledTimes(1);
    const [sql, params] = pool.query.mock.calls[0];
    expect(sql).toContain("WHERE user_id = $1");
    expect(params).toEqual([userA]);
  });

  // Test SCOPE quan trọng nhất: findMcpServerById PHẢI truyền cả `id` LẪN
  // `userId` của người gọi vào WHERE — không được chỉ lọc theo id (nếu vậy,
  // user A truyền ID của user B sẽ đọc được server của B).
  it("findMcpServerById: WHERE lọc CẢ id LẪN user_id của người gọi", async () => {
    pool.query.mockResolvedValue({ rows: [] });
    await repo.findMcpServerById(userA, "server-of-b");

    const [sql, params] = pool.query.mock.calls[0];
    expect(sql).toContain("WHERE id = $1 AND user_id = $2");
    expect(params).toEqual(["server-of-b", userA]);
  });

  it("createMcpServer: INSERT gắn đúng user_id của người tạo", async () => {
    pool.query.mockResolvedValue({
      rows: [{ id: "srv-1", user_id: userA, name: "n", url: "u" }],
    });
    await repo.createMcpServer(userA, { name: "n", url: "u", api_key: "k" });

    const [sql, params] = pool.query.mock.calls[0];
    expect(sql).toContain("INSERT INTO user_mcp_servers");
    expect(params).toEqual([userA, "n", "u", "k"]);
  });

  it("updateMcpServer: UPDATE ... WHERE id = $x AND user_id = $x+1 (2 tham số cuối là id, userId gọi)", async () => {
    pool.query.mockResolvedValue({ rows: [] });
    await repo.updateMcpServer(userA, "server-of-b", { name: "hacked" });

    const [sql, params] = pool.query.mock.calls[0];
    expect(sql).toMatch(/WHERE id = \$\d+ AND user_id = \$\d+/);
    // 2 tham số cuối cùng luôn là [id, userId] — xem users.repository.ts.
    expect(params.slice(-2)).toEqual(["server-of-b", userA]);
  });

  it("updateMcpServer: không có field nào để update → fallback findMcpServerById (vẫn scoped)", async () => {
    pool.query.mockResolvedValue({ rows: [] });
    await repo.updateMcpServer(userA, "srv-1", {});

    const [sql, params] = pool.query.mock.calls[0];
    expect(sql).toContain("WHERE id = $1 AND user_id = $2");
    expect(params).toEqual(["srv-1", userA]);
  });

  it("deleteMcpServer: DELETE ... WHERE id = $1 AND user_id = $2 (userId của người gọi, không phải từ input khác)", async () => {
    pool.query.mockResolvedValue({ rowCount: 0 });
    const deleted = await repo.deleteMcpServer(userA, "server-of-b");

    expect(deleted).toBe(false);
    const [sql, params] = pool.query.mock.calls[0];
    expect(sql).toContain("WHERE id = $1 AND user_id = $2");
    expect(params).toEqual(["server-of-b", userA]);
  });

  it("deleteMcpServer: rowCount > 0 → trả true (xoá đúng resource của mình)", async () => {
    pool.query.mockResolvedValue({ rowCount: 1 });
    const deleted = await repo.deleteMcpServer(userA, "srv-owned-by-a");
    expect(deleted).toBe(true);
  });
});

describe("UsersRepository — Custom Skills", () => {
  let pool: ReturnType<typeof makeFakePool>;
  let repo: UsersRepository;

  beforeEach(() => {
    pool = makeFakePool();
    repo = new UsersRepository(pool);
  });

  it("findUserSkills: query lọc theo user_id của người gọi", async () => {
    pool.query.mockResolvedValue({ rows: [] });
    await repo.findUserSkills(userA);

    const [sql, params] = pool.query.mock.calls[0];
    expect(sql).toContain("WHERE user_id = $1");
    expect(params).toEqual([userA]);
  });

  it("findUserSkillById: WHERE lọc CẢ id LẪN user_id — user A không đọc được skill của user B", async () => {
    pool.query.mockResolvedValue({ rows: [] });
    await repo.findUserSkillById(userA, "skill-of-b");

    const [sql, params] = pool.query.mock.calls[0];
    expect(sql).toContain("WHERE id = $1 AND user_id = $2");
    expect(params).toEqual(["skill-of-b", userA]);
  });

  it("createUserSkill: INSERT gắn đúng user_id của người tạo, giữ nguyên triggers", async () => {
    pool.query.mockResolvedValue({
      rows: [{ id: "sk-1", user_id: userA }],
    });
    await repo.createUserSkill(userA, {
      name: "n",
      description: "d",
      when_to_use: "w",
      content: "c",
      triggers: ["t1", "t2"],
    });

    const [sql, params] = pool.query.mock.calls[0];
    expect(sql).toContain("INSERT INTO user_skills");
    expect(params).toEqual([userA, "n", "d", "w", "c", ["t1", "t2"]]);
  });

  it("updateUserSkill: 2 tham số cuối luôn là [id, userId của người gọi]", async () => {
    pool.query.mockResolvedValue({ rows: [] });
    await repo.updateUserSkill(userA, "skill-of-b", { content: "hacked" });

    const [sql, params] = pool.query.mock.calls[0];
    expect(sql).toMatch(/WHERE id = \$\d+ AND user_id = \$\d+/);
    expect(params.slice(-2)).toEqual(["skill-of-b", userA]);
  });

  it("deleteUserSkill: DELETE scoped theo user_id của người gọi — không xoá được skill user khác", async () => {
    pool.query.mockResolvedValue({ rowCount: 0 });
    const deleted = await repo.deleteUserSkill(userA, "skill-of-b");

    expect(deleted).toBe(false);
    const [sql, params] = pool.query.mock.calls[0];
    expect(sql).toContain("WHERE id = $1 AND user_id = $2");
    expect(params).toEqual(["skill-of-b", userA]);
  });
});

describe("UsersRepository — Disabled builtin skills (toggle)", () => {
  let pool: ReturnType<typeof makeFakePool>;
  let repo: UsersRepository;

  beforeEach(() => {
    pool = makeFakePool();
    repo = new UsersRepository(pool);
  });

  it("findDisabledSkills: lọc theo user_id, trả về mảng tên skill", async () => {
    pool.query.mockResolvedValue({
      rows: [{ skill_name: "code-review" }, { skill_name: "debug" }],
    });
    const names = await repo.findDisabledSkills(userA);

    expect(names).toEqual(["code-review", "debug"]);
    const [sql, params] = pool.query.mock.calls[0];
    expect(sql).toContain("WHERE user_id = $1");
    expect(params).toEqual([userA]);
  });

  it("toggleBuiltinSkill(enabled=false): INSERT vào user_disabled_skills với đúng user_id + skill_name", async () => {
    pool.query.mockResolvedValue({});
    await repo.toggleBuiltinSkill(userA, "code-review", false);

    const [sql, params] = pool.query.mock.calls[0];
    expect(sql).toContain("INSERT INTO user_disabled_skills");
    expect(params).toEqual([userA, "code-review"]);
  });

  it("toggleBuiltinSkill(enabled=true): DELETE khỏi user_disabled_skills chỉ của user_id gọi (không ảnh hưởng user khác)", async () => {
    pool.query.mockResolvedValue({});
    await repo.toggleBuiltinSkill(userA, "code-review", true);

    const [sql, params] = pool.query.mock.calls[0];
    expect(sql).toContain("DELETE FROM user_disabled_skills");
    expect(sql).toContain("WHERE user_id = $1 AND skill_name = $2");
    expect(params).toEqual([userA, "code-review"]);
  });

  it("2 user khác nhau bật/tắt cùng tên skill không đụng nhau (kiểm tra params độc lập theo userId)", async () => {
    pool.query.mockResolvedValue({});
    await repo.toggleBuiltinSkill(userA, "debug", false);
    await repo.toggleBuiltinSkill(userB, "debug", false);

    const [, paramsA] = pool.query.mock.calls[0];
    const [, paramsB] = pool.query.mock.calls[1];
    expect(paramsA).toEqual([userA, "debug"]);
    expect(paramsB).toEqual([userB, "debug"]);
  });
});
