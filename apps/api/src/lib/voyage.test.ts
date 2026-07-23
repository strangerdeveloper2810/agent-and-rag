import { describe, it, expect } from "vitest";
import { buildEmbeddingRequest, batchTexts } from "./voyage";

describe("buildEmbeddingRequest", () => {
  it("builds body for documents", () => {
    const body = buildEmbeddingRequest(["a", "b"], "document");
    expect(body.input).toEqual(["a", "b"]);
    expect(body.input_type).toBe("document");
    expect(body.model).toBe("voyage-3");
  });

  it("builds body for query", () => {
    const body = buildEmbeddingRequest(["hỏi gì đó"], "query");
    expect(body.input_type).toBe("query");
  });
});

describe("batchTexts", () => {
  it("chia đúng batch khi vượt kích thước", () => {
    const batches = batchTexts(["a", "b", "c", "d", "e"], 2);
    expect(batches).toEqual([["a", "b"], ["c", "d"], ["e"]]);
  });

  it("một batch khi nhỏ hơn kích thước", () => {
    expect(batchTexts(["a", "b"], 96)).toEqual([["a", "b"]]);
  });

  it("mảng rỗng → không có batch", () => {
    expect(batchTexts([], 96)).toEqual([]);
  });

  it("ném lỗi khi batchSize < 1", () => {
    expect(() => batchTexts(["a"], 0)).toThrow();
  });
});
