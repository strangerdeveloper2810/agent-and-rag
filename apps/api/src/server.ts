import { buildApp } from "./app.js";
import { config } from "./config.js";
import { connectMongo, closeMongo, ensureIndexes } from "./lib/mongo.js";

const app = buildApp();

async function start() {
  await connectMongo();
  await ensureIndexes();
  app.log.info("MongoDB connected");
  const address = await app.listen({ port: config.PORT, host: "0.0.0.0" });
  app.log.info(`API listening at ${address}`);
}

// Tắt gọn gàng: đóng HTTP server (ngừng nhận request mới, đợi request đang chạy)
// rồi đóng kết nối Mongo. Quan trọng khi chạy trong container (Docker/k8s gửi SIGTERM).
async function shutdown(signal: string) {
  app.log.info(`Nhận ${signal} → đang tắt...`);
  try {
    await app.close();
    await closeMongo();
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
