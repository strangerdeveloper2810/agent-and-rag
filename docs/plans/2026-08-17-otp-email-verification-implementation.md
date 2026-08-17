# OTP Email Verification + Tenant Isolation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Thêm bước xác minh email bằng OTP vào flow register/login của `apps/api`, đồng thời hoàn thiện tenant isolation còn dang dở cho module `documents` và propagate `X-Tenant-ID` sang `agent-go`.

**Architecture:** Fastify (apps/api) + PostgreSQL (auth) + MongoDB (RAG documents) + Redis (OTP state, đã có sẵn `ioredis`) + Resend (gửi email) + React/Zustand (apps/web). Tenant model: 1 user = 1 tenant, `tenantId = user.id` (không có bảng tenants riêng). Xem thiết kế đầy đủ tại `docs/plans/2026-08-17-otp-email-verification-design.md`.

**Tech Stack:** TypeScript, Fastify, pg, ioredis, zod, bcrypt, vitest, Resend SDK, React + Zustand + React Hook Form.

---

## ⚠️ Lưu ý quan trọng trước khi bắt đầu

1. **2 tầng Mongo song song trong `apps/api`:** `apps/api/src/database/mongo/*` là code CHẾT (không ai import ngoài chính nó) — có sẵn `tenantId` khớp thiết kế `2026-07-25` nhưng KHÔNG được wire vào `app.ts`. Tầng THẬT đang chạy là `apps/api/src/lib/mongo.ts` + `apps/api/src/lib/collections.ts` (không có `tenantId`). **Mọi task Mongo trong plan này sửa tầng `lib/`, không đụng vào `database/mongo/`.**
2. `chat.repository.ts` và `tasks.repository.ts` cũng chưa tenant-isolated (cùng bệnh với `documents`) nhưng **ngoài phạm vi plan này** — user chỉ yêu cầu fix `documents`. Không tự ý mở rộng.
3. Trước mỗi task sửa file đã tồn tại, **đọc lại file thật trước khi sửa** — một số path dưới đây lấy từ khảo sát gián tiếp, có thể lệch 1-2 cấp thư mục con.
4. Test framework là **vitest** (`apps/api/package.json`: `"test": "vitest run"`), file test đặt cạnh source (`*.test.ts`), test hàm pure là chính — không mock Mongo/Postgres phức tạp.

---

### Task 1: Migration — thêm cột `email_verified`

**Files:**
- Create: `apps/api/src/database/postgres/migrations/002-add-email-verification.sql`

**Step 1: Viết migration**

```sql
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified BOOLEAN NOT NULL DEFAULT false;
```

**Step 2: Chạy thử migration (postgres module tự chạy mọi file `.sql` trong thư mục theo thứ tự tên khi `initPostgres()` được gọi)**

Run: `pnpm --filter @app/api dev` (hoặc khởi động lại server dev đang chạy) rồi kiểm tra:
Run: `psql "$PG_CONNECTION_STRING" -c "\d users"`
Expected: cột `email_verified` kiểu `boolean`, default `false`, xuất hiện trong output.

**Step 3: Commit**

```bash
git add apps/api/src/database/postgres/migrations/002-add-email-verification.sql
git commit -m "feat(auth): add email_verified column migration"
```

---

### Task 2: Thêm `EmailNotVerifiedError` vào `apps/api/src/lib/errors.ts`

**Files:**
- Modify: `apps/api/src/lib/errors.ts` (đã có `RateLimitError` khoảng dòng 33-41 với `retryAfterSeconds` — đọc class đó trước để copy đúng style kế thừa `AppError`)

**Step 1: Đọc file thật**

Run: mở `apps/api/src/lib/errors.ts`, xác nhận tên base class (`AppError` hay tên khác) và constructor signature `(message, statusCode, code)`.

**Step 2: Thêm class mới (theo đúng style base class tìm thấy ở Step 1)**

```typescript
export class EmailNotVerifiedError extends AppError {
  constructor(public readonly email: string) {
    super('Email chưa được xác minh', 403, 'EMAIL_NOT_VERIFIED');
  }
}
```

**Step 3: Commit**

```bash
git add apps/api/src/lib/errors.ts
git commit -m "feat(auth): add EmailNotVerifiedError"
```

---

### Task 3: OTP service (Redis) — có unit test

**Files:**
- Create: `apps/api/src/modules/auth/otp.service.ts`
- Create: `apps/api/src/modules/auth/otp.service.test.ts`

**Step 1: Viết implementation**

