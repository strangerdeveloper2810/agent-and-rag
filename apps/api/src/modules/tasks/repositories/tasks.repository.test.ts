import { describe, it, expect } from "vitest";
import type { Collection } from "mongodb";
import { createTaskRepository } from "./tasks.repository";
import type { TaskDoc } from "../../../lib/collections";

// Nhờ factory DI: test repository với collection GIẢ, không cần Mongo thật.
describe("createTaskRepository (DI)", () => {
  it("createTask dùng collection được inject + set timestamps/_id", async () => {
    const inserted: unknown[] = [];
    const fakeCol = {
      insertOne: async (doc: unknown) => {
        inserted.push(doc);
        return { insertedId: "fake-id" };
      },
    } as unknown as Collection<TaskDoc>;

    const repo = createTaskRepository(() => fakeCol);
    const res = await repo.createTask(
      { title: "viết test", status: "todo" } as never,
      "agent",
    );

    expect(res._id).toBe("fake-id");
    expect(res.title).toBe("viết test");
    expect(res.source).toBe("agent");
    expect(res.createdAt).toBeInstanceOf(Date);
    expect(inserted).toHaveLength(1);
  });
});
