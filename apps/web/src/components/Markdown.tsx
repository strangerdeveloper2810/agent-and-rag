import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

/**
 * Render nội dung markdown (câu trả lời của Claude) thành HTML.
 * - remark-gfm: hỗ trợ bảng, gạch ngang, checklist, autolink...
 * - class `prose` (Tailwind typography): style sẵn heading/list/code/quote...
 *   `prose-sm` cỡ nhỏ vừa khung chat, `max-w-none` bỏ giới hạn chiều rộng
 *   của prose (đã giới hạn bằng max-w-2xl ở ngoài).
 */
export default function Markdown({ content }: { content: string }) {
  return (
    <div className="prose prose-sm max-w-none prose-pre:bg-gray-800 prose-pre:text-gray-100">
      <ReactMarkdown remarkPlugins={[remarkGfm]}>{content}</ReactMarkdown>
    </div>
  );
}
