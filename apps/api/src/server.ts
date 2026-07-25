import { buildApp } from "./app.js";
import { config } from "./config.js";
import { connectMongo, closeMongo, ensureIndexes } from "./lib/mongo.js";
import { initPostgres, closePostgres, initRedis, closeRedis } from "./database/index.js";

const app = buildApp();

async function start() {
  // MongoDB (AI data: chat, documents, tasks, memories)
  await connectMongo();
  await ensureIndexes();
  app.log.info("MongoDB connected");

  // PostgreSQL (auth data: users, credentials, refresh tokens)
  await initPostgres({ connectionString: config.PG_CONNECTION_STRING });
  app.log.info("PostgreSQL connected");

  // Redis (rate limiting, embedding/chat/tool cache)
  await initRedis({ url: config.REDIS_URL });
  app.log.info("Redis connected");

  const address = await app.listen({ port: config.PORT, host: "0.0.0.0" });
  app.log.info(`API listening at ${address}`);
}

// Graceful shutdown: đóng HTTP server (ngừng nhận request mới, đợi request đang
// chạy) rồi đóng tất cả kết nối DB. Quan trọng khi chạy trong container
// (Docker/k8s gửi SIGTERM).
async function shutdown(signal: string) {
  app.log.info(`Nhận ${signal} → đang tắt...`);
  try {
    await app.close();
    await Promise.all([closeMongo(), closePostgres(), closeRedis()]);
    process.exit(0);
  } catch (err) {
    app.log.error(err);
    process.exit(1);
  }
}

process.on("SIGINT", () => void shutdown("SIGINT"));
process.on("SIGTERM", () => void shutdown("SIGTERM"));

start().catch((err) => {
  app.log.error(err);
  process.exit(1);
});
