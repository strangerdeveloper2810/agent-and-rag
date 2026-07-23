import { getDb, COLLECTIONS } from "./mongo";

/**
 * Truy cập collection Mongo với tên tập trung (COLLECTIONS). Repository dùng cái
 * này thay cho `getDb().collection("...")` rải rác → hết magic string + typo,
 * đổi tên collection chỉ 1 chỗ.
 */
export const collections = {
  conversations: () => getDb().collection(COLLECTIONS.conversations),
  messages: () => getDb().collection(COLLECTIONS.messages),
  tasks: () => getDb().collection(COLLECTIONS.tasks),
  documents: () => getDb().collection(COLLECTIONS.documents),
  documentVersions: () => getDb().collection(COLLECTIONS.documentVersions),
};
