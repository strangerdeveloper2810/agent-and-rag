import React, { useEffect, useId, useState } from "react";
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

export const MermaidBlock: React.FC<MermaidBlockProps> = ({ code }) => {
  const { t, i18n } = useTranslation();
  const isEn = i18n?.language?.startsWith("en") || false;
  const reactId = useId().replace(/:/g, "_");
  const uniqueId = `mermaid_${reactId}_${Math.random().toString(36).substring(2, 7)}`;

  const [svgHtml, setSvgHtml] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [showSource, setShowSource] = useState(false);
  const [copied, setCopied] = useState(false);
  const [loading, setLoading] = useState(true);

  const cleanCode = code.trim();

  useEffect(() => {
    let isMounted = true;
    setLoading(true);
    setError(null);

    const renderDiagram = async () => {
      try {
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

        // Use mermaid.render to generate SVG
        const { svg } = await mermaid.render(uniqueId, cleanCode);
        if (isMounted) {
          setSvgHtml(svg);
          setLoading(false);
        }
      } catch (err: any) {
        if (isMounted) {
          setError(err?.message || "Failed to render diagram");
          setLoading(false);
        }
      }
    };

    renderDiagram();

    return () => {
      isMounted = false;
      // Clean up any lingering temporary DOM nodes created by mermaid
      const el = document.getElementById(uniqueId);
      if (el) el.remove();
      const parentEl = document.getElementById(`d${uniqueId}`);
      if (parentEl) parentEl.remove();
    };
  }, [cleanCode, uniqueId]);

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
        <div className="flex min-h-[140px] items-center justify-center p-4 sm:p-6 overflow-x-auto bg-background/50">
          {loading ? (
            <div className="flex flex-col items-center justify-center gap-2.5 py-8 text-muted-foreground">
              <div className="h-6 w-6 animate-spin rounded-full border-2 border-primary border-t-transparent" />
              <span className="text-xs font-medium">
                {isEn ? "Rendering diagram..." : "Đang vẽ sơ đồ..."}
              </span>
            </div>
          ) : svgHtml ? (
            <div
              className="mermaid-svg-container flex justify-center w-full max-w-full overflow-x-auto transition-all"
              dangerouslySetInnerHTML={{ __html: svgHtml }}
            />
          ) : null}
        </div>
      )}
    </div>
  );
};

export default MermaidBlock;
