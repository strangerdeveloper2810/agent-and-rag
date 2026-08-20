import { useState, useRef, useEffect } from "react";
import { useTranslation } from "react-i18next";
import {
  BoltIcon,
  CpuChipIcon,
  MagnifyingGlassIcon,
  CodeBracketIcon,
  ChevronDownIcon,
  CheckIcon,
} from "@heroicons/react/24/outline";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";

export type AIMode = "auto" | "reasoning" | "rag" | "code";

export interface ModeOption {
  id: AIMode;
  label: string;
  desc: string;
  icon: React.ComponentType<{ className?: string }>;
  badge?: string;
}

const MODE_META: {
  id: AIMode;
  icon: React.ComponentType<{ className?: string }>;
  hasBadge?: boolean;
}[] = [
  { id: "auto", icon: BoltIcon, hasBadge: true },
  { id: "reasoning", icon: CpuChipIcon },
  { id: "rag", icon: MagnifyingGlassIcon },
  { id: "code", icon: CodeBracketIcon },
];

interface ModeSelectorProps {
  currentMode: AIMode;
  onSelectMode: (mode: AIMode) => void;
}

export const ModeSelector: React.FC<ModeSelectorProps> = ({
  currentMode,
  onSelectMode,
}) => {
  const { t } = useTranslation("chat");
  const [open, setOpen] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);

  const MODE_OPTIONS: ModeOption[] = MODE_META.map((meta) => ({
    id: meta.id,
    icon: meta.icon,
    label: t(`modeSelector.modes.${meta.id}.label`),
    desc: t(`modeSelector.modes.${meta.id}.desc`),
    badge: meta.hasBadge ? t(`modeSelector.modes.${meta.id}.badge`) : undefined,
  }));

  const activeOption =
    MODE_OPTIONS.find((m) => m.id === currentMode) ?? MODE_OPTIONS[0];
  const ActiveIcon = activeOption.icon;

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (
        dropdownRef.current &&
        !dropdownRef.current.contains(e.target as Node)
      ) {
        setOpen(false);
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  return (
    <div className="relative inline-block text-left" ref={dropdownRef}>
      <Button
        type="button"
        variant="outline"
        size="sm"
        onClick={() => setOpen((p) => !p)}
        className="gap-2 bg-card/80 backdrop-blur-md border-border shadow-sm hover:bg-muted"
        aria-expanded={open}
        aria-label={t("modeSelector.selectAria")}
      >
        <ActiveIcon className="h-4 w-4 text-primary" />
        {/* Label đầy đủ ("Auto (Smart Router)") chiếm quá nhiều chỗ trên
            header mobile — cộng dồn với hamburger + title + LanguageSwitcher +
            ThemeToggle sẽ tràn ra ngoài viewport 375px (thấy rõ khi test bằng
            screenshot thật). Icon + chevron vẫn đủ để bấm mở dropdown. */}
        <span className="hidden sm:inline font-bold tracking-tight text-foreground">
          {activeOption.label}
        </span>
        <ChevronDownIcon
          className={`h-3.5 w-3.5 text-muted-foreground transition-transform duration-200 ${
            open ? "rotate-180" : ""
          }`}
        />
      </Button>

      {open && (
        <div className="glass absolute left-1/2 top-full z-50 mt-1.5 w-[min(20rem,calc(100vw-2rem))] -translate-x-1/2 rounded-2xl p-2 shadow-2xl animate-scale-in bg-popover text-popover-foreground border border-border">
          {/* Căn giữa dropdown theo TÂM của trigger (left-1/2 + -translate-x-1/2)
              thay vì neo theo mép trái (left-0): trigger nằm giữa header, không
              sát mép màn hình — neo trái khiến dropdown tràn hẳn ra ngoài mép
              phải trên mobile dù đã giới hạn max-width (thấy rõ qua screenshot
              thật ở 375px). */}
          <div className="px-3 py-1.5 mb-1.5 border-b border-border flex items-center justify-between">
            <p className="text-[10px] font-extrabold uppercase tracking-wider text-muted-foreground">
              {t("modeSelector.menuLabel")}
            </p>
          </div>

          <div className="space-y-1">
            {MODE_OPTIONS.map((opt) => {
              const isSelected = opt.id === currentMode;
              const Icon = opt.icon;
              return (
                <button
                  key={opt.id}
                  type="button"
                  onClick={() => {
                    onSelectMode(opt.id);
                    setOpen(false);
                  }}
                  className={`flex w-full items-start gap-3 rounded-xl p-2.5 text-left transition duration-150 border ${
                    isSelected
                      ? "bg-primary/10 border-primary/30 text-primary shadow-sm"
                      : "hover:bg-muted/70 border-transparent text-foreground"
                  }`}
                >
                  <div
                    className={`p-1.5 rounded-lg shrink-0 mt-0.5 ${isSelected ? "bg-primary/20 text-primary" : "bg-muted text-muted-foreground"}`}
                  >
                    <Icon className="h-4 w-4" />
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center justify-between gap-1.5">
                      <span
                        className={`text-sm font-bold truncate ${isSelected ? "text-primary" : "text-foreground"}`}
                      >
                        {opt.label}
                      </span>
                      {opt.badge && (
                        <Badge
                          variant="secondary"
                          className="text-[9.5px] px-2 py-0.5 shrink-0 bg-primary/15 text-primary border-primary/20 font-bold"
                        >
                          {opt.badge}
                        </Badge>
                      )}
                    </div>
                    <p
                      className={`text-xs leading-relaxed mt-0.5 ${isSelected ? "text-primary/90 font-medium" : "text-muted-foreground"}`}
                    >
                      {opt.desc}
                    </p>
                  </div>
                  {isSelected && (
                    <CheckIcon className="h-4 w-4 text-primary font-bold shrink-0 mt-1" />
                  )}
                </button>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
};

export default ModeSelector;
