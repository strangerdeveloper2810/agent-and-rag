// ----- Tầng dữ liệu MongoDB -----
export {
  connectMongo,
  getDb,
  closeMongo,
  COLLECTIONS,
  ensureIndexes,
  collections,
  tenantFilter,
} from "./mongo/mongo.module";
export type {
  ConversationDoc,
  MessageDoc,
  TaskDoc,
  DocChunkDoc,
  DocVersionDoc,
} from "./mongo/mongo.module";

// ----- Tầng dữ liệu PostgreSQL (auth, users, refresh tokens) -----
export {
  initPostgres,
  getPgPool,
  closePostgres,
} from "./postgres/postgres.module";
export type { PostgresConfig } from "./postgres/postgres.module";

// ----- Redis (rate limiting, cache) -----
export {
  initRedis,
  getRedis,
  closeRedis,
  cacheSet,
  cacheGet,
  cacheDel,
  cacheDelPattern,
  cacheKey,
} from "./redis/redis.module";
export type { RedisConfig } from "./redis/redis.module";
