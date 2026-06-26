import { claude, CLAUDE_MODEL } from "../../lib/claude";
import { MessageRole } from "../../schemas/message";

type ChatMessage = {
  role: MessageRole;
  content: string;
};

const SYSTEM_PROMPT =
  "Bạn là một trợ lý AI hữu ích, trả lời ngắn gọn, rõ ràng bằng tiếng Việt.";

const toAnthropicMessages = (history: ChatMessage[]) =>
  history
    .filter(
      (message) => message.role === "user" || message.role === "assistant",
    )
    .map((message) => ({
      role: message.role as "user" | "assistant",
      content: message.content,
    }));

export const generateReply = async (
  history: ChatMessage[],
): Promise<string> => {
  const response = await claude.messages.create({
    model: CLAUDE_MODEL,
    max_tokens: 1024,
    system: SYSTEM_PROMPT,
    messages: toAnthropicMessages(history),
  });

  const textBlock = response.content.find((b) => b.type === "text");
  return textBlock && textBlock.type === "text" ? textBlock.text : "";
};

export { toAnthropicMessages, SYSTEM_PROMPT };
