import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  PaperClipIcon,
  PaperAirplaneIcon,
  MicrophoneIcon,
  XMarkIcon,
  DocumentTextIcon,
  SparklesIcon,
  CommandLineIcon,
  ArrowUpTrayIcon,
} from "@heroicons/react/24/outline";

import { useToast } from "@/design-system/molecules/Toast";
import SlashCommandMenu, { type SlashCommand } from "./SlashCommandMenu";
import { Button } from "@/components/ui/button";

import type {
  NextIdFn,
  FormatSizeFn,
  ReadAsDataURLFn,
  PendingAttachment,
} from "@/types";

export type { PendingAttachment };

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

let _attachId = 0;
const nextId: NextIdFn = (): string => `att-${++_attachId}`;
const isImageType = (file: File): boolean => file.type.startsWith("image/");

const readAsDataURL: ReadAsDataURLFn = (file: File): Promise<string> =>
  new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(reader.result as string);
    reader.onerror = () => reject(new Error(`Failed to read ${file.name}`));
    reader.readAsDataURL(file);
  });

const formatSize: FormatSizeFn = (bytes: number): string => {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
};

/**
 * Composer — Shadcn UI Kit input bar with live timer, Floating Stop Pill, Slash Commands & Voice Dictation.
 */
export const Composer: React.FC<{
  value: string;
  onChange: (v: string) => void;
  onSend: () => void;
  disabled: boolean;
  onStop?: () => void;
  attachments: PendingAttachment[];
  onAttachmentsChange: (atts: PendingAttachment[]) => void;
}> = ({
  value,
  onChange,
  onSend,
  disabled,
  onStop,
  attachments,
  onAttachmentsChange,
}) => {
  const { t } = useTranslation("chat");
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const toast = useToast();

  const [isRecording, setIsRecording] = useState(false);
  const [showSlashMenu, setShowSlashMenu] = useState(false);
  const [slashFilter, setSlashFilter] = useState("");
  const [isDraggingOver, setIsDraggingOver] = useState(false);
  const [elapsedSeconds, setElapsedSeconds] = useState(0);

  useEffect(() => {
    let interval: ReturnType<typeof setInterval>;
    if (disabled) {
      setElapsedSeconds(0);
      interval = setInterval(() => {
        setElapsedSeconds((s) => s + 1);
      }, 1000);
    } else {
      setElapsedSeconds(0);
    }
    return () => clearInterval(interval);
  }, [disabled]);

  useEffect(() => {
    const el = textareaRef.current;
    if (!el) return;
    el.style.height = "auto";
    el.style.height = `${Math.min(el.scrollHeight, 220)}px`;
  }, [value]);

  useEffect(() => {
    if (value.startsWith("/")) {
      setShowSlashMenu(true);
      setSlashFilter(value.slice(1));
    } else {
      setShowSlashMenu(false);
    }
  }, [value]);

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === "Enter" && !e.shiftKey && !showSlashMenu) {
      e.preventDefault();
      onSend();
    }
  };

  const handleSlashSelect = (cmd: SlashCommand) => {
    onChange(`${cmd.prompt} `);
    setShowSlashMenu(false);
    textareaRef.current?.focus();
  };

  const toggleRecording = () => {
    if (isRecording) {
      setIsRecording(false);
      toast.success(t("composer.voice.stopped"));
    } else {
      setIsRecording(true);
      toast.info(t("composer.voice.listening"));
      setTimeout(() => {
        onChange((value ? `${value} ` : "") + t("composer.voice.sampleText"));
        setIsRecording(false);
        toast.success(t("composer.voice.transcribed"));
      }, 3000);
    }
  };

  const handleFileProcess = async (files: File[]) => {
    const errors: string[] = [];
    const newAttachments: PendingAttachment[] = [];

    for (const file of files) {
      if (file.size > MAX_SIZE) {
        errors.push(t("composer.errors.fileTooLarge", { name: file.name }));
        continue;
      }

      const type = isImageType(file) ? "image" : "file";
      const currentImageCount =
        attachments.filter((a) => a.type === "image").length +
        newAttachments.filter((a) => a.type === "image").length;
      const currentFileCount =
        attachments.filter((a) => a.type === "file").length +
        newAttachments.filter((a) => a.type === "file").length;

      if (type === "image" && currentImageCount >= MAX_IMAGES) {
        errors.push(t("composer.errors.maxImages", { max: MAX_IMAGES }));
        break;
      }
      if (type === "file" && currentFileCount >= MAX_FILES) {
        errors.push(t("composer.errors.maxFiles", { max: MAX_FILES }));
        break;
      }

      try {
        const preview = type === "image" ? await readAsDataURL(file) : "";
        newAttachments.push({
          id: nextId(),
          file,
          type,
          preview,
          name: file.name,
          size: file.size,
        });
      } catch {
        errors.push(t("composer.errors.readFileFailed", { name: file.name }));
      }
    }

    if (errors.length > 0) errors.forEach((msg) => toast.error(msg));
    if (newAttachments.length > 0)
      onAttachmentsChange([...attachments, ...newAttachments]);
  };

  const handleFileChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(e.target.files ?? []);
    if (fileInputRef.current) fileInputRef.current.value = "";
    await handleFileProcess(files);
  };

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDraggingOver(true);
  };

  const handleDragLeave = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDraggingOver(false);
  };

  const handleDrop = async (e: React.DragEvent) => {
    e.preventDefault();
    setIsDraggingOver(false);
    const files = Array.from(e.dataTransfer.files ?? []);
    if (files.length > 0) {
      await handleFileProcess(files);
    }
  };

  const removeAttachment = (id: string) =>
    onAttachmentsChange(attachments.filter((a) => a.id !== id));

  const imageCount = attachments.filter((a) => a.type === "image").length;
  const fileCount = attachments.filter((a) => a.type === "file").length;
  const totalCount = attachments.length;
  const atImageLimit = imageCount >= MAX_IMAGES;
  const atFileLimit = fileCount >= MAX_FILES;
  const canSend = (value.trim().length > 0 || totalCount > 0) && !disabled;

  return (
    <div
      className="px-4 pb-5 pt-2 sm:px-6 relative"
      onDragOver={handleDragOver}
      onDragLeave={handleDragLeave}
      onDrop={handleDrop}
    >
      {/* Floating Stop Generation Pill with Live Execution Timer */}
      {disabled && onStop && (
        <div className="flex justify-center mb-2.5 animate-fade-in">
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={onStop}
            className="gap-2 rounded-full border-border bg-card/95 shadow-lg text-xs font-semibold hover:bg-destructive hover:text-white transition-all backdrop-blur-md px-4 py-1.5"
          >
            <span className="relative flex h-2 w-2">
              <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-primary opacity-75" />
              <span className="relative inline-flex rounded-full h-2 w-2 bg-primary" />
            </span>
            <span>{t("composer.stopPill", { seconds: elapsedSeconds })}</span>
            <span className="h-3 border-r border-border mx-1" />
            <XMarkIcon className="h-3.5 w-3.5" />
            <span className="font-bold">{t("composer.stop")}</span>
          </Button>
        </div>
      )}

      {/* File Drag Overlay */}
      {isDraggingOver && (
        <div className="absolute inset-x-4 bottom-5 top-2 z-40 flex flex-col items-center justify-center rounded-3xl border-2 border-dashed border-primary bg-card/95 backdrop-blur-md animate-fade-in shadow-2xl">
          <ArrowUpTrayIcon className="h-10 w-10 text-primary animate-bounce mb-2" />
          <p className="text-sm font-bold text-foreground">
            {t("composer.dropHint")}
          </p>
          <p className="text-xs text-muted-foreground">
            {t("composer.dropSupportedTypes")}
          </p>
        </div>
      )}

      <input
        ref={fileInputRef}
        type="file"
        multiple
        accept={ACCEPT}
        onChange={handleFileChange}
        className="hidden"
        aria-label={t("composer.attachFileInputAria")}
      />

      <div className="mx-auto max-w-3xl relative">
        {/* Slash Command Menu Popup */}
        {showSlashMenu && (
          <SlashCommandMenu
            filterText={slashFilter}
            onSelect={handleSlashSelect}
            onClose={() => setShowSlashMenu(false)}
          />
        )}

        {/* Attachment Previews */}
        {totalCount > 0 && (
          <div className="mb-2.5 space-y-1.5 animate-fade-in">
            <ul className="flex flex-wrap gap-2" role="list">
              {attachments.map((att) => (
                <li key={att.id} className="relative group">
                  {att.type === "image" ? (
                    <div className="relative h-16 w-16 overflow-hidden rounded-2xl border border-border bg-card shadow-sm">
                      <img
                        src={att.preview}
                        alt={att.name}
                        className="h-full w-full object-cover"
                      />
                      <button
                        type="button"
                        onClick={() => removeAttachment(att.id)}
                        aria-label={t("composer.removeAttachmentAria", {
                          name: att.name,
                        })}
                        className="absolute right-1 top-1 flex h-5 w-5 items-center justify-center rounded-full bg-black/75 text-white opacity-0 transition hover:bg-destructive group-hover:opacity-100"
                      >
                        <XMarkIcon className="h-3 w-3" />
                      </button>
                    </div>
                  ) : (
                    <div className="relative flex items-center gap-2 rounded-xl border border-border bg-card px-3 py-2 pr-8 shadow-sm">
                      <DocumentTextIcon className="h-4 w-4 text-primary" />
                      <div className="min-w-0">
                        <p className="truncate text-xs font-medium text-foreground">
                          {att.name}
                        </p>
                        <p className="text-[10px] text-muted-foreground font-mono">
                          {formatSize(att.size)}
                        </p>
                      </div>
                      <button
                        type="button"
                        onClick={() => removeAttachment(att.id)}
                        aria-label={t("composer.removeAttachmentAria", {
                          name: att.name,
                        })}
                        className="absolute right-1.5 top-1.5 flex h-5 w-5 items-center justify-center rounded-full text-muted-foreground hover:bg-muted"
                      >
                        <XMarkIcon className="h-3 w-3" />
                      </button>
                    </div>
                  )}
                </li>
              ))}
            </ul>
            <p className="text-[10px] font-mono text-muted-foreground">
              {t("composer.attachmentsCount", {
                images: imageCount,
                maxImages: MAX_IMAGES,
                files: fileCount,
                maxFiles: MAX_FILES,
              })}
            </p>
          </div>
        )}

        {/* Shadcn Glass Input Bar */}
        <div className="glass relative flex items-end gap-2 px-3.5 py-2.5 rounded-3xl border border-border bg-card/80 backdrop-blur-xl focus-within:border-primary focus-within:ring-2 focus-within:ring-primary/20 transition-all duration-200">
          {/* Attach file button */}
          <Button
            type="button"
            variant="ghost"
            size="iconSm"
            onClick={() => fileInputRef.current?.click()}
            disabled={disabled || (atImageLimit && atFileLimit)}
            aria-label={t("composer.attachDocAria")}
            title={t("composer.attachDocTitle")}
            className="mb-0.5 h-8 w-8 text-muted-foreground hover:text-foreground"
          >
            <PaperClipIcon className="h-4 w-4" />
          </Button>

          {/* Slash command button */}
          <Button
            type="button"
            variant="ghost"
            size="iconSm"
            onClick={() => onChange("/")}
            title={t("composer.slashMenuTitle")}
            className="mb-0.5 hidden sm:flex h-8 w-8 text-muted-foreground hover:text-primary"
          >
            <CommandLineIcon className="h-4 w-4" />
          </Button>

          {/* Main Textarea */}
          <textarea
            ref={textareaRef}
            rows={1}
            value={value}
            onChange={(e) => onChange(e.target.value)}
            onKeyDown={handleKeyDown}
            disabled={disabled}
            placeholder={t("composer.placeholder")}
            className="scroll-fine max-h-52 flex-1 resize-none bg-transparent px-1.5 py-1 text-base leading-relaxed outline-none placeholder:text-muted-foreground disabled:opacity-50 font-sans text-foreground"
          />

          {/* Voice Input Mic Button */}
          <Button
            type="button"
            variant={isRecording ? "destructive" : "ghost"}
            size="iconSm"
            onClick={toggleRecording}
            aria-label={t("composer.voiceInputAria")}
            title={t("composer.voiceInputAria")}
            className={`mb-0.5 h-8 w-8 ${isRecording ? "animate-pulse" : "text-muted-foreground hover:text-foreground"}`}
          >
            <MicrophoneIcon className="h-4 w-4" />
          </Button>

          {/* Send / Stop Button */}
          {disabled && onStop ? (
            <Button
              type="button"
              variant="destructive"
              size="iconSm"
              onClick={onStop}
              aria-label={t("composer.stopGenerationAria")}
              title={t("composer.stopGenerationAria")}
              className="mb-0.5 h-8 w-8 rounded-xl shadow-md"
            >
              <XMarkIcon className="h-4 w-4" />
            </Button>
          ) : (
            <Button
              type="button"
              variant={canSend ? "gradient" : "secondary"}
              size="iconSm"
              onClick={onSend}
              disabled={!canSend}
              aria-label={t("composer.sendAria")}
              className="mb-0.5 h-8 w-8"
            >
              <PaperAirplaneIcon className="h-4 w-4" />
            </Button>
          )}
        </div>

        {/* Input Bar Footer hint */}
        <div className="mt-1.5 flex items-center px-2 text-[10px] text-muted-foreground">
          <span className="flex items-center gap-1">
            <SparklesIcon className="h-3 w-3 text-primary" />
            <span>{t("composer.shiftEnterHint")}</span>
          </span>
        </div>
      </div>
    </div>
  );
};

export default Composer;
