# Auth + Multi-Tenant — Implementation Plan

> **Design doc:** `docs/plans/2026-07-25-auth-tenant-design.md`
> **Date:** 2026-07-25

---

## Tổng quan các phase

| Phase | Topic | Mô tả |
|-------|-------|-------|
| 1 | PostgreSQL Setup | Tạo DB, migration tables, connection pool |
| 2 | Common Layer | Guards, filters, pipes, interfaces |
| 3 | Auth Module | Register, login, OAuth, token service |
| 4 | Users Module | Admin user management |
| 5 | BFF Refactor | Chuyển chat/documents/tasks sang module pattern, thêm tenant filter |
| 6 | Go Agent | Nhận X-Tenant-ID, filter theo tenant |
| 7 | Frontend | Auth pages, guards, store, HTTP interceptor |
| 8 | Docker + Env | Cấu hình env vars, docker-compose updates |

---

## Phase 1: PostgreSQL Setup

### 1.1 Cài đặt dependencies

```bash
cd apps/api
pnpm add pg
pnpm add -D @types/pg
```

### 1.2 Tạo migration file

**File:** `apps/api/src/database/postgres/migrations/001-create-auth-tables.sql`

```sql
-- Enable UUID generation
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Users table
CREATE TABLE IF NOT EXISTS users (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email       VARCHAR(255) UNIQUE NOT NULL,
    name        VARCHAR(255) NOT NULL,
    avatar_url  VARCHAR(512),
    role        VARCHAR(20) NOT NULL DEFAULT 'user' CHECK (role IN ('user', 'admin')),
    status      VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled', 'deleted')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Credentials table
CREATE TABLE IF NOT EXISTS credentials (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    method          VARCHAR(20) NOT NULL CHECK (method IN ('email', 'google')),
    password_hash   VARCHAR(255),
    google_id       VARCHAR(255),
    google_email    VARCHAR(255),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, method),
    UNIQUE(google_id)
);

-- Refresh tokens table
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  VARCHAR(255) UNIQUE NOT NULL,
    family      UUID NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user ON refresh_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_expires ON refresh_tokens(expires_at);

-- Auto-update updated_at
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER credentials_updated_at
    BEFORE UPDATE ON credentials
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
```

### 1.3 PostgreSQL module

**File:** `apps/api/src/database/postgres/postgres.module.ts`

```typescript
import { Pool, type PoolConfig } from 'pg';
import fs from 'fs';
import path from 'path';

let pool: Pool | null = null;

export interface PostgresConfig {
  connectionString: string;        // postgresql://user:pass@host:5432/db
  max?: number;                    // pool size, default 10
}

export async function initPostgres(config: PostgresConfig): Promise<Pool> {
  if (pool) return pool;

  pool = new Pool({
    connectionString: config.connectionString,
    max: config.max ?? 10,
  });

  // Test connection
  await pool.query('SELECT 1');

  // Auto-run migrations
  await runMigrations(pool);

  return pool;
}

async function runMigrations(pool: Pool): Promise<void> {
  const migrationsDir = path.join(__dirname, 'migrations');
  const files = fs.readdirSync(migrationsDir)
    .filter(f => f.endsWith('.sql'))
    .sort(); // 001-..., 002-... chạy theo thứ tự

  for (const file of files) {
    const sql = fs.readFileSync(path.join(migrationsDir, file), 'utf-8');
    await pool.query(sql);
  }
}

export function getPgPool(): Pool {
  if (!pool) throw new Error('PostgreSQL not initialized. Call initPostgres() first.');
  return pool;
}

export async function closePostgres(): Promise<void> {
  await pool?.end();
  pool = null;
}
```

### 1.4 Cập nhật config

**File:** `apps/api/src/config/env.ts` — thêm:

```typescript
// PostgreSQL (Auth DB)
PG_CONNECTION_STRING: z.string().min(1, "PG_CONNECTION_STRING is required"),

// JWT
JWT_SECRET: z.string().min(32, "JWT_SECRET must be at least 32 characters"),
JWT_REFRESH_SECRET: z.string().min(32, "JWT_REFRESH_SECRET must be at least 32 characters"),

// Google OAuth
GOOGLE_CLIENT_ID: z.string().min(1, "GOOGLE_CLIENT_ID is required"),
GOOGLE_CLIENT_SECRET: z.string().min(1, "GOOGLE_CLIENT_SECRET is required"),
GOOGLE_REDIRECT_URI: z.string().url("GOOGLE_REDIRECT_URI must be a valid URL"),
```

---

## Phase 2: Common Layer

### 2.1 Interfaces

**File:** `apps/api/src/common/interfaces/auth-context.ts`

```typescript
export interface JwtPayload {
  sub: string;      // user.id
  email: string;
  role: 'user' | 'admin';
}

export interface AuthContext {
  tenantId: string; // = user.id
  user: {
    id: string;
    email: string;
    role: 'user' | 'admin';
  };
}

// Augment Fastify request
declare module 'fastify' {
  interface FastifyRequest {
    tenantId: string;
    user: JwtPayload;
  }
}
```

### 2.2 Error classes

**File:** `apps/api/src/common/errors/app-errors.ts`

```typescript
export class AppError extends Error {
  constructor(
    message: string,
    public statusCode: number = 500,
    public code: string = 'INTERNAL_ERROR',
  ) {
    super(message);
    this.name = 'AppError';
  }
}

export class UnauthorizedError extends AppError {
  constructor(message = 'Authentication required') {
    super(message, 401, 'UNAUTHORIZED');
  }
}

export class ForbiddenError extends AppError {
  constructor(message = 'Access denied') {
    super(message, 403, 'FORBIDDEN');
  }
}

export class NotFoundError extends AppError {
  constructor(message = 'Resource not found') {
    super(message, 404, 'NOT_FOUND');
  }
}

export class ConflictError extends AppError {
  constructor(message = 'Resource already exists') {
    super(message, 409, 'CONFLICT');
  }
}

export class ValidationError extends AppError {
  public fieldErrors: Record<string, string[]>;
  constructor(fieldErrors: Record<string, string[]>) {
    super('Validation failed', 422, 'VALIDATION_ERROR');
    this.fieldErrors = fieldErrors;
  }
}
```

### 2.3 Auth Guard

**File:** `apps/api/src/common/guards/auth.guard.ts`

```typescript
import type { preHandlerHookHandler } from 'fastify';
import jwt from 'jsonwebtoken';
import { config } from '../../config';
import { UnauthorizedError } from '../errors/app-errors';
import type { JwtPayload } from '../interfaces/auth-context';

export const authGuard: preHandlerHookHandler = async (req) => {
  const token = req.cookies?.access_token;
  if (!token) {
    throw new UnauthorizedError('Authentication required');
  }

  try {
    const payload = jwt.verify(token, config.JWT_SECRET) as JwtPayload;
    req.tenantId = payload.sub;
    req.user = payload;
  } catch (err) {
    if (err instanceof jwt.TokenExpiredError) {
      throw new UnauthorizedError('Token expired');
    }
    throw new UnauthorizedError('Invalid token');
  }
};
```