```typescript
// apps/api/src/modules/auth/otp.service.ts
import crypto from 'crypto';
import { cacheSet, cacheGet, cacheDel, cacheKey } from '../../database/redis/redis.module';
// ⚠️ Xác nhận đúng path import redis.module theo file thật (khảo sát: apps/api/src/database/redis/redis.module.ts)

const OTP_TTL_SECONDS = 10 * 60; // 10 phút
const COOLDOWN_SECONDS = 2 * 60; // 2 phút
const MAX_ATTEMPTS = 5;

interface OtpRecord {
  codeHash: string;
  attempts: number;
}

interface CooldownRecord {
  availableAt: number; // epoch ms
}

export type VerifyResult = 'ok' | 'invalid' | 'expired' | 'locked';

function otpKey(email: string): string {
  return cacheKey('otp', email.toLowerCase());
}

function cooldownKey(email: string): string {
  return cacheKey('otp-cooldown', email.toLowerCase());
}

export function generateOtp(): string {
  return crypto.randomInt(100000, 1000000).toString();
}

export function hashOtp(otp: string): string {
  return crypto.createHash('sha256').update(otp).digest('hex');
}

export class OtpService {
  /** Sinh OTP mới, lưu Redis, set cooldown. Trả về mã OTP (plaintext) để gửi email. */
  async issue(email: string): Promise<string> {
    const otp = generateOtp();
    const record: OtpRecord = { codeHash: hashOtp(otp), attempts: 0 };
    await cacheSet(otpKey(email), record, OTP_TTL_SECONDS);
    await cacheSet(
      cooldownKey(email),
      { availableAt: Date.now() + COOLDOWN_SECONDS * 1000 } satisfies CooldownRecord,
      COOLDOWN_SECONDS + 10, // TTL dư 10s so với cooldown thật, tránh lệch giờ
    );
    return otp;
  }

  /** Số giây còn lại phải chờ trước khi được phép resend. 0 = có thể resend ngay. */
  async cooldownRemaining(email: string): Promise<number> {
    const record = await cacheGet<CooldownRecord>(cooldownKey(email));
    if (!record) return 0;
    const remainingMs = record.availableAt - Date.now();
    return remainingMs > 0 ? Math.ceil(remainingMs / 1000) : 0;
  }

  /** Kiểm tra OTP người dùng nhập. Tăng attempts nếu sai; xoá record nếu đúng hoặc chạm max attempts. */
  async verify(email: string, otp: string): Promise<VerifyResult> {
    const record = await cacheGet<OtpRecord>(otpKey(email));
    if (!record) return 'expired';
    if (record.attempts >= MAX_ATTEMPTS) {
      await cacheDel(otpKey(email));
      return 'locked';
    }

    if (record.codeHash !== hashOtp(otp)) {
      const attempts = record.attempts + 1;
      if (attempts >= MAX_ATTEMPTS) {
        await cacheDel(otpKey(email));
        return 'locked';
      }
      await cacheSet(otpKey(email), { ...record, attempts }, OTP_TTL_SECONDS);
      return 'invalid';
    }

    await cacheDel(otpKey(email));
    await cacheDel(cooldownKey(email));
    return 'ok';
  }
}
```

**Step 2: Viết test cho phần thuần (generateOtp/hashOtp) — không cần mock Redis**

```typescript
// apps/api/src/modules/auth/otp.service.test.ts
import { describe, it, expect } from 'vitest';
import { generateOtp, hashOtp } from './otp.service';

describe('generateOtp', () => {
  it('sinh mã 6 chữ số', () => {
    for (let i = 0; i < 20; i++) {
      const otp = generateOtp();
      expect(otp).toMatch(/^\d{6}$/);
    }
  });
});

describe('hashOtp', () => {
  it('cùng input cho cùng hash, khác input cho hash khác', () => {
    expect(hashOtp('123456')).toBe(hashOtp('123456'));
    expect(hashOtp('123456')).not.toBe(hashOtp('654321'));
  });

  it('không trả về plaintext OTP trong hash', () => {
    expect(hashOtp('123456')).not.toContain('123456');
  });
});
```

**Step 3: Chạy test**

Run: `pnpm --filter @app/api exec vitest run src/modules/auth/otp.service.test.ts`
Expected: PASS (2 describe blocks, 3 test).

**Step 4: Commit**

```bash
git add apps/api/src/modules/auth/otp.service.ts apps/api/src/modules/auth/otp.service.test.ts
git commit -m "feat(auth): add OTP service backed by Redis"
```

---

### Task 4: Email service (Resend)

**Files:**
- Create: `apps/api/src/common/email/resend.client.ts`
- Create: `apps/api/src/common/email/email.service.ts`
- Modify: `apps/api/package.json` (thêm dependency `resend`)
- Modify: file config env (xác nhận path thật — khảo sát trước đó gọi là `apps/api/src/config.ts`, kiểm tra lại)

**Step 1: Cài package**

Run: `pnpm add resend --filter @app/api`
Expected: `resend` xuất hiện trong `apps/api/package.json` dependencies.

**Step 2: Thêm env config**

Thêm vào file config (zod schema env, xác nhận đúng path/pattern hiện có):
```typescript
RESEND_API_KEY: z.string().min(1, 'RESEND_API_KEY is required'),
EMAIL_FROM: z.string().min(1).default('JARVIS <onboarding@resend.dev>'),
```
Thêm vào `.env.example` (nếu có) và `.env` local:
```
RESEND_API_KEY=re_xxx
EMAIL_FROM=JARVIS <onboarding@resend.dev>
```

**Step 3: Resend client**

