import { useEffect, useRef, useState } from "react";
import { useOutletContext } from "react-router-dom";
import {
  listDocuments,
  uploadDocuments,
  updateDocument,
  deleteDocument,
  getVersions,
  getVersionContent,
  type DocumentInfo,
  type DocumentVersion,
  type VersionContent,
} from "@/modules/documents/documents.api";
import type { OutletCtx } from "@/shared/components/AppLayout";
import {
  UploadIcon,
  DocIcon,
  TrashIcon,
  MenuIcon,
  CloseIcon,
} from "@/shared/components/icons";
import ConfirmDialog from "@/shared/components/ConfirmDialog";
import { useToast } from "@/shared/components/Toast";

export default function DocumentsView() {
  const { toggleSidebar } = useOutletContext<OutletCtx>();
  const toast = useToast();
  const [docs, setDocs] = useState<DocumentInfo[]>([]);
  const [uploading, setUploading] = useState(false);
  const [updatingId, setUpdatingId] = useState<string | null>(null);
  const fileRef = useRef<HTMLInputElement>(null);

  const [openHistory, setOpenHistory] = useState<string | null>(null);
  const [versions, setVersions] = useState<DocumentVersion[]>([]);
  const [viewing, setViewing] = useState<VersionContent | null>(null);
  const [confirming, setConfirming] = useState<DocumentInfo | null>(null);
  const [removing, setRemoving] = useState(false);

  const refresh = () => listDocuments().then(setDocs);
  useEffect(() => { refresh(); }, []);

  const onUpload = async (files: File[]) => {
    if (files.length === 0) return;
    if (files.length > 7) {
      toast.error("Max 7 files at a time.");
      if (fileRef.current) fileRef.current.value = "";
      return;
    }
    setUploading(true);
    try {
      const { results } = await uploadDocuments(files);
      await refresh();
      const ok = results.filter((r) => r.ok);
      const failed = results.filter((r) => !r.ok);
      const failNames = failed.map((f) => f.filename).join(", ");
      if (failed.length === 0) {
        toast.success(`Uploaded ${ok.length} documents.`);
      } else if (ok.length === 0) {
        toast.error(`Failed to upload ${failed.length} files: ${failNames}`);
      } else {
        toast.success(`Uploaded ${ok.length}, ${failed.length} failed (${failNames}).`);
      }
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Upload failed.");
    } finally {
      setUploading(false);
      if (fileRef.current) fileRef.current.value = "";
    }
  };

  const onUpdate = async (documentId: string, file: File) => {
    setUpdatingId(documentId);
    try {
      const res = await updateDocument(documentId, file);
      await refresh();
      if (openHistory === documentId) await loadHistory(documentId);
      toast.success(`Updated ${res.source} -> v${res.version}.`);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Update failed.");
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
      toast.success(`Deleted ${source}.`);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Delete failed.");
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
    <main className="flex min-w-0 flex-1 flex-col" style={{ minHeight: 0 }}>
      <header
        className="flex shrink-0 items-center gap-3 px-4 py-3 sm:px-6"
        style={{ borderBottom: "1px solid var(--border)", backgroundColor: "var(--surface)" }}
      >
        <button
          type="button"
          onClick={toggleSidebar}
          aria-label="Toggle menu"
          className="rounded-full p-2 transition hover:bg-[var(--border)]"
          style={{ color: "var(--text-secondary)" }}
        >
          <MenuIcon />
        </button>
        <h1 className="text-sm font-medium tracking-wider" style={{ color: "var(--text)" }}>
          Documents
        </h1>
      </header>

      <div className="scroll-fine overflow-y-auto" style={{ flex: 1, minHeight: 0 }}>
        <div className="mx-auto max-w-2xl px-4 py-8 sm:px-6">
          <h2 className="text-2xl font-medium tracking-tight" style={{ color: "var(--accent)" }}>
            Knowledge Base
          </h2>
          <p className="mt-2 text-xs leading-relaxed" style={{ color: "var(--text-secondary)" }}>
            Upload{" "}
            <code className="rounded px-1 py-0.5 text-[11px]" style={{ color: "var(--accent)", backgroundColor: "var(--border)" }}>
              .txt
            </code>
            ,{" "}
            <code className="rounded px-1 py-0.5 text-[11px]" style={{ color: "var(--accent)", backgroundColor: "var(--border)" }}>
              .md
            </code>
            , or{" "}
            <code className="rounded px-1 py-0.5 text-[11px]" style={{ color: "var(--accent)", backgroundColor: "var(--border)" }}>
              .pdf
            </code>
            {" "}files for RAG retrieval.
          </p>

          {/* Upload zone */}
          <label
            className={`mt-6 flex cursor-pointer flex-col items-center justify-center gap-2 rounded-xl border-2 border-dashed px-6 py-10 text-center transition-all duration-200 hover:shadow-[0_0_15px_rgba(0,240,255,0.1)] ${
              uploading ? "pointer-events-none opacity-60" : ""
            }`}
            style={{
              borderColor: "var(--border)",
              backgroundColor: "var(--bg-raised)",
            }}
          >
            <input
              ref={fileRef}
              type="file"
              multiple
              accept=".txt,.md,.pdf,text/plain,text/markdown,application/pdf"
              className="hidden"
              onChange={(e) => {
                const files = Array.from(e.target.files ?? []);
                if (files.length) onUpload(files);
              }}
            />
            <div
              className="flex h-12 w-12 items-center justify-center rounded-full"
              style={{
                backgroundColor: "var(--accent-bg)",
                color: "var(--accent)",
              }}
            >
              <UploadIcon width={20} height={20} />
            </div>
            <span className="text-xs font-medium" style={{ color: "var(--text)" }}>
              {uploading ? "Uploading & embedding..." : "Click to upload files (max 7)"}
            </span>
            <span className="text-[10px]" style={{ color: "var(--text-tertiary)" }}>
              .txt / .md / .pdf - max 7 files per batch
            </span>
          </label>

          {/* Document list */}
          <div className="mt-8">
            <h3
              className="mb-3 text-[10px] font-medium uppercase tracking-widest"
              style={{ color: "var(--text-tertiary)" }}
            >
              Uploaded ({docs.length})
            </h3>
            {docs.length === 0 ? (
              <div
                className="rounded-xl px-4 py-8 text-center text-xs"
                style={{
                  backgroundColor: "var(--bg-raised)",
                  color: "var(--text-tertiary)",
                  border: "1px solid var(--border)",
                }}
              >
                No documents yet.
              </div>
            ) : (
              <ul className="space-y-2">
                {docs.map((d) => (
                  <li
                    key={d.documentId}
                    className="rounded-xl px-4 py-3 transition-all"
                    style={{
                      backgroundColor: "var(--bg-raised)",
                      border: "1px solid var(--border)",
                    }}
                  >
                    <div className="group flex items-center gap-3">
                      <div
                        className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full"
                        style={{ backgroundColor: "var(--accent-bg)", color: "var(--accent)" }}
                      >
                        <DocIcon width={16} height={16} />
                      </div>
                      <div className="min-w-0 flex-1">
                        <p className="flex items-center gap-2 truncate text-xs font-medium" style={{ color: "var(--text)" }}>
                          {d.source}
                          <span
                            className="rounded-full px-2 py-0.5 text-[10px] font-medium"
                            style={{ backgroundColor: "var(--accent-bg)", color: "var(--accent)" }}
                          >
                            v{d.version}
                          </span>
                        </p>
                        <p className="text-[10px]" style={{ color: "var(--text-tertiary)" }}>
                          {d.chunks} chunks
                        </p>
                      </div>

                      <label
                        className={`cursor-pointer rounded-lg px-3 py-1.5 text-[10px] font-medium transition hover:bg-[var(--border)] ${
                          updatingId === d.documentId ? "pointer-events-none opacity-60" : ""
                        }`}
                        style={{ color: "var(--text-secondary)" }}
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
                        {updatingId === d.documentId ? "..." : "Update"}
                      </label>

                      <button
                        type="button"
                        onClick={() => toggleHistory(d.documentId)}
                        className="rounded-lg px-3 py-1.5 text-[10px] font-medium transition hover:bg-[var(--border)]"
                        style={{ color: "var(--text-secondary)" }}
                      >
                        History
                      </button>

                      <button
                        type="button"
                        onClick={() => setConfirming(d)}
                        aria-label={`Delete ${d.source}`}
                        className="rounded-full p-2 opacity-0 transition hover:bg-[rgba(255,51,102,0.1)] hover:text-[var(--danger)] focus:opacity-100 group-hover:opacity-100"
                        style={{ color: "var(--text-tertiary)" }}
                      >
                        <TrashIcon width={14} height={14} />
                      </button>
                    </div>

                    {openHistory === d.documentId && (
                      <ul className="mt-3 space-y-1 border-t pt-3" style={{ borderColor: "var(--border)" }}>
                        {versions.map((v) => (
                          <li key={v.version} className="flex items-center gap-2 text-xs">
                            <span
                              className="rounded px-1.5 py-0.5 text-[10px] font-medium"
                              style={{ backgroundColor: "var(--border)", color: "var(--text-secondary)" }}
                            >
                              v{v.version}
                            </span>
                            <span className="min-w-0 flex-1 truncate" style={{ color: "var(--text-secondary)" }}>
                              {v.source}
                            </span>
                            {v.isLatest && (
                              <span className="text-[10px]" style={{ color: "var(--accent)" }}>
                                latest
                              </span>
                            )}
                            <button
                              type="button"
                              onClick={() => onViewVersion(d.documentId, v.version)}
                              className="rounded-lg px-2 py-1 text-[10px] font-medium transition hover:bg-[var(--accent-bg)]"
                              style={{ color: "var(--accent)" }}
                            >
                              View
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

      {/* Version content modal */}
      {viewing && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
          onClick={() => setViewing(null)}
        >
          <div
            className="flex max-h-[80vh] w-full max-w-2xl flex-col rounded-2xl p-6 shadow-2xl"
            style={{
              backgroundColor: "var(--surface)",
              border: "1px solid var(--border)",
            }}
            onClick={(e) => e.stopPropagation()}
          >
            <div className="mb-4 flex items-center gap-3">
              <h3 className="min-w-0 flex-1 truncate text-sm font-medium" style={{ color: "var(--text)" }}>
                {viewing.source}{" "}
                <span style={{ color: "var(--text-tertiary)" }}>v{viewing.version}</span>
              </h3>
              <button
                type="button"
                onClick={() => setViewing(null)}
                aria-label="Close"
                className="rounded-full p-2 transition hover:bg-[var(--bg-raised)]"
                style={{ color: "var(--text-tertiary)" }}
              >
                <CloseIcon width={16} height={16} />
              </button>
            </div>
            <pre
              className="scroll-fine flex-1 overflow-y-auto whitespace-pre-wrap break-words rounded-xl px-4 py-3 text-xs leading-relaxed"
              style={{
                backgroundColor: "var(--bg-raised)",
                color: "var(--text-secondary)",
                border: "1px solid var(--border)",
              }}
            >
              {viewing.content}
            </pre>
          </div>
        </div>
      )}

      <ConfirmDialog
        open={confirming !== null}
        title="Delete document?"
        message={
          confirming
            ? `All versions of "${confirming.source}" will be permanently deleted. This action cannot be undone.`
            : undefined
        }
        confirmLabel={removing ? "Deleting..." : "Delete"}
        danger
        onConfirm={onRemove}
        onCancel={() => !removing && setConfirming(null)}
      />
    </main>
  );
}
