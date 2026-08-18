import bcrypt from "bcrypt";
import type { UserRow } from "../auth/auth.repository";
import {
  UsersRepository,
  type UserSettingsRow,
  type McpServerRow,
  type UserSkillRow,
} from "./users.repository";
import {
  NotFoundError,
  ForbiddenError,
  ValidationError,
  UnauthorizedError,
} from "../../common/errors/app-errors";
import type { UpdateProfileInput } from "./dto/update-profile.dto";
import type { ChangePasswordInput } from "./dto/change-password.dto";
import type { UpdateSettingsInput } from "./dto/update-settings.dto";
import type {
  CreateMcpServerInput,
  UpdateMcpServerInput,
} from "./dto/mcp-server.dto";
import type {
  CreateUserSkillInput,
  UpdateUserSkillInput,
} from "./dto/user-skill.dto";

const BCRYPT_ROUNDS = 12;

// ── Users Service ──

export class UsersService {
  constructor(private repo: UsersRepository) {}

  /** Lấy danh sách user (phân trang). */
  listUsers = async (limit?: number, offset?: number): Promise<UserRow[]> => {
    return this.repo.findAll(limit, offset);
  };

  /** Lấy chi tiết 1 user theo ID. */
  getUser = async (id: string): Promise<UserRow> => {
    const user = await this.repo.findById(id);
    if (!user) {
      throw new NotFoundError("Không tìm thấy người dùng.");
    }
    return user;
  };

  /** Cập nhật profile người dùng. */
  updateProfile = async (
    userId: string,
    input: UpdateProfileInput,
  ): Promise<UserRow> => {
    const updated = await this.repo.updateProfile(userId, input);
    if (!updated) {
      throw new NotFoundError("Không tìm thấy người dùng.");
    }
    return updated;
  };

  /** Đổi mật khẩu người dùng. */
  changePassword = async (
    userId: string,
    input: ChangePasswordInput,
  ): Promise<{ message: string }> => {
    const cred = await this.repo.findEmailCredential(userId);
    if (!cred || !cred.password_hash) {
      throw new ValidationError(
        "Tài khoản của bạn đăng nhập bằng Google, không có mật khẩu để đổi.",
      );
    }

    const match = await bcrypt.compare(input.oldPassword, cred.password_hash);
    if (!match) {
      throw new UnauthorizedError("Mật khẩu hiện tại không chính xác.");
    }

    const newHash = await bcrypt.hash(input.newPassword, BCRYPT_ROUNDS);
    await this.repo.updatePassword(userId, newHash);

    return { message: "Đổi mật khẩu thành công." };
  };

  /** Lấy cài đặt Persona / Custom Instructions của người dùng. */
  getSettings = async (userId: string): Promise<UserSettingsRow> => {
    const existing = await this.repo.findSettings(userId);
    if (existing) return existing;

    // Default settings nếu chưa tạo
    return {
      user_id: userId,
      persona_preset: "default",
      formality: "neutral",
      verbosity: "normal",
      humor: "none",
      custom_instructions: "",
      agent_avatar_url: null,
      created_at: new Date(),
      updated_at: new Date(),
    };
  };

  /** Cập nhật cài đặt Persona / Custom Instructions của người dùng. */
  updateSettings = async (
    userId: string,
    input: UpdateSettingsInput,
  ): Promise<UserSettingsRow> => {
    return this.repo.upsertSettings(userId, input);
  };

  // ── MCP Servers ──

  /** Lấy danh sách MCP servers của user. */
  listMcpServers = async (userId: string): Promise<McpServerRow[]> => {
    return this.repo.findMcpServers(userId);
  };

  /** Thêm MCP server cho user. */
  createMcpServer = async (
    userId: string,
    input: CreateMcpServerInput,
  ): Promise<McpServerRow> => {
    return this.repo.createMcpServer(userId, input);
  };

  /** Cập nhật MCP server. */
  updateMcpServer = async (
    userId: string,
    id: string,
    input: UpdateMcpServerInput,
  ): Promise<McpServerRow> => {
    const updated = await this.repo.updateMcpServer(userId, id, input);
    if (!updated) {
      throw new NotFoundError("Không tìm thấy MCP server.");
    }
    return updated;
  };

  /** Xoá MCP server. */
  deleteMcpServer = async (userId: string, id: string): Promise<void> => {
    const deleted = await this.repo.deleteMcpServer(userId, id);
    if (!deleted) {
      throw new NotFoundError("Không tìm thấy MCP server.");
    }
  };

  // ── Skills ──

  /** Lấy danh sách custom skills của user. */
  listUserSkills = async (userId: string): Promise<UserSkillRow[]> => {
    return this.repo.findUserSkills(userId);
  };

  /** Lấy danh sách builtin skills bị disable. */
  listDisabledSkills = async (userId: string): Promise<string[]> => {
    return this.repo.findDisabledSkills(userId);
  };

  /** Thêm custom skill cho user. */
  createUserSkill = async (
    userId: string,
    input: CreateUserSkillInput,
  ): Promise<UserSkillRow> => {
    return this.repo.createUserSkill(userId, {
      name: input.name,
      description: input.description ?? "",
      when_to_use: input.when_to_use ?? "",
      content: input.content,
      triggers: input.triggers ?? [],
    });
  };

  /** Cập nhật custom skill. */
  updateUserSkill = async (
    userId: string,
    id: string,
    input: UpdateUserSkillInput,
  ): Promise<UserSkillRow> => {
    const updated = await this.repo.updateUserSkill(userId, id, input);
    if (!updated) {
      throw new NotFoundError("Không tìm thấy skill.");
    }
    return updated;
  };

  /** Xoá custom skill. */
  deleteUserSkill = async (userId: string, id: string): Promise<void> => {
    const deleted = await this.repo.deleteUserSkill(userId, id);
    if (!deleted) {
      throw new NotFoundError("Không tìm thấy skill.");
    }
  };

  /** Toggle builtin skill (bật/tắt). */
  toggleBuiltinSkill = async (
    userId: string,
    skillName: string,
    enabled: boolean,
  ): Promise<void> => {
    await this.repo.toggleBuiltinSkill(userId, skillName, enabled);
  };

  /** Admin vô hiệu hoá 1 user. Không được tự vô hiệu hoá chính mình. */
  disableUser = async (adminId: string, targetId: string): Promise<UserRow> => {
    if (adminId === targetId) {
      throw new ForbiddenError(
        "Không thể tự vô hiệu hoá tài khoản của chính mình.",
      );
    }

    const updated = await this.repo.updateStatus(targetId, "disabled");
    if (!updated) {
      throw new NotFoundError("Không tìm thấy người dùng để vô hiệu hoá.");
    }
    return updated;
  };

  /** Admin kích hoạt lại 1 user đã bị vô hiệu hoá. */
  enableUser = async (targetId: string): Promise<UserRow> => {
    const updated = await this.repo.updateStatus(targetId, "active");
    if (!updated) {
      throw new NotFoundError("Không tìm thấy người dùng để kích hoạt.");
    }
    return updated;
  };

  /** Tìm kiếm user theo tên hoặc email. */
  searchUsers = async (query: string): Promise<UserRow[]> => {
    return this.repo.search(query);
  };
}
