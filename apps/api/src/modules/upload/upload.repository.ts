import type { Collection, ObjectId } from "mongodb";
import { collections, type UploadDoc } from "../../lib/collections";

export function createUploadRepository(
  uploads: () => Collection<UploadDoc> = collections.uploads,
) {
  const insertUpload = async (
    record: Omit<UploadDoc, "_id">,
  ): Promise<UploadDoc> => {
    const { insertedId } = await uploads().insertOne(record as UploadDoc);
    return { _id: insertedId, ...record } as UploadDoc;
  };

  const findByKey = async (tenantId: string, key: string) =>
    uploads().findOne({ tenantId, key });

  const findById = async (tenantId: string, id: string) =>
    uploads().findOne({
      tenantId,
      _id: id as unknown as ObjectId,
    });

  const listByTenant = async (tenantId: string, category?: string) => {
    const filter: Record<string, unknown> = { tenantId };
    if (category) filter.category = category;
    return uploads().find(filter).sort({ createdAt: -1 }).toArray();
  };

  const deleteByKey = async (tenantId: string, key: string) => {
    const result = await uploads().deleteOne({ tenantId, key });
    return result.deletedCount > 0;
  };

  return { insertUpload, findByKey, findById, listByTenant, deleteByKey };
}

export const uploadRepository = createUploadRepository();
export const { insertUpload, findByKey, findById, listByTenant, deleteByKey } =
  uploadRepository;
