import { describe, it, expect } from "vitest";
import { buildEmbeddingRequest } from "./voyage";

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
