import Anthropic from "@anthropic-ai/sdk";
import { claude, CLAUDE_MODEL } from "../lib/claude";
import { toolDefinitions, getTool } from "./tools";

const SYSTEM_PROMPT = [
  "Bạn là một trợ lý AI có thể tra cứu tài liệu (RAG) và quản lý task.",
  "",
  "QUY TẮC QUAN TRỌNG:",
  "- TUYỆT ĐỐI KHÔNG bịa đặt thông tin (tên công ty, số liệu, ngày tháng, sự kiện...). Chỉ trả lời dựa trên ngữ cảnh hội thoại hoặc kết quả tool.",
  "- Nếu câu hỏi liên quan đến NỘI DUNG tài liệu, hãy GỌI LẠI ragSearch hoặc readDocument để lấy dữ liệu mới ở MỖI lượt — đừng trả lời từ trí nhớ, vì nội dung tài liệu KHÔNG được giữ lại giữa các lượt hội thoại.",
  "- Nếu không đủ thông tin để trả lời, hãy nói rõ bạn chưa có dữ liệu / cần hỏi lại, thay vì đoán.",
  "",
  "Công cụ:",
  "- ragSearch: tìm thông tin trong nội dung tài liệu.",
  "- listDocuments: đếm/liệt kê các tài liệu đã nạp.",
  "- readDocument: đọc toàn bộ một tài liệu theo tên file.",
  "- createTask/listTasks/updateTask/deleteTask: quản lý task.",
  "",
  "Trả lời rõ ràng bằng tiếng Việt. Khi dùng ragSearch, hãy dẫn nguồn (source).",
].join("\n");

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
  // Bỏ message rỗng (tránh lỗi API "content must be non-empty" + nhiễu ngữ cảnh)
  const messages: Anthropic.MessageParam[] = history
    .filter((m) => m.content && m.content.trim().length > 0)
    .map((m) => ({ role: m.role, content: m.content }));

  let finalText = "";

  // Giới hạn số vòng để tránh loop vô hạn (an toàn)
  for (let step = 0; step < 8; step++) {
    const res = await claude.messages.create({
      model: CLAUDE_MODEL,
      max_tokens: 4096, // đủ dài để câu trả lời không bị cắt cụt
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
