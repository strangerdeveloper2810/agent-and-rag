import { useState } from "react";
import {
  Bars3Icon,
  MagnifyingGlassIcon,
  ArrowDownTrayIcon,
  TrashIcon,
} from "@heroicons/react/24/outline";
import ThemeToggle from "../atoms/ThemeToggle";
import ModeSelector, {
  type AIMode,
} from "@/modules/chat/components/ModeSelector";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";

export interface HeaderProps {
  onToggleSidebar: () => void;
  title?: string;
  onClearChat?: () => void;
  onExportChat?: () => void;
  onToggleSearch?: () => void;
}

/**
 * Header — Shadcn UI styled header with Mode Selector, Heroicons, and Theme Toggle.
 */
export const Header: React.FC<HeaderProps> = ({
  onToggleSidebar,
  title = "J.A.R.V.I.S.",
  onClearChat,
  onExportChat,
  onToggleSearch,
}) => {
  const [currentMode, setCurrentMode] = useState<AIMode>("auto");

  return (
    <header className="glass relative z-30 flex shrink-0 items-center justify-between px-4 py-2 sm:px-6 transition-all duration-200 border-b border-border bg-card/70 backdrop-blur-xl">
      {/* Left: Sidebar toggle + Brand + Active Mode */}
      <div className="flex items-center gap-3">
        <Button
          type="button"
          variant="outline"
          size="iconSm"
          onClick={onToggleSidebar}
          aria-label="Toggle sidebar"
          className="h-8 w-8"
        >
          <Bars3Icon className="h-4 w-4 text-foreground" />
        </Button>

        <div className="flex items-center gap-2.5">
          <span className="font-display text-sm font-bold tracking-tight text-foreground">
            {title}
          </span>
          <Badge
            variant="success"
            className="hidden sm:flex items-center gap-1.5 font-mono text-[10px]"
          >
            <span className="h-1.5 w-1.5 rounded-full bg-emerald-400 animate-pulse" />
            ONLINE
          </Badge>
        </div>

        {/* AI Mode Switcher */}
        <div className="ml-1 sm:ml-2">
          <ModeSelector
            currentMode={currentMode}
            onSelectMode={(mode) => setCurrentMode(mode)}
          />
        </div>
      </div>

      {/* Right: Quick Action Tools + Theme Switcher */}
      <div className="flex items-center gap-1.5 sm:gap-2">
        {onToggleSearch && (
          <Button
            type="button"
            variant="ghost"
            size="iconSm"
            onClick={onToggleSearch}
            aria-label="Search conversation"
            title="Tìm kiếm trong cuộc hội thoại"
            className="h-8 w-8"
          >
            <MagnifyingGlassIcon className="h-4 w-4 text-muted-foreground" />
          </Button>
        )}

        {onExportChat && (
          <Button
            type="button"
            variant="ghost"
            size="iconSm"
            onClick={onExportChat}
            aria-label="Export chat log"
            title="Xuất lịch sử chat (Markdown/JSON)"
            className="h-8 w-8"
          >
            <ArrowDownTrayIcon className="h-4 w-4 text-muted-foreground" />
          </Button>
        )}

        {onClearChat && (
          <Button
            type="button"
            variant="ghost"
            size="iconSm"
            onClick={onClearChat}
            aria-label="Clear chat"
            title="Làm sạch cuộc hội thoại"
            className="hidden sm:flex h-8 w-8 text-muted-foreground hover:text-destructive hover:bg-destructive/10"
          >
            <TrashIcon className="h-4 w-4" />
          </Button>
        )}

        <ThemeToggle />
      </div>
    </header>
  );
};

export default Header;
