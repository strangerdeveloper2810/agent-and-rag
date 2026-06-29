import { describe, it, expect } from "vitest";
import { buildChunkDocs } from "./documents.service";

describe("buildChunkDocs", () => {
  const now = new Date("2026-06-29T00:00:00Z");

  it("gắn documentId, version và chunkIndex cho mỗi chunk", () => {
    const docs = buildChunkDocs(
      "doc-1",
      "test.txt",
      2,
      ["chunk a", "chunk b"],
      [[0.1], [0.2]],
      now,
    );

    expect(docs).toHaveLength(2);
    expect(docs[0]).toEqual({
      documentId: "doc-1",
      source: "test.txt",
      version: 2,
      chunkIndex: 0,
      text: "chunk a",
      embedding: [0.1],
      createdAt: now,
    });
    expect(docs[1].chunkIndex).toBe(1);
    expect(docs[1].embedding).toEqual([0.2]);
  });

  it("trả mảng rỗng khi không có chunk", () => {
    expect(buildChunkDocs("doc-1", "x.txt", 1, [], [], now)).toEqual([]);
  });
});
