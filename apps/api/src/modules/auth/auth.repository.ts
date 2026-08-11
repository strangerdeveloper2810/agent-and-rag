import type { Pool } from "pg";

// ── Row types (khớp với migration 001-create-auth-tables.sql) ──

export interface UserRow {
  id: string;
  email: string;
  name: string;
  avatar_url: string | null;
  role: "user" | "admin";
  status: "active" | "disabled" | "deleted";
  created_at: Date;
  updated_at: Date;
}

export interface CredentialRow {
  id: string;
  user_id: string;
  method: "email" | "google";
  password_hash: string | null;
  google_id: string | null;
  google_email: string | null;
}

export interface RefreshTokenRow {
  id: string;
  user_id: string;
  token_hash: string;
  family: string;
  expires_at: Date;
}

// ── Repository ──

export class AuthRepository {
  constructor(private pg: Pool) {}

  // ── Users ──

  async findUserByEmail(email: string): Promise<UserRow | null> {
    const { rows } = await this.pg.query<UserRow>(
      "SELECT * FROM users WHERE email = $1",
      [email.toLowerCase()],
    );
    return rows[0] ?? null;
  }

  async findUserById(id: string): Promise<UserRow | null> {
    const { rows } = await this.pg.query<UserRow>(
      "SELECT * FROM users WHERE id = $1",
      [id],
    );
    return rows[0] ?? null;
  }

  async createUser(
    email: string,
    name: string,
    avatarUrl?: string,
  ): Promise<UserRow> {
    const { rows } = await this.pg.query<UserRow>(
      `INSERT INTO users (email, name, avatar_url)
       VALUES ($1, $2, $3) RETURNING *`,
      [email.toLowerCase(), name, avatarUrl ?? null],
    );
    return rows[0];
  }

  async updateAvatar(userId: string, avatarUrl: string): Promise<void> {
    await this.pg.query("UPDATE users SET avatar_url = $1 WHERE id = $2", [
      avatarUrl,
      userId,
    ]);
  }

  // ── Credentials ──

  async findCredential(
    userId: string,
    method: "email" | "google",
  ): Promise<CredentialRow | null> {
    const { rows } = await this.pg.query<CredentialRow>(
      "SELECT * FROM credentials WHERE user_id = $1 AND method = $2",
      [userId, method],
    );
    return rows[0] ?? null;
  }

  async findCredentialByGoogleId(
    googleId: string,
  ): Promise<CredentialRow | null> {
    const { rows } = await this.pg.query<CredentialRow>(
      "SELECT * FROM credentials WHERE google_id = $1",
      [googleId],
    );
    return rows[0] ?? null;
  }

  async createEmailCredential(
    userId: string,
    passwordHash: string,
  ): Promise<void> {
    await this.pg.query(
      `INSERT INTO credentials (user_id, method, password_hash)
       VALUES ($1, 'email', $2)`,
      [userId, passwordHash],
    );
  }

  async createGoogleCredential(
    userId: string,
    googleId: string,
    googleEmail: string,
  ): Promise<void> {
    await this.pg.query(
      `INSERT INTO credentials (user_id, method, google_id, google_email)
       VALUES ($1, 'google', $2, $3)
       ON CONFLICT (google_id) DO NOTHING`,
      [userId, googleId, googleEmail],
    );
  }

  // ── Refresh Tokens ──

  async saveRefreshToken(
    tokenHash: string,
    userId: string,
    family: string,
    expiresAt: Date,
  ): Promise<void> {
    await this.pg.query(
      "INSERT INTO refresh_tokens (token_hash, user_id, family, expires_at) VALUES ($1, $2, $3, $4)",
      [tokenHash, userId, family, expiresAt],
    );
  }

  async findRefreshToken(
    tokenHash: string,
  ): Promise<RefreshTokenRow | null> {
    const { rows } = await this.pg.query<RefreshTokenRow>(
      "SELECT * FROM refresh_tokens WHERE token_hash = $1",
      [tokenHash],
    );
    return rows[0] ?? null;
  }

  async deleteRefreshToken(tokenHash: string): Promise<void> {
    await this.pg.query("DELETE FROM refresh_tokens WHERE token_hash = $1", [
      tokenHash,
    ]);
  }

  async deleteRefreshTokenFamily(family: string): Promise<void> {
    await this.pg.query("DELETE FROM refresh_tokens WHERE family = $1", [
      family,
    ]);
  }

  async deleteAllUserRefreshTokens(userId: string): Promise<void> {
    await this.pg.query("DELETE FROM refresh_tokens WHERE user_id = $1", [
      userId,
    ]);
  }
}
