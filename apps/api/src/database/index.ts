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

// ----- Tầng dữ liệu PostgreSQL (stub cho Phase 3) -----
export {
  initPostgres,
  getPgPool,
  closePostgres,
} from "./postgres/postgres.module";
export type { PostgresConfig } from "./postgres/postgres.module";
