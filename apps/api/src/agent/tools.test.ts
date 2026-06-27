import { describe, it, expect } from "vitest";
import { toolDefinitions, getTool } from "./tools";

describe("tool registry", () => {
  it("exposes anthropic tool definitions", () => {
    const names = toolDefinitions.map((t) => t.name);
    expect(names).toContain("ragSearch");
    expect(names).toContain("createTask");
    expect(toolDefinitions[0].input_schema).toBeDefined();
  });
  it("finds tool by name", () => {
    expect(getTool("createTask")).toBeDefined();
    expect(getTool("khong-ton-tai")).toBeUndefined();
  });
});
