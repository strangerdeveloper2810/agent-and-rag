import { describe, it, expect } from "vitest";
import { messageRoleSchema } from "./message";

describe("messageRoleSchema", () => {
  it("accepts valid role", () => {
    expect(messageRoleSchema.parse("user")).toBe("user");
    expect(messageRoleSchema.parse("assistant")).toBe("assistant");
  });
  it("rejects invalid role", () => {
    expect(() => messageRoleSchema.parse("robot")).toThrow();
  });
});
