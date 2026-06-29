import ReactMarkdown, { type Components } from "react-markdown";
import remarkGfm from "remark-gfm";

// Bảng tự cuộn ngang trên màn hẹp (tránh vỡ layout mobile)
const components: Components = {
  table: ({ children }) => (
    <div className="overflow-x-auto">
      <table>{children}</table>
    </div>
  ),
};

/**
 * Render markdown câu trả lời của Claude theo phong cách Google/Gemini:
 * chữ ink, link xanh Google, inline-code nền xám nhạt, code block nền tối nhẹ.
 */
export default function Markdown({ content }: { content: string }) {
  return (
    <div
      className="prose prose-sm max-w-none break-words text-ink
        prose-p:my-2 prose-p:leading-relaxed prose-p:text-ink
        prose-headings:font-medium prose-headings:text-ink
        prose-strong:text-ink prose-strong:font-semibold
        prose-a:text-gblue prose-a:no-underline hover:prose-a:underline
        prose-li:my-0.5 prose-ul:my-2 prose-ol:my-2 prose-li:text-ink
        prose-hr:border-line prose-hr:my-4
        prose-blockquote:border-l-2 prose-blockquote:border-line prose-blockquote:not-italic prose-blockquote:text-ink-soft
        prose-code:rounded prose-code:bg-subtle prose-code:px-1.5 prose-code:py-0.5
        prose-code:text-[0.85em] prose-code:text-ink prose-code:font-normal
        prose-code:before:content-[''] prose-code:after:content-['']
        prose-pre:bg-[#1e1f22] prose-pre:text-[#e8eaed] prose-pre:rounded-2xl
        prose-table:text-sm prose-th:text-ink prose-td:border-line"
    >
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={components}>
        {content}
      </ReactMarkdown>
    </div>
  );
}
