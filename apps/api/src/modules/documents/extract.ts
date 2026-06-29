import { extractText, getDocumentProxy } from "unpdf";

/** File không phải định dạng text được hỗ trợ (.txt/.md/.pdf). */
export class UnsupportedFileError extends Error {
  constructor(public filename: string) {
    super(`Định dạng không hỗ trợ: ${filename}. Chỉ nhận .txt, .md, .pdf.`);
    this.name = "UnsupportedFileError";
  }
}

/** Trích được nhưng nội dung text rỗng (vd PDF scan toàn ảnh → cần OCR). */
export class EmptyContentError extends Error {
  constructor(public filename: string) {
    super(
      `Không trích được nội dung text từ ${filename}. ` +
        "Nếu là PDF scan (ảnh), cần OCR trước khi nạp.",
    );
    this.name = "EmptyContentError";
  }
}

const TEXT_EXT = [".txt", ".md", ".markdown"];

/** Phân loại file theo đuôi (thuần — test được, không I/O). */
export function fileKind(filename: string): "text" | "pdf" | "unsupported" {
  const lower = filename.toLowerCase();
  if (lower.endsWith(".pdf")) return "pdf";
  if (TEXT_EXT.some((ext) => lower.endsWith(ext))) return "text";
  return "unsupported";
}

// Trích toàn bộ text từ PDF bằng unpdf (pdfjs) — gộp các trang lại.
async function extractPdf(buffer: Buffer): Promise<string> {
  const pdf = await getDocumentProxy(new Uint8Array(buffer));
  const { text } = await extractText(pdf, { mergePages: true });
  return text;
}

/**
 * Lấy text thô từ file upload tùy định dạng:
 *  - .txt/.md → đọc UTF-8
 *  - .pdf     → trích text bằng unpdf
 *  - khác     → ném UnsupportedFileError
 * Ném EmptyContentError nếu trích ra rỗng (tránh nạp dữ liệu rác).
 */
export async function extractDocumentText(
  filename: string,
  buffer: Buffer,
): Promise<string> {
  const kind = fileKind(filename);
  if (kind === "unsupported") throw new UnsupportedFileError(filename);

  const text =
    kind === "pdf" ? await extractPdf(buffer) : buffer.toString("utf-8");

  if (text.trim().length === 0) throw new EmptyContentError(filename);
  return text;
}
