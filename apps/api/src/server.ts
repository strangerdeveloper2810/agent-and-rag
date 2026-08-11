import { buildApp } from "./app.js";
import { config } from "./config.js";
import { connectMongo, closeMongo, ensureIndexes } from "./lib/mongo.js";
import { initPostgres, closePostgres, initRedis, closeRedis } from "./database/index.js";
import type { FastifyInstance } from "fastify";

let app: FastifyInstance;

async function start() {
  // ── Init tất cả DB connections TRƯỚC ──
  // Phải init trước buildApp() để getPgPool()/getRedis() hoạt động
  // khi Fastify plugin được register trong buildApp().
  // Mỗi DB là graceful: lỗi = warning + continue, không crash server.

  // MongoDB (AI data: chat, documents, tasks, memories)
  try {
    await connectMongo();
    await ensureIndexes();
    console.log("MongoDB connected");
  } catch (err) {
    console.warn("MongoDB unavailable — chat/document/task features degraded:", (err as Error).message);
  }

  // PostgreSQL (auth data: users, credentials, refresh tokens)
  try {
    await initPostgres({ connectionString: config.PG_CONNECTION_STRING });
    console.log("PostgreSQL connected");
  } catch (err) {
    console.warn("PostgreSQL unavailable — auth features degraded:", (err as Error).message);
  }

  // Redis (rate limiting, embedding/chat/tool cache)
  try {
    await initRedis({ url: config.REDIS_URL });
    console.log("Redis connected");
  } catch (err) {
    console.warn("Redis unavailable — caching & rate-limit degraded:", (err as Error).message);
  }

  // ── Build & start HTTP server ──
  app = buildApp();
  const address = await app.listen({ port: config.PORT, host: "0.0.0.0" });
  app.log.info(`API listening at ${address}`);
}

// Graceful shutdown: đóng HTTP server (ngừng nhận request mới, đợi request đang
// chạy) rồi đóng tất cả kết nối DB. Quan trọng khi chạy trong container
// (Docker/k8s gửi SIGTERM).
async function shutdown(signal: string) {
  console.log(`Nhận ${signal} → đang tắt...`);
  try {
    if (app) await app.close();
    await Promise.all([closeMongo(), closePostgres(), closeRedis()]);
    process.exit(0);
  } catch (err) {
    console.error(err);
    process.exit(1);
  }
}

process.on("SIGINT", () => void shutdown("SIGINT"));
process.on("SIGTERM", () => void shutdown("SIGTERM"));

start().catch((err) => {
  console.error(err);
  process.exit(1);
});
