import { useState, useMemo } from "react";
import ReactMarkdown, { type Components } from "react-markdown";
import remarkGfm from "remark-gfm";
import { ClipboardDocumentIcon, CheckIcon } from "@heroicons/react/24/outline";

function detectLanguage(code: string, givenLang?: string): string {
  if (givenLang && givenLang !== "code") return givenLang;
  const trimmed = code.trim();
  if (trimmed.startsWith("#!") || trimmed.includes("cat <<") || trimmed.includes("chmod +x") || trimmed.includes("mkdir -p")) {
    return "bash";
  }
  if (trimmed.includes("package main") || trimmed.includes("func main()") || trimmed.includes("import (")) {
    return "go";
  }
  if (trimmed.startsWith("{") && trimmed.endsWith("}")) {
    return "json";
  }
  if (trimmed.includes("import React") || trimmed.includes("export default") || trimmed.includes("const ") || trimmed.includes("interface ")) {
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
              <span className="text-emerald-400 font-semibold">Copied</span>
            </>
          ) : (
            <>
              <ClipboardDocumentIcon className="h-3.5 w-3.5" />
              <span>Copy</span>
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
    return (
      <CodeBlock
        language={match?.[1]}
        code={codeStr}
      />
    );
  },
  pre: ({ children }) => <>{children}</>,
  h1: ({ children }) => (
    <h1 className="mt-6 mb-3 text-xl font-bold text-foreground">
      {children}
    </h1>
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
    <p className="my-2 leading-relaxed text-foreground">
      {children}
    </p>
  ),
  strong: ({ children }) => (
    <strong className="font-bold text-foreground">
      {children}
    </strong>
  ),
  em: ({ children }) => (
    <em className="italic text-foreground">
      {children}
    </em>
  ),
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
 * Comprehensive markdown normalizer that fixes:
 * 1. Headings/code blocks glued to prose
 * 2. Em-dash (——) used instead of ASCII dash (---) in table separators
 * 3. All table rows concatenated on a single line
 * 4. Missing GFM separator row after table header
 * 5. Bare `---` used as table separator instead of `|---|---|`
 * 6. Double pipes `||` used as row boundaries
 */
function normalizeMarkdown(text: string): string {
  if (!text) return text;
  let s = text;

  // ── Phase 1: Structural breaks ──
  // Fix headings attached to previous sentence: "word.## Heading" -> "word.\n\n## Heading"
  s = s.replace(/([^\n])(#{1,6}\s+)/g, "$1\n\n$2");
  // Fix code blocks attached to previous text: "word.```bash" -> "word.\n\n```bash"
  s = s.replace(/([^\n])(```[a-zA-Z]*)/g, "$1\n\n$2");
  // Fix bullet lists attached directly: "word.- item" -> "word.\n\n- item"
  s = s.replace(/([a-zA-Z0-9.?!:])([*-]\s+[A-Z0-9])/g, "$1\n\n$2");

  // ── Phase 2: Table normalization ──
  if (s.includes("|")) {
    // 2a. Replace Unicode em-dash (—, U+2014) and en-dash (–, U+2013) with ASCII hyphen
    //     in contexts that look like table separators: |——|, |—— |, |——— etc.
    s = s.replace(/\|[\s]*[—–][-—–]*[\s]*/g, (m) =>
      m.replace(/[—–]/g, "-")
    );

    // 2b. Split concatenated double-pipes: `||` -> `|\n|`
    //     But skip if inside a code block (lines starting with spaces/tabs for indented code)
    s = s.replace(/\|\|/g, "|\n|");

    // 2c. Process line by line to fix table structure
    const lines = s.split("\n");
    const result: string[] = [];

    for (let i = 0; i < lines.length; i++) {
      let line = lines[i];

      // Skip empty lines and non-table lines
      if (!line.includes("|")) {
        result.push(line);
        continue;
      }

      // Trim the line for analysis
      const trimmed = line.trim();

      // If a line starts with `|` and contains mixed content + separator patterns,
      // e.g. `| Header1 | Header2 |--- |---| Data1 | Data2 |`
      // Split it into proper rows
      if (trimmed.startsWith("|")) {
        // Check if this single line contains a separator segment (|---|) mixed with data
        const sepPattern = /\|\s*:?-{2,}:?\s*(?=\|)/g;
        const hasSep = sepPattern.test(trimmed);

        if (hasSep && trimmed.replace(/\|\s*:?-{2,}:?\s*/g, "").replace(/\|/g, "").trim().length > 0) {
          // This line has both separator dashes AND text content -> needs splitting
          // Strategy: find the separator segment and split around it
          const parts = splitTableLine(trimmed);
          for (const part of parts) {
            if (part.trim()) result.push(part);
          }
          continue;
        }

        // Ensure the line ends with `|`
        if (!trimmed.endsWith("|")) {
          line = line.trimEnd() + " |";
        }
      }

      result.push(line);
    }

    s = result.join("\n");

    // 2d. After splitting, check for missing GFM separator rows
    // If we see a line like `| Header1 | Header2 |` followed by a line that is NOT
    // a separator (|---|---|) and IS another data row, insert a separator.
    s = insertMissingSeparators(s);

    // 2e. Fix bare `---` right after a table header row -> convert to proper separator
    s = s.replace(
      /^(\|(?:[^|\n]+\|)+)\s*\n---+\s*$/gm,
      (_, headerRow) => {
        const colCount = (headerRow.match(/\|/g) || []).length - 1;
        const sep = "|" + " --- |".repeat(Math.max(colCount, 1));
        return headerRow + "\n" + sep;
      }
    );

    // 2f. Fix table header starting directly on prose: "text| Header |" -> "text\n\n| Header |"
    s = s.replace(/([^\n|])(\s*\|(?:\s*[^|\n]+\s*\|)+)/g, "$1\n\n$2");
  }

  return fixUnclosedCodeBlocks(s);
}

/**
 * Split a single concatenated table line into multiple rows.
 * E.g. `| A | B |---| C | D |` -> [`| A | B |`, `|---|`, `| C | D |`]
 */
function splitTableLine(line: string): string[] {
  const rows: string[] = [];
  // Tokenize by `|`, keeping track of cell content
  const cells = line.split("|").map((c) => c.trim());
  // cells[0] is before first |, cells[last] is after last |
  // Filter out empty leading/trailing
  let currentRow: string[] = [];
  let inSeparator = false;

  for (let i = 1; i < cells.length - 1; i++) {
    const cell = cells[i];
    const isSepCell = /^:?-{2,}:?$/.test(cell.trim());

    if (isSepCell && !inSeparator && currentRow.length > 0) {
      // Flush current data row
      rows.push("| " + currentRow.join(" | ") + " |");
      currentRow = [];
      inSeparator = true;
    }

    if (isSepCell) {
      currentRow.push(cell);
      inSeparator = true;
    } else {
      if (inSeparator && currentRow.length > 0) {
        // Flush separator row
        rows.push("| " + currentRow.join(" | ") + " |");
        currentRow = [];
        inSeparator = false;
      }
      currentRow.push(cell);
      inSeparator = false;
    }
  }

  // Flush remaining
  if (currentRow.length > 0) {
    rows.push("| " + currentRow.join(" | ") + " |");
  }

  return rows;
}

/**
 * Insert missing GFM separator rows.
 * If a `| ... |` line is followed by another `| ... |` data line (not a separator),
 * and there's no separator before it, insert `|---|---|` between them.
 * Only does this for the FIRST pair (header + first data row).
 */
function insertMissingSeparators(text: string): string {
  const lines = text.split("\n");
  const result: string[] = [];
  let inTable = false;
  let tableHasSep = false;

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    const trimmed = line.trim();
    const isTableRow = trimmed.startsWith("|") && trimmed.endsWith("|") && trimmed.length > 1;
    const isSepRow = isTableRow && /^\|(\s*:?-{2,}:?\s*\|)+$/.test(trimmed);

    if (isTableRow) {
      if (!inTable) {
        // Starting a new table block
        inTable = true;
        tableHasSep = false;
        result.push(line);
        continue;
      }

      if (isSepRow) {
        tableHasSep = true;
        result.push(line);
        continue;
      }

      // It's a data row inside a table
      if (!tableHasSep) {
        // No separator seen yet -> insert one before this row
        const prevLine = result[result.length - 1]?.trim() || "";
        if (prevLine.startsWith("|") && prevLine.endsWith("|")) {
          const colCount = Math.max((prevLine.match(/\|/g) || []).length - 1, 1);
          result.push("|" + " --- |".repeat(colCount));
          tableHasSep = true;
        }
      }

      result.push(line);
    } else {
      // Not a table row -> reset table tracking
      inTable = false;
      tableHasSep = false;
      result.push(line);
    }
  }

  return result.join("\n");
}

/**
 * Markdown component rendering formatted assistant text with GFM table & code block support.
 * Automatically fixes unclosed code blocks and missing head/block newlines.
 */
export const Markdown: React.FC<{ content: string }> = ({ content }) => {
  const normalizedContent = useMemo(
    () => normalizeMarkdown(content),
    [content]
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
