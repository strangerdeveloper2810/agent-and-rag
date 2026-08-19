import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import {
  SparklesIcon,
  ArrowRightIcon,
  CheckCircleIcon,
  CpuChipIcon,
  BoltIcon,
  MagnifyingGlassIcon,
} from "@heroicons/react/24/outline";

import { buttonVariants } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

const STAT_ITEMS = [
  { key: "models", icon: CpuChipIcon },
  { key: "skills", icon: BoltIcon },
  { key: "rag", icon: MagnifyingGlassIcon },
] as const;

/**
 * Hero — section mở đầu landing page.
 * Dùng lại kỹ thuật radial glow (bg-primary/10 blur-[130px]) + animate-float-slow
 * đã có sẵn trong index.css/LoginPage để tạo chiều sâu, tránh nhìn phẳng.
 */
export const Hero: React.FC = () => {
  const { t } = useTranslation("landing");

  return (
    <section className="relative overflow-hidden px-6 pb-20 pt-16 sm:pt-24">
      {/* Decorative radial glows — cùng kỹ thuật với LoginPage */}
      <div className="pointer-events-none absolute -left-24 top-10 h-[420px] w-[420px] rounded-full bg-primary/10 blur-[130px]" />
      <div className="pointer-events-none absolute -right-24 top-1/3 h-[380px] w-[380px] rounded-full bg-purple-500/10 blur-[130px]" />

      <div className="relative z-10 mx-auto flex max-w-4xl flex-col items-center text-center">
        <Badge
          variant="accent"
          className="mb-6 gap-1.5 py-1.5 px-3.5 animate-fade-in"
        >
          <SparklesIcon className="h-3.5 w-3.5" />
          <span>{t("hero.badge")}</span>
        </Badge>

        <div className="animate-float-slow mb-6 flex h-16 w-16 items-center justify-center rounded-3xl bg-gradient-to-br from-indigo-500 to-purple-600 shadow-xl shadow-indigo-500/30">
          <SparklesIcon className="h-8 w-8 text-white" />
        </div>

        <h1 className="font-display text-4xl font-extrabold leading-tight tracking-tight text-foreground sm:text-5xl lg:text-6xl animate-slide-up">
          {t("hero.title")}
          <br />
          <span className="bg-gradient-to-r from-indigo-400 via-purple-400 to-pink-400 bg-clip-text text-transparent">
            {t("hero.titleHighlight")}
          </span>
        </h1>

        <p className="mt-6 max-w-2xl text-sm leading-relaxed text-muted-foreground sm:text-base animate-slide-up">
          {t("hero.subtitle")}
        </p>

        <div className="mt-9 flex flex-col items-center gap-3 sm:flex-row animate-scale-in">
          <Link
            to="/register"
            className={cn(
              buttonVariants({ variant: "gradient", size: "lg" }),
              "gap-2 w-full sm:w-auto",
            )}
          >
            {t("hero.ctaPrimary")}
            <ArrowRightIcon className="h-4 w-4" />
          </Link>
          <Link
            to="/login"
            className={cn(
              buttonVariants({ variant: "outline", size: "lg" }),
              "w-full sm:w-auto",
            )}
          >
            {t("hero.ctaSecondary")}
          </Link>
        </div>

        <div className="mt-6 flex items-center gap-1.5 text-[11px] font-mono text-muted-foreground">
          <CheckCircleIcon className="h-3.5 w-3.5 text-emerald-400" />
          {t("hero.statusBadge")}
        </div>

        {/* Quick-stat chips */}
        <div className="mt-12 flex flex-wrap items-center justify-center gap-3">
          {STAT_ITEMS.map(({ key, icon: Icon }) => (
            <div
              key={key}
              className="glass flex items-center gap-2 rounded-full px-4 py-2 text-xs font-semibold text-foreground"
            >
              <Icon className="h-4 w-4 text-primary" />
              {t(`hero.stats.${key}`)}
            </div>
          ))}
        </div>
      </div>
    </section>
  );
};

export default Hero;
