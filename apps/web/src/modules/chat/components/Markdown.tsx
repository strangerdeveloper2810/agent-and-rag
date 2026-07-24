import { useState } from "react";
import ReactMarkdown, { type Components } from "react-markdown";
import remarkGfm from "remark-gfm";

function CodeBlock({ language, code }: { language?: string; code: string }) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    await navigator.clipboard.writeText(code);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="group relative my-4 rounded-xl border border-[var(--cyber-border)] bg-[#0a0a0f] overflow-hidden">
      {/* Header bar */}
      <div className="flex items-center justify-between px-4 py-2 border-b border-[var(--cyber-border)] bg-[#0d0d14]">
        <div className="flex items-center gap-2">
          <span className="h-2.5 w-2.5 rounded-full bg-[var(--cyber-error)]" />
          <span className="h-2.5 w-2.5 rounded-full bg-[var(--cyber-accent)]" />
          <span className="h-2.5 w-2.5 rounded-full bg-[var(--cyber-success)]" />
          {language && (
            <span className="ml-3 text-[11px] uppercase tracking-wider text-[var(--cyber-muted)]">
              {language}
            </span>
          )}
        </div>
        <button
          onClick={handleCopy}
          className="flex items-center gap-1.5 rounded-md px-2.5 py-1 text-[11px] text-[var(--cyber-muted)] hover:text-[var(--cyber-primary)] hover:bg-[var(--cyber-primary-soft)] transition-all"
        >
          {copied ? (
            <>
              <CheckIcon /> Copied
            </>
          ) : (
            <>
              <CopyIcon /> Copy
            </>
          )}
        </button>
      </div>
      {/* Code */}
      <pre className="overflow-x-auto p-4 text-[13px] leading-relaxed">
        <code className={`language-${language || "text"} text-[var(--cyber-success)]`}>
          {code}
        </code>
      </pre>
    </div>
  );
}

function CopyIcon() {
  return (
    <svg width={13} height={13} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.6}>
      <rect x={9} y={9} width={13} height={13} rx={2} />
      <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
    </svg>
  );
}

function CheckIcon() {
  return (
    <svg width={13} height={13} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2}>
      <path d="M20 6L9 17l-5-5" />
    </svg>
  );
}

const cyberComponents: Components = {
  // Tables with horizontal scroll + neon styling
  table: ({ children }) => (
    <div className="my-4 overflow-x-auto rounded-lg border border-[var(--cyber-border)]">
      <table className="min-w-full text-sm">{children}</table>
    </div>
  ),
  thead: ({ children }) => (
    <thead className="border-b border-[var(--cyber-border)] bg-[var(--cyber-subtle)]">
      {children}
    </thead>
  ),
  th: ({ children }) => (
    <th className="px-4 py-2.5 text-left text-xs font-medium uppercase tracking-wider text-[var(--cyber-primary)]">
      {children}
    </th>
  ),
  td: ({ children }) => (
    <td className="px-4 py-2.5 border-t border-[var(--cyber-border)] text-[var(--cyber-text)]">
      {children}
    </td>
  ),

  // Code blocks
  code: ({ className, children, ...props }) => {
    const match = /language-(\w+)/.exec(className || "");
    const isInline = !match && !String(children).includes("\n");

    if (isInline) {
      return (
        <code
          className="rounded-md bg-[var(--cyber-subtle2)] px-1.5 py-0.5 text-[0.88em] text-[var(--cyber-primary)] border border-[var(--cyber-border)]"
          {...props}
        >
          {children}
        </code>
      );
    }

    return (
      <CodeBlock
        language={match ? match[1] : undefined}
        code={String(children).replace(/\n$/, "")}
      />
    );
  },

  pre: ({ children }) => <>{children}</>,

  // Headings
  h1: ({ children }) => (
    <h1 className="mt-6 mb-3 text-xl font-bold text-[var(--cyber-primary)] neon-text">
      {children}
    </h1>
  ),
  h2: ({ children }) => (
    <h2 className="mt-5 mb-2.5 text-lg font-semibold text-[var(--cyber-primary)] border-b border-[var(--cyber-border)] pb-1.5">
      {children}
    </h2>
  ),
  h3: ({ children }) => (
    <h3 className="mt-4 mb-2 text-base font-medium text-[var(--cyber-text)]">
      ▸ {children}
    </h3>
  ),
  h4: ({ children }) => (
    <h4 className="mt-3 mb-1.5 text-sm font-medium text-[var(--cyber-muted)]">
      {children}
    </h4>
  ),

  // Links
  a: ({ href, children }) => (
    <a
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      className="text-[var(--cyber-primary)] underline decoration-[var(--cyber-primary-soft)] hover:decoration-[var(--cyber-primary)] transition-all hover:neon-text"
    >
      {children}
    </a>
  ),

  // Lists
  ul: ({ children }) => (
    <ul className="my-3 space-y-1.5 list-none">{children}</ul>
  ),
  ol: ({ children }) => (
    <ol className="my-3 space-y-1.5 list-decimal list-inside">{children}</ol>
  ),
  li: ({ children }) => (
    <li className="text-[var(--cyber-text)] leading-relaxed pl-1">
      <span className="text-[var(--cyber-primary)] mr-1.5">›</span>
      {children}
    </li>
  ),

  // Blockquote
  blockquote: ({ children }) => (
    <blockquote className="my-4 border-l-2 border-[var(--cyber-primary)] bg-[var(--cyber-subtle)] rounded-r-lg px-4 py-3 text-[var(--cyber-muted)] italic">
      {children}
    </blockquote>
  ),

  // Horizontal rule
  hr: () => (
    <hr className="my-5 border-0 h-px bg-gradient-to-r from-transparent via-[var(--cyber-border)] to-transparent" />
  ),

  // Paragraphs
  p: ({ children }) => (
    <p className="my-2 leading-relaxed text-[var(--cyber-text)]">{children}</p>
  ),

  // Strong & emphasis
  strong: ({ children }) => (
    <strong className="font-semibold text-[var(--cyber-primary)]">{children}</strong>
  ),
  em: ({ children }) => (
    <em className="italic text-[var(--cyber-secondary)]">{children}</em>
  ),

  // Images
  img: ({ src, alt }) => (
    <img
      src={src}
      alt={alt}
      className="my-3 max-w-full rounded-xl border border-[var(--cyber-border)]"
      loading="lazy"
    />
  ),
};

export default function Markdown({ content }: { content: string }) {
  return (
    <div className="cyber-markdown text-sm leading-relaxed">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={cyberComponents}
      >
        {content}
      </ReactMarkdown>
    </div>
  );
}
