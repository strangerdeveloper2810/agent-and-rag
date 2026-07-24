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
    <div
      className="group relative my-4 overflow-hidden rounded-lg border"
      style={{ borderColor: "var(--border)" }}
    >
      <div
        className="flex items-center justify-between px-4 py-2"
        style={{
          borderBottom: "1px solid var(--border)",
          backgroundColor: "var(--bg-raised)",
        }}
      >
        <span
          className="text-[11px] font-medium tracking-wider"
          style={{ color: "var(--text-secondary)" }}
        >
          {language || "code"}
        </span>
        <button
          onClick={handleCopy}
          className="rounded px-2 py-0.5 text-[11px] transition hover:bg-[var(--border)]"
          style={{ color: "var(--text-secondary)" }}
        >
          {copied ? "Copied" : "Copy"}
        </button>
      </div>
      <pre
        className="overflow-x-auto p-4 text-[13px] leading-relaxed"
        style={{ backgroundColor: "var(--bg-raised)", color: "var(--text)" }}
      >
        <code>{code}</code>
      </pre>
    </div>
  );
}

const components: Components = {
  table: ({ children }) => (
    <div
      className="my-4 overflow-x-auto rounded-lg border"
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
    const isInline = !match && !String(children).includes("\n");
    if (isInline)
      return (
        <code
          className="rounded px-1.5 py-0.5 text-[0.88em]"
          style={{
            backgroundColor: "var(--bg-raised)",
            color: "var(--accent)",
            border: "1px solid var(--border)",
          }}
          {...props}
        >
          {children}
        </code>
      );
    return (
      <CodeBlock
        language={match?.[1]}
        code={String(children).replace(/\n$/, "")}
      />
    );
  },
  pre: ({ children }) => <>{children}</>,
  h1: ({ children }) => (
    <h1
      className="mt-6 mb-3 text-xl font-bold"
      style={{ color: "var(--accent)" }}
    >
      {children}
    </h1>
  ),
  h2: ({ children }) => (
    <h2
      className="mt-5 mb-2.5 text-lg font-semibold pb-1.5"
      style={{
        color: "var(--accent)",
        borderBottom: "1px solid var(--border)",
      }}
    >
      {children}
    </h2>
  ),
  h3: ({ children }) => (
    <h3
      className="mt-4 mb-2 text-base font-medium"
      style={{ color: "var(--text)" }}
    >
      {children}
    </h3>
  ),
  h4: ({ children }) => (
    <h4
      className="mt-3 mb-1.5 text-sm font-medium"
      style={{ color: "var(--text-secondary)" }}
    >
      {children}
    </h4>
  ),
  a: ({ href, children }) => (
    <a
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      className="underline transition"
      style={{
        color: "var(--accent)",
        textDecorationColor: "var(--accent-bg)",
      }}
    >
      {children}
    </a>
  ),
  ul: ({ children }) => (
    <ul
      className="my-3 space-y-1.5 list-disc list-inside"
      style={{ color: "var(--text)" }}
    >
      {children}
    </ul>
  ),
  ol: ({ children }) => (
    <ol
      className="my-3 space-y-1.5 list-decimal list-inside"
      style={{ color: "var(--text)" }}
    >
      {children}
    </ol>
  ),
  li: ({ children }) => <li className="leading-relaxed">{children}</li>,
  blockquote: ({ children }) => (
    <blockquote
      className="my-4 border-l-2 rounded-r-lg px-4 py-3 italic"
      style={{
        borderColor: "var(--accent)",
        backgroundColor: "var(--bg-raised)",
        color: "var(--text-secondary)",
      }}
    >
      {children}
    </blockquote>
  ),
  hr: () => (
    <hr
      className="my-5 border-0 h-px"
      style={{
        background:
          "linear-gradient(to right, transparent, var(--border), transparent)",
      }}
    />
  ),
  p: ({ children }) => (
    <p className="my-2 leading-relaxed" style={{ color: "var(--text)" }}>
      {children}
    </p>
  ),
  strong: ({ children }) => (
    <strong className="font-semibold" style={{ color: "var(--accent)" }}>
      {children}
    </strong>
  ),
  em: ({ children }) => (
    <em className="italic" style={{ color: "var(--accent)" }}>
      {children}
    </em>
  ),
  img: ({ src, alt }) => (
    <img
      src={src}
      alt={alt}
      className="my-3 max-w-full rounded-lg border"
      style={{ borderColor: "var(--border)" }}
      loading="lazy"
    />
  ),
};

export default function Markdown({ content }: { content: string }) {
  return (
    <div className="text-sm leading-relaxed">
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={components}>
        {content}
      </ReactMarkdown>
    </div>
  );
}
