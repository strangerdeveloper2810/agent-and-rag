import React, { useState } from "react";
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

const TOOL_META: Record<
  string,
  { title: string; desc: string; icon: React.ComponentType<{ className?: string }> }
> = {
  "rag.search": {
    title: "Tra cứu tài liệu RAG",
    desc: "Tìm thông tin trong cơ sở tri thức local",
    icon: DocumentMagnifyingGlassIcon,
  },
  "web.search": {
    title: "Tìm kiếm Web",
    desc: "Tra cứu thông tin cập nhật trên Internet",
    icon: GlobeAltIcon,
  },
  "web.fetch": {
    title: "Đọc nội dung URL",
    desc: "Trích xuất văn bản từ trang web",
    icon: GlobeAltIcon,
  },
  "memory.recall": {
    title: "Truy xuất bộ nhớ",
    desc: "Kiểm tra thông tin đã lưu trong bộ nhớ cá nhân",
    icon: CircleStackIcon,
  },
  "memory.save": {
    title: "Ghi nhớ thông tin",
    desc: "Lưu dữ liệu quan trọng vào bộ nhớ dài hạn",
    icon: CircleStackIcon,
  },
  "file.search": {
    title: "Tìm kiếm tệp tin",
    desc: "Quét các file liên quan trong thư mục làm việc",
    icon: FolderIcon,
  },
  "file.read": {
    title: "Đọc tệp tin",
    desc: "Đọc nội dung mã nguồn hoặc tài liệu local",
    icon: FolderIcon,
  },
  "file.write": {
    title: "Ghi / Tạo tệp tin",
    desc: "Tạo mới hoặc cập nhật nội dung tệp tin",
    icon: CommandLineIcon,
  },
  "shell.exec": {
    title: "Thực thi lệnh Shell",
    desc: "Chạy câu lệnh terminal trong môi trường an toàn",
    icon: CommandLineIcon,
  },
  "git.tool": {
    title: "Thao tác Git",
    desc: "Kiểm tra commit, branch hoặc trạng thái kho chứa",
    icon: CommandLineIcon,
  },
};

const getToolMeta = (name: string) => {
  if (TOOL_META[name]) return TOOL_META[name];
  const formatted = name.replace(/([A-Z])/g, " $1").toLowerCase();
  return {
    title: name,
    desc: `Thực thi công cụ ${formatted}`,
    icon: WrenchIcon,
  };
};

export interface ToolCallGroupProps {
  tools: ToolCallState[];
}

export const ToolCallGroup: React.FC<ToolCallGroupProps> = ({ tools }) => {
  const [expanded, setExpanded] = useState(false);
  const [openLogIndex, setOpenLogIndex] = useState<number | null>(null);

  if (!tools || tools.length === 0) return null;

  const isAnyRunning = tools.some((t) => t.status === "running");
  const isAnyError = tools.some((t) => t.status === "error");
  const completedCount = tools.filter((t) => t.status === "done").length;

  const headerStatusText = isAnyRunning
    ? `Đang thực thi ${tools.length} công cụ hệ thống...`
    : isAnyError
      ? `Đã chạy ${tools.length} công cụ (${completedCount}/${tools.length} hoàn thành)`
      : `Đã hoàn thành ${tools.length} bước thực thi`;

  return (
    <div className="my-2.5 rounded-2xl border border-border/80 bg-card/70 backdrop-blur-md overflow-hidden shadow-xs transition-all duration-200">
      {/* Header bar */}
      <button
        type="button"
        onClick={() => setExpanded((p) => !p)}
        className="flex w-full items-center justify-between gap-3 px-3.5 py-2.5 text-left hover:bg-muted/40 transition duration-150"
        aria-expanded={expanded}
        aria-label="Danh sách tool call"
      >
        <div className="flex items-center gap-2.5 min-w-0">
          <div
            className={`flex h-7 w-7 shrink-0 items-center justify-center rounded-xl transition-all ${
              isAnyRunning
                ? "bg-primary/15 text-primary border border-primary/30"
                : isAnyError
                  ? "bg-destructive/15 text-destructive border border-destructive/30"
                  : "bg-emerald-500/15 text-emerald-500 border border-emerald-500/30"
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
                variant={isAnyRunning ? "secondary" : isAnyError ? "destructive" : "outline"}
                className={`text-[9.5px] px-2 py-0.2 font-mono font-bold uppercase ${
                  isAnyRunning
                    ? "bg-primary/15 text-primary animate-pulse border-primary/20"
                    : !isAnyError
                      ? "bg-emerald-500/10 text-emerald-500 border-emerald-500/20"
                      : ""
                }`}
              >
                {isAnyRunning ? "Running" : isAnyError ? "Warning" : "Done"}
              </Badge>
            </div>
            {/* Tool list preview */}
            <p className="truncate text-[11px] text-muted-foreground font-mono mt-0.5">
              {tools.map((t) => t.name).join(" • ")}
            </p>
          </div>
        </div>

        <div className="flex items-center gap-1.5 shrink-0 text-xs font-semibold text-muted-foreground bg-muted/60 hover:bg-muted px-2.5 py-1 rounded-lg transition border border-border/50">
          <span>{expanded ? "Thu gọn" : "Chi tiết"}</span>
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
          {tools.map((t, idx) => {
            const meta = getToolMeta(t.name);
            const Icon = meta.icon;
            const isRunning = t.status === "running";
            const isError = t.status === "error";
            const hasOutput = Boolean(t.result || t.error);
            const isLogOpen = openLogIndex === idx;

            return (
              <div
                key={`${t.name}-${idx}`}
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
                          {t.name}
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
                            : "text-emerald-500"
                      }`}
                    >
                      <span
                        className={`h-1.5 w-1.5 rounded-full ${
                          isRunning
                            ? "bg-primary animate-ping"
                            : isError
                              ? "bg-destructive"
                              : "bg-emerald-500"
                        }`}
                      />
                      {isRunning ? "Running" : isError ? "Error" : "Success"}
                    </span>

                    {hasOutput && (
                      <button
                        type="button"
                        onClick={() => setOpenLogIndex(isLogOpen ? null : idx)}
                        className="text-[10px] font-mono px-2 py-0.5 rounded-md bg-muted hover:bg-muted/80 text-muted-foreground transition border border-border/50"
                      >
                        {isLogOpen ? "Ẩn log" : "Xem log"}
                      </button>
                    )}
                  </div>
                </div>

                {/* Log drawer */}
                {isLogOpen && hasOutput && (
                  <div className="mt-2.5 border-t border-border/50 pt-2 animate-fade-in">
                    {t.result && (
                      <div>
                        <span className="text-[9px] font-mono font-bold text-primary uppercase tracking-wider block mb-1">
                          // Kết quả thực thi
                        </span>
                        <pre className="scroll-fine max-h-48 overflow-auto rounded-lg bg-black/90 p-2.5 text-[10.5px] font-mono text-emerald-400 leading-relaxed border border-border/40">
                          {t.result}
                        </pre>
                      </div>
                    )}
                    {t.error && (
                      <div>
                        <span className="text-[9px] font-mono font-bold text-destructive uppercase tracking-wider block mb-1">
                          // Nhật ký lỗi
                        </span>
                        <pre className="scroll-fine max-h-48 overflow-auto rounded-lg bg-destructive/10 p-2.5 text-[10.5px] font-mono text-destructive leading-relaxed border border-destructive/30">
                          {t.error}
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
