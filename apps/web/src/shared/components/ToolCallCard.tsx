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

  return (
    <div
      className={`my-2 rounded-xl border px-4 py-3 transition-colors ${
        isRunning
          ? "border-gblue-soft bg-gblue-soft/30 animate-pulse"
          : isError
            ? "border-red-200 bg-red-50 dark:border-red-900/50 dark:bg-red-900/20"
            : "border-line bg-subtle dark:bg-[var(--color-tool-card-bg)] dark:border-[var(--color-tool-card-border)]"
      }`}
    >
      <button
        type="button"
        onClick={() => hasDetail && setExpanded(!expanded)}
        className="flex w-full items-center gap-2.5 text-left"
        aria-expanded={expanded}
        aria-label={`Tool: ${toolLabel(tool.name)} - ${tool.status}`}
      >
        {/* Status icon */}
        <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full">
          {isRunning ? (
            <WrenchIcon
              width={15}
              height={15}
              className="animate-spin-slow text-gblue"
            />
          ) : isError ? (
            <span className="text-sm font-bold text-red-500">!</span>
          ) : (
            <CheckIcon width={15} height={15} className="text-green-600 dark:text-green-400" />
          )}
        </span>

        {/* Tool name + status */}
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-medium text-ink">
            {toolLabel(tool.name)}
          </p>
          <p className="text-xs text-ink-faint">
            {isRunning
              ? "Running..."
              : isError
                ? "Failed"
                : "Completed"}
          </p>
        </div>

        {/* Expand chevron */}
        {hasDetail && (
          <ChevronDownIcon
            width={16}
            height={16}
            className={`shrink-0 text-ink-faint transition-transform ${
              expanded ? "rotate-180" : ""
            }`}
          />
        )}
      </button>

      {/* Expandable detail */}
      {expanded && hasDetail && (
        <div className="mt-3 border-t border-line pt-3">
          {tool.result && (
            <div>
              <p className="mb-1 text-xs font-medium text-ink-faint">Result</p>
              <pre className="scroll-fine max-h-48 overflow-auto whitespace-pre-wrap rounded-lg bg-surface px-3 py-2 text-xs leading-relaxed text-ink-soft ring-1 ring-line">
                {tool.result}
              </pre>
            </div>
          )}
          {tool.error && (
            <div>
              <p className="mb-1 text-xs font-medium text-red-600 dark:text-red-400">
                Error
              </p>
              <p className="text-xs text-red-700 dark:text-red-300">
                {tool.error}
              </p>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
