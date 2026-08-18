import React, { useState } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import {
  WrenchIcon,
  CheckCircleIcon,
  XCircleIcon,
  ChevronDownIcon,
  ChevronUpIcon,
  CommandLineIcon,
  DocumentMagnifyingGlassIcon,
  CircleStackIcon,
  GlobeAltIcon,
  FolderIcon,
} from "@heroicons/react/24/outline";
import type { ToolCallState } from "@/modules/chat/chat.api";
import { Badge } from "@/components/ui/badge";

const TOOL_ICONS: Record<
  string,
  React.ComponentType<{ className?: string }>
> = {
  "rag.search": DocumentMagnifyingGlassIcon,
  "web.search": GlobeAltIcon,
  "web.fetch": GlobeAltIcon,
  "memory.recall": CircleStackIcon,
  "memory.save": CircleStackIcon,
  "file.search": FolderIcon,
  "file.read": FolderIcon,
  "file.write": CommandLineIcon,
  "shell.exec": CommandLineIcon,
  "git.tool": CommandLineIcon,
};

const getToolMeta = (name: string, t: TFunction<"chat">) => {
  if (TOOL_ICONS[name]) {
    return {
      title: t(`toolCallCard.tools.${name}.title`),
      desc: t(`toolCallCard.tools.${name}.desc`),
      icon: TOOL_ICONS[name],
    };
  }
  const formatted = name.replace(/([A-Z])/g, " $1").toLowerCase();
  return {
    title: name,
    desc: t("toolCallCard.defaultToolDesc", { formatted }),
    icon: WrenchIcon,
  };
};

export interface ToolCallGroupProps {
  tools: ToolCallState[];
}