### 2.4 Admin Guard

**File:** `apps/api/src/common/guards/admin.guard.ts`

```typescript
import type { preHandlerHookHandler } from 'fastify';
import { ForbiddenError } from '../errors/app-errors';

export const adminGuard: preHandlerHookHandler = async (req) => {
  if (req.user?.role !== 'admin') {
    throw new ForbiddenError('Admin access required');
  }
};
```

### 2.5 Error Filter (refactor từ middleware hiện tại)

**File:** `apps/api/src/common/filters/error.filter.ts`

```typescript
import type { FastifyInstance } from 'fastify';
import { AppError } from '../errors/app-errors';

export function registerErrorFilter(app: FastifyInstance): void {
  app.setErrorHandler((error, req, reply) => {
    // AppError → structured response
    if (error instanceof AppError) {
      return reply.status(error.statusCode).send({
        error: error.code,
        message: error.message,
        ...(error.fieldErrors ? { fields: error.fieldErrors } : {}),
      });
    }

    // Fastify validation error
    if (error.validation) {
      return reply.status(422).send({
        error: 'VALIDATION_ERROR',
        message: error.message,
        fields: error.validation.reduce((acc, e) => {
          const key = e.dataPath?.replace('/', '') || 'body';
          (acc[key] ??= []).push(e.message!);
          return acc;
        }, {} as Record<string, string[]>),
      });
    }

    // Unknown error → log + generic 500
    req.log.error(error);
    return reply.status(500).send({
      error: 'INTERNAL_ERROR',
      message: config.NODE_ENV === 'production'
        ? 'Something went wrong'
        : error.message,
    });
  });
}
```

### 2.6 Validation Pipe

**File:** `apps/api/src/common/pipes/validation.pipe.ts`

```typescript
import type { ZodSchema } from 'zod';
import { ValidationError } from '../errors/app-errors';

export function validate<T>(schema: ZodSchema<T>, data: unknown): T {
  const result = schema.safeParse(data);
  if (!result.success) {
    const fieldErrors: Record<string, string[]> = {};
    for (const issue of result.error.issues) {
      const key = issue.path.join('.') || 'body';
      (fieldErrors[key] ??= []).push(issue.message);
    }
    throw new ValidationError(fieldErrors);
  }
  return result.data;
}
```

### 2.7 MongoDB tenant filter helper

**File:** `apps/api/src/database/mongo/tenant.filter.ts`

```typescript
import type { FastifyRequest } from 'fastify';

/**
 * Tạo Mongo filter tự động gắn tenantId từ request context.
 * Dùng trong mọi repository để đảm bảo multi-tenant isolation.
 *
 * @example
 * db.collection('conversations').find(tenantFilter(req, { status: 'active' }))
 * // → { tenantId: 'uuid', status: 'active' }
 */
export function tenantFilter(
  req: FastifyRequest,
  extra: Record<string, unknown> = {},
): Record<string, unknown> {
  return { tenantId: req.tenantId, ...extra };
}
```

### 2.8 MongoDB — cập nhật collections + indexes

**File:** `apps/api/src/database/mongo/collections.ts` — thêm `tenantId`:

```typescript
export const COLLECTIONS = {
  conversations: 'conversations',
  messages: 'messages',
  tasks: 'tasks',
  documents: 'documents',
  documentVersions: 'document_versions',
} as const;
```

**File:** `apps/api/src/database/mongo/mongo.module.ts` — cập nhật `ensureIndexes`:

```typescript
export async function ensureIndexes(): Promise<void> {
  const database = getDb();
  await Promise.all([
    // --- Tenant-scoped indexes (MỚI) ---
    database.collection(COLLECTIONS.conversations)
      .createIndex({ tenantId: 1, updatedAt: -1 }),
    database.collection(COLLECTIONS.messages)
      .createIndex({ tenantId: 1, conversationId: 1, createdAt: 1 }),
    database.collection(COLLECTIONS.tasks)
      .createIndex({ tenantId: 1, status: 1, priority: 1 }),
    database.collection(COLLECTIONS.tasks)
      .createIndex({ tenantId: 1, tags: 1 }),
    database.collection(COLLECTIONS.documents)
      .createIndex({ tenantId: 1, documentId: 1, chunkIndex: 1 }),
    database.collection(COLLECTIONS.documents)
      .createIndex({ tenantId: 1, source: 1 }),
    database.collection(COLLECTIONS.documentVersions)
      .createIndex({ tenantId: 1, documentId: 1, version: 1 }),

    // --- Legacy indexes (giữ lại cho tương thích ngược — sẽ xoá sau migrate) ---
    // Giữ nguyên các index cũ trong giai đoạn chuyển tiếp
  ]);
}
```

---

## Phase 3: Auth Module

### 3.1 DTOs (Zod schemas)

**File:** `apps/api/src/modules/auth/dto/register.dto.ts`

```typescript
import { z } from 'zod';

export const registerSchema = z.object({
  email: z.string().email('Invalid email address'),
  password: z.string().min(8, 'Password must be at least 8 characters'),
  name: z.string().min(1, 'Name is required').max(100),
});

export type RegisterInput = z.infer<typeof registerSchema>;
```

**File:** `apps/api/src/modules/auth/dto/login.dto.ts`

```typescript
import { z } from 'zod';

export const loginSchema = z.object({
  email: z.string().email('Invalid email address'),
  password: z.string().min(1, 'Password is required'),
});

export type LoginInput = z.infer<typeof loginSchema>;
```

### 3.2 Auth Repository

**File:** `apps/api/src/modules/auth/auth.repository.ts`

