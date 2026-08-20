import { describe, it, expect, vi } from "vitest";
import type { Collection } from "mongodb";

// listConversations/getMessages short-circuit về [] khi Mongo chưa connect
// (không có Mongo thật trong test env) — ép isMongoConnected() = true để bài
// test chạm được vào query thật (fake collection) mà ta cần assert filter.
vi.mock("../../../lib/mongo", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../../lib/mongo")>()),
  isMongoConnected: () => true,
}));

import {
  buildConversationDocs,
  createChatRepository,
} from "./chat.repository";
import type { ConversationDoc, MessageDoc } from "../../../lib/collections";

describe("buildConversationDoc", () => {
  it("creates doc with tenantId, title and timestamps", () => {
    const now = new Date("2026-06-25T00:00:00Z");
    const doc = buildConversationDocs(
      "tenant-1",
      "Xin chào thế giới này là tiêu đề dài",
      now,
    );
    expect(doc.tenantId).toBe("tenant-1");
    expect(doc.title.length).toBeLessThanOrEqual(50);
    expect(doc.createdAt).toEqual(now);
    expect(doc.updatedAt).toEqual(now);
  });
});

// Bug: user tenant A từng thấy được hội thoại/tin nhắn của tenant B vì mọi
// query trong repository này không lọc theo tenantId. Các test dưới đây khoá
// lại hành vi ĐÚNG: mọi query/insert phải mang tenantId, không thể đọc/xoá
// chéo dữ liệu của tenant khác dù biết đúng conversationId.
describe("createChatRepository — tenant isolation", () => {
  const fakeCollections = () => {
    const fakeConversations = {
      find: vi.fn().mockReturnValue({
        sort: vi.fn().mockReturnValue({
          toArray: vi.fn().mockResolvedValue([]),
        }),
      }),
      insertOne: vi.fn().mockResolvedValue({ insertedId: "conv-id" }),
      updateOne: vi.fn().mockResolvedValue({}),
      deleteOne: vi.fn().mockResolvedValue({}),
    };
    const fakeMessages = {
      find: vi.fn().mockReturnValue({
        sort: vi.fn().mockReturnValue({
          toArray: vi.fn().mockResolvedValue([]),
          limit: vi.fn().mockReturnValue({
            toArray: vi.fn().mockResolvedValue([]),
          }),
        }),
      }),
      insertOne: vi.fn().mockResolvedValue({ insertedId: "msg-id" }),
      deleteMany: vi.fn().mockResolvedValue({}),
    };
    return { fakeConversations, fakeMessages };
  };

  it("listConversations lọc theo tenantId, không trả về hội thoại của tenant khác", async () => {
    const { fakeConversations, fakeMessages } = fakeCollections();
    const repo = createChatRepository(
      () => fakeConversations as unknown as Collection<ConversationDoc>,
      () => fakeMessages as unknown as Collection<MessageDoc>,
    );

    await repo.listConversations("tenant-a");

    expect(fakeConversations.find).toHaveBeenCalledWith({
      tenantId: "tenant-a",
    });
  });

  it("getMessages lọc theo tenantId + conversationId — không thể đọc tin nhắn của tenant khác dù biết đúng id", async () => {
    const { fakeConversations, fakeMessages } = fakeCollections();
    const repo = createChatRepository(
      () => fakeConversations as unknown as Collection<ConversationDoc>,
      () => fakeMessages as unknown as Collection<MessageDoc>,
    );

    await repo.getMessages("tenant-a", "conv-1");

    expect(fakeMessages.find).toHaveBeenCalledWith({
      tenantId: "tenant-a",
      conversationId: "conv-1",
    });
  });

  it("createConversation lưu kèm tenantId vào document", async () => {
    const { fakeConversations, fakeMessages } = fakeCollections();
    const repo = createChatRepository(
      () => fakeConversations as unknown as Collection<ConversationDoc>,
      () => fakeMessages as unknown as Collection<MessageDoc>,
    );

    await repo.createConversation("tenant-a", "hello");

    expect(fakeConversations.insertOne).toHaveBeenCalledWith(
      expect.objectContaining({ tenantId: "tenant-a" }),
    );
  });

  it("addMessage lưu kèm tenantId + chỉ bump updatedAt của đúng tenant", async () => {
    const { fakeConversations, fakeMessages } = fakeCollections();
    const repo = createChatRepository(
      () => fakeConversations as unknown as Collection<ConversationDoc>,
      () => fakeMessages as unknown as Collection<MessageDoc>,
    );

    const conversationId = "64b7f0000000000000000000";
    await repo.addMessage("tenant-a", conversationId, "user", "hi");

    expect(fakeMessages.insertOne).toHaveBeenCalledWith(
      expect.objectContaining({ tenantId: "tenant-a", conversationId }),
    );
    expect(fakeConversations.updateOne).toHaveBeenCalledWith(
      expect.objectContaining({ tenantId: "tenant-a" }),
      expect.anything(),
    );
  });

  it("deleteConversation chỉ xoá messages/conversation thuộc đúng tenantId", async () => {
    const { fakeConversations, fakeMessages } = fakeCollections();
    const repo = createChatRepository(
      () => fakeConversations as unknown as Collection<ConversationDoc>,
      () => fakeMessages as unknown as Collection<MessageDoc>,
    );

    await repo.deleteConversation(
      "tenant-a",
      "64b7f0000000000000000000",
    );

    expect(fakeMessages.deleteMany).toHaveBeenCalledWith({
      tenantId: "tenant-a",
      conversationId: "64b7f0000000000000000000",
    });
    expect(fakeConversations.deleteOne).toHaveBeenCalledWith(
      expect.objectContaining({ tenantId: "tenant-a" }),
    );
  });
});
