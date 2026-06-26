import { describe, it, expect } from "vitest";
import { buildConversationDocs } from "./chat.repository";

describe("buildConversationDoc", () => {
  it("creates doc with title and timestamps", () => {
    const now = new Date("2026-06-25T00:00:00Z");
    const doc = buildConversationDocs(
      "Xin chào thế giới này là tiêu đề dài",
      now,
    );
    expect(doc.title.length).toBeLessThanOrEqual(50);
    expect(doc.createdAt).toEqual(now);
    expect(doc.updatedAt).toEqual(now);
  });
});
