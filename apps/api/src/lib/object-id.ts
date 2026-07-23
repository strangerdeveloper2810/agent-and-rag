import { ObjectId } from "mongodb";
import { BadRequestError } from "./errors";

/**
 * Ép id chuỗi → ObjectId. id sai định dạng → BadRequestError (400) thay vì để
 * `new ObjectId(...)` ném BSONError rồi rơi vào nhánh 500 của error handler.
 */
export function toObjectId(id: string): ObjectId {
  if (!ObjectId.isValid(id)) {
    throw new BadRequestError(`id không hợp lệ: ${id}`);
  }
  return new ObjectId(id);
}
