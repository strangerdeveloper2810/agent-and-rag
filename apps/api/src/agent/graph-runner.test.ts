import { describe, it, expect } from "vitest";
import { mapGraphEvent } from "./graph-runner";

describe("mapGraphEvent", () => {
  it("maps chat model token", () => {
    const out = mapGraphEvent({
      event: "on_chat_model_stream",
      data: { chunk: { content: "xin chào" } },
    });
    expect(out).toEqual({ type: "text", text: "xin chào" });
  });
  it("maps tool start", () => {
    const out = mapGraphEvent({
      event: "on_tool_start",
      name: "ragSearch",
      data: {},
    });
    expect(out).toEqual({ type: "tool_start", name: "ragSearch" });
  });
  it("returns null for irrelevant events", () => {
    expect(mapGraphEvent({ event: "on_chain_start", data: {} })).toBeNull();
  });
});
