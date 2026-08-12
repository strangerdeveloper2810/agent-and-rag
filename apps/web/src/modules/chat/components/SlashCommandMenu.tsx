import { useEffect, useState } from "react";
import {
  DocumentTextIcon,
  CodeBracketIcon,
  MagnifyingGlassIcon,
  LightBulbIcon,
  WrenchScrewdriverIcon,
} from "@heroicons/react/24/outline";

export interface SlashCommand {
  cmd: string;
  label: string;
  desc: string;
  icon: React.ComponentType<{ className?: string }>;
  prompt: string;
}

export const SLASH_COMMANDS: SlashCommand[] = [
  {
    cmd: "/summarize",
    label: "Tóm tắt nhanh",
    desc: "Tóm tắt ngắn gọn các điểm chính của tài liệu hoặc văn bản",
    icon: DocumentTextIcon,
    prompt: "Hãy tóm tắt ngắn gọn các ý chính của nội dung sau:",
  },
  {
    cmd: "/code",
    label: "Viết mã nguồn",
    desc: "Tạo hoặc refactor mã nguồn chuẩn clean code",
    icon: CodeBracketIcon,
    prompt: "Hãy giúp tôi viết/refactor mã nguồn cho yêu cầu sau:",
  },
  {
    cmd: "/search",
    label: "Tra cứu Knowledge Base",
    desc: "Tìm kiếm thông tin chính xác từ RAG vector DB",
    icon: MagnifyingGlassIcon,
    prompt: "Tra cứu trong cơ sở dữ liệu tri thức về:",
  },
  {
    cmd: "/explain",
    label: "Giải thích khái niệm",
    desc: "Giải thích các vấn đề phức tạp theo cách dễ hiểu nhất",
    icon: LightBulbIcon,
    prompt: "Hãy giải thích chi tiết khái niệm sau một cách trực quan dễ hiểu:",
  },
  {
    cmd: "/bug",
    label: "Debug & Fix Error",
    desc: "Phân tích nguyên nhân và đưa ra lời giải sửa lỗi",
    icon: WrenchScrewdriverIcon,
    prompt: "Phân tích nguyên nhân lỗi và sửa lại đoạn mã sau:",
  },
];

interface SlashCommandMenuProps {
  filterText: string;
  onSelect: (cmd: SlashCommand) => void;
  onClose: () => void;
}

export const SlashCommandMenu: React.FC<SlashCommandMenuProps> = ({
  filterText,
  onSelect,
  onClose,
}) => {
  const [selectedIndex, setSelectedIndex] = useState(0);

  const filtered = SLASH_COMMANDS.filter(
    (c) =>
      c.cmd.toLowerCase().includes(filterText.toLowerCase()) ||
      c.label.toLowerCase().includes(filterText.toLowerCase()),
  );

  useEffect(() => {
    setSelectedIndex(0);
  }, [filterText]);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (filtered.length === 0) return;
      if (e.key === "ArrowDown") {
        e.preventDefault();
        setSelectedIndex((prev) => (prev + 1) % filtered.length);
      } else if (e.key === "ArrowUp") {
        e.preventDefault();
        setSelectedIndex(
          (prev) => (prev - 1 + filtered.length) % filtered.length,
        );
      } else if (e.key === "Enter" || e.key === "Tab") {
        e.preventDefault();
        if (filtered[selectedIndex]) {
          onSelect(filtered[selectedIndex]);
        }
      } else if (e.key === "Escape") {
        onClose();
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [filtered, selectedIndex, onSelect, onClose]);

  if (filtered.length === 0) return null;

  return (
    <div className="glass absolute bottom-full left-0 z-50 mb-2 w-80 overflow-hidden rounded-2xl p-2 shadow-2xl animate-slide-up bg-popover text-popover-foreground border border-border">
      <div className="px-3 py-1.5 mb-1.5 border-b border-border flex items-center justify-between">
        <span className="text-[10px] font-extrabold uppercase tracking-wider text-muted-foreground">
          SMART COMMANDS (PHÍM TẮT `/`)
        </span>
        <span className="text-[9.5px] text-muted-foreground font-mono font-medium">
          ↑↓ chọn · Enter dùng
        </span>
      </div>

      <div className="space-y-1 max-h-60 overflow-y-auto scroll-fine">
        {filtered.map((item, idx) => {
          const isSelected = idx === selectedIndex;
          const Icon = item.icon;
          return (
            <button
              key={item.cmd}
              type="button"
              onClick={() => onSelect(item)}
              onMouseEnter={() => setSelectedIndex(idx)}
              className={`flex w-full items-center gap-3 rounded-xl p-2.5 text-left transition duration-150 border ${
                isSelected
                  ? "bg-primary/10 border-primary/30 text-primary shadow-sm"
                  : "hover:bg-muted/70 border-transparent text-foreground"
              }`}
            >
              <div
                className={`p-1.5 rounded-lg shrink-0 ${isSelected ? "bg-primary/20 text-primary" : "bg-muted text-muted-foreground"}`}
              >
                <Icon className="h-4 w-4" />
              </div>
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <span className="font-mono text-xs font-bold text-primary">
                    {item.cmd}
                  </span>
                  <span
                    className={`text-xs font-bold truncate ${isSelected ? "text-primary" : "text-foreground"}`}
                  >
                    {item.label}
                  </span>
                </div>
                <p
                  className={`text-xs leading-tight truncate mt-0.5 ${isSelected ? "text-primary/90 font-medium" : "text-muted-foreground"}`}
                >
                  {item.desc}
                </p>
              </div>
            </button>
          );
        })}
      </div>
    </div>
  );
};

export default SlashCommandMenu;
