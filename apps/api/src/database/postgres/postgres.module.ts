/**
 * PostgreSQL module — STUB cho Phase 3.
 *
 * Phase 3 sẽ dùng PostgreSQL làm primary DB cho auth, users, và audit log.
 * Mongo vẫn giữ cho dữ liệu phi cấu trúc (chat history, documents, tasks).
 *
 * Để dùng: cài `pg` và `@types/pg`, rồi gọi `initPostgres()` trong server startup.
 */
// @ts-expect-error -- pg chưa cài (Phase 3 stub). Cài `npm i pg @types/pg` khi đến Phase 3.
import { Pool } from "pg";

let pool: Pool | null = null;

export interface PostgresConfig {
  connectionString: string;
  max?: number;
}

/**
 * Khởi tạo connection pool PostgreSQL.
 * Idempotent — gọi nhiều lần chỉ tạo 1 pool.
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
  return pool;
}

/**
 * Lấy instance Pool đã khởi tạo.
 *
 * @throws {Error} nếu initPostgres() chưa được gọi.
 */
export const getPgPool = (): Pool => {
  if (!pool) throw new Error("PostgreSQL not initialized. Call initPostgres() first.");
  return pool;
}

/**
 * Đóng connection pool PostgreSQL (graceful shutdown).
 */
export const closePostgres = async (): Promise<void> => {
  await pool?.end();
  pool = null;
}
