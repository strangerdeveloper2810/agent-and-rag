import { useState } from "react";
import { useTranslation } from "react-i18next";
import { PhotoIcon } from "@heroicons/react/24/outline";

import { cn } from "@/lib/utils";
import { useRevealOnScroll } from "@/modules/landing/hooks/useRevealOnScroll";

/**
 * ProductPreview — khung "browser chrome" chứa screenshot thật của giao diện chat.
 * Ảnh đặt tại public/screenshots/chat-preview.png (Vite serve nguyên trạng ở "/screenshots/...").
 * Nếu file chưa tồn tại, fallback về placeholder thay vì icon ảnh vỡ.
 */
export const ProductPreview: React.FC = () => {
  const { t } = useTranslation("landing");
  const { ref, isVisible } = useRevealOnScroll<HTMLDivElement>();
  const [imageFailed, setImageFailed] = useState(false);

  return (
    <section className="px-6 py-20">
      <div className="mx-auto max-w-2xl text-center">
        <span className="text-xs font-mono font-bold uppercase tracking-widest text-primary">
          {t("productPreview.eyebrow")}
        </span>
        <h2 className="mt-3 font-display text-3xl font-extrabold tracking-tight text-foreground sm:text-4xl">
          {t("productPreview.title")}
        </h2>
        <p className="mt-4 text-sm leading-relaxed text-muted-foreground sm:text-base">
          {t("productPreview.subtitle")}
        </p>
      </div>

      <div
        ref={ref}
        className={cn(
          "relative mx-auto mt-14 max-w-5xl",
          isVisible ? "animate-slide-up" : "opacity-0",
        )}
      >
        <div className="pointer-events-none absolute -inset-x-10 -top-10 h-64 rounded-full bg-primary/10 blur-[130px]" />

        {/* Browser chrome frame */}
        <div className="glass relative overflow-hidden rounded-2xl border border-border/60 shadow-2xl shadow-primary/10">
          <div className="flex items-center gap-1.5 border-b border-border/60 bg-muted/40 px-4 py-3">
            <span className="h-2.5 w-2.5 rounded-full bg-red-400/70" />
            <span className="h-2.5 w-2.5 rounded-full bg-amber-400/70" />
            <span className="h-2.5 w-2.5 rounded-full bg-emerald-400/70" />
            <span className="ml-3 truncate text-[11px] font-mono text-muted-foreground">
              app.jarvis.ai/chat
            </span>
          </div>

          <div className="aspect-[16/10] w-full bg-background sm:aspect-[16/9]">
            {imageFailed ? (
              <div className="flex h-full w-full flex-col items-center justify-center gap-3 border-2 border-dashed border-border/70 text-center">
                <PhotoIcon className="h-10 w-10 text-muted-foreground/50" />
                <div>
                  <p className="text-sm font-semibold text-foreground">
                    {t("productPreview.placeholderTitle")}
                  </p>
                  <p className="mt-1 text-xs text-muted-foreground">
                    {t("productPreview.placeholderDesc")}
                  </p>
                </div>
              </div>
            ) : (
              <img
                src="/screenshots/chat-preview.png"
                alt="J.A.R.V.I.S. chat interface"
                width={1600}
                height={1000}
                loading="lazy"
                className="h-full w-full object-cover object-top"
                onError={() => setImageFailed(true)}
              />
            )}
          </div>
        </div>
      </div>
    </section>
  );
};

export default ProductPreview;
