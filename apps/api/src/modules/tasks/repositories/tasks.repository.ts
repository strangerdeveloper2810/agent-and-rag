import type { Collection } from "mongodb";
import { collections, type TaskDoc } from "../../../lib/collections";
import { toObjectId } from "../../../lib/object-id";
import type {
  CreateTaskInput,
  UpdateTaskInput,
  ListTasksInput,
} from "../../../schemas/task";

/**
 * Factory tạo task repository. Nhận collection getter (mặc định `collections.tasks`)
 * → test inject được `() => fakeCollection`. Getter gọi LAZY (trong method) để
 * không chạm getDb() lúc import.
 */
export function createTaskRepository(
  col: () => Collection<TaskDoc> = collections.tasks,
) {
  return {
    createTask: async (input: CreateTaskInput, source: "user" | "agent") => {
      const now = new Date();
      const doc = { ...input, source, createdAt: now, updatedAt: now };
      const res = await col().insertOne(doc);
      return { _id: res.insertedId, ...doc };
    },

    listTasks: (filter: ListTasksInput) => {
      const q: Record<string, unknown> = {};
      if (filter.status) q.status = filter.status;
      if (filter.priority) q.priority = filter.priority;
      if (filter.tag) q.tags = filter.tag;
      return col().find(q).sort({ createdAt: -1 }).toArray();
    },

    updateTask: async (input: UpdateTaskInput) => {
      const { id, ...rest } = input;
      const _id = toObjectId(id);
      const set: Record<string, unknown> = { ...rest, updatedAt: new Date() };
      if (rest.status === "done") set.completedAt = new Date();
      await col().updateOne({ _id }, { $set: set });
      return col().findOne({ _id });
    },

    deleteTask: async (id: string) => {
      await col().deleteOne({ _id: toObjectId(id) });
      return { ok: true };
    },
  };
}

// Instance mặc định (wire vào Mongo thật) + named exports để caller không phải đổi.
export const taskRepository = createTaskRepository();
export const { createTask, listTasks, updateTask, deleteTask } = taskRepository;
