import { v4 as uuid } from "uuid";
import {
  uploadFile,
  createUploadUrl,
  getPublicUrl,
  deleteFile,
} from "../../common/storage/storage.service";
import type { StorageCategory } from "../../common/storage/storage.service";
import { config } from "../../config";
import {
  insertUpload,
  findByKey,
  listByTenant,
  deleteByKey,
} from "./upload.repository";
import type {
  CreateUploadInput,
  PresignedUploadInput,
  UploadRecord,
} from "./upload.types";
import type { UploadDoc } from "../../lib/collections";

const BUCKET = config.S3_BUCKET;

const toRecord = (doc: UploadDoc): UploadRecord => ({
  _id: doc._id!.toHexString(),
  tenantId: doc.tenantId as string,
  userId: doc.userId as string | undefined,
  filename: doc.filename as string,
  originalName: doc.originalName as string,
  mimeType: doc.mimeType as string,
  size: doc.size as number,
  url: doc.url as string,
  key: doc.key as string,
  bucket: doc.bucket as string,
  category: doc.category as StorageCategory,
  createdAt: doc.createdAt as Date,
});

const VALID_CATEGORIES: StorageCategory[] = [
  "images",
  "docs",
  "notes",
  "memories",
];

export const parseCategory = (cat?: unknown): StorageCategory => {
  const c = typeof cat === "string" ? cat : "images";
  return VALID_CATEGORIES.includes(c as StorageCategory)
    ? (c as StorageCategory)
    : "images";
};

export const createPresignedUpload = async (input: PresignedUploadInput) => {
  const ext = input.ext.replace(/^\./, "") || "bin";
  const filename = `${uuid()}.${ext}`;
  return createUploadUrl(
    input.tenantId,
    input.category,
    filename,
    input.contentType,
  );
};

export const uploadFileServer = async (
  input: CreateUploadInput,
): Promise<UploadRecord> => {
  const ext = input.originalName.split(".").pop() ?? "bin";
  const filename = input.filename ?? `${uuid()}.${ext}`;

  const result = await uploadFile(
    input.tenantId,
    input.category,
    filename,
    input.buffer,
    input.mimeType,
  );

  const rec = await insertUpload({
    tenantId: input.tenantId,
    userId: input.userId,
    filename,
    originalName: input.originalName,
    mimeType: input.mimeType,
    size: input.size,
    url: result.url,
    key: result.key,
    bucket: BUCKET,
    category: input.category,
    createdAt: new Date(),
  });

  return toRecord(rec);
};

export const getUploadByKey = async (tenantId: string, key: string) => {
  const doc = await findByKey(tenantId, key);
  return doc ? toRecord(doc) : null;
};

export const getDownloadUrl = async (key: string) => getPublicUrl(key);

export const listUploads = async (tenantId: string, category?: string) => {
  const docs = await listByTenant(tenantId, category);
  return docs.map(toRecord);
};

export const removeUpload = async (tenantId: string, key: string) => {
  await deleteFile(key);
  await deleteByKey(tenantId, key);
};
