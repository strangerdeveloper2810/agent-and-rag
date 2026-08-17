import { RecursiveCharacterTextSplitter } from "@langchain/textsplitters";

// chunkSize ~800: đủ giữ ngữ cảnh, không quá to gây nhiễu/tốn tiền
// chunkOverlap ~100: để câu vắt ngang ranh giới không bị cắt mất nghĩa
const splitter = new RecursiveCharacterTextSplitter({
  chunkSize: 800,
  chunkOverlap: 100,
});

const HEADING_RE = /^(#{1,6})\s+(.+)$/;

interface Section {
  /** Chuỗi heading tổ tiên dẫn tới section này (không kể văn bản trước heading đầu tiên). */
  breadcrumb: string[];
  text: string;
}

/**
 * Tách văn bản Markdown thành các section theo heading (#, ##, ...), giữ lại
 * breadcrumb (chuỗi heading cha) cho mỗi section. Văn bản không có heading
 * nào (PDF/text thuần) trả về đúng 1 section với breadcrumb rỗng.
 */
function splitByHeadings(text: string): Section[] {
  const lines = text.split("\n");
  const sections: Section[] = [];
  const stack: { level: number; text: string }[] = [];
  let currentLines: string[] = [];

  const flush = () => {
    const body = currentLines.join("\n").trim();
    if (body) {
      sections.push({ breadcrumb: stack.map((s) => s.text), text: body });
    }
    currentLines = [];
  };

  for (const line of lines) {
    const m = HEADING_RE.exec(line);
    if (m) {
      flush();
      const level = m[1].length;
      const headingText = m[2].trim();
      while (stack.length > 0 && stack[stack.length - 1].level >= level) {
        stack.pop();
      }
      stack.push({ level, text: headingText });
      continue; // dòng heading không đưa vào nội dung — đã nằm trong breadcrumb
    }
    currentLines.push(line);
  }
  flush();

  return sections;
}

/**
 * Contextual retrieval MIỄN PHÍ (không gọi LLM): mỗi chunk được prepend
 * breadcrumb "[source › H1 › H2]" dựa trên cấu trúc heading Markdown có sẵn,
 * trước khi cắt theo kích thước. Giúp embedding + kết quả trả về LLM có ngữ
 * cảnh "đoạn này thuộc phần nào của tài liệu" thay vì 1 đoạn trích trơ trọi.
 * File không có heading (PDF, resume...) → breadcrumb chỉ còn tên file (nếu
 * có truyền `source`), không có source thì không thêm prefix gì (giữ nguyên
 * hành vi cũ).
 */
export async function chunkText(text: string, source?: string): Promise<string[]> {
  const sections = splitByHeadings(text);
  const chunks: string[] = [];

  for (const section of sections) {
    const breadcrumbParts = [source, ...section.breadcrumb].filter(
      (p): p is string => Boolean(p),
    );
    const prefix = breadcrumbParts.length > 0 ? `[${breadcrumbParts.join(" › ")}]\n` : "";

    const subChunks = await splitter.splitText(section.text);
    for (const sub of subChunks) {
      chunks.push(prefix + sub);
    }
  }

  return chunks;
}