```typescript
import type { Pool } from 'pg';

export interface UserRow {
  id: string;
  email: string;
  name: string;
  avatar_url: string | null;
  role: 'user' | 'admin';
  status: 'active' | 'disabled' | 'deleted';
  created_at: Date;
  updated_at: Date;
}

export interface CredentialRow {
  id: string;
  user_id: string;
  method: 'email' | 'google';
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

export class AuthRepository {
  constructor(private pg: Pool) {}

  // ── Users ──

  async findUserByEmail(email: string): Promise<UserRow | null> {
    const { rows } = await this.pg.query<UserRow>(
      'SELECT * FROM users WHERE email = $1',
      [email.toLowerCase()],
    );
    return rows[0] ?? null;
  }

  async findUserById(id: string): Promise<UserRow | null> {
    const { rows } = await this.pg.query<UserRow>(
      'SELECT * FROM users WHERE id = $1',
      [id],
    );
    return rows[0] ?? null;
  }

  async createUser(email: string, name: string, avatarUrl?: string): Promise<UserRow> {
    const { rows } = await this.pg.query<UserRow>(
      `INSERT INTO users (email, name, avatar_url) 
       VALUES ($1, $2, $3) RETURNING *`,
      [email.toLowerCase(), name, avatarUrl ?? null],
    );
    return rows[0];
  }

  async updateAvatar(userId: string, avatarUrl: string): Promise<void> {
    await this.pg.query(
      'UPDATE users SET avatar_url = $1 WHERE id = $2',
      [avatarUrl, userId],
    );
  }

  // ── Credentials ──

  async findCredential(userId: string, method: 'email' | 'google'): Promise<CredentialRow | null> {
    const { rows } = await this.pg.query<CredentialRow>(
      'SELECT * FROM credentials WHERE user_id = $1 AND method = $2',
      [userId, method],
    );
    return rows[0] ?? null;
  }

  async findCredentialByGoogleId(googleId: string): Promise<CredentialRow | null> {
    const { rows } = await this.pg.query<CredentialRow>(
      'SELECT * FROM credentials WHERE google_id = $1',
      [googleId],
    );
    return rows[0] ?? null;
  }

  async createEmailCredential(userId: string, passwordHash: string): Promise<void> {
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

  async saveRefreshToken(tokenHash: string, userId: string, family: string, expiresAt: Date): Promise<void> {
    await this.pg.query(
      'INSERT INTO refresh_tokens (token_hash, user_id, family, expires_at) VALUES ($1, $2, $3, $4)',
      [tokenHash, userId, family, expiresAt],
    );
  }

  async findRefreshToken(tokenHash: string): Promise<RefreshTokenRow | null> {
    const { rows } = await this.pg.query<RefreshTokenRow>(
      'SELECT * FROM refresh_tokens WHERE token_hash = $1',
      [tokenHash],
    );
    return rows[0] ?? null;
  }

  async deleteRefreshToken(tokenHash: string): Promise<void> {
    await this.pg.query('DELETE FROM refresh_tokens WHERE token_hash = $1', [tokenHash]);
  }

  async deleteRefreshTokenFamily(family: string): Promise<void> {
    await this.pg.query('DELETE FROM refresh_tokens WHERE family = $1', [family]);
  }

  async deleteAllUserRefreshTokens(userId: string): Promise<void> {
    await this.pg.query('DELETE FROM refresh_tokens WHERE user_id = $1', [userId]);
  }
}
```

### 3.3 JWT Strategy

**File:** `apps/api/src/modules/auth/strategies/jwt.strategy.ts`

```typescript
import jwt from 'jsonwebtoken';
import crypto from 'crypto';
import type { FastifyReply } from 'fastify';
import { config } from '../../../config';
import type { JwtPayload } from '../../../common/interfaces/auth-context';
import type { UserRow } from '../auth.repository';

export class JwtStrategy {
  private readonly secret: string;
  private readonly accessExpiry: number;   // seconds
  private readonly refreshExpiry: number;  // seconds

  constructor() {
    this.secret = config.JWT_SECRET;
    this.accessExpiry = 15 * 60;           // 15 phút
    this.refreshExpiry = 7 * 24 * 60 * 60; // 7 ngày
  }

  // ── Token generation ──

  generateAccessToken(user: UserRow): string {
    return jwt.sign(
      { sub: user.id, email: user.email, role: user.role } satisfies JwtPayload,
      this.secret,
      { expiresIn: this.accessExpiry },
    );
  }

  generateRefreshToken(): { token: string; hash: string } {
    const token = crypto.randomBytes(48).toString('base64url');
    const hash = crypto.createHash('sha256').update(token).digest('hex');
    return { token, hash };
  }

  // ── Cookie setters ──

  setAccessTokenCookie(reply: FastifyReply, token: string): void {
    reply.setCookie('access_token', token, {
      httpOnly: true,
      secure: config.NODE_ENV === 'production',
      sameSite: 'lax',
      path: '/',
      maxAge: this.accessExpiry,
    });
  }

  setRefreshTokenCookie(reply: FastifyReply, token: string): void {
    reply.setCookie('refresh_token', token, {
      httpOnly: true,
      secure: config.NODE_ENV === 'production',
      sameSite: 'lax',
      path: '/api/auth',      // chỉ gửi đến auth endpoints
      maxAge: this.refreshExpiry,
    });
  }

  clearTokens(reply: FastifyReply): void {
    reply.clearCookie('access_token', { path: '/' });
    reply.clearCookie('refresh_token', { path: '/api/auth' });
  }

  // ── Verify ──

  verify(token: string): JwtPayload {
    return jwt.verify(token, this.secret) as JwtPayload;
  }
}
```

### 3.4 Token Service

**File:** `apps/api/src/modules/auth/strategies/token.service.ts`

```typescript
import crypto from 'crypto';
import { v4 as uuid } from 'uuid';
import type { AuthRepository, UserRow } from '../auth.repository';

export class TokenService {
  private readonly refreshExpiryMs = 7 * 24 * 60 * 60 * 1000; // 7 ngày

  constructor(
    private repo: AuthRepository,
    private jwt: JwtStrategy,
  ) {}

  /**
   * Cấp access + refresh token, lưu refresh hash vào DB.
   */
  async issueTokens(user: UserRow): Promise<{
    accessToken: string;
    refreshToken: string;
    refreshHash: string;
    family: string;
  }> {
    const accessToken = this.jwt.generateAccessToken(user);
    const { token: refreshToken, hash: refreshHash } = this.jwt.generateRefreshToken();
    const family = uuid();

    await this.repo.saveRefreshToken(
      refreshHash,
      user.id,
      family,
      new Date(Date.now() + this.refreshExpiryMs),
    );

    return { accessToken, refreshToken, refreshHash, family };
  }

  /**
   * Xoá refresh token hiện tại, cấp cặp mới (rotation).
   */
  async rotateTokens(
    user: UserRow,
    oldTokenHash: string,
    family: string,
  ): Promise<{ accessToken: string; refreshToken: string }> {
    // Xoá token cũ
    await this.repo.deleteRefreshToken(oldTokenHash);

    // Cấp token mới cùng family
    const { token: newRefreshToken, hash: newHash } = this.jwt.generateRefreshToken();
    await this.repo.saveRefreshToken(
      newHash,
      user.id,
      family,
      new Date(Date.now() + this.refreshExpiryMs),
    );

    return {
      accessToken: this.jwt.generateAccessToken(user),
      refreshToken: newRefreshToken,
    };
  }

  /**
   * Revoke ALL tokens của user (logout all devices).
   */
  async revokeAll(userId: string): Promise<void> {
    await this.repo.deleteAllUserRefreshTokens(userId);
  }
}
```

### 3.5 Google Strategy

**File:** `apps/api/src/modules/auth/strategies/google.strategy.ts`

