import { describe, it, expect } from "vitest";
import { lcTools } from "./lc-tools";

describe("lcTools", () => {
  it("exposes đủ 7 langchain tool", () => {
    const names = lcTools.map((t) => t.name);
    expect(names).toEqual(
      expect.arrayContaining([
        "ragSearch",
        "listDocuments",
        "readDocument",
        "createTask",
        "listTasks",
        "updateTask",
        "deleteTask",
      ]),
    );
  });
});
