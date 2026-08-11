import { agentGraph } from "./graph";

// In cấu trúc graph dạng Mermaid để học/visualize.
// Chạy: pnpm graph:print  → dán output vào https://mermaid.live
const repr = await agentGraph.getGraphAsync();
console.log(repr.drawMermaid());
