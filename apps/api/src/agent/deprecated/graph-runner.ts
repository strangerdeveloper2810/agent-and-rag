import { HumanMessage, AIMessage } from "@langchain/core/messages";
import { agentGraph } from "./graph";

/**
 * AgentEvent — kiểu sự kiện thống nhất cho cả LangGraph (in-process) và Go agent (SSE).
 *
 * LangGraph chỉ phát: text, tool_start, tool_end.
 * Go agent phát thêm: step (node hiện tại), error (lỗi agent), done (kết thúc + token usage),
 * citation (RAG sources), memory (thao tác memory), interrupt (HITL).
 */
export type AgentEvent =
  | { type: "text"; text: string }
  | { type: "tool_start"; name: string }
  | { type: "tool_end"; name: string }
  | { type: "step"; node?: string }
  | { type: "error"; message?: string }
  | { type: "done"; agent?: string; tokens?: number }
  | { type: "citation"; text?: string }
  | { type: "memory"; message?: string }
  | { type: "interrupt"; name?: string; message?: string };

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
  signal?: AbortSignal,
): AsyncGenerator<AgentEvent, string> {
  const stream = agentGraph.streamEvents(
    { messages: toLcMessages(history) },
    // signal: hủy khi client ngắt. recursionLimit: chốt an toàn số vòng
    // agent↔tools (mặc định 25) → tránh loop tốn token và đứt SSE bất ngờ.
    { version: "v2", signal, recursionLimit: 12 },
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
