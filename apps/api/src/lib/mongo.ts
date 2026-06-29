import { MongoClient, type Db } from "mongodb";
import { config } from "../config";

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
