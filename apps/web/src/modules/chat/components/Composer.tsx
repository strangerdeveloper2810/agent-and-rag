import { useEffect, useRef } from "react";
import { SendIcon, UploadIcon, CloseIcon, DocIcon } from "@/shared/components/icons";
import { useToast } from "@/shared/components/Toast";

// ── Types ──

/** An attachment pending in the Composer before being sent. */
export interface PendingAttachment {
  id: string;
  file: File;
  type: "image" | "file";
  preview: string; // data URL for images; empty for non-image files
  name: string;
  size: number;
}

// ── Constants ──

const MAX_IMAGES = 7;
const MAX_FILES = 7;
const MAX_SIZE = 10 * 1024 * 1024; // 10 MB

const ACCEPT = [
  // Images
  "image/png",
  "image/jpeg",
  "image/gif",
  "image/webp",
  "image/svg+xml",
  // Documents
  "application/pdf",
  "text/plain",
  "text/csv",
  "text/markdown",
  "application/json",
  "application/msword",
  "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
  "application/vnd.ms-excel",
  "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
].join(",");

// ── Helpers ──

let _attachId = 0;
function nextId(): string {
  return `att-${++_attachId}`;
}

function isImageType(file: File): boolean {
  return file.type.startsWith("image/");
}

/** Read a File as a data URL. */
function readAsDataURL(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(reader.result as string);
    reader.onerror = () => reject(new Error(`Failed to read ${file.name}`));
    reader.readAsDataURL(file);
  });
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

// ── Component ──

