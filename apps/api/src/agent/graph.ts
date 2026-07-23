import {
  StateGraph,
  MessagesAnnotation,
  END,
  START,
} from "@langchain/langgraph";
import { ToolNode } from "@langchain/langgraph/prebuilt";
import { SystemMessage } from "@langchain/core/messages";
import { lcTools } from "./lc-tools";
import { createAgentModel } from "./model";

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
  "- listDocuments: đếm/liệt kê các tài liệu đã nạp (kèm documentId).",
  "- readDocument: đọc toàn bộ một tài liệu theo documentId (lấy từ listDocuments/ragSearch).",
  "- createTask/listTasks/updateTask/deleteTask: quản lý task.",
  "",
  "Có thể kết hợp nhiều bước: ví dụ tìm trong tài liệu rồi tạo task.",
  "Trả lời rõ ràng bằng tiếng Việt. Khi dùng ragSearch, hãy dẫn nguồn (source).",
].join("\n");

const model = createAgentModel();

const toolNode = new ToolNode(lcTools);

async function agentNode(state: typeof MessagesAnnotation.State) {
  const response = await model.invoke([
    new SystemMessage(SYSTEM_PROMPT),
    ...state.messages,
  ]);
  return { messages: [response] };
}

// Quyết định đi tiếp: có tool_calls → "tools", không → END
function shouldContinue(state: typeof MessagesAnnotation.State) {
  const last = state.messages[state.messages.length - 1] as any;
  return last.tool_calls?.length ? "tools" : END;
}

export const agentGraph = new StateGraph(MessagesAnnotation)
  .addNode("agent", agentNode)
  .addNode("tools", toolNode)
  .addEdge(START, "agent")
  .addConditionalEdges("agent", shouldContinue, { tools: "tools", [END]: END })
  .addEdge("tools", "agent")
  .compile();
