import type { UserRow } from "../auth/auth.repository";
import { UsersRepository } from "./users.repository";
import { NotFoundError, ForbiddenError } from "../../common/errors/app-errors";

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

  /** Admin vô hiệu hoá 1 user. Không được tự vô hiệu hoá chính mình. */
  disableUser = async (
    adminId: string,
    targetId: string,
  ): Promise<UserRow> => {
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
