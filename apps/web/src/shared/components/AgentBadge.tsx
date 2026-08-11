import { AgentIcon, SparkIcon, BrainIcon, WrenchIcon } from "./icons";
import type { ComponentType, SVGProps } from "react";

type IconComponent = ComponentType<SVGProps<SVGSVGElement>>;

const AGENT_CONFIG: Record<string, { label: string; Icon: IconComponent }> = {
  general: { label: "Assistant", Icon: SparkIcon },
  code: { label: "Code Agent", Icon: WrenchIcon },
  research: { label: "Research Agent", Icon: BrainIcon },
};

export interface AgentBadgeProps {
  agent: string | null;
  message?: string;
}

/**
 * AgentBadge component for displaying active sub-agent badges (Code, Research, Assistant).
 */
export const AgentBadge: React.FC<AgentBadgeProps> = ({ agent, message }) => {
  if (!agent) return null;

  const config = AGENT_CONFIG[agent] ?? {
    label: agent,
    Icon: AgentIcon,
  };
  const { label, Icon } = config;

  return (
    <div
      className="inline-flex items-center gap-2 rounded-full px-3 py-1.5 text-[10px] font-medium animate-slide-up"
      style={{
        backgroundColor: "var(--accent-bg)",
        border: "1px solid rgba(0,240,255,0.3)",
      }}
      aria-label={message ?? label}
    >
      <span
        className="flex h-5 w-5 items-center justify-center rounded-full"
        style={{ color: "var(--accent)" }}
      >
        <Icon width={12} height={12} />
      </span>
      <span
        className="tracking-wider uppercase"
        style={{ color: "var(--accent)" }}
      >
        {message ?? label}
      </span>
    </div>
  );
};

export default AgentBadge;
