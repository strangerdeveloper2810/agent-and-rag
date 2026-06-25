# Mốc 0: Nền móng (Foundation) — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Dựng monorepo pnpm + Turborepo với `apps/api` (Fastify) và `apps/web` (Vite React), kết nối MongoDB Atlas, frontend gọi được API.

**Architecture:** Monorepo hai app. API là Fastify + TypeScript chạy bằng `tsx`. Web là Vite + React + Tailwind. Cấu hình env validate bằng Zod. Mọi thứ chạy local trừ MongoDB Atlas (cloud).

**Tech Stack:** pnpm workspaces, Turborepo, Fastify, TypeScript, Vite, React, TailwindCSS, MongoDB driver, Zod, Vitest, tsx.

---

## Chuẩn bị trước (làm thủ công, ngoài code)

1. **Tạo MongoDB Atlas cluster M0 (free):**
   - Đăng ký tại https://cloud.mongodb.com
   - Tạo cluster M0, region gần nhất.
   - Database Access → tạo user/password.
   - Network Access → Add IP `0.0.0.0/0` (cho học; production phải siết lại).
   - Lấy connection string: `mongodb+srv://<user>:<pass>@cluster.xxx.mongodb.net/ai_agent_tut`
2. **Lấy API keys:**
   - Anthropic: https://console.anthropic.com → API Keys
   - Voyage AI: https://www.voyageai.com → API Keys
3. Lưu lại để điền vào `.env` ở Task 5.

---

## Task 1: Khởi tạo monorepo root

**Files:**
- Create: `package.json`
- Create: `pnpm-workspace.yaml`
- Create: `turbo.json`
- Create: `.gitignore`
- Create: `.nvmrc`

**Step 1: Tạo `pnpm-workspace.yaml`**
```yaml
packages:
  - "apps/*"
```

**Step 2: Tạo `package.json` root**
```json
{
  "name": "ai-agent-tut",
  "version": "0.1.0",
  "private": true,
  "packageManager": "pnpm@10.30.3",
  "scripts": {
    "dev": "turbo run dev",
    "build": "turbo run build",
    "test": "turbo run test",
    "lint": "turbo run lint",
    "typecheck": "turbo run typecheck"
  },
  "devDependencies": {
    "turbo": "^2.3.0",
    "typescript": "^5.7.0"
  }
}
```

**Step 3: Tạo `turbo.json`**
```json
{
  "$schema": "https://turbo.build/schema.json",
  "tasks": {
    "dev": { "cache": false, "persistent": true },
    "build": { "dependsOn": ["^build"], "outputs": ["dist/**"] },
    "test": { "dependsOn": ["^build"] },
    "typecheck": {},
    "lint": {}
  }
}
```

**Step 4: Tạo `.nvmrc`**
```
22
```

**Step 5: Tạo `.gitignore`**
```
node_modules
dist
.env
.env.local
.turbo
*.log
.DS_Store
coverage
```

**Step 6: Commit**
```bash
git add package.json pnpm-workspace.yaml turbo.json .gitignore .nvmrc
git commit -m "chore: init monorepo with pnpm + turbo"
```

---

## Task 2: Tạo package config dùng chung (tsconfig base)

**Files:**
- Create: `tsconfig.base.json`

**Step 1: Tạo `tsconfig.base.json`**
```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "lib": ["ES2022"],
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true,
    "resolveJsonModule": true,
    "declaration": false,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true
  }
}
```

**Step 2: Commit**
```bash
git add tsconfig.base.json
git commit -m "chore: add shared tsconfig base"
```

---

## Task 3: Khởi tạo apps/api (Fastify hello world)

**Files:**
- Create: `apps/api/package.json`
- Create: `apps/api/tsconfig.json`
- Create: `apps/api/src/config.ts`
- Create: `apps/api/src/app.ts`
- Create: `apps/api/src/server.ts`
- Create: `apps/api/vitest.config.ts`

> **Ghi chú Fastify cho người mới:** Fastify giống Express nhưng nhanh & gọn hơn.
> - `fastify()` tạo instance app.
> - `app.get/post(...)` đăng ký route.
> - `app.register(plugin)` cắm module (sẽ dùng ở Mốc sau).
> - `app.listen(...)` chạy server.

