import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
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

const SLASH_COMMAND_IDS = [
  "summarize",
  "code",
  "search",
  "explain",
  "bug",
] as const;

const SLASH_COMMAND_ICONS: Record<
  (typeof SLASH_COMMAND_IDS)[number],
  React.ComponentType<{ className?: string }>
> = {
  summarize: DocumentTextIcon,
  code: CodeBracketIcon,
  search: MagnifyingGlassIcon,
  explain: LightBulbIcon,
  bug: WrenchScrewdriverIcon,
};

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
  const { t } = useTranslation("chat");
  const [selectedIndex, setSelectedIndex] = useState(0);

  const SLASH_COMMANDS: SlashCommand[] = SLASH_COMMAND_IDS.map((id) => ({
    cmd: `/${id}`,
    label: t(`slashCommandMenu.commands.${id}.label`),
    desc: t(`slashCommandMenu.commands.${id}.desc`),
    prompt: t(`slashCommandMenu.commands.${id}.prompt`),
    icon: SLASH_COMMAND_ICONS[id],
  }));

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
          {t("slashCommandMenu.header")}
        </span>
        <span className="text-[9.5px] text-muted-foreground font-mono font-medium">
          {t("slashCommandMenu.hint")}
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
