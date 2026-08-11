// =============================================================================
// Agent — barrel exports from all sub-modules.
// =============================================================================

// Client boundary (Fastify ↔ agent runtime)
export {
  createAgentClient,
  langGraphAgentClient,
  goAgentClient,
  checkGoAgentHealth,
} from "./client/index";
export type {
  AgentClient,
  AgentMessage,
  AgentStreamOptions,
} from "./client/index";

// Graph (LangGraph definition + runner + event types)
export { runGraph, mapGraphEvent, agentGraph } from "./graph/index";
export type { AgentEvent } from "./graph/index";

// Tools (RAG + task management)
export { toolDefinitions, getTool, lcTools } from "./tools/index";
