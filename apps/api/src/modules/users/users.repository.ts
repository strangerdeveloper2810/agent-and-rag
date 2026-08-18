import type { Pool } from "pg";
import type { UserRow, CredentialRow } from "../auth/auth.repository";

export interface UserSettingsRow {
  user_id: string;
  persona_preset: "default" | "coder" | "business" | "creative" | "custom";
  formality: "casual" | "neutral" | "formal";
  verbosity: "concise" | "normal" | "detailed";
  humor: "none" | "dry" | "playful";
  custom_instructions: string;
  created_at: Date;
  updated_at: Date;
}

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

  /** Cập nhật thông tin profile của user. */
  updateProfile = async (
    id: string,
    data: { name?: string; avatar_url?: string | null },
  ): Promise<UserRow | null> => {
    const fields: string[] = [];
    const values: any[] = [];
    let idx = 1;

    if (data.name !== undefined) {
      fields.push(`name = $${idx++}`);
      values.push(data.name);
    }
    if (data.avatar_url !== undefined) {
      fields.push(`avatar_url = $${idx++}`);
      values.push(data.avatar_url);
    }

    if (fields.length === 0) {
      return this.findById(id);
    }

    values.push(id);
    const { rows } = await this.pg.query<UserRow>(
      `UPDATE users
       SET ${fields.join(", ")}
       WHERE id = $${idx} AND status != 'deleted'
       RETURNING id, email, name, avatar_url, role, status, created_at, updated_at`,
      values,
    );
    return rows[0] ?? null;
  };

  /** Tìm credential email của user. */
  findEmailCredential = async (
    userId: string,
  ): Promise<CredentialRow | null> => {
    const { rows } = await this.pg.query<CredentialRow>(
      `SELECT * FROM credentials WHERE user_id = $1 AND method = 'email'`,
      [userId],
    );
    return rows[0] ?? null;
  };

  /** Cập nhật mật khẩu email của user. */
  updatePassword = async (
    userId: string,
    passwordHash: string,
  ): Promise<void> => {
    await this.pg.query(
      `UPDATE credentials
       SET password_hash = $1
       WHERE user_id = $2 AND method = 'email'`,
      [passwordHash, userId],
    );
  };

  /** Lấy cài đặt persona của user. */
  findSettings = async (userId: string): Promise<UserSettingsRow | null> => {
    const { rows } = await this.pg.query<UserSettingsRow>(
      `SELECT * FROM user_settings WHERE user_id = $1`,
      [userId],
    );
    return rows[0] ?? null;
  };

  /** Tạo hoặc cập nhật cài đặt persona của user. */
  upsertSettings = async (
    userId: string,
    data: Partial<UserSettingsRow>,
  ): Promise<UserSettingsRow> => {
    const { rows } = await this.pg.query<UserSettingsRow>(
      `INSERT INTO user_settings (
        user_id,
        persona_preset,
        formality,
        verbosity,
        humor,
        custom_instructions
      ) VALUES (
        $1,
        COALESCE($2, 'default'),
        COALESCE($3, 'neutral'),
        COALESCE($4, 'normal'),
        COALESCE($5, 'none'),
        COALESCE($6, '')
      )
      ON CONFLICT (user_id) DO UPDATE SET
        persona_preset = COALESCE($2, user_settings.persona_preset),
        formality = COALESCE($3, user_settings.formality),
        verbosity = COALESCE($4, user_settings.verbosity),
        humor = COALESCE($5, user_settings.humor),
        custom_instructions = COALESCE($6, user_settings.custom_instructions)
      RETURNING *`,
      [
        userId,
        data.persona_preset ?? null,
        data.formality ?? null,
        data.verbosity ?? null,
        data.humor ?? null,
        data.custom_instructions ?? null,
      ],
    );
    return rows[0];
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
