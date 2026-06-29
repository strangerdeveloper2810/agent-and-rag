import { useEffect, useRef, useState } from "react";
import { useOutletContext } from "react-router-dom";
import {
  listDocuments,
  uploadDocument,
  deleteDocument,
  type DocumentInfo,
} from "../lib/api";
import type { OutletCtx } from "./AppLayout";
import { UploadIcon, DocIcon, TrashIcon, MenuIcon } from "./icons";

export default function DocumentsView() {
  const { toggleSidebar } = useOutletContext<OutletCtx>();
  const [docs, setDocs] = useState<DocumentInfo[]>([]);
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const fileRef = useRef<HTMLInputElement>(null);

  const refresh = () => listDocuments().then(setDocs);
  useEffect(() => {
    refresh();
  }, []);

  const onUpload = async (file: File) => {
    setError(null);
    setUploading(true);
    try {
      await uploadDocument(file);
      await refresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Upload thất bại.");
    } finally {
      setUploading(false);
      if (fileRef.current) fileRef.current.value = "";
    }
  };

  const onRemove = async (source: string) => {
    await deleteDocument(source);
    await refresh();
  };

  return (
    <main className="flex min-w-0 flex-1 flex-col">
      <header className="flex items-center gap-3 px-4 py-3 sm:px-6">
        <button
          type="button"
          onClick={toggleSidebar}
          aria-label="Ẩn/hiện menu"
          className="rounded-full p-2 text-ink-soft hover:bg-subtle"
        >
          <MenuIcon />
        </button>
        <h1 className="font-medium text-ink">Tài liệu</h1>
      </header>

      <div className="scroll-fine flex-1 overflow-y-auto">
        <div className="mx-auto max-w-2xl px-4 py-8 sm:px-6">
          <h2 className="text-3xl font-medium tracking-tight">
            <span className="text-gemini">Tài liệu cho RAG</span>
          </h2>
          <p className="mt-2 text-ink-soft">
            Nạp file{" "}
            <code className="rounded bg-subtle px-1 text-ink">.txt</code> hoặc{" "}
            <code className="rounded bg-subtle px-1 text-ink">.md</code> — Agent
            sẽ tra cứu khi bạn hỏi.
          </p>

          {/* Vùng upload */}
          <label
            className={`mt-6 flex cursor-pointer flex-col items-center justify-center gap-2 rounded-3xl border-2 border-dashed border-line bg-subtle/50 px-6 py-10 text-center transition hover:border-gblue/40 hover:bg-gblue-soft/30 ${
              uploading ? "pointer-events-none opacity-60" : ""
            }`}
          >
            <input
              ref={fileRef}
              type="file"
              accept=".txt,.md,text/plain,text/markdown"
              className="hidden"
              onChange={(e) => {
                const f = e.target.files?.[0];
                if (f) onUpload(f);
              }}
            />
            <div className="flex h-12 w-12 items-center justify-center rounded-full bg-gemini text-white">
              <UploadIcon width={20} height={20} />
            </div>
            <span className="text-sm font-medium text-ink">
              {uploading ? "Đang nạp & embed…" : "Bấm để chọn file tải lên"}
            </span>
            <span className="text-xs text-ink-faint">.txt / .md</span>
          </label>

          {error && (
            <p className="mt-3 rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700">
              {error}
            </p>
          )}

          {/* Danh sách tài liệu */}
          <div className="mt-8">
            <h3 className="mb-3 text-xs font-medium uppercase tracking-wide text-ink-faint">
              Đã nạp ({docs.length})
            </h3>
            {docs.length === 0 ? (
              <p className="rounded-2xl bg-subtle px-4 py-8 text-center text-sm text-ink-faint">
                Chưa có tài liệu nào.
              </p>
            ) : (
              <ul className="space-y-2">
                {docs.map((d) => (
                  <li
                    key={d.source}
                    className="group flex items-center gap-3 rounded-2xl bg-subtle px-4 py-3"
                  >
                    <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-gblue-soft text-gblue">
                      <DocIcon width={18} height={18} />
                    </div>
                    <div className="min-w-0 flex-1">
                      <p className="truncate text-sm font-medium text-ink">
                        {d.source}
                      </p>
                      <p className="text-xs text-ink-faint">{d.chunks} chunk</p>
                    </div>
                    <button
                      type="button"
                      onClick={() => onRemove(d.source)}
                      aria-label={`Xóa ${d.source}`}
                      className="rounded-full p-2 text-ink-faint opacity-0 transition hover:bg-red-50 hover:text-red-600 focus:opacity-100 group-hover:opacity-100"
                    >
                      <TrashIcon width={16} height={16} />
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </div>
      </div>
    </main>
  );
}