```typescript
// apps/api/src/common/email/resend.client.ts
import { Resend } from 'resend';
import { config } from '../../config'; // xác nhận đúng path import config thật

let client: Resend | null = null;

export function getResendClient(): Resend {
  if (!client) {
    client = new Resend(config.RESEND_API_KEY);
  }
  return client;
}
```

**Step 4: Email service + template OTP**

```typescript
// apps/api/src/common/email/email.service.ts
import { getResendClient } from './resend.client';
import { config } from '../../config';

export async function sendOtpEmail(to: string, name: string, otp: string): Promise<void> {
  const resend = getResendClient();
  try {
    await resend.emails.send({
      from: config.EMAIL_FROM,
      to,
      subject: 'Mã xác minh JARVIS của bạn',
      html: `
        <div style="font-family: 'DM Sans', sans-serif; background:#0b0b0f; color:#f5f0e6; padding:32px; border-radius:12px;">
          <h1 style="color:#d4a94a; font-size:20px;">Xin chào ${name},</h1>
          <p>Mã xác minh email của bạn là:</p>
          <p style="font-family:'JetBrains Mono', monospace; font-size:32px; letter-spacing:8px; color:#d4a94a;">${otp}</p>
          <p style="color:#9a9a9a; font-size:13px;">Mã có hiệu lực trong 10 phút. Nếu bạn không yêu cầu mã này, hãy bỏ qua email.</p>
        </div>
      `,
    });
  } catch (err) {
    // KHÔNG throw — register vẫn phải thành công dù gửi mail lỗi, user có thể bấm "gửi lại".
    console.error('sendOtpEmail failed', err);
  }
}
```

**Step 5: Test thủ công (không cần vitest — gọi thật qua API key sandbox)**

Run: `curl -X POST https://api.resend.com/emails -H "Authorization: Bearer $RESEND_API_KEY" -H "Content-Type: application/json" -d '{"from":"onboarding@resend.dev","to":"<email đã thêm vào Resend test list>","subject":"test","html":"<p>test</p>"}'`
Expected: `200` với `{"id": "..."}`.

**Step 6: Commit**

```bash
git add apps/api/src/common/email apps/api/package.json apps/api/pnpm-lock.yaml
git commit -m "feat(auth): add Resend email service for OTP delivery"
```

---

### Task 5: `auth.repository.ts` — thêm field/method cho email_verified

**Files:**
- Modify: `apps/api/src/modules/auth/auth.repository.ts`

**Step 1: Đọc file thật, xác nhận interface `UserRow` và các hàm hiện có (`findUserByEmail`, `createUser`, `createEmailCredential`, ...)**

**Step 2: Thêm field vào `UserRow`**

```typescript
export interface UserRow {
  // ...các field hiện có...
  email_verified: boolean;
}
```

**Step 3: Thêm method mới**

```typescript
async updateEmailVerified(userId: string): Promise<void> {
  await this.pg.query('UPDATE users SET email_verified = true WHERE id = $1', [userId]);
}

async updateUserForReregister(userId: string, name: string): Promise<UserRow> {
  const { rows } = await this.pg.query<UserRow>(
    'UPDATE users SET name = $1 WHERE id = $2 RETURNING *',
    [name, userId],
  );
  return rows[0];
}

async updateEmailCredential(userId: string, passwordHash: string): Promise<void> {
  await this.pg.query(
    "UPDATE credentials SET password_hash = $1 WHERE user_id = $2 AND method = 'email'",
    [passwordHash, userId],
  );
}
```

**Step 4: Commit**

```bash
git add apps/api/src/modules/auth/auth.repository.ts
git commit -m "feat(auth): add email_verified repository methods"
```

---

### Task 6: `auth.service.ts` — sửa `register()`

**Files:**
- Modify: `apps/api/src/modules/auth/auth.service.ts` (hàm `register`, khảo sát ở dòng ~25-44)
- Modify: `apps/api/src/modules/auth/auth.module.ts` (wire `OtpService` vào DI của `AuthService`)

**Step 1: Đọc lại `register()` hiện tại để biết chính xác constructor `AuthService` (đang nhận `repo, tokenService, jwt, google` theo thiết kế cũ — xác nhận đúng thứ tự tham số thật)**

**Step 2: Sửa constructor + import**

```typescript
import { OtpService } from './otp.service';
import { sendOtpEmail } from '../../common/email/email.service';

export class AuthService {
  constructor(
    private repo: AuthRepository,
    private tokenService: TokenService,
    private jwt: JwtStrategy,
    private google: GoogleStrategy,
    private otp: OtpService, // MỚI
  ) {}
```

**Step 3: Viết lại `register()`**

```typescript
async register(input: RegisterInput): Promise<{ email: string }> {
  const existing = await this.repo.findUserByEmail(input.email);
  const hash = await bcrypt.hash(input.password, BCRYPT_ROUNDS);

  let user: UserRow;
  if (existing) {
    if (existing.email_verified) {
      throw new ConflictError('Email already registered');
    }
    user = await this.repo.updateUserForReregister(existing.id, input.name);
    await this.repo.updateEmailCredential(existing.id, hash);
  } else {
    user = await this.repo.createUser(input.email, input.name);
    await this.repo.createEmailCredential(user.id, hash);
  }

  const otpCode = await this.otp.issue(user.email);
  await sendOtpEmail(user.email, user.name, otpCode);

  return { email: user.email };
}
```

