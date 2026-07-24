import { useEffect, useRef } from "react";
import { SendIcon, UploadIcon, CloseIcon, DocIcon } from "@/shared/components/icons";
import { useToast } from "@/shared/components/Toast";

// ── Types ──

export interface PendingAttachment {
  id: string;
  file: File;
  type: "image" | "file";
  preview: string;
  name: string;
  size: number;
}

// ── Constants ──

const MAX_IMAGES = 7;
const MAX_FILES = 7;
const MAX_SIZE = 10 * 1024 * 1024;

const ACCEPT = [
  "image/png",
  "image/jpeg",
  "image/gif",
  "image/webp",
  "image/svg+xml",
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

  const handleFileChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(e.target.files ?? []);
    if (fileInputRef.current) fileInputRef.current.value = "";

    let errors: string[] = [];
    const newAttachments: PendingAttachment[] = [];

    for (const file of files) {
      if (file.size > MAX_SIZE) {
        errors.push(`${file.name} exceeds 10MB`);
        continue;
      }

      const isImage = isImageType(file);
      const type = isImage ? "image" : "file";

      const currentImageCount =
        attachments.filter((a) => a.type === "image").length +
        newAttachments.filter((a) => a.type === "image").length;
      const currentFileCount =
        attachments.filter((a) => a.type === "file").length +
        newAttachments.filter((a) => a.type === "file").length;

      if (type === "image" && currentImageCount >= MAX_IMAGES) {
        errors.push(`Max ${MAX_IMAGES} images`);
        break;
      }
      if (type === "file" && currentFileCount >= MAX_FILES) {
        errors.push(`Max ${MAX_FILES} files`);
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
        errors.push(`Could not read ${file.name}`);
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

  const imageCount = attachments.filter((a) => a.type === "image").length;
  const fileCount = attachments.filter((a) => a.type === "file").length;
  const totalCount = attachments.length;
  const atImageLimit = imageCount >= MAX_IMAGES;
  const atFileLimit = fileCount >= MAX_FILES;

  const canSend =
    (value.trim().length > 0 || totalCount > 0) && !disabled;

  return (
    <div className="px-4 pb-4 pt-2 sm:px-6">
      <input
        ref={fileInputRef}
        type="file"
        multiple
        accept={ACCEPT}
        onChange={handleFileChange}
        className="hidden"
        aria-label="Attach files"
      />

      <div className="mx-auto max-w-3xl">
        {/* Attachment previews */}
        {totalCount > 0 && (
          <div className="mb-2 space-y-1.5">
            <ul
              className="flex flex-wrap gap-2"
              role="list"
              aria-label="Attached files"
            >
              {attachments.map((att) => (
                <li key={att.id} className="relative group">
                  {att.type === "image" ? (
                    <div className="relative h-16 w-16 overflow-hidden rounded-xl border bg-[var(--cyber-subtle)]" style={{ borderColor: "var(--cyber-border)" }}>
                      <img
                        src={att.preview}
                        alt={att.name}
                        className="h-full w-full object-cover"
                      />
                      <button
                        type="button"
                        onClick={() => removeAttachment(att.id)}
                        aria-label={`Remove ${att.name}`}
                        className="absolute -right-1.5 -top-1.5 flex h-5 w-5 items-center justify-center rounded-full bg-black/60 text-white opacity-0 transition hover:bg-[var(--cyber-error)] group-hover:opacity-100 focus:opacity-100"
                      >
                        <CloseIcon width={11} height={11} />
                      </button>
                    </div>
                  ) : (
                    <div className="relative flex items-center gap-2 rounded-xl border px-3 py-2 pr-8 transition" style={{ borderColor: "var(--cyber-border)", backgroundColor: "var(--cyber-subtle)" }}>
                      <DocIcon
                        width={14}
                        height={14}
                        style={{ color: "var(--cyber-faint)" }}
                      />
                      <div className="min-w-0">
                        <p className="truncate text-[11px] font-medium" style={{ color: "var(--cyber-text)" }}>
                          {att.name}
                        </p>
                        <p className="text-[10px]" style={{ color: "var(--cyber-faint)" }}>
                          {formatSize(att.size)}
                        </p>
                      </div>
                      <button
                        type="button"
                        onClick={() => removeAttachment(att.id)}
                        aria-label={`Remove ${att.name}`}
                        className="absolute right-1 top-1 flex h-5 w-5 items-center justify-center rounded-full transition hover:bg-[var(--cyber-border)]"
                        style={{ color: "var(--cyber-faint)" }}
                      >
                        <CloseIcon width={11} height={11} />
                      </button>
                    </div>
                  )}
                </li>
              ))}
            </ul>
            <p className="text-[10px]" style={{ color: "var(--cyber-faint)" }}>
              {imageCount}/{MAX_IMAGES} images, {fileCount}/{MAX_FILES} files
            </p>
          </div>
        )}

        {/* Input row */}
        <div
          className="flex items-end gap-2 rounded-xl px-3 py-2 transition-all duration-200 focus-within:shadow-[0_0_12px_rgba(0,240,255,0.15)]"
          style={{
            backgroundColor: "var(--cyber-subtle)",
            border: "1px solid var(--cyber-border)",
          }}
        >
          {/* Attachment button */}
          <button
            type="button"
            onClick={() => fileInputRef.current?.click()}
            disabled={disabled || (atImageLimit && atFileLimit)}
            aria-label="Attach files"
            className={`mb-1 flex h-9 w-9 shrink-0 items-center justify-center rounded-full transition ${
              disabled || (atImageLimit && atFileLimit)
                ? "cursor-not-allowed opacity-20"
                : ""
            }`}
            style={{ color: "var(--cyber-muted)" }}
          >
            <UploadIcon width={18} height={18} />
          </button>

          {/* Prompt prefix */}
          <span
            className="mb-2.5 select-none text-sm opacity-50"
            style={{ color: "var(--cyber-primary)" }}
            aria-hidden="true"
          >
            &gt;
          </span>

          <textarea
            ref={textareaRef}
            rows={1}
            value={value}
            onChange={(e) => onChange(e.target.value)}
            onKeyDown={handleKeyDown}
            disabled={disabled}
            placeholder="Enter command..."
            className="scroll-fine max-h-52 flex-1 resize-none bg-transparent px-1 py-2 text-sm leading-relaxed outline-none placeholder:text-[var(--cyber-faint)] disabled:opacity-50"
            style={{
              color: "var(--cyber-text)",
              fontFamily: "'JetBrains Mono', ui-monospace, SF Mono, Consolas, monospace",
            }}
          />

          <button
            type="button"
            onClick={onSend}
            disabled={!canSend}
            aria-label="Send message"
            className={`mb-1 flex h-9 w-9 shrink-0 items-center justify-center rounded-full transition-all duration-200 ${
              canSend
                ? "text-[#0a0a0f]"
                : "cursor-not-allowed text-[var(--cyber-faint)]"
            }`}
            style={{
              backgroundColor: canSend ? "var(--cyber-primary)" : "transparent",
              boxShadow: canSend ? "0 0 12px rgba(0,240,255,0.3)" : "none",
            }}
          >
            <SendIcon width={17} height={17} />
          </button>
        </div>
        <p
          className="mt-2 text-center text-[10px]"
          style={{ color: "var(--cyber-faint)" }}
        >
          Press Enter to send · Shift+Enter for new line
        </p>
      </div>
    </div>
  );
}
