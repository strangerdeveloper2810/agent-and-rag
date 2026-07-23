import { ChatAnthropic } from "@langchain/anthropic";
import { ChatGoogleGenerativeAI } from "@langchain/google-genai";
import type { BaseChatModel } from "@langchain/core/language_models/chat_models";
import { config } from "../config";
import { lcTools } from "./lc-tools";

/**
 * Tạo chat model cho agent theo LLM_PROVIDER (anthropic | google).
 * Mặc định Anthropic (giữ nguyên hành vi cũ). Model trả về đã bind sẵn tools nên
 * graph không cần biết đang chạy provider nào — chỉ gọi .invoke().
 */
export function createAgentModel() {
  const base: BaseChatModel =
    config.LLM_PROVIDER === "google"
      ? new ChatGoogleGenerativeAI({
          apiKey: config.GOOGLE_API_KEY,
          model: config.GOOGLE_MODEL,
          maxOutputTokens: 4096,
        })
      : new ChatAnthropic({
          apiKey: config.ANTHROPIC_API_KEY,
          model: config.CLAUDE_MODEL,
          maxTokens: 4096,
        });

  return base.bindTools!(lcTools);
}