**Step 4: Sửa `auth.module.ts` — khởi tạo `OtpService` và truyền vào `AuthService`**

```typescript
const otp = new OtpService();
const authService = new AuthService(repo, tokenService, jwt, google, otp);
```

**Step 5: Sửa `auth.controller.ts` — `register` không còn set cookie**

```typescript
register = async (req: FastifyRequest, reply: FastifyReply) => {
  const input = validate(registerSchema, req.body);
  const result = await this.authService.register(input);
  return reply.status(201).send({ email: result.email });
};
```

**Step 6: Test thủ công**

Run: `curl -i -X POST http://localhost:3001/api/auth/register -H "Content-Type: application/json" -d '{"email":"test1@example.com","password":"password123","name":"Test User"}'`
Expected: `201`, body `{"email":"test1@example.com"}`, KHÔNG có `Set-Cookie` header, log server hiện `sendOtpEmail` gọi thành công (hoặc log lỗi Resend nếu domain chưa cấu hình — không chặn response).

**Step 7: Commit**

```bash
git add apps/api/src/modules/auth/auth.service.ts apps/api/src/modules/auth/auth.controller.ts apps/api/src/modules/auth/auth.module.ts
git commit -m "feat(auth): register sends OTP instead of issuing tokens immediately"
```

---

### Task 7: `auth.service.ts` — thêm `verifyEmail()` + `resendOtp()`, DTO mới

**Files:**
- Create: `apps/api/src/modules/auth/dto/verify-email.dto.ts`
- Create: `apps/api/src/modules/auth/dto/resend-otp.dto.ts`
- Modify: `apps/api/src/modules/auth/auth.service.ts`

**Step 1: DTOs**

```typescript
// dto/verify-email.dto.ts
import { z } from 'zod';
export const verifyEmailSchema = z.object({
  email: z.string().email('Invalid email address'),
  otp: z.string().length(6, 'OTP must be 6 digits'),
});
export type VerifyEmailInput = z.infer<typeof verifyEmailSchema>;
```

```typescript
// dto/resend-otp.dto.ts
import { z } from 'zod';
export const resendOtpSchema = z.object({
  email: z.string().email('Invalid email address'),
});
export type ResendOtpInput = z.infer<typeof resendOtpSchema>;
```

**Step 2: `verifyEmail()` trong `auth.service.ts`**

```typescript
async verifyEmail(input: VerifyEmailInput): Promise<{ user: UserRow; accessToken: string; refreshToken: string }> {
  const user = await this.repo.findUserByEmail(input.email);
  if (!user) throw new UnauthorizedError('Invalid email or OTP');

  const result = await this.otp.verify(input.email, input.otp);
  if (result === 'expired') throw new ValidationError({ otp: ['OTP đã hết hạn, vui lòng gửi lại'] });
  if (result === 'locked') throw new ValidationError({ otp: ['OTP không hợp lệ, vui lòng gửi lại'] });
  if (result === 'invalid') throw new ValidationError({ otp: ['OTP không đúng'] });

  await this.repo.updateEmailVerified(user.id);
  user.email_verified = true;

  const tokens = await this.tokenService.issueTokens(user);
  return {
    user: this.sanitizeUser(user),
    accessToken: tokens.accessToken,
    refreshToken: tokens.refreshToken,
  };
}
```

**Step 3: `resendOtp()` trong `auth.service.ts`**

```typescript
async resendOtp(email: string): Promise<void> {
  const user = await this.repo.findUserByEmail(email);
  if (!user) throw new NotFoundError('Email not found');
  if (user.email_verified) throw new ValidationError({ email: ['Email đã được xác minh'] });

  const remaining = await this.otp.cooldownRemaining(email);
  if (remaining > 0) throw new RateLimitError(remaining); // class có sẵn trong lib/errors.ts

  const otpCode = await this.otp.issue(email);
  await sendOtpEmail(email, user.name, otpCode);
}
```

**Step 4: Commit**

```bash
git add apps/api/src/modules/auth/dto/verify-email.dto.ts apps/api/src/modules/auth/dto/resend-otp.dto.ts apps/api/src/modules/auth/auth.service.ts
git commit -m "feat(auth): add verifyEmail and resendOtp service methods"
```

---

### Task 8: `auth.service.ts` — sửa `login()` chặn user chưa verify

**Files:**
- Modify: `apps/api/src/modules/auth/auth.service.ts` (hàm `login`, khảo sát ~dòng 48-77)

**Step 1: Đọc lại `login()` hiện tại — giữ nguyên toàn bộ logic tìm user/status/credential/bcrypt.compare, chỉ chèn thêm check sau khi password đã đúng**

**Step 2: Chèn check `email_verified` NGAY SAU bcrypt.compare thành công, TRƯỚC `issueTokens`**

