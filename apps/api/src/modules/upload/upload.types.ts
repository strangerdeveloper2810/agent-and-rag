import type { StorageCategory } from "../../common/storage/storage.service";

export interface UploadRecord {
  _id: string;
  tenantId: string;
  userId?: string;
  filename: string;
  originalName: string;
  mimeType: string;
  size: number;
  url: string;
  key: string;
  bucket: string;
  category: StorageCategory;
  createdAt: Date;
}

export interface CreateUploadInput {
  tenantId: string;
  userId?: string;
  filename?: string;
  originalName: string;
  mimeType: string;
  size: number;
  buffer: Buffer;
  category: StorageCategory;
}

export interface PresignedUploadInput {
  tenantId: string;
  userId?: string;
  category: StorageCategory;
  ext: string;
  contentType?: string;
}
