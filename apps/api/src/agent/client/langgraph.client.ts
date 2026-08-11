import { runGraph } from "../graph/index";
import type { AgentClient } from "./agent-client.interface";

// =============================================================================
// LangGraph (in-process) — legacy, giữ nguyên.
// =============================================================================

export const langGraphAgentClient: AgentClient = {
  stream(history, opts) {
    return runGraph(history, opts?.signal);
  },
};