```typescript
// ... sau khi bcrypt.compare(input.password, cred.password_hash) === true ...

if (!user.email_verified) {
  const remaining = await this.otp.cooldownRemaining(user.email);
  if (remaining <= 0) {
    const otpCode = await this.otp.issue(user.email);
    await sendOtpEmail(user.email, user.name, otpCode);
  }
  throw new EmailNotVerifiedError(user.email);
}

const tokens = await this.tokenService.issueTokens(user);
// ... phần return giữ nguyên như cũ ...
```

**Step 3: Sửa `auth.controller.ts` — bắt riêng `EmailNotVerifiedError` để trả đúng shape cho frontend**

```typescript
import { EmailNotVerifiedError } from '../../lib/errors'; // xác nhận đúng path import

login = async (req: FastifyRequest, reply: FastifyReply) => {
  const input = validate(loginSchema, req.body);
  try {
    const result = await this.authService.login(input);
    this.jwt.setAccessTokenCookie(reply, result.accessToken);
    this.jwt.setRefreshTokenCookie(reply, result.refreshToken);
    return reply.status(200).send({ user: result.user });
  } catch (err) {
    if (err instanceof EmailNotVerifiedError) {
      return reply.status(403).send({ error: 'EMAIL_NOT_VERIFIED', message: err.message, email: err.email });
    }
    throw err;
  }
};
```

**Step 4: Test thủ công**

Run: register 1 user mới (chưa verify), sau đó:
```bash
curl -i -X POST http://localhost:3001/api/auth/login -H "Content-Type: application/json" -d '{"email":"test1@example.com","password":"password123"}'
```
Expected: `403`, body `{"error":"EMAIL_NOT_VERIFIED","message":"...","email":"test1@example.com"}`, không có `Set-Cookie`.

**Step 5: Commit**

```bash
git add apps/api/src/modules/auth/auth.service.ts apps/api/src/modules/auth/auth.controller.ts
git commit -m "feat(auth): block login and auto-resend OTP when email not verified"
```

---

### Task 9: Wire routes `/api/auth/verify-email` và `/api/auth/resend-otp`

**Files:**
- Modify: `apps/api/src/modules/auth/auth.controller.ts` (thêm handler `verifyEmail`, `resendOtp`)
- Modify: `apps/api/src/modules/auth/auth.module.ts` (đăng ký 2 route mới)

**Step 1: Controller**

```typescript
verifyEmail = async (req: FastifyRequest, reply: FastifyReply) => {
  const input = validate(verifyEmailSchema, req.body);
  const result = await this.authService.verifyEmail(input);
  this.jwt.setAccessTokenCookie(reply, result.accessToken);
  this.jwt.setRefreshTokenCookie(reply, result.refreshToken);
  return reply.status(200).send({ user: result.user });
};

resendOtp = async (req: FastifyRequest, reply: FastifyReply) => {
  const input = validate(resendOtpSchema, req.body);
  await this.authService.resendOtp(input.email);
  return reply.status(200).send({ message: 'OTP đã được gửi lại' });
};
```

**Step 2: Routes**

```typescript
app.post('/api/auth/verify-email', controller.verifyEmail);
app.post('/api/auth/resend-otp', controller.resendOtp);
```

**Step 3: Test thủ công — full flow**

```bash
# 1. Register
curl -s -X POST http://localhost:3001/api/auth/register -H "Content-Type: application/json" \
  -d '{"email":"test2@example.com","password":"password123","name":"Test 2"}'
# Lấy OTP từ log server (sendOtpEmail log) hoặc từ Resend dashboard

# 2. Verify với OTP sai
curl -i -X POST http://localhost:3001/api/auth/verify-email -H "Content-Type: application/json" \
  -d '{"email":"test2@example.com","otp":"000000"}'
# Expected: 422 (ValidationError), fields.otp = ["OTP không đúng"]

# 3. Verify với OTP đúng (lấy từ log)
curl -i -X POST http://localhost:3001/api/auth/verify-email -H "Content-Type: application/json" \
  -d '{"email":"test2@example.com","otp":"<otp thật>"}'
# Expected: 200, Set-Cookie access_token + refresh_token, body {"user": {...}}

# 4. Resend khi đang cooldown (register xong gọi ngay)
curl -i -X POST http://localhost:3001/api/auth/resend-otp -H "Content-Type: application/json" \
  -d '{"email":"test2@example.com"}'
# Expected: 429 (nếu còn cooldown) hoặc 400 "Email đã được xác minh" (vì bước 3 đã verify xong)
```

**Step 4: Commit**

```bash
git add apps/api/src/modules/auth/auth.controller.ts apps/api/src/modules/auth/auth.module.ts
git commit -m "feat(auth): wire verify-email and resend-otp endpoints"
```

---

### Task 10: Documents tenant isolation — data model

**Files:**
- Modify: `apps/api/src/lib/collections.ts` (thêm `tenantId?: string` vào `DocChunkDoc`, `DocVersionDoc` — **KHÔNG sửa `database/mongo/collections.ts`, đó là dead code**)
- Modify: `apps/api/src/lib/mongo.ts` (hàm `ensureIndexes()`, khảo sát ~dòng 38-70)

