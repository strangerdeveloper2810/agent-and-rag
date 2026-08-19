import React, { useEffect, useRef, useState } from "react";
import mermaid from "mermaid";
import {
  ClipboardDocumentIcon,
  CheckIcon,
  CodeBracketIcon,
  EyeIcon,
  ExclamationTriangleIcon,
} from "@heroicons/react/24/outline";
import { useTranslation } from "react-i18next";

export interface MermaidBlockProps {
  code: string;
}

/** Singleton counter for stable, unique mermaid element IDs across all renders. */
let mermaidCounter = 0;

/** One-time global mermaid initialization (re-runs only when theme class changes). */
function initMermaid() {
  const isDark = document.documentElement.classList.contains("dark");
  mermaid.initialize({
    startOnLoad: false,
    theme: isDark ? "dark" : "default",
    securityLevel: "loose",
    fontFamily: "Inter, system-ui, sans-serif",
    themeVariables: isDark
      ? {
          primaryColor: "#6366f1",
          primaryTextColor: "#f8fafc",
          primaryBorderColor: "#818cf8",
          lineColor: "#94a3b8",
          secondaryColor: "#1e1b4b",
          tertiaryColor: "#0f172a",
          background: "#0d1117",
          mainBkg: "#1e293b",
          nodeBorder: "#6366f1",
          clusterBkg: "#0f172a",
          clusterBorder: "#334155",
          defaultLinkColor: "#818cf8",
          titleColor: "#f8fafc",
          edgeLabelBackground: "#1e293b",
        }
      : {
          primaryColor: "#4f46e5",
          primaryTextColor: "#1e293b",
          primaryBorderColor: "#6366f1",
          lineColor: "#64748b",
          secondaryColor: "#e0e7ff",
          tertiaryColor: "#f8fafc",
          background: "#ffffff",
          mainBkg: "#f1f5f9",
          nodeBorder: "#4f46e5",
          clusterBkg: "#f8fafc",
          clusterBorder: "#cbd5e1",
          defaultLinkColor: "#4f46e5",
          titleColor: "#0f172a",
          edgeLabelBackground: "#ffffff",
        },
  });
}

