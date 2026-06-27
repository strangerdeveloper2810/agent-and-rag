import Anthropic from "@anthropic-ai/sdk";
import { claude, CLAUDE_MODEL } from "../lib/claude";
import { toolDefinitions, getTool } from "./tools";

const SYSTEM_PROMPT =
  "Bạn là một trợ lý AI có thể tra cứu tài liệu và quản lý task. " +
  "Khi cần thông tin TRONG nội dung tài liệu, dùng tool ragSearch. " +
  "Khi người dùng hỏi có bao nhiêu/những tài liệu nào, dùng tool listDocuments. " +
  "Khi người dùng muốn tạo/sửa/xem/xóa task, dùng các tool task tương ứng. " +
  "Trả lời ngắn gọn, rõ ràng bằng tiếng Việt. Nếu dùng ragSearch, hãy dẫn nguồn (source).";

export type AgentEvent =
  | { type: "text"; text: string }
  | { type: "tool_start"; name: string }
  | { type: "tool_end"; name: string };

/**
 * Agent loop thủ công (vòng reason → act → observe).
 *
 * Mỗi vòng:
 *  1) Gửi messages + danh sách tool cho Claude
 *  2) Phát text Claude trả ra ngoài (yield)
 *  3) Nếu Claude KHÔNG gọi tool (stop_reason !== "tool_use") → xong
 *  4) Nếu CÓ gọi tool: chạy execute từng tool, đẩy kết quả lại Claude, lặp tiếp
 *
 * Dùng async generator để vừa chạy vừa stream event (text + báo gọi tool) về caller.
 * Giá trị return cuối cùng = toàn bộ text đã trả (để lưu vào DB).
 */
export async function* runAgent(
  history: { role: "user" | "assistant"; content: string }[],
): AsyncGenerator<AgentEvent, string> {
  const messages: Anthropic.MessageParam[] = history.map((m) => ({
    role: m.role,
    content: m.content,
  }));

  let finalText = "";

  // Giới hạn số vòng để tránh loop vô hạn (an toàn)
  for (let step = 0; step < 8; step++) {
    const res = await claude.messages.create({
      model: CLAUDE_MODEL,
      max_tokens: 1024,
      system: SYSTEM_PROMPT,
      tools: toolDefinitions as Anthropic.Tool[],
      messages,
    });

    // Phát phần text Claude nói ra (nếu có)
    for (const block of res.content) {
      if (block.type === "text") {
        finalText += block.text;
        yield { type: "text", text: block.text };
      }
    }

    // Claude không yêu cầu gọi tool → kết thúc loop
    if (res.stop_reason !== "tool_use") {
      return finalText;
    }

    // Lưu nguyên phản hồi assistant (gồm các block tool_use) vào lịch sử
    messages.push({ role: "assistant", content: res.content });

    // Chạy từng tool Claude yêu cầu, gom kết quả lại
    const toolResults: Anthropic.ToolResultBlockParam[] = [];
    for (const block of res.content) {
      if (block.type !== "tool_use") continue;
      yield { type: "tool_start", name: block.name };
      const tool = getTool(block.name);
      let result: unknown;
      try {
        result = tool
          ? await tool.execute(block.input)
          : { error: `Unknown tool ${block.name}` };
      } catch (err) {
        result = { error: String(err) };
      }
      yield { type: "tool_end", name: block.name };
      toolResults.push({
        type: "tool_result",
        tool_use_id: block.id,
        content: JSON.stringify(result),
      });
    }

    // Gửi kết quả tool lại cho Claude (đóng vai "user") để nó trả lời tiếp
    messages.push({ role: "user", content: toolResults });
  }

  return finalText;
}
