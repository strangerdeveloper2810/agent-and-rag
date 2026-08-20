import { describe, it, expect, vi } from "vitest";
import type { Collection } from "mongodb";
import { createDocumentRepository } from "./documents.repository";
import type { DocChunkDoc, DocVersionDoc } from "../../../lib/collections";

// User không có nhu cầu xem tri thức agent-go tự học (documentId "learned-*",
// xem learner.go) trong màn hình Documents — nó phải bị loại khỏi listDocuments
// dù nằm chung collection `documents` với file user upload.
describe("createDocumentRepository — listDocuments loại trừ tri thức tự học", () => {
  it("$match loại bỏ documentId có prefix learned-", async () => {
    const fakeDocs = {
      aggregate: vi
        .fn()
        .mockReturnValue({ toArray: vi.fn().mockResolvedValue([]) }),
    };
    const repo = createDocumentRepository(
      () => fakeDocs as unknown as Collection<DocChunkDoc>,
      () => ({}) as unknown as Collection<DocVersionDoc>,
    );

    await repo.listDocuments("tenant-a");

    expect(fakeDocs.aggregate).toHaveBeenCalledWith(
      expect.arrayContaining([
        {
          $match: { tenantId: "tenant-a", documentId: { $not: /^learned-/ } },
        },
      ]),
    );
  });
});