**Step 1: Tạo `apps/api/package.json`**
```json
{
  "name": "@app/api",
  "version": "0.1.0",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "tsx watch src/server.ts",
    "build": "tsc",
    "start": "node dist/server.js",
    "test": "vitest run",
    "test:watch": "vitest",
    "typecheck": "tsc --noEmit"
  },
  "dependencies": {
    "fastify": "^5.2.0",
    "@fastify/cors": "^10.0.0",
    "zod": "^3.24.0",
    "mongodb": "^6.12.0"
  },
  "devDependencies": {
    "tsx": "^4.19.0",
    "typescript": "^5.7.0",
    "vitest": "^2.1.0",
    "@types/node": "^22.10.0"
  }
}
```

**Step 2: Tạo `apps/api/tsconfig.json`**
```json
{
  "extends": "../../tsconfig.base.json",
  "compilerOptions": {
    "outDir": "dist",
    "rootDir": "src",
    "types": ["node"]
  },
  "include": ["src/**/*"]
}
```

**Step 3: Tạo `apps/api/src/config.ts` (env validation bằng Zod)**
```ts
import { z } from "zod";

const envSchema = z.object({
  PORT: z.coerce.number().default(3001),
  MONGODB_URI: z.string().min(1, "MONGODB_URI is required"),
  ANTHROPIC_API_KEY: z.string().min(1, "ANTHROPIC_API_KEY is required"),
  VOYAGE_API_KEY: z.string().min(1, "VOYAGE_API_KEY is required"),
  CLAUDE_MODEL: z.string().default("claude-haiku-4-5-20251001"),
});

// Parse ngay lúc import → thiếu env là crash sớm với message rõ ràng
export const config = envSchema.parse(process.env);
export type Config = z.infer<typeof envSchema>;
```

**Step 4: Tạo `apps/api/src/app.ts`**
```ts
import Fastify, { type FastifyInstance } from "fastify";
import cors from "@fastify/cors";

export function buildApp(): FastifyInstance {
  const app = Fastify({ logger: true });

  app.register(cors, { origin: true });

  app.get("/health", async () => ({ status: "ok" }));

  return app;
}
```

**Step 5: Tạo `apps/api/src/server.ts`**
```ts
import { buildApp } from "./app.js";
import { config } from "./config.js";

const app = buildApp();

app
  .listen({ port: config.PORT, host: "0.0.0.0" })
  .then((address) => app.log.info(`API listening at ${address}`))
  .catch((err) => {
    app.log.error(err);
    process.exit(1);
  });
```

**Step 6: Tạo `apps/api/vitest.config.ts`**
```ts
import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    globals: true,
    environment: "node",
  },
});
```

**Step 7: Commit**
```bash
git add apps/api
git commit -m "feat(api): scaffold fastify app with health route and env config"
```

---

## Task 4: Test endpoint /health (TDD đầu tiên)

**Files:**
- Create: `apps/api/src/app.test.ts`

**Step 1: Write the failing test**
```ts
import { describe, it, expect } from "vitest";
import { buildApp } from "./app.js";

describe("health route", () => {
  it("returns status ok", async () => {
    const app = buildApp();
    const res = await app.inject({ method: "GET", url: "/health" });
    expect(res.statusCode).toBe(200);
    expect(res.json()).toEqual({ status: "ok" });
    await app.close();
  });
});
```
> **Ghi chú:** `app.inject()` là cách Fastify test route mà không cần mở port thật — rất tiện.

**Step 2: Run test to verify it passes**
```bash
cd apps/api && pnpm test
```
Expected: PASS (route đã có sẵn từ Task 3).

**Step 3: Commit**
```bash
git add apps/api/src/app.test.ts
git commit -m "test(api): cover health route"
```

---

## Task 5: File .env và kiểm tra config

**Files:**
- Create: `apps/api/.env.example`
- Create: `apps/api/.env` (KHÔNG commit — đã trong .gitignore)

