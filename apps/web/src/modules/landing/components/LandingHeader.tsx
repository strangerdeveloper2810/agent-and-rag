import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { SparklesIcon } from "@heroicons/react/24/outline";

import { buttonVariants } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import ThemeToggle from "@/design-system/atoms/ThemeToggle";
import LanguageSwitcher from "@/design-system/atoms/LanguageSwitcher";

/**
 * LandingHeader — sticky top nav cho landing page public.
 * Tái sử dụng brand mark (gradient indigo→purple box + wordmark) đã chốt ở LoginPage/RegisterPage.
 */
export const LandingHeader: React.FC = () => {
  const { t } = useTranslation("landing");

  return (
    <header className="sticky top-0 z-30 border-b border-border bg-background/80 backdrop-blur-xl">
      <div className="mx-auto flex h-16 max-w-6xl items-center justify-between px-6">
        <Link to="/landing-preview" className="flex items-center gap-2.5">
          <div className="flex h-9 w-9 items-center justify-center rounded-2xl bg-gradient-to-br from-indigo-500 to-purple-600 shadow-lg shadow-indigo-500/25">
            <SparklesIcon className="h-5 w-5 text-white" />
          </div>
          <div className="leading-none">
            <span className="font-display text-base font-extrabold tracking-tight text-foreground">
              J.A.R.V.I.S.
            </span>
            <p className="text-[9px] font-mono font-bold uppercase tracking-widest text-primary">
              {t("header.tagline")}
            </p>
          </div>
        </Link>

        <nav className="flex items-center gap-1.5 sm:gap-3">
          <div className="flex items-center gap-1.5">
            <LanguageSwitcher />
            <ThemeToggle />
          </div>
          <div className="mx-1 h-5 w-px bg-border sm:mx-1.5" />
          <Link
            to="/login"
            className={cn(
              buttonVariants({ variant: "ghost", size: "sm" }),
              "hidden sm:inline-flex",
            )}
          >
            {t("header.loginLink")}
          </Link>
          <Link
            to="/register"
            className={cn(buttonVariants({ variant: "gradient", size: "sm" }))}
          >
            {t("header.registerLink")}
          </Link>
        </nav>
      </div>
    </header>
  );
};

export default LandingHeader;
