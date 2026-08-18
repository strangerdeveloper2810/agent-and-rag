import type { Pool } from "pg";
import type { UserRow, CredentialRow } from "../auth/auth.repository";

export interface UserSettingsRow {
  user_id: string;
  persona_preset: "default" | "coder" | "business" | "creative" | "custom";
  formality: "casual" | "neutral" | "formal";
  verbosity: "concise" | "normal" | "detailed";
  humor: "none" | "dry" | "playful";
  custom_instructions: string;
  agent_avatar_url: string | null;
  created_at: Date;
  updated_at: Date;
}

export interface McpServerRow {
  id: string;
  user_id: string;
  name: string;
  transport: "sse";
  url: string;
  api_key: string | null;
  enabled: boolean;
  created_at: Date;
  updated_at: Date;
}

export interface UserSkillRow {
  id: string;
  user_id: string;
  name: string;
  description: string;
  when_to_use: string;
  content: string;
  triggers: string[];
  enabled: boolean;
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
        custom_instructions,
        agent_avatar_url
      ) VALUES (
        $1,
        COALESCE($2, 'default'),
        COALESCE($3, 'neutral'),
        COALESCE($4, 'normal'),
        COALESCE($5, 'none'),
        COALESCE($6, ''),
        $7
      )
      ON CONFLICT (user_id) DO UPDATE SET
        persona_preset = COALESCE($2, user_settings.persona_preset),
        formality = COALESCE($3, user_settings.formality),
        verbosity = COALESCE($4, user_settings.verbosity),
        humor = COALESCE($5, user_settings.humor),
        custom_instructions = COALESCE($6, user_settings.custom_instructions),
        agent_avatar_url = COALESCE($7, user_settings.agent_avatar_url)
      RETURNING *`,
      [
        userId,
        data.persona_preset ?? null,
        data.formality ?? null,
        data.verbosity ?? null,
        data.humor ?? null,
        data.custom_instructions ?? null,
        data.agent_avatar_url ?? null,
      ],
    );
    return rows[0];
  };

  // ── MCP Servers ──

  /** Lấy danh sách MCP servers của user. */
  findMcpServers = async (userId: string): Promise<McpServerRow[]> => {
    const { rows } = await this.pg.query<McpServerRow>(
      `SELECT * FROM user_mcp_servers WHERE user_id = $1 ORDER BY created_at ASC`,
      [userId],
    );
    return rows;
  };

  /** Lấy MCP server theo ID (phải thuộc user). */
  findMcpServerById = async (
    userId: string,
    id: string,
  ): Promise<McpServerRow | null> => {
    const { rows } = await this.pg.query<McpServerRow>(
      `SELECT * FROM user_mcp_servers WHERE id = $1 AND user_id = $2`,
      [id, userId],
    );
    return rows[0] ?? null;
  };

  /** Thêm MCP server cho user. */
  createMcpServer = async (
    userId: string,
    data: { name: string; url: string; api_key?: string | null },
  ): Promise<McpServerRow> => {
    const { rows } = await this.pg.query<McpServerRow>(
      `INSERT INTO user_mcp_servers (user_id, name, url, api_key)
       VALUES ($1, $2, $3, $4)
       RETURNING *`,
      [userId, data.name, data.url, data.api_key ?? null],
    );
    return rows[0];
  };

  /** Cập nhật MCP server. */
  updateMcpServer = async (
    userId: string,
    id: string,
    data: {
      name?: string;
      url?: string;
      api_key?: string | null;
      enabled?: boolean;
    },
  ): Promise<McpServerRow | null> => {
    const fields: string[] = [];
    const values: any[] = [];
    let idx = 1;

    if (data.name !== undefined) {
      fields.push(`name = $${idx++}`);
      values.push(data.name);
    }
    if (data.url !== undefined) {
      fields.push(`url = $${idx++}`);
      values.push(data.url);
    }
    if (data.api_key !== undefined) {
      fields.push(`api_key = $${idx++}`);
      values.push(data.api_key);
    }
    if (data.enabled !== undefined) {
      fields.push(`enabled = $${idx++}`);
      values.push(data.enabled);
    }

    if (fields.length === 0) {
      return this.findMcpServerById(userId, id);
    }

    values.push(id, userId);
    const { rows } = await this.pg.query<McpServerRow>(
      `UPDATE user_mcp_servers
       SET ${fields.join(", ")}
       WHERE id = $${idx} AND user_id = $${idx + 1}
       RETURNING *`,
      values,
    );
    return rows[0] ?? null;
  };

  /** Xoá MCP server. */
  deleteMcpServer = async (userId: string, id: string): Promise<boolean> => {
    const result = await this.pg.query(
      `DELETE FROM user_mcp_servers WHERE id = $1 AND user_id = $2`,
      [id, userId],
    );
    return (result.rowCount ?? 0) > 0;
  };

  // ── User Skills ──

  /** Lấy danh sách custom skills của user. */
  findUserSkills = async (userId: string): Promise<UserSkillRow[]> => {
    const { rows } = await this.pg.query<UserSkillRow>(
      `SELECT * FROM user_skills WHERE user_id = $1 ORDER BY created_at ASC`,
      [userId],
    );
    return rows;
  };

  /** Lấy custom skill theo ID (phải thuộc user). */
  findUserSkillById = async (
    userId: string,
    id: string,
  ): Promise<UserSkillRow | null> => {
    const { rows } = await this.pg.query<UserSkillRow>(
      `SELECT * FROM user_skills WHERE id = $1 AND user_id = $2`,
      [id, userId],
    );
    return rows[0] ?? null;
  };

  /** Thêm custom skill cho user. */
  createUserSkill = async (
    userId: string,
    data: {
      name: string;
      description: string;
      when_to_use: string;
      content: string;
      triggers: string[];
    },
  ): Promise<UserSkillRow> => {
    const { rows } = await this.pg.query<UserSkillRow>(
      `INSERT INTO user_skills (user_id, name, description, when_to_use, content, triggers)
       VALUES ($1, $2, $3, $4, $5, $6)
       RETURNING *`,
      [
        userId,
        data.name,
        data.description,
        data.when_to_use,
        data.content,
        data.triggers,
      ],
    );
    return rows[0];
  };

  /** Cập nhật custom skill. */
  updateUserSkill = async (
    userId: string,
    id: string,
    data: {
      name?: string;
      description?: string;
      when_to_use?: string;
      content?: string;
      triggers?: string[];
      enabled?: boolean;
    },
  ): Promise<UserSkillRow | null> => {
    const fields: string[] = [];
    const values: any[] = [];
    let idx = 1;

    if (data.name !== undefined) {
      fields.push(`name = $${idx++}`);
      values.push(data.name);
    }
    if (data.description !== undefined) {
      fields.push(`description = $${idx++}`);
      values.push(data.description);
    }
    if (data.when_to_use !== undefined) {
      fields.push(`when_to_use = $${idx++}`);
      values.push(data.when_to_use);
    }
    if (data.content !== undefined) {
      fields.push(`content = $${idx++}`);
      values.push(data.content);
    }
    if (data.triggers !== undefined) {
      fields.push(`triggers = $${idx++}`);
      values.push(data.triggers);
    }
    if (data.enabled !== undefined) {
      fields.push(`enabled = $${idx++}`);
      values.push(data.enabled);
    }

    if (fields.length === 0) {
      return this.findUserSkillById(userId, id);
    }

    values.push(id, userId);
    const { rows } = await this.pg.query<UserSkillRow>(
      `UPDATE user_skills
       SET ${fields.join(", ")}
       WHERE id = $${idx} AND user_id = $${idx + 1}
       RETURNING *`,
      values,
    );
    return rows[0] ?? null;
  };

  /** Xoá custom skill. */
  deleteUserSkill = async (userId: string, id: string): Promise<boolean> => {
    const result = await this.pg.query(
      `DELETE FROM user_skills WHERE id = $1 AND user_id = $2`,
      [id, userId],
    );
    return (result.rowCount ?? 0) > 0;
  };

  // ── Disabled Builtin Skills ──

  /** Lấy danh sách tên builtin skill bị disable. */
  findDisabledSkills = async (userId: string): Promise<string[]> => {
    const { rows } = await this.pg.query<{ skill_name: string }>(
      `SELECT skill_name FROM user_disabled_skills WHERE user_id = $1`,
      [userId],
    );
    return rows.map((r) => r.skill_name);
  };

  /** Toggle builtin skill: disable hoặc enable. */
  toggleBuiltinSkill = async (
    userId: string,
    skillName: string,
    enabled: boolean,
  ): Promise<void> => {
    if (enabled) {
      // Enable = remove from disabled list
      await this.pg.query(
        `DELETE FROM user_disabled_skills WHERE user_id = $1 AND skill_name = $2`,
        [userId, skillName],
      );
    } else {
      // Disable = add to disabled list
      await this.pg.query(
        `INSERT INTO user_disabled_skills (user_id, skill_name)
         VALUES ($1, $2)
         ON CONFLICT DO NOTHING`,
        [userId, skillName],
      );
    }
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