**Step 1: Tạo `apps/api/.env.example`**
```
PORT=3001
MONGODB_URI=mongodb+srv://user:pass@cluster.xxx.mongodb.net/ai_agent_tut
ANTHROPIC_API_KEY=sk-ant-...
VOYAGE_API_KEY=pa-...
CLAUDE_MODEL=claude-haiku-4-5-20251001
```

**Step 2: Tạo `apps/api/.env` thật** (điền key thật từ phần Chuẩn bị)

**Step 3: Cài tsx đọc .env tự động — cập nhật script dev trong `apps/api/package.json`**
```json
"dev": "tsx watch --env-file=.env src/server.ts",
```

**Step 4: Chạy thử server**
```bash
cd apps/api && pnpm dev
```
Expected: log `API listening at http://0.0.0.0:3001`. Mở `http://localhost:3001/health` → `{"status":"ok"}`.

**Step 5: Commit (chỉ .env.example và package.json)**
```bash
git add apps/api/.env.example apps/api/package.json
git commit -m "chore(api): add env example and load .env in dev"
```

---

## Task 6: Kết nối MongoDB Atlas

**Files:**
- Create: `apps/api/src/lib/mongo.ts`
- Modify: `apps/api/src/server.ts`

**Step 1: Tạo `apps/api/src/lib/mongo.ts`**
```ts
import { MongoClient, type Db } from "mongodb";
import { config } from "../config.js";

let client: MongoClient | null = null;
let db: Db | null = null;

export async function connectMongo(): Promise<Db> {
  if (db) return db;
  client = new MongoClient(config.MONGODB_URI);
  await client.connect();
  db = client.db(); // dùng db name trong URI
  return db;
}

export function getDb(): Db {
  if (!db) throw new Error("Mongo not connected. Call connectMongo() first.");
  return db;
}

export async function closeMongo(): Promise<void> {
  await client?.close();
  client = null;
  db = null;
}
```

**Step 2: Modify `apps/api/src/server.ts` — connect trước khi listen**
```ts
import { buildApp } from "./app.js";
import { config } from "./config.js";
import { connectMongo } from "./lib/mongo.js";

const app = buildApp();

async function start() {
  await connectMongo();
  app.log.info("MongoDB connected");
  const address = await app.listen({ port: config.PORT, host: "0.0.0.0" });
  app.log.info(`API listening at ${address}`);
}

start().catch((err) => {
  app.log.error(err);
  process.exit(1);
});
```

**Step 3: Chạy thử**
```bash
cd apps/api && pnpm dev
```
Expected: log `MongoDB connected` rồi `API listening`. Nếu lỗi auth/network → kiểm tra lại URI & Network Access trên Atlas.

**Step 4: Commit**
```bash
git add apps/api/src/lib/mongo.ts apps/api/src/server.ts
git commit -m "feat(api): connect to MongoDB Atlas on startup"
```

---

## Task 7: Khởi tạo apps/web (Vite + React + Tailwind)

**Files:**
- Create: `apps/web/package.json`
- Create: `apps/web/tsconfig.json`
- Create: `apps/web/vite.config.ts`
- Create: `apps/web/index.html`
- Create: `apps/web/src/main.tsx`
- Create: `apps/web/src/App.tsx`
- Create: `apps/web/src/index.css`
- Create: `apps/web/tailwind.config.js`
- Create: `apps/web/postcss.config.js`

**Step 1: Tạo `apps/web/package.json`**
```json
{
  "name": "@app/web",
  "version": "0.1.0",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc && vite build",
    "preview": "vite preview",
    "typecheck": "tsc --noEmit"
  },
  "dependencies": {
    "react": "^19.0.0",
    "react-dom": "^19.0.0"
  },
  "devDependencies": {
    "@types/react": "^19.0.0",
    "@types/react-dom": "^19.0.0",
    "@vitejs/plugin-react": "^4.3.0",
    "autoprefixer": "^10.4.0",
    "postcss": "^8.4.0",
    "tailwindcss": "^3.4.0",
    "typescript": "^5.7.0",
    "vite": "^6.0.0"
  }
}
```

