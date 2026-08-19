import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { ArrowRightIcon, SparklesIcon } from "@heroicons/react/24/outline";

import { buttonVariants } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { useRevealOnScroll } from "@/modules/landing/hooks/useRevealOnScroll";

/**
 * CtaSection — nhắc lại Đăng ký/Đăng nhập ở cuối trang, đặt trong khối glass nổi bật
 * để tạo điểm dừng thị giác cuối cùng trước footer.
 */
export const CtaSection: React.FC = () => {
  const { t } = useTranslation("landing");
  const { ref, isVisible } = useRevealOnScroll<HTMLDivElement>();

  return (
    <section className="px-6 py-24 sm:py-28">
      <div
        ref={ref}
        className={cn(
          "glass relative mx-auto max-w-4xl overflow-hidden rounded-3xl p-10 text-center sm:p-14",
          isVisible ? "animate-scale-in" : "opacity-0",
        )}
      >
        <div className="pointer-events-none absolute -bottom-16 left-1/2 h-64 w-64 -translate-x-1/2 rounded-full bg-primary/15 blur-[110px]" />

        <div className="relative z-10 mx-auto flex h-12 w-12 items-center justify-center rounded-2xl bg-gradient-to-br from-indigo-500 to-purple-600 shadow-lg shadow-indigo-500/25">
          <SparklesIcon className="h-6 w-6 text-white" />
        </div>

        <h2 className="relative z-10 mt-6 font-display text-2xl font-extrabold tracking-tight text-foreground sm:text-3xl">
          {t("cta.title")}
        </h2>
        <p className="relative z-10 mx-auto mt-3 max-w-xl text-sm leading-relaxed text-muted-foreground sm:text-base">
          {t("cta.subtitle")}
        </p>

        <div className="relative z-10 mt-8 flex flex-col items-center justify-center gap-3 sm:flex-row">
          <Link
            to="/register"
            className={cn(
              buttonVariants({ variant: "gradient", size: "lg" }),
              "gap-2 w-full sm:w-auto",
            )}
          >
            {t("cta.primary")}
            <ArrowRightIcon className="h-4 w-4" />
          </Link>
          <Link
            to="/login"
            className={cn(
              buttonVariants({ variant: "outline", size: "lg" }),
              "w-full sm:w-auto",
            )}
          >
            {t("cta.secondary")}
          </Link>
        </div>
      </div>
    </section>
  );
};

export default CtaSection;
