import { HumanMessage, AIMessage } from "@langchain/core/messages";
import { agentGraph } from "./graph";

export type AgentEvent =
  | { type: "text"; text: string }
  | { type: "tool_start"; name: string }
  | { type: "tool_end"; name: string };

// Map một event của LangGraph streamEvents → AgentEvent (hoặc null nếu bỏ qua)
export function mapGraphEvent(ev: any): AgentEvent | null {
  if (ev.event === "on_chat_model_stream") {
    const content = ev.data?.chunk?.content;
    const text =
      typeof content === "string"
        ? content
        : Array.isArray(content)
          ? content
              .map((c: any) => (typeof c === "string" ? c : (c.text ?? "")))
              .join("")
          : "";
    return text ? { type: "text", text } : null;
  }
  if (ev.event === "on_tool_start")
    return { type: "tool_start", name: ev.name };
  if (ev.event === "on_tool_end") return { type: "tool_end", name: ev.name };
  return null;
}

// Chuyển lịch sử DB → LangChain messages
function toLcMessages(history: { role: string; content: string }[]) {
  return history.map((m) =>
    m.role === "assistant"
      ? new AIMessage(m.content)
      : new HumanMessage(m.content),
  );
}

export async function* runGraph(
  history: { role: string; content: string }[],
): AsyncGenerator<AgentEvent, string> {
  const stream = agentGraph.streamEvents(
    { messages: toLcMessages(history) },
    { version: "v2" },
  );

  let finalText = "";
  for await (const ev of stream) {
    const mapped = mapGraphEvent(ev);
    if (!mapped) continue;
    if (mapped.type === "text") finalText += mapped.text;
    yield mapped;
  }
  return finalText;
}
