import { useState, useMemo } from "react";
import ReactMarkdown, { type Components } from "react-markdown";
import remarkGfm from "remark-gfm";
import { ClipboardDocumentIcon, CheckIcon } from "@heroicons/react/24/outline";
import { useTranslation } from "react-i18next";
import { MermaidBlock } from "@/design-system/molecules/MermaidBlock";

function isMermaidCode(code: string, givenLang?: string): boolean {
  if (givenLang?.toLowerCase() === "mermaid") return true;
  const trimmed = code.trim();
  return (
    trimmed.startsWith("graph ") ||
    trimmed.startsWith("flowchart ") ||
    trimmed.startsWith("sequenceDiagram") ||
    trimmed.startsWith("classDiagram") ||
    trimmed.startsWith("stateDiagram") ||
    trimmed.startsWith("erDiagram") ||
    trimmed.startsWith("gantt") ||
    trimmed.startsWith("pie") ||
    trimmed.startsWith("mindmap") ||
    trimmed.startsWith("gitGraph") ||
    trimmed.startsWith("C4Context")
  );
}

function detectLanguage(code: string, givenLang?: string): string {
  if (givenLang && givenLang !== "code") return givenLang;
  const trimmed = code.trim();
  if (
    trimmed.startsWith("#!") ||
    trimmed.includes("cat <<") ||
    trimmed.includes("chmod +x") ||
    trimmed.includes("mkdir -p")
  ) {
    return "bash";
  }
  if (
    trimmed.includes("package main") ||
    trimmed.includes("func main()") ||
    trimmed.includes("import (")
  ) {
    return "go";
  }
  if (trimmed.startsWith("{") && trimmed.endsWith("}")) {
    return "json";
  }
  if (
    trimmed.includes("import React") ||
    trimmed.includes("export default") ||
    trimmed.includes("const ") ||
    trimmed.includes("interface ")
  ) {
    return "typescript";
  }
  return givenLang || "code";
}

function fixUnclosedCodeBlocks(markdown: string): string {
  const backtickMatches = markdown.match(/```/g);
  if (backtickMatches && backtickMatches.length % 2 !== 0) {
    return markdown + "\n```";
  }
  return markdown;
}

/**
 * CodeBlock component rendering syntax-highlighted code with copy to clipboard button.
 */
const CodeBlock: React.FC<{ language?: string; code: string }> = ({
  language,
  code,
}) => {
  const { t } = useTranslation();
  const [copied, setCopied] = useState(false);
  const detectedLang = detectLanguage(code, language);

  const handleCopy = async () => {
    await navigator.clipboard.writeText(code);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="group relative my-3.5 overflow-hidden rounded-2xl border border-border bg-[#0d1117] text-slate-100 shadow-md">
      <div className="flex items-center justify-between border-b border-white/10 bg-white/5 px-4 py-2 text-xs">
        <span className="font-mono text-[11px] font-bold uppercase tracking-wider text-indigo-400">
          {detectedLang}
        </span>
        <button
          type="button"
          onClick={handleCopy}
          className="flex items-center gap-1 rounded-lg px-2.5 py-1 text-[11px] font-medium text-slate-300 transition hover:bg-white/10 hover:text-white"
        >
          {copied ? (
            <>
              <CheckIcon className="h-3.5 w-3.5 text-emerald-400" />
              <span className="text-emerald-400 font-semibold">
                {t("copied")}
              </span>
            </>
          ) : (
            <>
              <ClipboardDocumentIcon className="h-3.5 w-3.5" />
              <span>{t("copy")}</span>
            </>
          )}
        </button>
      </div>
      <pre className="scroll-fine max-h-[600px] overflow-x-auto p-4 text-[13px] font-mono leading-relaxed text-slate-200">
        <code>{code}</code>
      </pre>
    </div>
  );
};