```typescript
import { config } from '../../../config';

interface GoogleTokens {
  access_token: string;
  id_token: string;
}

interface GoogleUser {
  sub: string;        // Google ID
  email: string;
  name: string;
  picture: string;
  email_verified: boolean;
}

export class GoogleStrategy {
  /**
   * Tạo URL redirect đến Google OAuth consent screen.
   */
  getAuthUrl(): string {
    const params = new URLSearchParams({
      client_id: config.GOOGLE_CLIENT_ID,
      redirect_uri: config.GOOGLE_REDIRECT_URI,
      response_type: 'code',
      scope: 'openid email profile',
      access_type: 'offline',
      prompt: 'consent',
    });
    return `https://accounts.google.com/o/oauth2/v2/auth?${params}`;
  }

  /**
   * Exchange authorization code lấy tokens.
   */
  async exchangeCode(code: string): Promise<GoogleTokens> {
    const response = await fetch('https://oauth2.googleapis.com/token', {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      body: new URLSearchParams({
        code,
        client_id: config.GOOGLE_CLIENT_ID,
        client_secret: config.GOOGLE_CLIENT_SECRET,
        redirect_uri: config.GOOGLE_REDIRECT_URI,
        grant_type: 'authorization_code',
      }),
    });

    if (!response.ok) {
      throw new Error(`Google token exchange failed: ${await response.text()}`);
    }

    return response.json();
  }

  /**
   * Lấy user info từ Google.
   */
  async getUserInfo(accessToken: string): Promise<GoogleUser> {
    const response = await fetch('https://www.googleapis.com/oauth2/v3/userinfo', {
      headers: { Authorization: `Bearer ${accessToken}` },
    });

    if (!response.ok) {
      throw new Error(`Google userinfo failed: ${await response.text()}`);
    }

    return response.json();
  }
}
```

### 3.6 Auth Service

**File:** `apps/api/src/modules/auth/auth.service.ts`

```typescript
import bcrypt from 'bcrypt';
import type { AuthRepository, UserRow } from './auth.repository';
import { TokenService } from './strategies/token.service';
import { GoogleStrategy } from './strategies/google.strategy';
import { ConflictError, UnauthorizedError, ForbiddenError } from '../../../common/errors/app-errors';
import type { RegisterInput } from './dto/register.dto';
import type { LoginInput } from './dto/login.dto';

const BCRYPT_ROUNDS = 12;

export class AuthService {
  constructor(
    private repo: AuthRepository,
    private tokenService: TokenService,
    private jwt: JwtStrategy,
    private google: GoogleStrategy,
  ) {}

  // ── Email/Password Register ──

  async register(input: RegisterInput): Promise<{ user: UserRow; accessToken: string; refreshToken: string }> {
    // Check email unique
    const existing = await this.repo.findUserByEmail(input.email);
    if (existing) {
      throw new ConflictError('Email already registered');
    }

    // Create user
    const user = await this.repo.createUser(input.email, input.name);

    // Hash password + create credential
    const hash = await bcrypt.hash(input.password, BCRYPT_ROUNDS);
    await this.repo.createEmailCredential(user.id, hash);

    // Issue tokens
    const tokens = await this.tokenService.issueTokens(user);

    return {
      user: this.sanitizeUser(user),
      accessToken: tokens.accessToken,
      refreshToken: tokens.refreshToken,
    };
  }

  // ── Email/Password Login ──

  async login(input: LoginInput): Promise<{ user: UserRow; accessToken: string; refreshToken: string }> {
    // Find user
    const user = await this.repo.findUserByEmail(input.email);
    if (!user || user.status === 'deleted') {
      throw new UnauthorizedError('Invalid email or password');
    }

    // Check status
    if (user.status === 'disabled') {
      throw new ForbiddenError('Account has been disabled. Contact support.');
    }

    // Find email credential
    const cred = await this.repo.findCredential(user.id, 'email');
    if (!cred?.password_hash) {
      throw new UnauthorizedError('Invalid email or password');
    }

    // Verify password
    const valid = await bcrypt.compare(input.password, cred.password_hash);
    if (!valid) {
      throw new UnauthorizedError('Invalid email or password');
    }

    // Issue tokens
    const tokens = await this.tokenService.issueTokens(user);

    return {
      user: this.sanitizeUser(user),
      accessToken: tokens.accessToken,
      refreshToken: tokens.refreshToken,
    };
  }

  // ── Google OAuth ──

  getGoogleAuthUrl(): string {
    return this.google.getAuthUrl();
  }

  async googleLogin(code: string): Promise<{ user: UserRow; accessToken: string; refreshToken: string; isNew: boolean }> {
    // Exchange code
    const tokens = await this.google.exchangeCode(code);
    const googleUser = await this.google.getUserInfo(tokens.access_token);

    if (!googleUser.email_verified) {
      throw new UnauthorizedError('Google email not verified');
    }

    // Tìm credential theo Google ID
    const existingCred = await this.repo.findCredentialByGoogleId(googleUser.sub);

    let user: UserRow;
    let isNew = false;

    if (existingCred) {
      // User đã từng login bằng Google → tìm user + update avatar
      const u = await this.repo.findUserById(existingCred.user_id);
      if (!u) throw new Error('User not found for existing Google credential');
      user = u;
      if (user.avatar_url !== googleUser.picture) {
        await this.repo.updateAvatar(user.id, googleUser.picture);
        user.avatar_url = googleUser.picture;
      }
    } else {
      // Tìm user theo email (hợp nhất nếu đã có email/pass account)
      const userByEmail = await this.repo.findUserByEmail(googleUser.email);

      if (userByEmail) {
        // Hợp nhất: thêm Google credential vào user hiện có
        await this.repo.createGoogleCredential(userByEmail.id, googleUser.sub, googleUser.email);
        user = userByEmail;
      } else {
        // User hoàn toàn mới
        user = await this.repo.createUser(googleUser.email, googleUser.name, googleUser.picture);
        await this.repo.createGoogleCredential(user.id, googleUser.sub, googleUser.email);
        isNew = true;
      }
    }

    // Check status
    if (user.status === 'disabled') {
      throw new ForbiddenError('Account has been disabled');
    }

    const issuedTokens = await this.tokenService.issueTokens(user);

    return {
      user: this.sanitizeUser(user),
      accessToken: issuedTokens.accessToken,
      refreshToken: issuedTokens.refreshToken,
      isNew,
    };
  }

  // ── Token Refresh ──

  async refreshAccessToken(refreshToken: string): Promise<{ user: UserRow; accessToken: string; refreshToken: string }> {
    const hash = crypto.createHash('sha256').update(refreshToken).digest('hex');
    const stored = await this.repo.findRefreshToken(hash);

    if (!stored) {
      // Token không tồn tại → có thể đã bị xoá do rotation
      throw new UnauthorizedError('Invalid refresh token');
    }

    if (stored.expires_at < new Date()) {
      await this.repo.deleteRefreshToken(hash);
      throw new UnauthorizedError('Refresh token expired');
    }

    // GET USER
    const user = await this.repo.findUserById(stored.user_id);
    if (!user || user.status === 'disabled') {
      await this.repo.deleteRefreshTokenFamily(stored.family);
      throw new UnauthorizedError('User not found or disabled');
    }

    // Rotate: xoá token cũ, cấp mới
    const tokens = await this.tokenService.rotateTokens(user, hash, stored.family);

    return {
      user: this.sanitizeUser(user),
      accessToken: tokens.accessToken,
      refreshToken: tokens.refreshToken,
    };
  }

