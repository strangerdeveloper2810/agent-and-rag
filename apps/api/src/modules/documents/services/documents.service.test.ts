import { describe, it, expect, vi, beforeEach } from "vitest";

// Mock các phụ thuộc I/O để test thuần logic orchestration của service.
vi.mock("../chunk", () => ({
  chunkText: vi.fn(async (text: string) => [text]),
}));
vi.mock("../../../lib/voyage", () => ({ embedBatched: vi.fn() }));
vi.mock("../repositories", () => ({
  getCurrentVersion: vi.fn(),
  archiveCurrentVersion: vi.fn(),
  insertChunks: vi.fn(),
  listDocuments: vi.fn(),
  getVersions: vi.fn(),
  getVersionContent: vi.fn(),
  deleteDocument: vi.fn(),
}));

import {
  buildChunkDocs,
  updateDocument,
  toUploadResult,
} from "./documents.service";
import * as repo from "../repositories";
import * as voyage from "../../../lib/voyage";
import { NotFoundError } from "../../../lib/errors";

describe("buildChunkDocs", () => {
  const now = new Date("2026-06-29T00:00:00Z");

  it("gắn tenantId, documentId, version và chunkIndex cho mỗi chunk", () => {
    const docs = buildChunkDocs(
      "tenant-1",
      "doc-1",
      "test.txt",
      2,
      ["chunk a", "chunk b"],
      [[0.1], [0.2]],
      now,
    );

    expect(docs).toHaveLength(2);
    expect(docs[0]).toEqual({
      tenantId: "tenant-1",
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
    expect(
      buildChunkDocs("tenant-1", "doc-1", "x.txt", 1, [], [], now),
    ).toEqual([]);
  });
});

describe("updateDocument", () => {
  beforeEach(() => vi.clearAllMocks());

  it("ném NotFoundError khi tài liệu không tồn tại", async () => {
    vi.mocked(repo.getCurrentVersion).mockResolvedValue(null);

    await expect(
      updateDocument("tenant-1", "missing", "x.txt", "nội dung"),
    ).rejects.toBeInstanceOf(NotFoundError);
    expect(repo.archiveCurrentVersion).not.toHaveBeenCalled();
  });

  it("KHÔNG archive/xóa bản cũ khi embed lỗi (chống mất dữ liệu)", async () => {
    vi.mocked(repo.getCurrentVersion).mockResolvedValue({
      documentId: "d1",
      version: 2,
      source: "x.txt",
      content: "nội dung cũ",
    });
    vi.mocked(voyage.embedBatched).mockRejectedValue(new Error("Voyage 429"));

    await expect(
      updateDocument("tenant-1", "d1", "x.txt", "nội dung mới"),
    ).rejects.toThrow();
    // Điểm mấu chốt: bản cũ KHÔNG bị đụng tới khi embed thất bại.
    expect(repo.archiveCurrentVersion).not.toHaveBeenCalled();
    expect(repo.insertChunks).not.toHaveBeenCalled();
  });

  it("archive TRƯỚC rồi insert bản mới khi embed thành công", async () => {
    vi.mocked(repo.getCurrentVersion).mockResolvedValue({
      documentId: "d1",
      version: 2,
      source: "x.txt",
      content: "nội dung cũ",
    });
    vi.mocked(voyage.embedBatched).mockResolvedValue([[0.1]]);

    const order: string[] = [];
    vi.mocked(repo.archiveCurrentVersion).mockImplementation(async () => {
      order.push("archive");
      return 2;
    });
    vi.mocked(repo.insertChunks).mockImplementation(async () => {
      order.push("insert");
    });

    const res = await updateDocument("tenant-1", "d1", "x.txt", "nội dung mới");

    expect(order).toEqual(["archive", "insert"]);
    expect(res.version).toBe(3);
    expect(res.chunks).toBe(1);
  });
});

describe("toUploadResult", () => {
  it("fulfilled → ok:true kèm dữ liệu file", () => {
    const r = toUploadResult("a.txt", {
      status: "fulfilled",
      value: { documentId: "d1", source: "a.txt", version: 1, chunks: 3 },
    });
    expect(r).toEqual({
      filename: "a.txt",
      ok: true,
      documentId: "d1",
      source: "a.txt",
      version: 1,
      chunks: 3,
    });
  });

  it("rejected (Error) → ok:false kèm message", () => {
    const r = toUploadResult("b.pdf", {
      status: "rejected",
      reason: new Error("PDF rỗng"),
    });
    expect(r).toEqual({ filename: "b.pdf", ok: false, error: "PDF rỗng" });
  });

  it("rejected (non-Error) → String(reason)", () => {
    const r = toUploadResult("c.txt", {
      status: "rejected",
      reason: "boom",
    });
    expect(r).toEqual({ filename: "c.txt", ok: false, error: "boom" });
  });
});