const components: Components = {
  table: ({ children }) => (
    <div
      className="my-4 overflow-x-auto rounded-xl border"
      style={{ borderColor: "var(--border)" }}
    >
      <table className="min-w-full text-sm">{children}</table>
    </div>
  ),
  thead: ({ children }) => (
    <thead
      style={{
        borderBottom: "1px solid var(--border)",
        backgroundColor: "var(--bg-raised)",
      }}
    >
      {children}
    </thead>
  ),
  th: ({ children }) => (
    <th
      className="px-4 py-2.5 text-left text-xs font-semibold tracking-wider"
      style={{ color: "var(--accent)" }}
    >
      {children}
    </th>
  ),
  td: ({ children }) => (
    <td
      className="px-4 py-2.5"
      style={{ borderTop: "1px solid var(--border)", color: "var(--text)" }}
    >
      {children}
    </td>
  ),
  code: ({ className, children, ...props }) => {
    const match = /language-(\w+)/.exec(className || "");
    const codeStr = String(children).replace(/\n$/, "");
    const isInline = !match && !codeStr.includes("\n");

    if (isInline) {
      return (
        <code
          className="rounded-md px-1.5 py-0.5 font-mono text-[0.85em] bg-primary/10 text-primary border border-primary/20"
          {...props}
        >
          {children}
        </code>
      );
    }

    const lang = match?.[1];
    if (isMermaidCode(codeStr, lang)) {
      return <MermaidBlock code={codeStr} />;
    }

    return <CodeBlock language={lang} code={codeStr} />;
  },
  pre: ({ children }) => <>{children}</>,
  h1: ({ children }) => (
    <h1 className="mt-6 mb-3 text-xl font-bold text-foreground">{children}</h1>
  ),
  h2: ({ children }) => (
    <h2 className="mt-5 mb-2.5 text-lg font-semibold pb-1.5 text-foreground border-b border-border">
      {children}
    </h2>
  ),
  h3: ({ children }) => (
    <h3 className="mt-4 mb-2 text-base font-semibold text-foreground">
      {children}
    </h3>
  ),
  h4: ({ children }) => (
    <h4 className="mt-3 mb-1.5 text-sm font-medium text-muted-foreground">
      {children}
    </h4>
  ),
  a: ({ href, children }) => (
    <a
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      className="text-primary underline underline-offset-4 hover:opacity-80 transition"
    >
      {children}
    </a>
  ),
  ul: ({ children }) => (
    <ul className="my-3 space-y-1.5 list-disc list-inside text-foreground">
      {children}
    </ul>
  ),
  ol: ({ children }) => (
    <ol className="my-3 space-y-1.5 list-decimal list-inside text-foreground">
      {children}
    </ol>
  ),
  li: ({ children }) => <li className="leading-relaxed">{children}</li>,
  blockquote: ({ children }) => (
    <blockquote className="my-4 border-l-4 border-primary bg-primary/5 rounded-r-xl px-4 py-3 italic text-muted-foreground">
      {children}
    </blockquote>
  ),
  hr: () => (
    <hr className="my-5 border-0 h-px bg-gradient-to-r from-transparent via-border to-transparent" />
  ),
  p: ({ children }) => (
    <p className="my-2 leading-relaxed text-foreground">{children}</p>
  ),
  strong: ({ children }) => (
    <strong className="font-bold text-foreground">{children}</strong>
  ),
  em: ({ children }) => <em className="italic text-foreground">{children}</em>,
  img: ({ src, alt }) => (
    <img
      src={src}
      alt={alt}
      className="my-3 max-w-full rounded-2xl border border-border shadow-sm"
      loading="lazy"
    />
  ),
};

/**
 * Comprehensive Markdown Normalizer for LLM outputs:
 * 1. Inserts line breaks before headings (#), lists (- / *), code blocks (```)
 * 2. Unifies Unicode em/en dashes (— / –) to ASCII hyphens in tables
 * 3. Reconstructs all tables into valid, clean GitHub Flavored Markdown (GFM) tables
 * 4. Fixes unclosed code blocks
 */
