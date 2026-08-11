import { useEffect, useRef } from "react";
import {
  SendIcon,
  UploadIcon,
  CloseIcon,
  DocIcon,
} from "@/shared/components/icons";
import { useToast } from "@/shared/components/Toast";

import type { NextIdFn, FormatSizeFn, ReadAsDataURLFn, PendingAttachment } from "@/types";

// Re-export PendingAttachment for backwards compatibility
export type { PendingAttachment };

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

/**
 * Generates an incremental unique ID for pending attachments.
 *
 * @returns Unique attachment ID string
 */
const nextId: NextIdFn = (): string => `att-${++_attachId}`;

/**
 * Checks if a file is an image based on its MIME type.
 *
 * @param file - File object to check
 * @returns True if file is an image, false otherwise
 */
const isImageType = (file: File): boolean => file.type.startsWith("image/");

/**
 * Reads a File object as a Data URL string.
 *
 * @param file - File object to read
 * @returns Promise resolving to Data URL string
 */
const readAsDataURL: ReadAsDataURLFn = (file: File): Promise<string> => {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(reader.result as string);
    reader.onerror = () => reject(new Error(`Failed to read ${file.name}`));
    reader.readAsDataURL(file);
  });
};

/**
 * Formats byte size into human-readable B, KB, or MB string.
 *
 * @param bytes - Size in bytes
 * @returns Formatted size string
 */
const formatSize: FormatSizeFn = (bytes: number): string => {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
};

// ── Component ──

/**
 * Composer component providing a Raycast-style prompt input bar
 * with file attachment preview chips and submit keybindings.
 */
export const Composer: React.FC<{
  value: string;
  onChange: (v: string) => void;
  onSend: () => void;
  disabled: boolean;
  attachments: PendingAttachment[];
  onAttachmentsChange: (atts: PendingAttachment[]) => void;
}> = ({
  value,
  onChange,
  onSend,
  disabled,
  attachments,
  onAttachmentsChange,
}) => {
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

    const errors: string[] = [];
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

  const canSend = (value.trim().length > 0 || totalCount > 0) && !disabled;

  return (
    <div className="px-4 pb-5 pt-2 sm:px-6">
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
          <div className="mb-2.5 space-y-1.5 animate-fade-in">
            <ul
              className="flex flex-wrap gap-2"
              role="list"
              aria-label="Attached files"
            >
              {attachments.map((att) => (
                <li key={att.id} className="relative group">
                  {att.type === "image" ? (
                    <div
                      className="relative h-16 w-16 overflow-hidden rounded-2xl border bg-[var(--bg-raised)] shadow-md"
                      style={{ borderColor: "var(--border)" }}
                    >
                      <img
                        src={att.preview}
                        alt={att.name}
                        className="h-full w-full object-cover"
                      />
                      <button
                        type="button"
                        onClick={() => removeAttachment(att.id)}
                        aria-label={`Remove ${att.name}`}
                        className="absolute right-1 top-1 flex h-5 w-5 items-center justify-center rounded-full bg-black/70 text-white opacity-0 transition hover:bg-[var(--danger)] group-hover:opacity-100 focus:opacity-100"
                      >
                        <CloseIcon width={11} height={11} />
                      </button>
                    </div>
                  ) : (
                    <div
                      className="relative flex items-center gap-2 rounded-xl border px-3 py-2 pr-8 transition shadow-sm"
                      style={{
                        borderColor: "var(--border)",
                        backgroundColor: "var(--bg-raised)",
                      }}
                    >
                      <DocIcon
                        width={14}
                        height={14}
                        style={{ color: "var(--accent)" }}
                      />
                      <div className="min-w-0">
                        <p
                          className="truncate text-[11px] font-medium"
                          style={{ color: "var(--text)" }}
                        >
                          {att.name}
                        </p>
                        <p
                          className="text-[10px]"
                          style={{ color: "var(--text-tertiary)" }}
                        >
                          {formatSize(att.size)}
                        </p>
                      </div>
                      <button
                        type="button"
                        onClick={() => removeAttachment(att.id)}
                        aria-label={`Remove ${att.name}`}
                        className="absolute right-1.5 top-1.5 flex h-5 w-5 items-center justify-center rounded-full transition hover:bg-[var(--border)]"
                        style={{ color: "var(--text-tertiary)" }}
                      >
                        <CloseIcon width={11} height={11} />
                      </button>
                    </div>
                  )}
                </li>
              ))}
            </ul>
            <p
              className="text-[10px] font-mono"
              style={{ color: "var(--text-tertiary)" }}
            >
              {imageCount}/{MAX_IMAGES} images, {fileCount}/{MAX_FILES} files
            </p>
          </div>
        )}

        {/* Clean Input Box */}
        <div
          className="relative flex items-end gap-2 rounded-2xl px-3.5 py-2.5 transition-all duration-200 focus-within:border-[var(--accent)] focus-within:ring-2 focus-within:ring-[var(--accent-bg)] shadow-sm"
          style={{
            backgroundColor: "var(--surface)",
            border: "1px solid var(--border)",
          }}
        >
          {/* Attachment button */}
          <button
            type="button"
            onClick={() => fileInputRef.current?.click()}
            disabled={disabled || (atImageLimit && atFileLimit)}
            aria-label="Attach files"
            className={`mb-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-xl transition hover:bg-[var(--bg-hover)] active:scale-95 ${
              disabled || (atImageLimit && atFileLimit)
                ? "cursor-not-allowed opacity-30"
                : ""
            }`}
            style={{ color: "var(--text-secondary)" }}
          >
            <UploadIcon width={18} height={18} />
          </button>

          <textarea
            ref={textareaRef}
            rows={1}
            value={value}
            onChange={(e) => onChange(e.target.value)}
            onKeyDown={handleKeyDown}
            disabled={disabled}
            placeholder="Instruct J.A.R.V.I.S..."
            className="scroll-fine max-h-52 flex-1 resize-none bg-transparent px-1 py-1 text-sm leading-relaxed outline-none placeholder:text-[var(--text-tertiary)] disabled:opacity-50"
            style={{
              color: "var(--text)",
            }}
          />

          <button
            type="button"
            onClick={onSend}
            disabled={!canSend}
            aria-label="Send message"
            className={`mb-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-xl transition duration-150 active:scale-95 ${
              canSend
                ? "text-white hover:opacity-90"
                : "cursor-not-allowed opacity-40 text-[var(--text-tertiary)]"
            }`}
            style={{
              backgroundColor: canSend ? "var(--accent)" : "var(--bg-hover)",
            }}
          >
            <SendIcon width={16} height={16} />
          </button>
        </div>
      </div>
    </div>
  );
};

export default Composer;