**Step 1: Đọc lại `lib/collections.ts` — xác nhận tên field/interface chính xác của `DocChunkDoc`/`DocVersionDoc`**

**Step 2: Thêm field**

```typescript
export interface DocChunkDoc {
  // ...các field hiện có (documentId, source, content, chunkIndex, ...)...
  tenantId: string;
}

export interface DocVersionDoc {
  // ...các field hiện có...
  tenantId: string;
}
```

**Step 3: Thêm index trong `ensureIndexes()` của `lib/mongo.ts`**

```typescript
await db.collection('documents').createIndex({ tenantId: 1, documentId: 1, chunkIndex: 1 });
await db.collection('document_versions').createIndex({ tenantId: 1, documentId: 1, version: 1 });
// Xác nhận đúng tên collection thật (grep 'documents'/'document_versions' trong lib/collections.ts)
```

**Step 4: Commit**

```bash
git add apps/api/src/lib/collections.ts apps/api/src/lib/mongo.ts
git commit -m "feat(documents): add tenantId field + index to document collections"
```

---

### Task 11: Documents tenant isolation — repository/service/controller/routes

**Files:**
- Modify: `apps/api/src/modules/documents/repositories/documents.repository.ts` (hoặc `documents.repository.ts` trực tiếp — xác nhận path bằng `find apps/api/src/modules/documents -name "documents.repository.ts"`)
- Modify: `apps/api/src/modules/documents/documents.service.ts`
- Modify: `apps/api/src/modules/documents/documents.controller.ts`
- Modify: `apps/api/src/modules/documents/documents.routes.ts`
- Reference pattern: `apps/api/src/modules/upload/upload.routes.ts:17-18` — `getTenantId = (req) => (req as any).tenantId ?? "default"`

**Step 1: Copy helper `getTenantId` sang documents (hoặc import chung nếu upload export nó — kiểm tra, nếu chưa export thì copy y hệt vào `documents.routes.ts`)**

```typescript
const getTenantId = (req: FastifyRequest): string => (req as any).tenantId ?? 'default';
```

**Step 2: Sửa MỌI hàm trong `documents.repository.ts` để nhận `tenantId` làm tham số đầu tiên và filter/set nó**

Ví dụ cho từng hàm hiện có (đọc file thật để biết chữ ký chính xác trước khi sửa):

```typescript
// Trước:
async listDocuments() {
  return coll.aggregate([{ $group: { _id: '$documentId', ... } }]).toArray();
}
// Sau:
async listDocuments(tenantId: string) {
  return coll.aggregate([
    { $match: { tenantId } },
    { $group: { _id: '$documentId', ... } },
  ]).toArray();
}
```

```typescript
// searchSimilar — thêm $match tenantId SAU $vectorSearch (giống cách vừa fix bên agent-go,
// KHÔNG dùng filter trong-stage của $vectorSearch trừ khi Atlas index đã khai báo tenantId
// là filter field — xem PR #10 "fix(rag): loại filter trong-stage khỏi $vectorSearch").
async searchSimilar(tenantId: string, ...) {
  return coll.aggregate([
    { $vectorSearch: { ... } },
    { $match: { tenantId } },
    ...
  ]).toArray();
}
```

Áp dụng tương tự cho `insertChunks` (set `tenantId` khi insert), `getDocumentContent`, `getCurrentVersion`, `archiveCurrentVersion`, `deleteDocument`, `getVersions`, `getVersionContent` — mọi hàm đều nhận `tenantId` và filter theo nó.

**Step 3: Sửa `documents.service.ts` — thread `tenantId` xuống repository (thêm tham số vào mọi method public)**

**Step 4: Sửa `documents.controller.ts` — lấy `tenantId` từ `getTenantId(req)`, truyền xuống service**

```typescript
list = async (req: FastifyRequest, reply: FastifyReply) => {
  const tenantId = getTenantId(req);
  const docs = await this.service.listDocuments(tenantId);
  return reply.status(200).send({ documents: docs });
};
```

**Step 5: Migration dữ liệu cũ (dev only) — gán `tenantId: "default"` cho document đã tồn tại trước field mới**

```bash
# Chạy 1 lần trong mongosh hoặc script riêng, KHÔNG đưa vào migration tự động
db.documents.updateMany({ tenantId: { $exists: false } }, { $set: { tenantId: "default" } })
db.document_versions.updateMany({ tenantId: { $exists: false } }, { $set: { tenantId: "default" } })
```

**Step 6: Test thủ công — xác nhận cô lập**

```bash
# Login user A (đã verify) và user B khác nhau, lấy cookie 2 phiên riêng
# User A upload 1 document
curl -i -b cookie_A.txt -X POST http://localhost:3001/api/documents -F "file=@test.pdf"
# User B list documents — KHÔNG được thấy document của A
curl -s -b cookie_B.txt http://localhost:3001/api/documents
# Expected: {"documents": []}  (rỗng, không thấy tài liệu của A)
```

**Step 7: Commit**

```bash
git add apps/api/src/modules/documents
git commit -m "feat(documents): enforce tenant isolation across all document queries"
```

---