**Step 2: Tạo `apps/web/tsconfig.json`**
```json
{
  "extends": "../../tsconfig.base.json",
  "compilerOptions": {
    "lib": ["ES2022", "DOM", "DOM.Iterable"],
    "jsx": "react-jsx",
    "moduleResolution": "Bundler",
    "noEmit": true,
    "types": ["vite/client"]
  },
  "include": ["src"]
}
```

**Step 3: Tạo `apps/web/vite.config.ts`** (proxy /api → API để tránh CORS lúc dev)
```ts
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      "/api": "http://localhost:3001",
    },
  },
});
```

**Step 4: Tạo `apps/web/index.html`**
```html
<!doctype html>
<html lang="vi">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>AI Agent Tut</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

**Step 5: Tạo `apps/web/tailwind.config.js`**
```js
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: { extend: {} },
  plugins: [],
};
```

**Step 6: Tạo `apps/web/postcss.config.js`**
```js
export default {
  plugins: { tailwindcss: {}, autoprefixer: {} },
};
```

**Step 7: Tạo `apps/web/src/index.css`**
```css
@tailwind base;
@tailwind components;
@tailwind utilities;
```

**Step 8: Tạo `apps/web/src/main.tsx`**
```tsx
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import App from "./App.tsx";
import "./index.css";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
```

**Step 9: Tạo `apps/web/src/App.tsx`** (gọi /health để verify kết nối)
```tsx
import { useEffect, useState } from "react";

export default function App() {
  const [health, setHealth] = useState<string>("checking...");

  useEffect(() => {
    fetch("/api/health")
      .then((r) => r.json())
      .then((d) => setHealth(d.status))
      .catch(() => setHealth("error"));
  }, []);

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-50">
      <div className="text-center">
        <h1 className="text-2xl font-bold text-gray-800">AI Agent Tut</h1>
        <p className="mt-2 text-gray-600">
          API health: <span className="font-mono">{health}</span>
        </p>
      </div>
    </div>
  );
}
```

> **Lưu ý:** Web gọi `/api/health` nhưng API route là `/health`. Cần cập nhật API thêm prefix `/api`, hoặc đổi proxy. Làm ở Step 10.

**Step 10: Thêm prefix `/api` cho API — Modify `apps/api/src/app.ts`**
Đổi route health thành:
```ts
app.get("/api/health", async () => ({ status: "ok" }));
```
Và cập nhật test `apps/api/src/app.test.ts` url thành `/api/health`. Chạy lại `pnpm test` → PASS.

**Step 11: Chạy cả hai app**
```bash
# từ root
pnpm install
pnpm dev
```
Expected: API ở :3001, Web ở :5173. Mở `http://localhost:5173` → thấy "API health: ok".

**Step 12: Commit**
```bash
git add apps/web apps/api/src/app.ts apps/api/src/app.test.ts
git commit -m "feat(web): scaffold vite react app calling api health"
```

---

## Task 8: README ngắn cho repo

**Files:**
- Create: `README.md`

**Step 1: Tạo `README.md`**
```markdown
# AI Agent Tut

Dự án học tập build AI Agent chatbot (RAG + task management).
Xem thiết kế: `docs/plans/2026-06-25-ai-agent-chatbot-design.md`

## Chạy local
1. `pnpm install`
2. Tạo `apps/api/.env` từ `apps/api/.env.example`
3. `pnpm dev` → API :3001, Web :5173

## Lộ trình
- Mốc 0: nền móng (xong)
- Mốc 1: chatbot có memory + SSE
- Mốc 2: agent + tools (RAG + task)
- Mốc 3: LangGraph multi-step
```

**Step 2: Commit**
```bash
git add README.md
git commit -m "docs: add project readme"
```

---

## Định nghĩa "Done" cho Mốc 0
- [ ] `pnpm dev` chạy cả API và Web không lỗi
- [ ] `http://localhost:5173` hiển thị "API health: ok"
- [ ] MongoDB Atlas kết nối thành công (log "MongoDB connected")
- [ ] `pnpm test` trong apps/api PASS
- [ ] Tất cả env keys đã có trong `.env`
