import { describe, it, expect, vi, beforeEach } from "vitest";
import { UsersService } from "./users.service";
import { NotFoundError } from "../../common/errors/app-errors";
import type { UsersRepository } from "./users.repository";

// Fake repo tối giản: chỉ implement những method UsersService thực sự gọi.
// Dùng `as unknown as UsersRepository` để không phải mock cả class thật.
const makeFakeRepo = () => ({
  findMcpServers: vi.fn(),
  createMcpServer: vi.fn(),
  updateMcpServer: vi.fn(),
  deleteMcpServer: vi.fn(),
  findUserSkills: vi.fn(),
  findDisabledSkills: vi.fn(),
  createUserSkill: vi.fn(),
  updateUserSkill: vi.fn(),
  deleteUserSkill: vi.fn(),
  toggleBuiltinSkill: vi.fn(),
});

const userId = "11111111-1111-1111-1111-111111111111";

describe("UsersService — MCP servers", () => {
  let repo: ReturnType<typeof makeFakeRepo>;
  let service: UsersService;

  beforeEach(() => {
    repo = makeFakeRepo();
    service = new UsersService(repo as unknown as UsersRepository);
  });

  it("listMcpServers: forward đúng userId xuống repo (không tự chế biến)", async () => {
    repo.findMcpServers.mockResolvedValue([]);
    await service.listMcpServers(userId);
    expect(repo.findMcpServers).toHaveBeenCalledWith(userId);
  });

  it("createMcpServer: forward userId + input (transport, auth_token) xuống repo nguyên vẹn", async () => {
    const input = {
      name: "n",
      transport: "http" as const,
      url: "https://x",
      auth_token: "secret",
    };
    repo.createMcpServer.mockResolvedValue({ id: "1", ...input });
    await service.createMcpServer(userId, input);
    expect(repo.createMcpServer).toHaveBeenCalledWith(userId, input);
  });

  // Đây là điểm chặn SCOPE quan trọng nhất ở tầng service: khi repo báo
  // "không tìm thấy" (null — nghĩa là id không tồn tại HOẶC không thuộc
  // userId này), service phải ném NotFoundError — TUYỆT ĐỐI không được coi
  // đây là thành công hay trả dữ liệu rỗng im lặng.
  it("updateMcpServer: repo trả null (không thuộc user này) → ném NotFoundError, KHÔNG rò rỉ dữ liệu", async () => {
    repo.updateMcpServer.mockResolvedValue(null);
    await expect(
      service.updateMcpServer(userId, "server-of-another-user", { name: "x" }),
    ).rejects.toBeInstanceOf(NotFoundError);
  });

  it("deleteMcpServer: repo trả false (0 row bị xoá) → ném NotFoundError", async () => {
    repo.deleteMcpServer.mockResolvedValue(false);
    await expect(
      service.deleteMcpServer(userId, "server-of-another-user"),
    ).rejects.toBeInstanceOf(NotFoundError);
  });

  it("deleteMcpServer: repo trả true → không ném lỗi", async () => {
    repo.deleteMcpServer.mockResolvedValue(true);
    await expect(
      service.deleteMcpServer(userId, "own-server"),
    ).resolves.toBeUndefined();
  });
});

describe("UsersService — Custom skills", () => {
  let repo: ReturnType<typeof makeFakeRepo>;
  let service: UsersService;

  beforeEach(() => {
    repo = makeFakeRepo();
    service = new UsersService(repo as unknown as UsersRepository);
  });

  it("createUserSkill: điền default rỗng cho description/when_to_use/triggers khi không truyền", async () => {
    repo.createUserSkill.mockResolvedValue({ id: "1" });
    await service.createUserSkill(userId, { name: "n", content: "c" });

    expect(repo.createUserSkill).toHaveBeenCalledWith(userId, {
      name: "n",
      description: "",
      when_to_use: "",
      content: "c",
      triggers: [],
    });
  });

  it("updateUserSkill: repo trả null → NotFoundError (chặn cross-user update)", async () => {
    repo.updateUserSkill.mockResolvedValue(null);
    await expect(
      service.updateUserSkill(userId, "skill-of-another-user", {
        content: "hacked",
      }),
    ).rejects.toBeInstanceOf(NotFoundError);
  });

  it("deleteUserSkill: repo trả false → NotFoundError (chặn cross-user delete)", async () => {
    repo.deleteUserSkill.mockResolvedValue(false);
    await expect(
      service.deleteUserSkill(userId, "skill-of-another-user"),
    ).rejects.toBeInstanceOf(NotFoundError);
  });

  it("toggleBuiltinSkill: forward đúng userId, name, enabled xuống repo", async () => {
    repo.toggleBuiltinSkill.mockResolvedValue(undefined);
    await service.toggleBuiltinSkill(userId, "code-review", false);
    expect(repo.toggleBuiltinSkill).toHaveBeenCalledWith(
      userId,
      "code-review",
      false,
    );
  });

  it("listDisabledSkills: forward đúng userId xuống repo", async () => {
    repo.findDisabledSkills.mockResolvedValue(["debug"]);
    const result = await service.listDisabledSkills(userId);
    expect(repo.findDisabledSkills).toHaveBeenCalledWith(userId);
    expect(result).toEqual(["debug"]);
  });
});