### Task 12: Propagate `X-Tenant-ID` sang agent-go

**Files:**
- Modify: `apps/api/src/agent/client/go-agent.client.ts` (hàm `stream()`, khảo sát dòng ~197 phần headers, 347 dòng tổng)
- Modify: chat controller gọi `goAgentClient.stream()` (tìm bằng grep `goAgentClient.stream(` trong `apps/api/src/modules/chat/`)

**Step 1: Đọc lại chữ ký hàm `stream()` hiện tại (nhận `history, opts` — xác nhận tên tham số thật)**

**Step 2: Thêm `tenantId` vào options, set header**

```typescript
// go-agent.client.ts — trong hàm stream(), sửa phần dựng headers (~dòng 197)
const headers: Record<string, string> = {
  'Content-Type': 'application/json',
};
if (opts.tenantId) {
  headers['X-Tenant-ID'] = opts.tenantId;
}
// ... fetch(..., { headers, ... })
```

Cập nhật type của `opts` (interface tham số thứ 2 của `stream`) để có field `tenantId?: string`.

**Step 3: Sửa chat controller — truyền `req.tenantId` khi gọi `stream()`**

```typescript
await goAgentClient.stream(history, { ...existingOpts, tenantId: req.tenantId });
```

**Step 4: Test thủ công — xác nhận header thật sự được gửi**

Run: bật log request bên `services/agent-go` (middleware đã log tenantID) rồi gửi 1 request chat qua BFF:
```bash
curl -i -b cookie_A.txt -X POST http://localhost:3001/api/chat -H "Content-Type: application/json" -d '{"message":"hello"}'
```
Expected: log `agent-go` KHÔNG còn `tenantId=default` (hoặc `""`) mà là `tenantId=<user A id>` — xem `services/agent-go/internal/middleware/tenant.go` (không cần sửa, chỉ cần verify header tới đúng).

**Step 5: Commit**

```bash
git add apps/api/src/agent/client/go-agent.client.ts apps/api/src/modules/chat
git commit -m "feat(chat): propagate X-Tenant-ID header to agent-go"
```

---

### Task 13: Frontend — auth store

**Files:**
- Modify: `apps/web/src/stores/auth.store.ts` (action `register`, khảo sát ~dòng 85-98)

**Step 1: Đọc lại action `register` hiện tại (đang set `user` ngay sau response — bỏ dòng đó)**

**Step 2: Sửa `register()` — không set `user`, chỉ trả về email**

```typescript
register: async (email: string, password: string, name: string) => {
  await api.post('/api/auth/register', { email, password, name });
  // KHÔNG set user — chưa đăng nhập, còn phải verify OTP
},
```

**Step 3: Thêm `verifyEmail()` và `resendOtp()`**

```typescript
verifyEmail: async (email: string, otp: string) => {
  const { user } = await api.post('/api/auth/verify-email', { email, otp });
  set({ user });
},

resendOtp: async (email: string) => {
  await api.post('/api/auth/resend-otp', { email });
},
```

Thêm 2 action này vào interface `AuthState` (tìm định nghĩa interface trong cùng file).

**Step 4: Commit**

```bash
git add apps/web/src/stores/auth.store.ts
git commit -m "feat(auth): update auth store for OTP verification flow"
```

---

### Task 14: Frontend — RegisterPage, VerifyEmailPage, LoginPage, routes

**Files:**
- Modify: `apps/web/src/pages/auth/RegisterPage.tsx` (đoạn `onSubmit`, khảo sát ~dòng 65-76)
- Create: `apps/web/src/pages/auth/VerifyEmailPage.tsx`
- Modify: `apps/web/src/pages/auth/LoginPage.tsx`
- Modify: `apps/web/src/App.tsx` (thêm route `/verify-email`)

**Step 1: Sửa `RegisterPage.tsx` — điều hướng sang verify thay vì `/`**

```typescript
// Trong onSubmit, sau khi registerUser(...) thành công:
await registerUser(email, password, name);
navigate(`/verify-email?email=${encodeURIComponent(email)}`);
// Bỏ toast "success" cũ nếu nó ngụ ý đã đăng nhập; đổi message thành
// "Đăng ký thành công, vui lòng kiểm tra email để lấy mã OTP"
```

**Step 2: Tạo `VerifyEmailPage.tsx`**

