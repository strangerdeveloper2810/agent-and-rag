import { config } from "../../config";
import type { AgentClient } from "./agent-client.interface";
import { langGraphAgentClient } from "./langgraph.client";
import { goAgentClient } from "./go-agent.client";

// ----- Re-exports -----

export type { AgentMessage, AgentStreamOptions, AgentClient } from "./agent-client.interface";
export { langGraphAgentClient } from "./langgraph.client";
export { goAgentClient, checkGoAgentHealth } from "./go-agent.client";

// =============================================================================
// Factory
// =============================================================================

/**
 * Chọn agent backend theo `config.AGENT_BACKEND`:
 * - `"langgraph"` (mặc định): LangGraph in-process (legacy).
 * - `"go"`: Proxy HTTP+SSE sang service agent-go (khuyến nghị).
 */
export function createAgentClient(): AgentClient {
  switch (config.AGENT_BACKEND) {
    case "go":
      return goAgentClient;
    case "langgraph":
    default:
      return langGraphAgentClient;
  }
}
