import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

/**
 * Render nội dung markdown (câu trả lời của Claude) thành HTML.
 * - remark-gfm: hỗ trợ bảng, gạch ngang, checklist, autolink...
 * - class `prose` (Tailwind typography) + các prose-* để khớp theme warm/teal:
 *   chữ màu ink, link accent, inline-code nền sand, code block nền ink tối.
 */
export default function Markdown({ content }: { content: string }) {
  return (
    <div
      className="prose prose-sm max-w-none
        prose-p:my-2 prose-p:leading-relaxed
        prose-headings:font-display prose-headings:tracking-tight prose-headings:text-ink
        prose-strong:text-ink prose-strong:font-semibold
        prose-a:text-accent prose-a:font-medium prose-a:no-underline hover:prose-a:underline
        prose-li:my-0.5 prose-ul:my-2 prose-ol:my-2
        prose-hr:border-line prose-hr:my-4
        prose-blockquote:border-l-2 prose-blockquote:border-accent/40 prose-blockquote:not-italic prose-blockquote:text-ink-soft
        prose-code:rounded prose-code:bg-accent-soft prose-code:px-1.5 prose-code:py-0.5
        prose-code:text-[0.85em] prose-code:text-accent-ink prose-code:font-medium
        prose-code:before:content-[''] prose-code:after:content-['']
        prose-pre:bg-ink prose-pre:text-paper prose-pre:rounded-xl prose-pre:shadow-soft
        prose-table:text-sm prose-th:text-ink prose-td:border-line"
    >
      <ReactMarkdown remarkPlugins={[remarkGfm]}>{content}</ReactMarkdown>
    </div>
  );
}
