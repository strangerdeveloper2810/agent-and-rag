import { describe, it, expect } from "vitest";
import { createTaskInputSchema } from "./task";

describe("createTaskInputSchema", () => {
  it("applies defaults", () => {
    const t = createTaskInputSchema.parse({ title: "Mua sữa" });
    expect(t.status).toBe("todo");
    expect(t.priority).toBe("medium");
    expect(t.tags).toEqual([]);
  });
  it("rejects empty title", () => {
    expect(() => createTaskInputSchema.parse({ title: "" })).toThrow();
  });
});