  // ── Logout ──

  async logout(refreshToken: string): Promise<void> {
    const hash = crypto.createHash('sha256').update(refreshToken).digest('hex');
    await this.repo.deleteRefreshToken(hash);
  }

  async logoutAll(userId: string): Promise<void> {
    await this.tokenService.revokeAll(userId);
  }

  // ── Helper ──

  sanitizeUser(user: UserRow): UserRow {
    return user; // Không expose password_hash
  }
}
```

### 3.7 Auth Controller

**File:** `apps/api/src/modules/auth/auth.controller.ts`

```typescript
import type { FastifyRequest, FastifyReply } from 'fastify';
import type { AuthService } from './auth.service';
import type { JwtStrategy } from './strategies/jwt.strategy';
import { validate } from '../../../common/pipes/validation.pipe';
import { registerSchema } from './dto/register.dto';
import { loginSchema } from './dto/login.dto';

export class AuthController {
  constructor(
    private authService: AuthService,
    private jwt: JwtStrategy,
  ) {}

  register = async (req: FastifyRequest, reply: FastifyReply) => {
    const input = validate(registerSchema, req.body);
    const result = await this.authService.register(input);

    this.jwt.setAccessTokenCookie(reply, result.accessToken);
    this.jwt.setRefreshTokenCookie(reply, result.refreshToken);

    return reply.status(201).send({ user: result.user });
  };

  login = async (req: FastifyRequest, reply: FastifyReply) => {
    const input = validate(loginSchema, req.body);
    const result = await this.authService.login(input);

    this.jwt.setAccessTokenCookie(reply, result.accessToken);
    this.jwt.setRefreshTokenCookie(reply, result.refreshToken);

    return reply.status(200).send({ user: result.user });
  };

  googleRedirect = async (_req: FastifyRequest, reply: FastifyReply) => {
    const url = this.authService.getGoogleAuthUrl();
    return reply.redirect(url);
  };

  googleCallback = async (req: FastifyRequest, reply: FastifyReply) => {
    const { code } = req.query as { code?: string };
    if (!code) {
      return reply.status(400).send({ error: 'Missing authorization code' });
    }

    const result = await this.authService.googleLogin(code);

    this.jwt.setAccessTokenCookie(reply, result.accessToken);
    this.jwt.setRefreshTokenCookie(reply, result.refreshToken);

    // Redirect về frontend
    const frontendUrl = config.NODE_ENV === 'production'
      ? config.FRONTEND_URL
      : 'http://localhost:5173';
    return reply.redirect(`${frontendUrl}/`);
  };

  refresh = async (req: FastifyRequest, reply: FastifyReply) => {
    const token = req.cookies?.refresh_token;
    if (!token) {
      return reply.status(401).send({ error: 'No refresh token' });
    }

    const result = await this.authService.refreshAccessToken(token);

    this.jwt.setAccessTokenCookie(reply, result.accessToken);
    this.jwt.setRefreshTokenCookie(reply, result.refreshToken);

    return reply.status(200).send({ user: result.user });
  };

  logout = async (req: FastifyRequest, reply: FastifyReply) => {
    const token = req.cookies?.refresh_token;
    if (token) {
      await this.authService.logout(token);
    }
    this.jwt.clearTokens(reply);
    return reply.status(200).send({ message: 'Logged out' });
  };

  me = async (req: FastifyRequest, reply: FastifyReply) => {
    // authGuard đã set req.user
    return reply.status(200).send({ user: req.user });
  };
}
```

### 3.8 Auth Module (Fastify plugin)

**File:** `apps/api/src/modules/auth/auth.module.ts`

```typescript
import type { FastifyInstance } from 'fastify';
import type { Pool } from 'pg';
import { AuthRepository } from './auth.repository';
import { JwtStrategy } from './strategies/jwt.strategy';
import { TokenService } from './strategies/token.service';
import { GoogleStrategy } from './strategies/google.strategy';
import { AuthService } from './auth.service';
import { AuthController } from './auth.controller';
import { authGuard } from '../../../common/guards/auth.guard';

export interface AuthModuleOptions {
  pgPool: Pool;
}

export async function authModule(app: FastifyInstance, opts: AuthModuleOptions): Promise<void> {
  // Wire dependencies (manual DI)
  const repo = new AuthRepository(opts.pgPool);
  const jwt = new JwtStrategy();
  const tokenService = new TokenService(repo, jwt);
  const google = new GoogleStrategy();
  const authService = new AuthService(repo, tokenService, jwt, google);
  const controller = new AuthController(authService, jwt);

  // Public routes
  app.post('/api/auth/register', controller.register);
  app.post('/api/auth/login', controller.login);
  app.get('/api/auth/google', controller.googleRedirect);
  app.get('/api/auth/google/callback', controller.googleCallback);
  app.post('/api/auth/refresh', controller.refresh);
  app.post('/api/auth/logout', controller.logout);

  // Protected route
  app.get('/api/auth/me', { preHandler: [authGuard] }, controller.me);
}
```

---

## Phase 4: Users Module (Admin)

### 4.1 Users Repository

**File:** `apps/api/src/modules/users/users.repository.ts`

```typescript
import type { Pool } from 'pg';
import type { UserRow } from '../auth/auth.repository';

export class UsersRepository {
  constructor(private pg: Pool) {}

  async findAll(limit = 50, offset = 0): Promise<UserRow[]> {
    const { rows } = await this.pg.query<UserRow>(
      'SELECT * FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2',
      [limit, offset],
    );
    return rows;
  }

  async findById(id: string): Promise<UserRow | null> {
    const { rows } = await this.pg.query<UserRow>(
      'SELECT * FROM users WHERE id = $1',
      [id],
    );
    return rows[0] ?? null;
  }

  async updateStatus(id: string, status: 'active' | 'disabled'): Promise<UserRow | null> {
    const { rows } = await this.pg.query<UserRow>(
      'UPDATE users SET status = $1 WHERE id = $2 RETURNING *',
      [status, id],
    );
    return rows[0] ?? null;
  }

  async search(query: string, limit = 50): Promise<UserRow[]> {
    const { rows } = await this.pg.query<UserRow>(
      `SELECT * FROM users 
       WHERE email ILIKE $1 OR name ILIKE $1 
       ORDER BY created_at DESC LIMIT $2`,
      [`%${query}%`, limit],
    );
    return rows;
  }
}
```

### 4.2 Users Service

**File:** `apps/api/src/modules/users/users.service.ts`

```typescript
import type { UsersRepository } from './users.repository';
import type { UserRow } from '../auth/auth.repository';
import { NotFoundError, ForbiddenError } from '../../../common/errors/app-errors';

