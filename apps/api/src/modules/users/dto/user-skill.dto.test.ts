import { describe, it, expect } from "vitest";
import {
  createUserSkillSchema,
  updateUserSkillSchema,
  toggleSkillSchema,
} from "./user-skill.dto";

describe("createUserSkillSchema", () => {
  it("chấp nhận input tối thiểu hợp lệ (chỉ name + content bắt buộc)", () => {
    const result = createUserSkillSchema.safeParse({
      name: "invoice-formatter",
      content: "Luôn dùng VNĐ.",
    });
    expect(result.success).toBe(true);
  });

  it("chấp nhận đầy đủ field", () => {
    const result = createUserSkillSchema.safeParse({
      name: "invoice-formatter",
      description: "Format hoá đơn",
      when_to_use: "Khi user yêu cầu xuất hoá đơn",
      content: "Luôn dùng VNĐ.",
      triggers: ["hoá đơn", "invoice"],
    });
    expect(result.success).toBe(true);
  });

  it("từ chối name rỗng", () => {
    const result = createUserSkillSchema.safeParse({ name: "", content: "x" });
    expect(result.success).toBe(false);
  });

  it("từ chối name chứa ký tự không hợp lệ", () => {
    const result = createUserSkillSchema.safeParse({
      name: "invoice formatter!!",
      content: "x",
    });
    expect(result.success).toBe(false);
  });

  it("từ chối thiếu content (bắt buộc)", () => {
    const result = createUserSkillSchema.safeParse({ name: "my-skill" });
    expect(result.success).toBe(false);
  });

  it("từ chối content rỗng", () => {
    const result = createUserSkillSchema.safeParse({
      name: "my-skill",
      content: "",
    });
    expect(result.success).toBe(false);
  });

  it("từ chối content dài hơn 10.000 ký tự (chặn nhồi prompt quá lớn)", () => {
    const result = createUserSkillSchema.safeParse({
      name: "my-skill",
      content: "a".repeat(10001),
    });
    expect(result.success).toBe(false);
  });

  it("từ chối description dài hơn 500 ký tự", () => {
    const result = createUserSkillSchema.safeParse({
      name: "my-skill",
      content: "x",
      description: "a".repeat(501),
    });
    expect(result.success).toBe(false);
  });

  it("từ chối when_to_use dài hơn 1000 ký tự", () => {
    const result = createUserSkillSchema.safeParse({
      name: "my-skill",
      content: "x",
      when_to_use: "a".repeat(1001),
    });
    expect(result.success).toBe(false);
  });

  it("từ chối quá 20 triggers", () => {
    const result = createUserSkillSchema.safeParse({
      name: "my-skill",
      content: "x",
      triggers: Array.from({ length: 21 }, (_, i) => `t${i}`),
    });
    expect(result.success).toBe(false);
  });

  it("từ chối 1 trigger dài hơn 100 ký tự", () => {
    const result = createUserSkillSchema.safeParse({
      name: "my-skill",
      content: "x",
      triggers: ["a".repeat(101)],
    });
    expect(result.success).toBe(false);
  });
});

describe("updateUserSkillSchema", () => {
  it("chấp nhận object rỗng (PATCH từng phần)", () => {
    const result = updateUserSkillSchema.safeParse({});
    expect(result.success).toBe(true);
  });

  it("chấp nhận chỉ cập nhật enabled", () => {
    const result = updateUserSkillSchema.safeParse({ enabled: true });
    expect(result.success).toBe(true);
  });

  it("từ chối content dài hơn 10.000 ký tự khi có truyền", () => {
    const result = updateUserSkillSchema.safeParse({
      content: "a".repeat(10001),
    });
    expect(result.success).toBe(false);
  });

  it("từ chối name không khớp regex khi có truyền", () => {
    const result = updateUserSkillSchema.safeParse({ name: "tên có dấu cách" });
    expect(result.success).toBe(false);
  });
});

describe("toggleSkillSchema", () => {
  it("chấp nhận enabled=true", () => {
    expect(toggleSkillSchema.safeParse({ enabled: true }).success).toBe(true);
  });

  it("chấp nhận enabled=false", () => {
    expect(toggleSkillSchema.safeParse({ enabled: false }).success).toBe(true);
  });

  it("từ chối thiếu enabled", () => {
    expect(toggleSkillSchema.safeParse({}).success).toBe(false);
  });

  it("từ chối enabled không phải boolean (vd string 'true')", () => {
    expect(toggleSkillSchema.safeParse({ enabled: "true" }).success).toBe(
      false,
    );
  });
});
