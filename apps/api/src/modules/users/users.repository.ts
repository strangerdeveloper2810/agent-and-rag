import type { Pool } from "pg";
import type { UserRow } from "../auth/auth.repository";

// ── Users Repository ──

export class UsersRepository {
  constructor(private pg: Pool) {}

  /** Lấy danh sách user (không bao gồm user đã xoá mềm), phân trang. */
  findAll = async (limit = 50, offset = 0): Promise<UserRow[]> => {
    const { rows } = await this.pg.query<UserRow>(
      `SELECT id, email, name, avatar_url, role, status, created_at, updated_at
       FROM users
       WHERE status != 'deleted'
       ORDER BY created_at DESC
       LIMIT $1 OFFSET $2`,
      [limit, offset],
    );
    return rows;
  };

  /** Tìm user theo ID (không phân biệt status). */
  findById = async (id: string): Promise<UserRow | null> => {
    const { rows } = await this.pg.query<UserRow>(
      `SELECT id, email, name, avatar_url, role, status, created_at, updated_at
       FROM users
       WHERE id = $1`,
      [id],
    );
    return rows[0] ?? null;
  };

  /** Cập nhật status user. Chỉ áp dụng cho user không bị xoá mềm. */
  updateStatus = async (
    id: string,
    status: "active" | "disabled",
  ): Promise<UserRow | null> => {
    const { rows } = await this.pg.query<UserRow>(
      `UPDATE users
       SET status = $1
       WHERE id = $2 AND status != 'deleted'
       RETURNING id, email, name, avatar_url, role, status, created_at, updated_at`,
      [status, id],
    );
    return rows[0] ?? null;
  };

  /** Tìm kiếm user theo tên hoặc email (ILIKE, không phân biệt hoa thường). */
  search = async (query: string, limit = 50): Promise<UserRow[]> => {
    const { rows } = await this.pg.query<UserRow>(
      `SELECT id, email, name, avatar_url, role, status, created_at, updated_at
       FROM users
       WHERE status != 'deleted'
         AND (name ILIKE $1 OR email ILIKE $1)
       ORDER BY created_at DESC
       LIMIT $2`,
      [`%${query}%`, limit],
    );
    return rows;
  };
}
