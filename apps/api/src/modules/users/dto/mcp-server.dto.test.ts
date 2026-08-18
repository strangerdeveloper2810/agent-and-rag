import { describe, it, expect } from "vitest";
import { createMcpServerSchema, updateMcpServerSchema } from "./mcp-server.dto";

// Các test này bảo vệ ranh giới INPUT của endpoint MCP server: nếu ai đó nới
// lỏng regex/độ dài trong tương lai mà không cố ý, test dưới đây phải đỏ.

describe("createMcpServerSchema", () => {
  it("chấp nhận input hợp lệ", () => {
    const result = createMcpServerSchema.safeParse({
      name: "my-server_01",
      transport: "http",
      url: "https://mcp.example.com/sse",
      auth_token: "secret",
    });
    expect(result.success).toBe(true);
  });

  it("transport mặc định là 'http' khi không truyền", () => {
    const result = createMcpServerSchema.safeParse({
      name: "my-server",
      url: "https://mcp.example.com/sse",
    });
    expect(result.success).toBe(true);
    if (result.success) {
      expect(result.data.transport).toBe("http");
    }
  });

  it("chấp nhận transport = 'sse' (legacy, tương thích ngược)", () => {
    const result = createMcpServerSchema.safeParse({
      name: "my-server",
      transport: "sse",
      url: "https://mcp.example.com/sse",
    });
    expect(result.success).toBe(true);
    if (result.success) {
      expect(result.data.transport).toBe("sse");
    }
  });

  it("từ chối transport không hợp lệ (vd 'stdio', 'websocket')", () => {
    const result = createMcpServerSchema.safeParse({
      name: "my-server",
      transport: "stdio",
      url: "https://mcp.example.com/sse",
    });
    expect(result.success).toBe(false);
  });

  it("chấp nhận không có auth_token (optional)", () => {
    const result = createMcpServerSchema.safeParse({
      name: "my-server",
      url: "https://mcp.example.com/sse",
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

  it("từ chối auth_token dài hơn 4096 ký tự", () => {
    const result = createMcpServerSchema.safeParse({
      name: "my-server",
      url: "https://mcp.example.com/sse",
      auth_token: "a".repeat(4097),
    });
    expect(result.success).toBe(false);
  });

  it("chấp nhận auth_token = chuỗi rỗng (tương đương không có token)", () => {
    const result = createMcpServerSchema.safeParse({
      name: "my-server",
      url: "https://mcp.example.com/sse",
      auth_token: "",
    });
    expect(result.success).toBe(true);
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

  it("từ chối transport không hợp lệ khi có truyền", () => {
    const result = updateMcpServerSchema.safeParse({ transport: "stdio" });
    expect(result.success).toBe(false);
  });

  it("chấp nhận transport = 'http' hoặc 'sse' khi có truyền", () => {
    expect(updateMcpServerSchema.safeParse({ transport: "http" }).success).toBe(
      true,
    );
    expect(updateMcpServerSchema.safeParse({ transport: "sse" }).success).toBe(
      true,
    );
  });

  // auth_token: "" nghĩa là XOÁ token (semantics xử lý ở repository, không
  // phải ở DTO) — DTO chỉ cần CHẤP NHẬN chuỗi rỗng, không được reject.
  it("chấp nhận auth_token = chuỗi rỗng (ý nghĩa: xoá token hiện có)", () => {
    const result = updateMcpServerSchema.safeParse({ auth_token: "" });
    expect(result.success).toBe(true);
  });

  it("không truyền auth_token → field vắng mặt trong kết quả parse (giữ nguyên token cũ)", () => {
    const result = updateMcpServerSchema.safeParse({ name: "renamed" });
    expect(result.success).toBe(true);
    if (result.success) {
      expect("auth_token" in result.data).toBe(false);
    }
  });

  it("từ chối auth_token dài hơn 4096 ký tự", () => {
    const result = updateMcpServerSchema.safeParse({
      auth_token: "a".repeat(4097),
    });
    expect(result.success).toBe(false);
  });
});
