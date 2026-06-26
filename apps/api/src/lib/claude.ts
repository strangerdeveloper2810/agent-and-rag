import Anthropic from "@anthropic-ai/sdk";
import { config } from "../config";

export const claude = new Anthropic({
  apiKey: config.ANTHROPIC_API_KEY,
});

export const CLAUDE_MODEL = config.CLAUDE_MODEL;
