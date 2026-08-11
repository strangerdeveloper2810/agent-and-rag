import { useState } from "react";
import { WrenchIcon, CheckIcon, ChevronDownIcon } from "@app/ui";
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

/**
 * Resolves user-friendly display labels for agent tool names.
 *
 * @param name - Raw tool name string
 * @returns Human-readable label string
 */
const toolLabel = (name: string): string => TOOL_LABELS[name] ?? name;

export interface ToolCallCardProps {
  tool: ToolCallState;
}

/**
 * ToolCallCard component for rendering real-time agent tool executions and logs.
 */
export const ToolCallCard: React.FC<ToolCallCardProps> = ({ tool }) => {
  const [expanded, setExpanded] = useState(false);
  const isRunning = tool.status === "running";
  const isError = tool.status === "error";
  const hasDetail = tool.result || tool.error;

  const bgColor = isRunning
    ? "rgba(0, 240, 255, 0.04)"
    : isError
      ? "rgba(255, 71, 87, 0.06)"
      : "var(--bg-raised)";
  const borderColor = isRunning
    ? "rgba(0, 240, 255, 0.3)"
    : isError
      ? "rgba(255, 71, 87, 0.3)"
      : "var(--border)";

  return (
    <div
      className="my-2.5 rounded-2xl border px-3.5 py-2.5 transition-all duration-200 backdrop-blur-md shadow-sm"
      style={{ backgroundColor: bgColor, borderColor }}
    >
      <button
        type="button"
        onClick={() => hasDetail && setExpanded(!expanded)}
        className="flex w-full items-center gap-3 text-left group"
        aria-expanded={expanded}
        aria-label={`Tool: ${toolLabel(tool.name)} - ${tool.status}`}
      >
        <span
          className="flex h-7 w-7 shrink-0 items-center justify-center rounded-xl transition-transform group-hover:scale-105"
          style={{
            backgroundColor: isRunning
              ? "var(--accent-bg)"
              : isError
                ? "var(--danger-bg)"
                : "var(--success-bg)",
          }}
        >
          {isRunning ? (
            <WrenchIcon
              width={14}
              height={14}
              className="animate-spin-slow"
              style={{ color: "var(--accent)" }}
            />
          ) : isError ? (
            <span
              className="text-xs font-bold font-mono"
              style={{ color: "var(--danger)" }}
            >
              !
            </span>
          ) : (
            <CheckIcon
              width={14}
              height={14}
              style={{ color: "var(--success)" }}
            />
          )}
        </span>

        <div className="min-w-0 flex-1">
          <p
            className="truncate text-xs font-semibold tracking-wide"
            style={{ color: "var(--text)" }}
          >
            {toolLabel(tool.name)}
          </p>
          <div className="flex items-center gap-1.5 mt-0.5">
            <span
              className={`h-1.5 w-1.5 rounded-full ${isRunning ? "animate-pulse" : ""}`}
              style={{
                backgroundColor: isRunning
                  ? "var(--accent)"
                  : isError
                    ? "var(--danger)"
                    : "var(--success)",
              }}
            />
            <span
              className="text-[10px] font-mono tracking-wider uppercase"
              style={{
                color: isError ? "var(--danger)" : "var(--text-tertiary)",
              }}
            >
              {isRunning ? "Executing..." : isError ? "Failed" : "Completed"}
            </span>
          </div>
        </div>

        {hasDetail && (
          <span className="flex items-center gap-1 rounded-lg px-2 py-1 text-[10px] font-mono text-[var(--text-tertiary)] bg-black/10 group-hover:bg-black/20">
            <span>{expanded ? "Hide" : "Logs"}</span>
            <ChevronDownIcon
              width={12}
              height={12}
              style={{ color: "var(--text-tertiary)" }}
              className={`shrink-0 transition-transform duration-200 ${expanded ? "rotate-180" : ""}`}
            />
          </span>
        )}
      </button>

      {expanded && hasDetail && (
        <div
          className="mt-3 border-t pt-3 animate-fade-in"
          style={{ borderColor: "var(--border)" }}
        >
          {tool.result && (
            <div>
              <p
                className="mb-1 text-[9px] font-mono font-bold uppercase tracking-widest"
                style={{ color: "var(--accent)" }}
              >
                // Execution Result Output
              </p>
              <pre
                className="scroll-fine max-h-56 overflow-auto whitespace-pre-wrap rounded-xl px-3.5 py-2.5 text-[11px] font-mono leading-relaxed shadow-inner"
                style={{
                  color: "var(--text-secondary)",
                  backgroundColor: "var(--bg)",
                  border: "1px solid var(--border)",
                }}
              >
                {tool.result}
              </pre>
            </div>
          )}
          {tool.error && (
            <div>
              <p
                className="mb-1 text-[9px] font-mono font-bold uppercase tracking-widest"
                style={{ color: "var(--danger)" }}
              >
                // Error Log Output
              </p>
              <pre
                className="scroll-fine max-h-56 overflow-auto whitespace-pre-wrap rounded-xl px-3.5 py-2.5 text-[11px] font-mono leading-relaxed"
                style={{
                  color: "var(--danger)",
                  backgroundColor: "var(--danger-bg)",
                  border: "1px solid rgba(255, 71, 87, 0.2)",
                }}
              >
                {tool.error}
              </pre>
            </div>
          )}
        </div>
      )}
    </div>
  );
};

export default ToolCallCard;
