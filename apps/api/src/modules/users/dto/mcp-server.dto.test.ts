import { describe, it, expect } from "vitest";
import { createMcpServerSchema, updateMcpServerSchema } from "./mcp-server.dto";

// Các test này bảo vệ ranh giới INPUT của endpoint MCP server: nếu ai đó nới
// lỏng regex/độ dài trong tương lai mà không cố ý, test dưới đây phải đỏ.

describe("createMcpServerSchema", () => {
  it("chấp nhận input hợp lệ", () => {
    const result = createMcpServerSchema.safeParse({
      name: "my-server_01",
      url: "https://mcp.example.com/sse",
      api_key: "secret",
    });
    expect(result.success).toBe(true);
  });

  it("chấp nhận không có api_key (optional)", () => {
    const result = createMcpServerSchema.safeParse({
      name: "my-server",
      url: "https://mcp.example.com/sse",
    });
    expect(result.success).toBe(true);
  });

  it("chấp nhận api_key = null", () => {
    const result = createMcpServerSchema.safeParse({
      name: "my-server",
      url: "https://mcp.example.com/sse",
      api_key: null,
    });
    expect(result.success).toBe(true);
  });

  it("từ chối name rỗng", () => {
    const result = createMcpServerSchema.safeParse({
      name: "",
      url: "https://mcp.example.com/sse",
    });
    expect(result.success).toBe(false);
  });

  it("từ chối name chứa ký tự đặc biệt (chống injection tên dùng làm định danh)", () => {
    const result = createMcpServerSchema.safeParse({
      name: "server; DROP TABLE users;--",
      url: "https://mcp.example.com/sse",
    });
    expect(result.success).toBe(false);
  });

  it("từ chối name dài hơn 100 ký tự", () => {
    const result = createMcpServerSchema.safeParse({
      name: "a".repeat(101),
      url: "https://mcp.example.com/sse",
    });
    expect(result.success).toBe(false);
  });

  it("từ chối url không hợp lệ", () => {
    const result = createMcpServerSchema.safeParse({
      name: "my-server",
      url: "không-phải-url",
    });
    expect(result.success).toBe(false);
  });

  it("từ chối thiếu url", () => {
    const result = createMcpServerSchema.safeParse({ name: "my-server" });
    expect(result.success).toBe(false);
  });

  it("từ chối api_key dài hơn 500 ký tự", () => {
    const result = createMcpServerSchema.safeParse({
      name: "my-server",
      url: "https://mcp.example.com/sse",
      api_key: "a".repeat(501),
    });
    expect(result.success).toBe(false);
  });
});

describe("updateMcpServerSchema", () => {
  it("chấp nhận object rỗng (mọi field optional — PATCH từng phần)", () => {
    const result = updateMcpServerSchema.safeParse({});
    expect(result.success).toBe(true);
  });

  it("chấp nhận chỉ cập nhật enabled", () => {
    const result = updateMcpServerSchema.safeParse({ enabled: false });
    expect(result.success).toBe(true);
  });

  it("từ chối enabled không phải boolean", () => {
    const result = updateMcpServerSchema.safeParse({ enabled: "false" });
    expect(result.success).toBe(false);
  });

  it("từ chối url không hợp lệ khi có truyền", () => {
    const result = updateMcpServerSchema.safeParse({
      url: "không phải url hợp lệ",
    });
    expect(result.success).toBe(false);
  });

  it("từ chối name không khớp regex khi có truyền", () => {
    const result = updateMcpServerSchema.safeParse({ name: "có dấu cách" });
    expect(result.success).toBe(false);
  });
});