export const ToolCallGroup: React.FC<ToolCallGroupProps> = ({ tools }) => {
  const { t } = useTranslation("chat");
  const [expanded, setExpanded] = useState(false);
  const [openLogIndex, setOpenLogIndex] = useState<number | null>(null);

  if (!tools || tools.length === 0) return null;

  const isAnyRunning = tools.some((tool) => tool.status === "running");
  const isAnyError = tools.some((tool) => tool.status === "error");
  const completedCount = tools.filter((tool) => tool.status === "done").length;

  const headerStatusText = isAnyRunning
    ? t("toolCallCard.headerStatus.running", { total: tools.length })
    : isAnyError
      ? t("toolCallCard.headerStatus.errorSummary", {
          total: tools.length,
          done: completedCount,
        })
      : t("toolCallCard.headerStatus.done", { total: tools.length });

  return (
    <div className="my-2.5 rounded-2xl border border-border/80 bg-card/70 backdrop-blur-md overflow-hidden shadow-xs transition-all duration-200">
      {/* Header bar */}
      <button
        type="button"
        onClick={() => setExpanded((p) => !p)}
        className="flex w-full items-center justify-between gap-3 px-3.5 py-2.5 text-left hover:bg-muted/40 transition duration-150"
        aria-expanded={expanded}
        aria-label={t("toolCallCard.detailsAria")}
      >
        <div className="flex items-center gap-2.5 min-w-0">
          <div
            className={`flex h-7 w-7 shrink-0 items-center justify-center rounded-xl transition-all ${
              isAnyRunning
                ? "bg-primary/15 text-primary border border-primary/30"
                : isAnyError
                  ? "bg-destructive/15 text-destructive border border-destructive/30"
                  : "bg-[var(--success-bg)] text-[var(--success)] border border-[var(--success)]"
            }`}
          >
            {isAnyRunning ? (
              <WrenchIcon className="h-4 w-4 animate-spin" />
            ) : isAnyError ? (
              <XCircleIcon className="h-4 w-4" />
            ) : (
              <CheckCircleIcon className="h-4 w-4" />
            )}
          </div>

          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              <span className="text-xs font-bold tracking-tight text-foreground">
                {headerStatusText}
              </span>
              <Badge
                variant={
                  isAnyRunning
                    ? "secondary"
                    : isAnyError
                      ? "destructive"
                      : "outline"
                }
                className={`text-[9.5px] px-2 py-0.2 font-mono font-bold uppercase ${
                  isAnyRunning
                    ? "bg-primary/15 text-primary animate-pulse border-primary/20"
                    : !isAnyError
                      ? "bg-[var(--success-bg)] text-[var(--success)] border-[var(--success)]"
                      : ""
                }`}
              >
                {isAnyRunning
                  ? t("toolCallCard.groupBadge.running")
                  : isAnyError
                    ? t("toolCallCard.groupBadge.warning")
                    : t("toolCallCard.groupBadge.done")}
              </Badge>
            </div>
            {/* Tool list preview */}
            <p className="truncate text-[11px] text-muted-foreground font-mono mt-0.5">
              {tools.map((tool) => tool.name).join(" • ")}
            </p>
          </div>
        </div>

        <div className="flex items-center gap-1.5 shrink-0 text-xs font-semibold text-muted-foreground bg-muted/60 hover:bg-muted px-2.5 py-1 rounded-lg transition border border-border/50">
          <span>
            {expanded ? t("toolCallCard.collapse") : t("toolCallCard.expand")}
          </span>
          {expanded ? (
            <ChevronUpIcon className="h-3.5 w-3.5" />
          ) : (
            <ChevronDownIcon className="h-3.5 w-3.5" />
          )}
        </div>
      </button>

      {/* Expanded tool items */}
      {expanded && (
        <div className="border-t border-border/60 bg-muted/20 p-2.5 space-y-2 animate-fade-in">
          {tools.map((tool, idx) => {
            const meta = getToolMeta(tool.name, t);
            const Icon = meta.icon;
            const isRunning = tool.status === "running";
            const isError = tool.status === "error";
            const hasOutput = Boolean(tool.result || tool.error);
            const isLogOpen = openLogIndex === idx;

            return (
              <div
                key={`${tool.name}-${idx}`}
                className="rounded-xl border border-border/60 bg-card/90 p-2.5 transition shadow-xs"
              >
                <div className="flex items-start justify-between gap-3">
                  <div className="flex items-start gap-2.5 min-w-0">
                    <div
                      className={`p-1.5 rounded-lg shrink-0 mt-0.5 ${
                        isRunning
                          ? "bg-primary/10 text-primary"
                          : isError
                            ? "bg-destructive/10 text-destructive"
                            : "bg-muted text-muted-foreground"
                      }`}
                    >
                      <Icon className="h-4 w-4" />
                    </div>

                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
                        <span className="text-xs font-bold text-foreground truncate">
                          {meta.title}
                        </span>
                        <code className="text-[10px] font-mono px-1.5 py-0.2 rounded bg-muted text-muted-foreground">
                          {tool.name}
                        </code>
                      </div>
                      <p className="text-[11px] text-muted-foreground mt-0.5">
                        {meta.desc}
                      </p>
                    </div>
                  </div>

                  <div className="flex items-center gap-2 shrink-0">
                    <span
                      className={`inline-flex items-center gap-1 text-[10px] font-mono font-bold uppercase ${
                        isRunning
                          ? "text-primary animate-pulse"
                          : isError
                            ? "text-destructive"
                            : "text-[var(--success)]"
                      }`}
                    >
                      <span
                        className={`h-1.5 w-1.5 rounded-full ${
                          isRunning
                            ? "bg-primary animate-ping"
                            : isError
                              ? "bg-destructive"
                              : "bg-[var(--success)]"
                        }`}
                      />
                      {isRunning
                        ? t("toolCallCard.statusBadge.running")
                        : isError
                          ? t("toolCallCard.statusBadge.error")
                          : t("toolCallCard.statusBadge.success")}
                    </span>

                    {hasOutput && (
                      <button
                        type="button"
                        onClick={() => setOpenLogIndex(isLogOpen ? null : idx)}
                        className="text-[10px] font-mono px-2 py-0.5 rounded-md bg-muted hover:bg-muted/80 text-muted-foreground transition border border-border/50"
                      >
                        {isLogOpen
                          ? t("toolCallCard.hideLog")
                          : t("toolCallCard.viewLog")}
                      </button>
                    )}
                  </div>
                </div>

                {/* Log drawer */}
                {isLogOpen && hasOutput && (
                  <div className="mt-2.5 border-t border-border/50 pt-2 animate-fade-in">
                    {tool.result && (
                      <div>
                        <span className="text-[9px] font-mono font-bold text-primary uppercase tracking-wider block mb-1">
                          {t("toolCallCard.executionResult")}
                        </span>
                        <pre className="scroll-fine max-h-48 overflow-auto rounded-lg bg-black/90 p-2.5 text-[10.5px] font-mono text-[var(--success)] leading-relaxed border border-border/40">
                          {tool.result}
                        </pre>
                      </div>
                    )}
                    {tool.error && (
                      <div>
                        <span className="text-[9px] font-mono font-bold text-destructive uppercase tracking-wider block mb-1">
                          {t("toolCallCard.errorLog")}
                        </span>
                        <pre className="scroll-fine max-h-48 overflow-auto rounded-lg bg-destructive/10 p-2.5 text-[10.5px] font-mono text-destructive leading-relaxed border border-destructive/30">
                          {tool.error}
                        </pre>
                      </div>
                    )}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
};

export const ToolCallCard: React.FC<{ tool: ToolCallState }> = ({ tool }) => {
  return <ToolCallGroup tools={[tool]} />;
};

export default ToolCallGroup;
