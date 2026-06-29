import { useEffect, useRef, useState } from "react";
import { useOutletContext } from "react-router-dom";
import {
  listDocuments,
  uploadDocument,
  updateDocument,
  deleteDocument,
  getVersions,
  getVersionContent,
  type DocumentInfo,
  type DocumentVersion,
  type VersionContent,
} from "@/modules/documents/documents.api";
import type { OutletCtx } from "@/shared/components/AppLayout";
import { UploadIcon, DocIcon, TrashIcon, MenuIcon, CloseIcon } from "@/shared/components/icons";
import ConfirmDialog from "@/shared/components/ConfirmDialog";
import { useToast } from "@/shared/components/Toast";

export default function DocumentsView() {
  const { toggleSidebar } = useOutletContext<OutletCtx>();
  const toast = useToast();
  const [docs, setDocs] = useState<DocumentInfo[]>([]);
  const [uploading, setUploading] = useState(false);
  const [updatingId, setUpdatingId] = useState<string | null>(null);
  const fileRef = useRef<HTMLInputElement>(null);

  // Lịch sử version: documentId đang mở + danh sách version của nó
  const [openHistory, setOpenHistory] = useState<string | null>(null);
  const [versions, setVersions] = useState<DocumentVersion[]>([]);
  // Modal xem nội dung 1 version
  const [viewing, setViewing] = useState<VersionContent | null>(null);
  // Dialog xác nhận xóa
  const [confirming, setConfirming] = useState<DocumentInfo | null>(null);
  const [removing, setRemoving] = useState(false);

  const refresh = () => listDocuments().then(setDocs);
  useEffect(() => {
    refresh();
  }, []);

  const onUpload = async (file: File) => {
    setUploading(true);
    try {
      const res = await uploadDocument(file);
      await refresh();
      toast.success(`Đã nạp ${res.source} (${res.chunks} chunk).`);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Upload thất bại.");
    } finally {
      setUploading(false);
      if (fileRef.current) fileRef.current.value = "";
    }
  };

  // Cập nhật tài liệu đã có → tạo version mới
  const onUpdate = async (documentId: string, file: File) => {
    setUpdatingId(documentId);
    try {
      const res = await updateDocument(documentId, file);
      await refresh();
      if (openHistory === documentId) await loadHistory(documentId);
      toast.success(`Đã cập nhật ${res.source} → v${res.version}.`);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Cập nhật thất bại.");
    } finally {
      setUpdatingId(null);
    }
  };

  const onRemove = async () => {
    if (!confirming) return;
    const { documentId, source } = confirming;
    setRemoving(true);
    try {
      await deleteDocument(documentId);
      if (openHistory === documentId) setOpenHistory(null);
      await refresh();
      setConfirming(null);
      toast.success(`Đã xóa ${source}.`);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Xóa thất bại.");
    } finally {
      setRemoving(false);
    }
  };

  const loadHistory = async (documentId: string) => {
    setVersions(await getVersions(documentId));
  };

  const toggleHistory = async (documentId: string) => {
    if (openHistory === documentId) {
      setOpenHistory(null);
      return;
    }
    setOpenHistory(documentId);
    await loadHistory(documentId);
  };

  const onViewVersion = async (documentId: string, version: number) => {
    setViewing(await getVersionContent(documentId, version));
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
            <code className="rounded bg-subtle px-1 text-ink">.txt</code>,{" "}
            <code className="rounded bg-subtle px-1 text-ink">.md</code> hoặc{" "}
            <code className="rounded bg-subtle px-1 text-ink">.pdf</code> — Agent
            sẽ tra cứu khi bạn hỏi.
          </p>

          {/* Vùng upload tài liệu mới */}
          <label
            className={`mt-6 flex cursor-pointer flex-col items-center justify-center gap-2 rounded-3xl border-2 border-dashed border-line bg-subtle/50 px-6 py-10 text-center transition hover:border-gblue/40 hover:bg-gblue-soft/30 ${
              uploading ? "pointer-events-none opacity-60" : ""
            }`}
          >
            <input
              ref={fileRef}
              type="file"
              accept=".txt,.md,.pdf,text/plain,text/markdown,application/pdf"
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
            <span className="text-xs text-ink-faint">.txt / .md / .pdf</span>
          </label>

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
                    key={d.documentId}
                    className="rounded-2xl bg-subtle px-4 py-3"
                  >
                    <div className="group flex items-center gap-3">
                      <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-gblue-soft text-gblue">
                        <DocIcon width={18} height={18} />
                      </div>
                      <div className="min-w-0 flex-1">
                        <p className="flex items-center gap-2 truncate text-sm font-medium text-ink">
                          {d.source}
                          <span className="rounded-full bg-gblue-soft px-2 py-0.5 text-xs font-medium text-gblue">
                            v{d.version}
                          </span>
                        </p>
                        <p className="text-xs text-ink-faint">{d.chunks} chunk</p>
                      </div>

                      {/* Cập nhật (upload file mới → version mới) */}
                      <label
                        className={`cursor-pointer rounded-full px-3 py-1.5 text-xs font-medium text-ink-soft transition hover:bg-line/60 ${
                          updatingId === d.documentId
                            ? "pointer-events-none opacity-60"
                            : ""
                        }`}
                      >
                        <input
                          type="file"
                          accept=".txt,.md,.pdf,text/plain,text/markdown,application/pdf"
                          className="hidden"
                          onChange={(e) => {
                            const f = e.target.files?.[0];
                            if (f) onUpdate(d.documentId, f);
                            e.target.value = "";
                          }}
                        />
                        {updatingId === d.documentId ? "Đang…" : "Cập nhật"}
                      </label>

                      {/* Lịch sử version */}
                      <button
                        type="button"
                        onClick={() => toggleHistory(d.documentId)}
                        className="rounded-full px-3 py-1.5 text-xs font-medium text-ink-soft transition hover:bg-line/60"
                      >
                        Lịch sử
                      </button>

                      <button
                        type="button"
                        onClick={() => setConfirming(d)}
                        aria-label={`Xóa ${d.source}`}
                        className="rounded-full p-2 text-ink-faint opacity-0 transition hover:bg-red-50 hover:text-red-600 focus:opacity-100 group-hover:opacity-100"
                      >
                        <TrashIcon width={16} height={16} />
                      </button>
                    </div>

                    {/* Bảng lịch sử version */}
                    {openHistory === d.documentId && (
                      <ul className="mt-3 space-y-1 border-t border-line/60 pt-3">
                        {versions.map((v) => (
                          <li
                            key={v.version}
                            className="flex items-center gap-2 text-sm"
                          >
                            <span className="rounded bg-subtle px-1.5 py-0.5 text-xs font-medium text-ink-soft">
                              v{v.version}
                            </span>
                            <span className="min-w-0 flex-1 truncate text-ink-soft">
                              {v.source}
                            </span>
                            {v.isLatest && (
                              <span className="text-xs text-gemini">
                                mới nhất
                              </span>
                            )}
                            <button
                              type="button"
                              onClick={() =>
                                onViewVersion(d.documentId, v.version)
                              }
                              className="rounded-full px-2 py-1 text-xs font-medium text-gblue hover:bg-gblue-soft/50"
                            >
                              Xem
                            </button>
                          </li>
                        ))}
                      </ul>
                    )}
                  </li>
                ))}
              </ul>
            )}
          </div>
        </div>
      </div>

      {/* Modal xem nội dung 1 version */}
      {viewing && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4"
          onClick={() => setViewing(null)}
        >
          <div
            className="flex max-h-[80vh] w-full max-w-2xl flex-col rounded-3xl bg-white p-6 shadow-xl"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="mb-4 flex items-center gap-3">
              <h3 className="min-w-0 flex-1 truncate font-medium text-ink">
                {viewing.source}{" "}
                <span className="text-ink-faint">· v{viewing.version}</span>
              </h3>
              <button
                type="button"
                onClick={() => setViewing(null)}
                aria-label="Đóng"
                className="rounded-full p-2 text-ink-faint hover:bg-subtle"
              >
                <CloseIcon width={18} height={18} />
              </button>
            </div>
            <pre className="scroll-fine flex-1 overflow-y-auto whitespace-pre-wrap break-words rounded-2xl bg-subtle px-4 py-3 text-sm text-ink-soft">
              {viewing.content}
            </pre>
          </div>
        </div>
      )}

      {/* Dialog xác nhận xóa (tái dùng ConfirmDialog) */}
      <ConfirmDialog
        open={confirming !== null}
        title="Xóa tài liệu?"
        message={
          confirming
            ? `Toàn bộ ${confirming.source} (bản mới nhất v${confirming.version} và tất cả version cũ) sẽ bị xóa vĩnh viễn. Hành động này không thể hoàn tác.`
            : undefined
        }
        confirmLabel={removing ? "Đang xóa…" : "Xóa"}
        danger
        onConfirm={onRemove}
        onCancel={() => !removing && setConfirming(null)}
      />
    </main>
  );
}