export class UsersService {
  constructor(private repo: UsersRepository) {}

  async listUsers(limit = 50, offset = 0): Promise<UserRow[]> {
    return this.repo.findAll(limit, offset);
  }

  async getUser(id: string): Promise<UserRow> {
    const user = await this.repo.findById(id);
    if (!user) throw new NotFoundError('User not found');
    return user;
  }

  async disableUser(adminId: string, targetId: string): Promise<UserRow> {
    // Không cho admin tự disable chính mình
    if (adminId === targetId) {
      throw new ForbiddenError('Cannot disable your own account');
    }
    const updated = await this.repo.updateStatus(targetId, 'disabled');
    if (!updated) throw new NotFoundError('User not found');
    return updated;
  }

  async enableUser(targetId: string): Promise<UserRow> {
    const updated = await this.repo.updateStatus(targetId, 'active');
    if (!updated) throw new NotFoundError('User not found');
    return updated;
  }
}
```

### 4.3 Users Controller

**File:** `apps/api/src/modules/users/users.controller.ts`

```typescript
import type { FastifyRequest, FastifyReply } from 'fastify';
import type { UsersService } from './users.service';

export class UsersController {
  constructor(private service: UsersService) {}

  list = async (req: FastifyRequest, reply: FastifyReply) => {
    const { limit = '50', offset = '0' } = req.query as Record<string, string>;
    const users = await this.service.listUsers(Number(limit), Number(offset));
    return reply.status(200).send({ users });
  };

  get = async (req: FastifyRequest, reply: FastifyReply) => {
    const { id } = req.params as { id: string };
    const user = await this.service.getUser(id);
    return reply.status(200).send({ user });
  };

  disable = async (req: FastifyRequest, reply: FastifyReply) => {
    const { id } = req.params as { id: string };
    const user = await this.service.disableUser(req.user.sub, id);
    return reply.status(200).send({ user });
  };

  enable = async (req: FastifyRequest, reply: FastifyReply) => {
    const { id } = req.params as { id: string };
    const user = await this.service.enableUser(id);
    return reply.status(200).send({ user });
  };
}
```

### 4.4 Users Module

**File:** `apps/api/src/modules/users/users.module.ts`

```typescript
import type { FastifyInstance } from 'fastify';
import type { Pool } from 'pg';
import { UsersRepository } from './users.repository';
import { UsersService } from './users.service';
import { UsersController } from './users.controller';
import { authGuard } from '../../../common/guards/auth.guard';
import { adminGuard } from '../../../common/guards/admin.guard';

export interface UsersModuleOptions {
  pgPool: Pool;
}

export async function usersModule(app: FastifyInstance, opts: UsersModuleOptions): Promise<void> {
  const repo = new UsersRepository(opts.pgPool);
  const service = new UsersService(repo);
  const controller = new UsersController(service);

  // All admin routes: auth + admin role required
  app.get('/api/admin/users', { preHandler: [authGuard, adminGuard] }, controller.list);
  app.get('/api/admin/users/:id', { preHandler: [authGuard, adminGuard] }, controller.get);
  app.patch('/api/admin/users/:id/disable', { preHandler: [authGuard, adminGuard] }, controller.disable);
  app.patch('/api/admin/users/:id/enable', { preHandler: [authGuard, adminGuard] }, controller.enable);
}
```

---

## Phase 5: BFF Refactor — Các module hiện có

### 5.1 Refactor chat module

Mỗi file trong `modules/chat/` cần thay đổi:

**Repository:** thêm `tenantFilter(req)` vào mọi query:

```typescript
// Trước:
db.collection('conversations').find({}).sort({ updatedAt: -1 })

// Sau:
db.collection('conversations').find(tenantFilter(req)).sort({ updatedAt: -1 })

// Trước:
db.collection('messages').find({ conversationId: id })

// Sau:
db.collection('messages').find(tenantFilter(req, { conversationId: id }))
```

**Routes:** bọc trong `{ preHandler: [authGuard] }`:

```typescript
export async function chatModule(app: FastifyInstance) {
  // Tất cả route chat đều cần auth
  app.addHook('preHandler', authGuard);

  app.post('/api/chat', controller.chat);
  app.get('/api/conversations', controller.listConversations);
  app.get('/api/conversations/:id', controller.getConversation);
  app.delete('/api/conversations/:id', controller.deleteConversation);
}
```

### 5.2 Refactor documents module

Tương tự chat:
- Thêm `tenantFilter(req)` vào mọi MongoDB query
- Thêm `authGuard` vào tất cả route

### 5.3 Refactor tasks module

Tương tự:
- Thêm `tenantFilter(req)` vào mọi MongoDB query
- Thêm `authGuard` vào tất cả route

### 5.4 Cập nhật app.ts

**File:** `apps/api/src/app.ts`

```typescript
import Fastify from 'fastify';
import cors from '@fastify/cors';
import multipart from '@fastify/multipart';
import rateLimit from '@fastify/rate-limit';
import cookie from '@fastify/cookie';
import { config } from './config';
import { initPostgres } from './database/postgres/postgres.module';
import { connectMongo, ensureIndexes } from './database/mongo/mongo.module';
import { registerErrorFilter } from './common/filters/error.filter';
import { authModule } from './modules/auth/auth.module';
import { usersModule } from './modules/users/users.module';
import { chatModule } from './modules/chat/chat.module';
import { documentsModule } from './modules/documents/documents.module';
import { tasksModule } from './modules/tasks/tasks.module';

export async function buildApp() {
  const app = Fastify({
    logger: config.NODE_ENV !== 'test',
    bodyLimit: 50 * 1024 * 1024,
  });

  // ── Plugins ──
  app.register(cors, { origin: config.CORS_ORIGIN?.split(',') ?? true });
  app.register(multipart, { limits: { fileSize: 25 * 1024 * 1024, files: 7 } });
  app.register(rateLimit, { max: 120, timeWindow: '1 minute' });
  app.register(cookie, { secret: config.JWT_SECRET });

  // ── Error filter ──
  registerErrorFilter(app);

  // ── Database ──
  const pgPool = await initPostgres({ connectionString: config.PG_CONNECTION_STRING });
  const mongoDb = await connectMongo();
  await ensureIndexes();

  // ── Health endpoints (no auth) ──
  app.get('/api/health', async () => ({ status: 'ok' }));
  app.get('/api/healthz', async (req, reply) => {
    // ... giữ nguyên logic health check
  });
  app.get('/api/ready', async (req, reply) => {
    // ... giữ nguyên logic readiness
  });

  // ── Feature modules ──
  app.register(authModule, { pgPool });
  app.register(usersModule, { pgPool });
  app.register(chatModule, { mongoDb });
  app.register(documentsModule, { mongoDb });
  app.register(tasksModule, { mongoDb });

  return app;
}
```

---

## Phase 6: Go Agent Changes

### 6.1 Cập nhật context

**File:** `services/agent-go/internal/agent/context.go`

```go
package agent

import "context"

