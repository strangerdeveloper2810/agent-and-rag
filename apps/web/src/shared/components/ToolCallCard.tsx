import { useState } from "react";
import { WrenchIcon, CheckIcon, ChevronDownIcon } from "./icons";
import type { ToolCallState } from "@/modules/chat/chat.api";

const TOOL_LABELS: Record<string, string> = {
  ragSearch: "Searching documents",
  listDocuments: "Listing documents",
  readDocument: "Reading document",
  searchWeb: "Searching web",
  fetchUrl: "Fetching URL",
  createTask: "Creating task",
  listTasks: "Listing tasks",
  updateTask: "Updating task",
  deleteTask: "Deleting task",
  translate: "Translating",
  calendar: "Checking calendar",
  notes: "Managing notes",
};

function toolLabel(name: string): string {
  return TOOL_LABELS[name] ?? name;
}

interface ToolCallCardProps {
  tool: ToolCallState;
}

export default function ToolCallCard({ tool }: ToolCallCardProps) {
  const [expanded, setExpanded] = useState(false);
  const isRunning = tool.status === "running";
  const isError = tool.status === "error";
  const hasDetail = tool.result || tool.error;

  const bgColor = isRunning
    ? "rgba(0,240,255,0.06)"
    : isError
      ? "rgba(255,51,102,0.08)"
      : "transparent";
  const borderColor = isRunning
    ? "rgba(0,240,255,0.3)"
    : isError
      ? "rgba(255,51,102,0.3)"
      : "var(--cyber-border)";

  return (
    <div
      className="my-2 rounded-xl border px-4 py-3 transition-all duration-200"
      style={{ backgroundColor: bgColor, borderColor }}
    >
      <button
        type="button"
        onClick={() => hasDetail && setExpanded(!expanded)}
        className="flex w-full items-center gap-2.5 text-left"
        aria-expanded={expanded}
        aria-label={`Tool: ${toolLabel(tool.name)} - ${tool.status}`}
      >
        <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full">
          {isRunning ? (
            <WrenchIcon
              width={14}
              height={14}
              className="animate-spin-slow"
              style={{ color: "var(--cyber-primary)" }}
            />
          ) : isError ? (
            <span className="text-sm font-bold" style={{ color: "var(--cyber-error)" }}>
              !
            </span>
          ) : (
            <CheckIcon width={14} height={14} style={{ color: "var(--cyber-success)" }} />
          )}
        </span>

        <div className="min-w-0 flex-1">
          <p className="truncate text-xs font-medium" style={{ color: "var(--cyber-text)" }}>
            {toolLabel(tool.name)}
          </p>
          <p className="text-[10px]" style={{ color: isError ? "var(--cyber-error)" : "var(--cyber-muted)" }}>
            {isRunning ? "Running..." : isError ? "Failed" : "Completed"}
          </p>
        </div>

        {hasDetail && (
          <ChevronDownIcon
            width={14}
            height={14}
            style={{ color: "var(--cyber-faint)" }}
            className={`shrink-0 transition-transform ${expanded ? "rotate-180" : ""}`}
          />
        )}
      </button>

      {expanded && hasDetail && (
        <div className="mt-3 border-t pt-3" style={{ borderColor: "var(--cyber-border)" }}>
          {tool.result && (
            <div>
              <p className="mb-1 text-[10px] font-medium uppercase tracking-wider" style={{ color: "var(--cyber-faint)" }}>
                Result
              </p>
              <pre
                className="scroll-fine max-h-48 overflow-auto whitespace-pre-wrap rounded-lg px-3 py-2 text-[11px] leading-relaxed"
                style={{
                  color: "var(--cyber-muted)",
                  backgroundColor: "var(--cyber-bg)",
                  border: "1px solid var(--cyber-border)",
                }}
              >
                {tool.result}
              </pre>
            </div>
          )}
          {tool.error && (
            <div>
              <p className="mb-1 text-[10px] font-medium uppercase tracking-wider" style={{ color: "var(--cyber-error)" }}>
                Error
              </p>
              <p className="text-xs" style={{ color: "var(--cyber-error)" }}>
                {tool.error}
              </p>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
