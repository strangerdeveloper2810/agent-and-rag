import { AgentIcon, SparkIcon, BrainIcon, WrenchIcon } from "./icons";
import type { ComponentType, SVGProps } from "react";

type IconComponent = ComponentType<SVGProps<SVGSVGElement>>;

const AGENT_CONFIG: Record<
  string,
  { label: string; Icon: IconComponent; colorClass: string }
> = {
  general: {
    label: "Assistant",
    Icon: SparkIcon,
    colorClass: "text-gblue bg-gblue-soft",
  },
  code: {
    label: "Code Agent",
    Icon: WrenchIcon,
    colorClass: "text-emerald-600 bg-emerald-100 dark:text-emerald-400 dark:bg-emerald-900/30",
  },
  research: {
    label: "Research Agent",
    Icon: BrainIcon,
    colorClass: "text-purple-600 bg-purple-100 dark:text-purple-400 dark:bg-purple-900/30",
  },
};

interface AgentBadgeProps {
  agent: string | null;
  message?: string;
}

export default function AgentBadge({ agent, message }: AgentBadgeProps) {
  if (!agent) {
    // Default: generic agent badge when no specific agent is identified
    return null;
  }

  const config = AGENT_CONFIG[agent] ?? {
    label: agent,
    Icon: AgentIcon,
    colorClass: "text-ink-soft bg-subtle",
  };
  const { label, Icon, colorClass } = config;

  return (
    <div
      className="inline-flex items-center gap-2 rounded-full px-3 py-1.5 text-xs font-medium animate-slide-up"
      aria-label={message ?? label}
    >
      <span
        className={`flex h-6 w-6 items-center justify-center rounded-full ${colorClass}`}
      >
        <Icon width={14} height={14} />
      </span>
      <span className="text-ink-soft">
        {message ?? label}
      </span>
    </div>
  );
}