type contextKey string

const (
    tenantIDKey contextKey = "tenant_id"
)

// NewContext tạo context với tenantID từ HTTP request.
func NewContext(ctx context.Context, tenantID string) context.Context {
    if tenantID == "" {
        return ctx
    }
    return context.WithValue(ctx, tenantIDKey, tenantID)
}

// TenantIDFromContext lấy tenantID từ context (dùng trong tools: RAG, memory, tasks).
func TenantIDFromContext(ctx context.Context) string {
    if v, ok := ctx.Value(tenantIDKey).(string); ok {
        return v
    }
    return ""
}
```

### 6.2 Cập nhật chat handler

**File:** `services/agent-go/internal/transport/http/chat.go`

```go
func NewChatHandler(orch *orchestrator.Orchestrator) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        tenantID := r.Header.Get("X-Tenant-ID")
        
        // Parse body
        var req ChatRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }
        
        // Nếu tenantID không có trong header, thử lấy từ body
        if tenantID == "" {
            tenantID = req.TenantID
        }
        
        // Tạo context với tenant
        ctx := agent.NewContext(r.Context(), tenantID)
        
        // ... phần còn lại của handler
    }
}
```

### 6.3 Cập nhật RAG tool — filter theo tenant

**File:** `services/agent-go/internal/tools/rag.go` — thêm tenant filter:

```go
func (t *RAGSearchTool) searchDocuments(ctx context.Context, query string) ([]DocChunk, error) {
    tenantID := agent.TenantIDFromContext(ctx)
    
    filter := bson.M{}
    if tenantID != "" {
        filter["tenantId"] = tenantID
    }
    
    // Thêm điều kiện search vào filter
    // ... vector search với filter
}
```

### 6.4 Cập nhật Memory tool — filter theo tenant

**File:** `services/agent-go/internal/tools/memory_tools.go`:

```go
func (t *SaveMemoryTool) Execute(ctx context.Context, args map[string]any) (string, error) {
    tenantID := agent.TenantIDFromContext(ctx)
    // Lưu memory với tenantID
}

func (t *RecallMemoryTool) Execute(ctx context.Context, args map[string]any) (string, error) {
    tenantID := agent.TenantIDFromContext(ctx)
    // Query memories WHERE tenantId = tenantID
}
```

---

## Phase 7: Frontend

### 7.1 Cài đặt dependencies

```bash
cd apps/web
pnpm add zustand react-router-dom
```

### 7.2 User type

**File:** `apps/web/src/shared/types/user.ts`

```typescript
export interface User {
  id: string;
  email: string;
  name: string;
  avatarUrl: string | null;
  role: 'user' | 'admin';
  status: 'active' | 'disabled';
}
```

### 7.3 Auth API client

**File:** `apps/web/src/modules/auth/auth.api.ts`

```typescript
import { http } from '@/shared/api/http';
import type { User } from '@/shared/types/user';

export const authApi = {
  register: (data: { email: string; password: string; name: string }) =>
    http.post<{ user: User }>('/api/auth/register', data),

  login: (data: { email: string; password: string }) =>
    http.post<{ user: User }>('/api/auth/login', data),

  logout: () =>
    http.post('/api/auth/logout'),

  refresh: () =>
    http.post<{ user: User }>('/api/auth/refresh'),

  me: () =>
    http.get<{ user: User }>('/api/auth/me'),

  googleLogin: () => {
    window.location.href = '/api/auth/google';
  },
};
```

### 7.4 Auth store (Zustand)

**File:** `apps/web/src/shared/stores/auth.store.ts`

```typescript
import { create } from 'zustand';
import { authApi } from '@/modules/auth/auth.api';
import type { User } from '@/shared/types/user';

interface AuthState {
  user: User | null;
  isLoading: boolean;
  isError: boolean;

  login: (email: string, password: string) => Promise<void>;
  register: (email: string, password: string, name: string) => Promise<void>;
  loginWithGoogle: () => void;
  logout: () => Promise<void>;
  fetchMe: () => Promise<void>;
  refreshToken: () => Promise<boolean>;
  reset: () => void;
}

export const useAuthStore = create<AuthState>((set, get) => ({
  user: null,
  isLoading: true,  // start = loading để check auth state
  isError: false,

  login: async (email, password) => {
    set({ isLoading: true, isError: false });
    try {
      const { user } = await authApi.login({ email, password });
      set({ user, isLoading: false });
    } catch {
      set({ isError: true, isLoading: false });
      throw new Error('Login failed');
    }
  },

  register: async (email, password, name) => {
    set({ isLoading: true, isError: false });
    try {
      const { user } = await authApi.register({ email, password, name });
      set({ user, isLoading: false });
    } catch {
      set({ isError: true, isLoading: false });
      throw new Error('Registration failed');
    }
  },

  loginWithGoogle: () => {
    authApi.googleLogin();
  },

  logout: async () => {
    try {
      await authApi.logout();
    } finally {
      set({ user: null, isLoading: false, isError: false });
    }
  },

  fetchMe: async () => {
    try {
      const { user } = await authApi.me();
      set({ user, isLoading: false });
    } catch {
      set({ user: null, isLoading: false, isError: true });
    }
  },

  refreshToken: async () => {
    try {
      const { user } = await authApi.refresh();
      set({ user });
      return true;
    } catch {
      set({ user: null, isError: true });
      return false;
    }
  },

  reset: () => set({ user: null, isLoading: false, isError: false }),
}));
```

### 7.5 Auth Guard

**File:** `apps/web/src/shared/guards/AuthGuard.tsx`

```tsx
import { useEffect } from 'react';
import { Navigate, Outlet } from 'react-router-dom';
import { useAuthStore } from '@/shared/stores/auth.store';
import { FullPageSpinner } from '@/shared/components/FullPageSpinner';

export function AuthGuard() {
  const { user, isLoading, fetchMe } = useAuthStore();

  useEffect(() => {
    if (!user) fetchMe();
  }, []);

  if (isLoading) return <FullPageSpinner />;
  if (!user) return <Navigate to="/login" replace />;

  return <Outlet />;
}
```

### 7.6 Admin Guard

**File:** `apps/web/src/shared/guards/AdminGuard.tsx`

```tsx
import { Navigate, Outlet } from 'react-router-dom';
import { useAuthStore } from '@/shared/stores/auth.store';

export function AdminGuard() {
  const user = useAuthStore((s) => s.user);

  if (!user || user.role !== 'admin') {
    return <Navigate to="/" replace />;
  }

  return <Outlet />;
}
```

### 7.7 HTTP interceptor (auto refresh)

**File:** `apps/web/src/shared/api/http.ts` — cập nhật:

```typescript
import { useAuthStore } from '@/shared/stores/auth.store';

const http = axios.create({
  baseURL: import.meta.env.VITE_API_URL || '',
  withCredentials: true, // gửi cookie
});

