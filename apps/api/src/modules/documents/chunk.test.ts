import { describe, it, expect } from "vitest";
import { chunkText } from "./chunk";

describe("chunkText", () => {
  it("splits long text into multiple chunks", async () => {
    const text = "câu một. ".repeat(500);
    const chunks = await chunkText(text);
    expect(chunks.length).toBeGreaterThan(1);
    expect(chunks[0].length).toBeGreaterThan(0);
  });

  it("returns single chunk for short text", async () => {
    const chunks = await chunkText("ngắn thôi");
    expect(chunks).toEqual(["ngắn thôi"]);
  });
});
