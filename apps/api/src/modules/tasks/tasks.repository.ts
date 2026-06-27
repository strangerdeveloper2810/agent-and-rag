import { ObjectId } from "mongodb";
import { getDb } from "../../lib/mongo";
import type {
  CreateTaskInput,
  UpdateTaskInput,
  ListTasksInput,
} from "../../schemas/task";

const col = () => getDb().collection("tasks");

export const createTask = async (
  input: CreateTaskInput,
  source: "user" | "agent",
) => {
  const now = new Date();
  const doc = { ...input, source, createdAt: now, updatedAt: now };
  const res = await col().insertOne(doc);
  return { _id: res.insertedId, ...doc };
};

export const listTasks = (filter: ListTasksInput) => {
  const q: Record<string, unknown> = {};
  if (filter.status) q.status = filter.status;
  if (filter.priority) q.priority = filter.priority;
  if (filter.tag) q.tags = filter.tag;
  return col().find(q).sort({ createdAt: -1 }).toArray();
};

export const updateTask = async (input: UpdateTaskInput) => {
  const { id, ...rest } = input;
  const set: Record<string, unknown> = { ...rest, updatedAt: new Date() };
  if (rest.status === "done") set.completedAt = new Date();
  await col().updateOne({ _id: new ObjectId(id) }, { $set: set });
  return col().findOne({ _id: new ObjectId(id) });
};

export const deleteTask = async (id: string) => {
  await col().deleteOne({ _id: new ObjectId(id) });
  return { ok: true };
};