export default function Composer({
  value,
  onChange,
  onSend,
  disabled,
  attachments,
  onAttachmentsChange,
}: {
  value: string;
  onChange: (v: string) => void;
  onSend: () => void;
  disabled: boolean;
  attachments: PendingAttachment[];
  onAttachmentsChange: (atts: PendingAttachment[]) => void;
}) {
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const toast = useToast();

  // Auto-resize textarea
  useEffect(() => {
    const el = textareaRef.current;
    if (!el) return;
    el.style.height = "auto";
    el.style.height = `${Math.min(el.scrollHeight, 200)}px`;
  }, [value]);

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      onSend();
    }
  };

  // ── File picking ──

  const handleFileChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(e.target.files ?? []);
    // Reset input so the same file can be picked again
    if (fileInputRef.current) fileInputRef.current.value = "";

    let errors: string[] = [];
    const newAttachments: PendingAttachment[] = [];

    for (const file of files) {
      // Size check
      if (file.size > MAX_SIZE) {
        errors.push(`${file.name} vượt quá 10MB`);
        continue;
      }

      const isImage = isImageType(file);
      const type = isImage ? "image" : "file";

      // Count check
      const currentImageCount =
        attachments.filter((a) => a.type === "image").length +
        newAttachments.filter((a) => a.type === "image").length;
      const currentFileCount =
        attachments.filter((a) => a.type === "file").length +
        newAttachments.filter((a) => a.type === "file").length;

      if (type === "image" && currentImageCount >= MAX_IMAGES) {
        errors.push(`Tối đa ${MAX_IMAGES} ảnh`);
        break;
      }
      if (type === "file" && currentFileCount >= MAX_FILES) {
        errors.push(`Tối đa ${MAX_FILES} files`);
        break;
      }

      try {
        const preview = isImage ? await readAsDataURL(file) : "";
        newAttachments.push({
          id: nextId(),
          file,
          type,
          preview,
          name: file.name,
          size: file.size,
        });
      } catch {
        errors.push(`Không đọc được ${file.name}`);
      }
    }

    if (errors.length > 0) {
      errors.forEach((msg) => toast.error(msg));
    }

    if (newAttachments.length > 0) {
      onAttachmentsChange([...attachments, ...newAttachments]);
    }
  };

  const removeAttachment = (id: string) => {
    onAttachmentsChange(attachments.filter((a) => a.id !== id));
  };

  // ── Derived counts ──

  const imageCount = attachments.filter((a) => a.type === "image").length;
  const fileCount = attachments.filter((a) => a.type === "file").length;
  const totalCount = attachments.length;
  const atImageLimit = imageCount >= MAX_IMAGES;
  const atFileLimit = fileCount >= MAX_FILES;

  const canSend =
    (value.trim().length > 0 || totalCount > 0) && !disabled;

  return (
    <div className="px-4 pb-4 pt-2 sm:px-6">
      {/* Hidden file input */}
      <input
        ref={fileInputRef}
        type="file"
        multiple
        accept={ACCEPT}
        onChange={handleFileChange}
        className="hidden"
        aria-label="Chọn tệp đính kèm"
      />

      <div className="mx-auto max-w-3xl">
        {/* Attachment previews */}
        {totalCount > 0 && (
          <div className="mb-2 space-y-1.5">
            <ul
              className="flex flex-wrap gap-2"
              role="list"
              aria-label="Tệp đính kèm đã chọn"
            >
              {attachments.map((att) => (
                <li key={att.id} className="relative group">
                  {att.type === "image" ? (
                    <div className="relative h-16 w-16 overflow-hidden rounded-xl border border-line bg-subtle">
                      <img
                        src={att.preview}
                        alt={att.name}
                        className="h-full w-full object-cover"
                      />
                      <button
                        type="button"
                        onClick={() => removeAttachment(att.id)}
                        aria-label={`Xóa ${att.name}`}
                        className="absolute -right-1.5 -top-1.5 flex h-5 w-5 items-center justify-center rounded-full bg-black/60 text-white opacity-0 transition hover:bg-red-600 group-hover:opacity-100 focus:opacity-100"
                      >
                        <CloseIcon width={11} height={11} />
                      </button>
                    </div>
                  ) : (
                    <div className="relative flex items-center gap-2 rounded-xl border border-line bg-subtle px-3 py-2 pr-8 transition">
                      <DocIcon
                        width={16}
                        height={16}
                        className="shrink-0 text-ink-faint"
                      />
                      <div className="min-w-0">
                        <p className="truncate text-xs font-medium text-ink">
                          {att.name}
                        </p>
                        <p className="text-[11px] text-ink-faint">
                          {formatSize(att.size)}
                        </p>
                      </div>
                      <button
                        type="button"
                        onClick={() => removeAttachment(att.id)}
                        aria-label={`Xóa ${att.name}`}
                        className="absolute right-1 top-1 flex h-5 w-5 items-center justify-center rounded-full text-ink-faint transition hover:bg-line hover:text-ink"
                      >
                        <CloseIcon width={11} height={11} />
                      </button>
                    </div>
                  )}
                </li>
              ))}
            </ul>
            <p className="text-[11px] text-ink-faint">
              {imageCount}/{MAX_IMAGES} ảnh, {fileCount}/{MAX_FILES} files
            </p>
          </div>
        )}

        {/* Input row */}
        <div className="flex items-end gap-2 rounded-[28px] bg-subtle px-2 py-1.5 transition focus-within:bg-subtle2">
          {/* Attachment button */}
          <button
            type="button"
            onClick={() => fileInputRef.current?.click()}
            disabled={disabled || (atImageLimit && atFileLimit)}
            aria-label="Đính kèm tệp"
            className={`mb-0.5 flex h-10 w-10 shrink-0 items-center justify-center rounded-full transition ${
              disabled || (atImageLimit && atFileLimit)
                ? "cursor-not-allowed text-ink-faint opacity-30"
                : "text-ink-faint hover:bg-line hover:text-ink"
            }`}
          >
            <UploadIcon width={19} height={19} />
          </button>

          <textarea
            ref={textareaRef}
            rows={1}
            value={value}
            onChange={(e) => onChange(e.target.value)}
            onKeyDown={handleKeyDown}
            disabled={disabled}
            placeholder="Nhập câu lệnh tại đây"
            className="scroll-fine max-h-52 flex-1 resize-none bg-transparent px-1 py-2.5 text-[0.95rem] leading-relaxed text-ink outline-none placeholder:text-ink-faint disabled:opacity-60"
          />

          <button
            type="button"
            onClick={onSend}
            disabled={!canSend}
            aria-label="Gửi tin nhắn"
            className={`mb-0.5 flex h-10 w-10 shrink-0 items-center justify-center rounded-full transition ${
              canSend
                ? "bg-gblue text-white hover:bg-gblue-bright"
                : "cursor-not-allowed bg-line text-ink-faint"
            }`}
          >
            <SendIcon width={19} height={19} />
          </button>
        </div>
        <p className="mt-2 text-center text-xs text-ink-faint">
          Agent Tut có thể mắc lỗi. Hãy kiểm chứng thông tin quan trọng.
        </p>
      </div>
    </div>
  );
}