// Response interceptor: auto refresh token
http.interceptors.response.use(
  (res) => res,
  async (error) => {
    const original = error.config;
    if (error.response?.status === 401 && !original._retry) {
      original._retry = true;
      const ok = await useAuthStore.getState().refreshToken();
      if (ok) return http(original);
      // Refresh failed → redirect login
      useAuthStore.getState().reset();
      window.location.href = '/login';
    }
    return Promise.reject(error);
  }
);

export { http };
```

### 7.8 Login Page

**File:** `apps/web/src/modules/auth/components/LoginPage.tsx`

```tsx
import { useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useAuthStore } from '@/shared/stores/auth.store';

export default function LoginPage() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const { login, loginWithGoogle, isLoading } = useAuthStore();
  const navigate = useNavigate();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    try {
      await login(email, password);
      navigate('/', { replace: true });
    } catch {
      setError('Invalid email or password');
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-background">
      <div className="w-full max-w-md p-8 space-y-6 bg-card rounded-xl shadow-lg">
        <div className="text-center">
          <h1 className="text-2xl font-bold">Welcome back</h1>
          <p className="text-muted-foreground">Sign in to JARVIS</p>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          {error && <div className="text-red-500 text-sm">{error}</div>}

          <input
            type="email"
            placeholder="Email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
            className="w-full px-4 py-2 border rounded-lg bg-background"
          />
          <input
            type="password"
            placeholder="Password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            className="w-full px-4 py-2 border rounded-lg bg-background"
          />
          <button
            type="submit"
            disabled={isLoading}
            className="w-full py-2 bg-primary text-primary-foreground rounded-lg disabled:opacity-50"
          >
            {isLoading ? 'Signing in...' : 'Sign in'}
          </button>
        </form>

        <div className="relative">
          <div className="absolute inset-0 flex items-center">
            <div className="w-full border-t" />
          </div>
          <div className="relative flex justify-center text-sm">
            <span className="px-2 bg-card text-muted-foreground">or</span>
          </div>
        </div>

        <button
          onClick={loginWithGoogle}
          className="w-full py-2 border rounded-lg flex items-center justify-center gap-2 hover:bg-muted"
        >
          <GoogleIcon /> Continue with Google
        </button>

        <p className="text-center text-sm text-muted-foreground">
          Don't have an account? <Link to="/register" className="text-primary">Sign up</Link>
        </p>
      </div>
    </div>
  );
}
```

### 7.9 Register Page

Tương tự LoginPage, thêm field `name`, gọi `register()` từ store.

### 7.10 Admin Users Page

**File:** `apps/web/src/modules/admin/components/UsersManagement.tsx`

```tsx
// Table hiển thị danh sách users với các action: Disable, Enable
// Gọi GET /api/admin/users, PATCH /api/admin/users/:id/disable, /enable
```

### 7.11 Cập nhật App.tsx

```tsx
import { Routes, Route } from 'react-router-dom';
import { lazy } from 'react';
import { AuthGuard } from '@/shared/guards/AuthGuard';
import { AdminGuard } from '@/shared/guards/AdminGuard';

const LoginPage = lazy(() => import('@/modules/auth/components/LoginPage'));
const RegisterPage = lazy(() => import('@/modules/auth/components/RegisterPage'));
const ChatPage = lazy(() => import('@/modules/chat/components/ChatPage'));
const DocumentsView = lazy(() => import('@/modules/documents/components/DocumentsView'));
const UsersManagement = lazy(() => import('@/modules/admin/components/UsersManagement'));
const AppLayout = lazy(() => import('@/shared/components/AppLayout'));

export default function App() {
  return (
    <Routes>
      {/* Public */}
      <Route path="/login" element={<LoginPage />} />
      <Route path="/register" element={<RegisterPage />} />

      {/* Protected */}
      <Route element={<AuthGuard />}>
        <Route element={<AppLayout />}>
          <Route path="/" element={<ChatPage />} />
          <Route path="/messages/:id" element={<ChatPage />} />
          <Route path="/documents" element={<DocumentsView />} />

          {/* Admin only */}
          <Route element={<AdminGuard />}>
            <Route path="/admin/users" element={<UsersManagement />} />
          </Route>
        </Route>
      </Route>
    </Routes>
  );
}
```

---

## Phase 8: Docker + Env

### 8.1 Env vars cho BFF (`apps/api/.env`)

```bash
# --- Hiện có ---
PORT=3001
MONGODB_URI=mongodb+srv://...
ANTHROPIC_API_KEY=sk-ant-...
LLM_PROVIDER=anthropic
VOYAGE_API_KEY=...
AGENT_BACKEND=go
AGENT_GO_URL=http://localhost:3002
CORS_ORIGIN=http://localhost:5173

# --- MỚI: PostgreSQL ---
PG_CONNECTION_STRING=postgresql://user:pass@vps-host:5432/jarvis_auth

# --- MỚI: JWT ---
JWT_SECRET=your-secret-at-least-32-characters-long-here
JWT_REFRESH_SECRET=another-secret-at-least-32-characters-long

# --- MỚI: Google OAuth ---
GOOGLE_CLIENT_ID=your-google-client-id.apps.googleusercontent.com
GOOGLE_CLIENT_SECRET=GOCSPX-your-google-client-secret
GOOGLE_REDIRECT_URI=http://localhost:3001/api/auth/google/callback

# --- MỚI: Frontend URL ---
FRONTEND_URL=http://localhost:5173
```

### 8.2 Docker compose updates

```yaml
# docker/docker-compose.yml — thêm env cho api service
api:
  # ... giữ nguyên
  environment:
    # ... giữ nguyên
    - PG_CONNECTION_STRING=${PG_CONNECTION_STRING}
    - JWT_SECRET=${JWT_SECRET}
    - JWT_REFRESH_SECRET=${JWT_REFRESH_SECRET}
    - GOOGLE_CLIENT_ID=${GOOGLE_CLIENT_ID}
    - GOOGLE_CLIENT_SECRET=${GOOGLE_CLIENT_SECRET}
    - GOOGLE_REDIRECT_URI=${GOOGLE_REDIRECT_URI}
    - FRONTEND_URL=${FRONTEND_URL}
```

---

## Checklist triển khai

- [ ] Phase 1: PostgreSQL setup (migration + connection pool)
- [ ] Phase 2: Common layer (guards, filters, pipes, interfaces)
- [ ] Phase 3: Auth module (register, login, OAuth, token refresh, logout)
- [ ] Phase 4: Users module (admin: list, disable, enable)
- [ ] Phase 5: Refactor chat/documents/tasks modules (tenant filter + auth guard)
- [ ] Phase 6: Go agent (X-Tenant-ID, context, filter RAG/memory/tasks)
- [ ] Phase 7: Frontend (auth pages, guards, store, HTTP interceptor, admin page)
- [ ] Phase 8: Docker + env vars + Google OAuth console setup
- [ ] Testing: curl flow register → login → chat → verify tenant isolation
- [ ] Migration: seed admin user, update existing data với tenantId mặc định
