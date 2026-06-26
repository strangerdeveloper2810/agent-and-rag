import { describe, it, expect } from "vitest";
import { toAnthropicMessages } from "./chat.service";

describe("toAnthropicMessages", () => {
  it("filters out tool messages and maps roles", () => {
    const out = toAnthropicMessages([
      { role: "user", content: "hi" },
      { role: "tool", content: "ignored" },
      { role: "assistant", content: "hello" },
    ]);
    expect(out).toEqual([
      { role: "user", content: "hi" },
      { role: "assistant", content: "hello" },
    ]);
  });
});