```tsx
import { useEffect, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useAuthStore } from '@/stores/auth.store';
import { ApiError } from '@/lib/http';

const COOLDOWN_SECONDS = 120;

export default function VerifyEmailPage() {
  const [searchParams] = useSearchParams();
  const email = searchParams.get('email') ?? '';
  const navigate = useNavigate();
  const { verifyEmail, resendOtp } = useAuthStore();

  const [otp, setOtp] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [cooldown, setCooldown] = useState(COOLDOWN_SECONDS); // giả định vừa register xong, đang trong cooldown

  useEffect(() => {
    if (cooldown <= 0) return;
    const t = setInterval(() => setCooldown((s) => Math.max(0, s - 1)), 1000);
    return () => clearInterval(t);
  }, [cooldown]);

  const handleVerify = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      await verifyEmail(email, otp);
      navigate('/', { replace: true });
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Xác minh thất bại');
    } finally {
      setLoading(false);
    }
  };

  const handleResend = async () => {
    setError('');
    try {
      await resendOtp(email);
      setCooldown(COOLDOWN_SECONDS);
    } catch (err) {
      if (err instanceof ApiError && err.status === 429) {
        // Đồng bộ lại countdown theo server nếu client lệch giờ
        const secondsRemaining = (err as any).body?.secondsRemaining;
        if (typeof secondsRemaining === 'number') setCooldown(secondsRemaining);
      } else {
        setError(err instanceof ApiError ? err.message : 'Gửi lại thất bại');
      }
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-background">
      <div className="w-full max-w-md p-8 space-y-6 bg-card rounded-xl shadow-lg">
        <div className="text-center">
          <h1 className="text-2xl font-display font-bold">Xác minh email</h1>
          <p className="text-muted-foreground">Nhập mã 6 số đã gửi tới {email}</p>
        </div>

        <form onSubmit={handleVerify} className="space-y-4">
          {error && <div className="text-red-500 text-sm">{error}</div>}
          <input
            type="text"
            inputMode="numeric"
            maxLength={6}
            placeholder="000000"
            value={otp}
            onChange={(e) => setOtp(e.target.value.replace(/\D/g, ''))}
            className="w-full px-4 py-2 border rounded-lg bg-background text-center font-mono text-2xl tracking-widest"
            required
          />
          <button
            type="submit"
            disabled={loading || otp.length !== 6}
            className="w-full py-2 bg-primary text-primary-foreground rounded-lg disabled:opacity-50"
          >
            {loading ? 'Đang xác minh...' : 'Xác minh'}
          </button>
        </form>

        <button
          onClick={handleResend}
          disabled={cooldown > 0}
          className="w-full py-2 border rounded-lg disabled:opacity-50"
        >
          {cooldown > 0 ? `Gửi lại sau ${cooldown}s` : 'Gửi lại mã'}
        </button>
      </div>
    </div>
  );
}
```

**Step 3: Sửa `LoginPage.tsx` — bắt lỗi 403 EMAIL_NOT_VERIFIED**

```typescript
try {
  await login(email, password);
  navigate('/', { replace: true });
} catch (err) {
  if (err instanceof ApiError && err.status === 403 && (err as any).body?.error === 'EMAIL_NOT_VERIFIED') {
    navigate(`/verify-email?email=${encodeURIComponent(email)}`);
    return;
  }
  setError('Invalid email or password');
}
```

Kiểm tra lại shape thật của `ApiError` trong `apps/web/src/lib/http.ts` (có giữ `body` gốc không, hay chỉ có `message`) — nếu `ApiError` hiện tại KHÔNG giữ `body.error`/`body.email`, cần sửa `http.ts` để đính kèm response JSON gốc vào `ApiError` trước khi throw.

**Step 4: Route mới trong `App.tsx`**

```tsx
const VerifyEmailPage = lazy(() => import('@/pages/auth/VerifyEmailPage'));
// ...
<Route path="/verify-email" element={<VerifyEmailPage />} />
// Đặt cùng nhóm với /login, /register (bọc GuestGuard nếu 2 route kia cũng bọc,
// vì GuestGuard chỉ redirect đi nếu ĐÃ có user — verify-email cũng chưa có user tại thời điểm này)
```

**Step 5: Test thủ công trên trình duyệt**

Run: `pnpm --filter @app/web dev`, mở `http://localhost:5173/register`, đăng ký user mới, xác nhận:
- Sau submit, chuyển sang `/verify-email?email=...`
- Nút "Gửi lại mã" bị disable, đếm ngược từ 120s
- Nhập OTP sai → hiện lỗi, không chuyển trang
- Nhập OTP đúng (lấy từ log server) → chuyển vào `/`, đã đăng nhập

**Step 6: Commit**

```bash
git add apps/web/src/pages/auth apps/web/src/App.tsx
git commit -m "feat(auth): add email verification UI flow"
```

---

## Checklist tổng

- [ ] Task 1: Migration `email_verified`
- [ ] Task 2: `EmailNotVerifiedError`
- [ ] Task 3: OTP service + test
- [ ] Task 4: Email service (Resend)
- [ ] Task 5: `auth.repository.ts` methods
- [ ] Task 6: `register()` gửi OTP
- [ ] Task 7: `verifyEmail()` + `resendOtp()`
- [ ] Task 8: `login()` chặn chưa verify
- [ ] Task 9: Wire routes verify-email/resend-otp
- [ ] Task 10: Documents — data model tenantId
- [ ] Task 11: Documents — repository/service/controller/routes
- [ ] Task 12: X-Tenant-ID sang agent-go
- [ ] Task 13: Frontend auth store
- [ ] Task 14: Frontend pages (Register/VerifyEmail/Login/routes)
- [ ] Verify toàn bộ: `pnpm --filter @app/api exec vitest run`, `pnpm --filter @app/api typecheck`, `pnpm --filter @app/web typecheck`, `pnpm --filter @app/web build`