function normalizeMarkdown(text: string): string {
  if (!text) return text;
  let s = text;

  // 1. Structural line breaks around Markdown blocks
  // Marker (bullet "-"/"*" hoặc số thứ tự "1."/"1)") bị tách khỏi nội dung bởi
  // (một hay nhiều) dòng trống -> gộp lại thành 1 dòng để CommonMark parse đúng
  // thành 1 list item duy nhất thay vì 1 item RỖNG + 1 paragraph rời rạc phía sau:
  // "1.\n\n**text**" -> "1. **text**"
  // Lưu ý: KHÔNG đụng tới horizontal rule ("---"/"***" đứng một mình trên dòng)
  // vì [-*] trong group chỉ khớp ĐÚNG 1 ký tự; ký tự thứ 2 của "---"/"***" không
  // phải khoảng trắng/newline nên `\n+` ngay sau đó không match được -> an toàn.
  s = s.replace(/^([ \t]*(?:\d{1,3}[.)]|[-*]))[ \t]*\n+(?=\S)/gm, "$1 ");
  // Headings attached to previous sentence: "word.## Heading" -> "word.\n\n## Heading"
  s = s.replace(/([^\n])(#{1,6}\s+)/g, "$1\n\n$2");
  // Code blocks attached to text: "word.```bash" -> "word.\n\n```bash"
  s = s.replace(/([^\n])(```[a-zA-Z]*)/g, "$1\n\n$2");
  // Lists attached to text: "word.- item" -> "word.\n\n- item"
  s = s.replace(/([a-zA-Z0-9.?!:])([*-]\s+[A-Z0-9])/g, "$1\n\n$2");

  // 2. Table normalization
  if (s.includes("|")) {
    s = fixAllTablesInMarkdown(s);
  }

  return fixUnclosedCodeBlocks(s);
}

/**
 * Reconstructs all tables in the Markdown document into valid GFM tables.
 * Handles single-line, multi-line, and double-spaced table rows seamlessly.
 */
function fixAllTablesInMarkdown(text: string): string {
  // Normalize unicode dashes (— / –) to ASCII hyphen
  const s = text.replace(/\|[\s]*[—–][-—–]*[\s]*/g, (m) =>
    m.replace(/[—–]/g, "-"),
  );

  const lines = s.split("\n");
  const result: string[] = [];
  let i = 0;

  while (i < lines.length) {
    const line = lines[i];

    // Check if this line starts a table block
    if (line.includes("|")) {
      // Gather all lines belonging to this table region (allowing blank lines between table rows)
      const tableLines: string[] = [];
      let j = i;

      while (j < lines.length) {
        const curLine = lines[j];
        if (curLine.includes("|")) {
          tableLines.push(curLine);
          j++;
        } else if (curLine.trim() === "") {
          // Empty line: check if the next non-empty line still has `|` (table continuation)
          let lookahead = j + 1;
          while (lookahead < lines.length && lines[lookahead].trim() === "") {
            lookahead++;
          }
          if (lookahead < lines.length && lines[lookahead].includes("|")) {
            // Next non-empty line is still part of table -> advance to it
            j = lookahead;
          } else {
            // End of table block
            break;
          }
        } else {
          // Non-table text line encountered -> end of table block
          break;
        }
      }

      const tableBlock = tableLines.join("\n");
      const reconstructed = formatSingleTableBlock(tableBlock);
      if (reconstructed) {
        result.push(reconstructed);
      } else {
        result.push(tableBlock);
      }

      i = j;
    } else {
      result.push(line);
      i++;
    }
  }

  return result.join("\n");
}

function formatSingleTableBlock(tableBlock: string): string | null {
  const firstPipe = tableBlock.indexOf("|");
  const lastPipe = tableBlock.lastIndexOf("|");
  if (firstPipe === -1 || lastPipe === -1 || firstPipe === lastPipe)
    return null;

  const preText = tableBlock.slice(0, firstPipe).trim();
  const postText = tableBlock.slice(lastPipe + 1).trim();
  const tablePart = tableBlock.slice(firstPipe, lastPipe + 1);

  // Extract all cell tokens strictly between pipes `|`
  const rawTokens = tablePart
    .split("|")
    .map((t) => t.trim())
    .filter((t) => t.length > 0);

  if (rawTokens.length < 4) return null;

  // Check if there is a separator cell (e.g. "---", "--", ":---:", ":---")
  const isSepToken = (t: string) => /^:?-{2,}:?$/.test(t);
  const firstSepIdx = rawTokens.findIndex(isSepToken);

  if (firstSepIdx <= 0) {
    return null;
  }

  // Find end of separator tokens
  let sepEndIdx = firstSepIdx;
  while (sepEndIdx < rawTokens.length && isSepToken(rawTokens[sepEndIdx])) {
    sepEndIdx++;
  }
  const numSepCols = sepEndIdx - firstSepIdx;

  // Header tokens are everything before the first separator
  const headerTokens = rawTokens.slice(0, firstSepIdx);

  // If separator has more columns than headers (e.g. empty first corner cell `| | Col1 | Col2 |`),
  // pad headers at the beginning so column count matches separator!
  if (headerTokens.length < numSepCols) {
    while (headerTokens.length < numSepCols) {
      headerTokens.unshift("");
    }
  }

  const numCols = Math.max(headerTokens.length, numSepCols);
  if (numCols < 2) return null;

  // Data tokens are everything after separator
  const dataTokens = rawTokens.slice(sepEndIdx);

  // Build the clean GFM table
  const headerRow = "| " + headerTokens.join(" | ") + " |";
  const sepRow = "|" + " --- |".repeat(numCols);
  const dataRows: string[] = [];

  for (let i = 0; i < dataTokens.length; i += numCols) {
    const chunk = dataTokens.slice(i, i + numCols);
    while (chunk.length < numCols) {
      chunk.push("");
    }
    dataRows.push("| " + chunk.join(" | ") + " |");
  }

  const tableStr = [headerRow, sepRow, ...dataRows].join("\n");
  return [preText, tableStr, postText].filter(Boolean).join("\n\n");
}

/**
 * Markdown component rendering formatted assistant text with GFM table & code block support.
 * Automatically fixes unclosed code blocks and missing head/block newlines.
 */
export const Markdown: React.FC<{ content: string }> = ({ content }) => {
  const normalizedContent = useMemo(
    () => normalizeMarkdown(content),
    [content],
  );

  return (
    <div className="text-sm leading-relaxed">
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={components}>
        {normalizedContent}
      </ReactMarkdown>
    </div>
  );
};

export default Markdown;
