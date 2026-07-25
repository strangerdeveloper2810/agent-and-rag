/**
 * PostgreSQL module — connection pool + auto-migration.
 *
 * PostgreSQL là primary DB cho auth, users, và audit log.
 * Mongo vẫn giữ cho dữ liệu phi cấu trúc (chat history, documents, tasks).
 *
 * Migration files được lưu trong `migrations/` và chạy tự động theo thứ tự
 * alphabet khi `initPostgres()` được gọi.
 */
import { Pool } from "pg";
import fs from "fs";
import path from "path";

let pool: Pool | null = null;

export interface PostgresConfig {
  connectionString: string;
  max?: number;
}

/**
 * Chạy tất cả file SQL trong thư mục migrations theo thứ tự alphabet.
 * Dùng CREATE IF NOT EXISTS nên an toàn để chạy lại nhiều lần.
 */
const runMigrations = async (pgPool: Pool): Promise<void> => {
  // import.meta.dirname = thư mục chứa file này (Node 22+)
  const migrationsDir = path.join(import.meta.dirname, "migrations");
  const files = fs
    .readdirSync(migrationsDir)
    .filter((f) => f.endsWith(".sql"))
    .sort(); // 001-..., 002-... chạy theo thứ tự

  for (const file of files) {
    const sql = fs.readFileSync(path.join(migrationsDir, file), "utf-8");
    await pgPool.query(sql);
  }
};

/**
 * Khởi tạo connection pool PostgreSQL.
 * Idempotent — gọi nhiều lần chỉ tạo 1 pool.
 * Tự động chạy migrations sau khi kết nối thành công.
 *
 * @throws {Error} nếu PG_CONNECTION_STRING không được cấu hình.
 */
export const initPostgres = async (config: PostgresConfig): Promise<Pool> => {
  if (!config.connectionString) {
    throw new Error("PG_CONNECTION_STRING not configured");
  }
  if (pool) return pool;
  pool = new Pool({
    connectionString: config.connectionString,
    max: config.max ?? 10,
  });
  await pool.query("SELECT 1"); // verify connection
  await runMigrations(pool);
  return pool;
};

/**
 * Lấy instance Pool đã khởi tạo.
 *
 * @throws {Error} nếu initPostgres() chưa được gọi.
 */
export const getPgPool = (): Pool => {
  if (!pool)
    throw new Error("PostgreSQL not initialized. Call initPostgres() first.");
  return pool;
};

/**
 * Đóng connection pool PostgreSQL (graceful shutdown).
 */
export const closePostgres = async (): Promise<void> => {
  await pool?.end();
  pool = null;
};