export const MermaidBlock: React.FC<MermaidBlockProps> = ({ code }) => {
  const { t, i18n } = useTranslation();
  const isEn = i18n?.language?.startsWith("en") || false;

  // Stable ID that never changes across re-renders for this component instance
  const idRef = useRef<string | null>(null);
  if (!idRef.current) {
    idRef.current = `mermaid-${++mermaidCounter}`;
  }
  const diagramId = idRef.current;

  const containerRef = useRef<HTMLDivElement | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [showSource, setShowSource] = useState(false);
  const [copied, setCopied] = useState(false);
  const [rendered, setRendered] = useState(false);

  const cleanCode = code.trim();

  useEffect(() => {
    if (!containerRef.current) return;

    let cancelled = false;
    setError(null);
    setRendered(false);

    const render = async () => {
      try {
        initMermaid();
        const { svg } = await mermaid.render(diagramId, cleanCode);
        if (cancelled || !containerRef.current) return;
        containerRef.current.innerHTML = svg;
        // Make SVG responsive
        const svgEl = containerRef.current.querySelector("svg");
        if (svgEl) {
          svgEl.style.maxWidth = "100%";
          svgEl.style.height = "auto";
        }
        setRendered(true);
      } catch (err: unknown) {
        if (cancelled) return;
        const msg =
          err instanceof Error ? err.message : "Failed to render diagram";
        setError(msg);
        setRendered(true); // stop loading spinner
      } finally {
        // Clean up any detached DOM nodes mermaid may have appended to <body>
        const stray = document.getElementById(diagramId);
        if (stray && !containerRef.current?.contains(stray)) {
          stray.remove();
        }
      }
    };

    render();
    return () => {
      cancelled = true;
    };
  }, [cleanCode, diagramId]);

  const handleCopy = async () => {
    await navigator.clipboard.writeText(cleanCode);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="group relative my-4 overflow-hidden rounded-2xl border border-primary/25 bg-card/90 backdrop-blur-md shadow-lg transition-all duration-300">
      {/* Header Bar */}
      <div className="flex items-center justify-between border-b border-border/60 bg-muted/40 px-4 py-2.5 text-xs">
        <div className="flex items-center gap-2">
          <span className="flex h-2 w-2 rounded-full bg-indigo-500 animate-pulse" />
          <span className="font-mono text-[11px] font-bold uppercase tracking-wider text-indigo-600 dark:text-indigo-400">
            {isEn ? "Architecture Diagram" : "Sơ đồ kiến trúc (Mermaid)"}
          </span>
        </div>

        <div className="flex items-center gap-1.5">
          {/* Toggle View Mode Button */}
          {!error && (
            <button
              type="button"
              onClick={() => setShowSource((s) => !s)}
              className="flex items-center gap-1 rounded-lg px-2.5 py-1 text-[11px] font-semibold text-muted-foreground hover:bg-accent hover:text-foreground transition-colors cursor-pointer"
              title={showSource ? "View Diagram" : "View Code"}
            >
              {showSource ? (
                <>
                  <EyeIcon className="h-3.5 w-3.5 text-primary" />
                  <span>{isEn ? "Diagram" : "Sơ đồ"}</span>
                </>
              ) : (
                <>
                  <CodeBracketIcon className="h-3.5 w-3.5 text-muted-foreground" />
                  <span>{isEn ? "Source Code" : "Mã nguồn"}</span>
                </>
              )}
            </button>
          )}

          {/* Copy Button */}
          <button
            type="button"
            onClick={handleCopy}
            className="flex items-center gap-1 rounded-lg px-2.5 py-1 text-[11px] font-medium text-muted-foreground hover:bg-accent hover:text-foreground transition-colors cursor-pointer"
          >
            {copied ? (
              <>
                <CheckIcon className="h-3.5 w-3.5 text-emerald-500" />
                <span className="text-emerald-500 font-bold">
                  {t("copied", "Đã sao chép")}
                </span>
              </>
            ) : (
              <>
                <ClipboardDocumentIcon className="h-3.5 w-3.5" />
                <span>{t("copy", "Sao chép")}</span>
              </>
            )}
          </button>
        </div>
      </div>

      {/* Main Content Area */}
      {showSource || error ? (
        <div className="relative">
          {error && (
            <div className="flex items-center gap-2 border-b border-amber-500/20 bg-amber-500/10 px-4 py-2 text-xs text-amber-600 dark:text-amber-400">
              <ExclamationTriangleIcon className="h-4 w-4 shrink-0" />
              <span>
                {isEn
                  ? "Displaying source code (syntax preview):"
                  : "Hiển thị mã nguồn (xem trước cú pháp):"}
              </span>
            </div>
          )}
          <pre className="scroll-fine max-h-[600px] overflow-x-auto p-4 text-[13px] font-mono leading-relaxed bg-[#0d1117] text-slate-200">
            <code>{cleanCode}</code>
          </pre>
        </div>
      ) : (
        <div className="relative flex min-h-[140px] items-center justify-center p-4 sm:p-6 overflow-x-auto bg-background/50">
          {/* Loading spinner — hidden once rendered */}
          {!rendered && (
            <div className="absolute inset-0 flex flex-col items-center justify-center gap-2.5 text-muted-foreground pointer-events-none">
              <div className="h-6 w-6 animate-spin rounded-full border-2 border-primary border-t-transparent" />
              <span className="text-xs font-medium">
                {isEn ? "Rendering diagram..." : "Đang vẽ sơ đồ..."}
              </span>
            </div>
          )}
          {/* Mermaid renders directly into this div via containerRef */}
          <div
            ref={containerRef}
            className={`mermaid-svg-container flex justify-center w-full max-w-full overflow-x-auto transition-opacity duration-300 ${rendered ? "opacity-100" : "opacity-0"}`}
          />
        </div>
      )}
    </div>
  );
};

export default MermaidBlock;
