import { useEffect, useReducer, useRef } from "react";
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
import { UploadIcon, DocIcon, TrashIcon, CloseIcon } from "@app/ui";
import ConfirmDialog from "@/design-system/molecules/ConfirmDialog";
import { useToast } from "@/design-system/molecules/Toast";
import { useDocumentTitle } from "@/hooks/useDocumentTitle";

interface State {
  docs: DocumentInfo[];
  uploading: boolean;
  updatingId: string | null;
  openHistory: string | null;
  versions: DocumentVersion[];
  viewing: VersionContent | null;
  confirming: DocumentInfo | null;
}

type Action =
  | { type: "SET_DOCS"; payload: DocumentInfo[] }
  | { type: "REMOVE_DOC"; payload: string }
  | { type: "SET_UPLOADING"; payload: boolean }
  | { type: "SET_UPDATING_ID"; payload: string | null }
  | {
      type: "SET_OPEN_HISTORY";
      payload: { id: string | null; versions?: DocumentVersion[] };
    }
  | { type: "SET_VERSIONS"; payload: DocumentVersion[] }
  | { type: "SET_VIEWING"; payload: VersionContent | null }
  | { type: "SET_CONFIRMING"; payload: DocumentInfo | null };

const initialState: State = {
  docs: [],
  uploading: false,
  updatingId: null,
  openHistory: null,
  versions: [],
  viewing: null,
  confirming: null,
};

function reducer(state: State, action: Action): State {
  switch (action.type) {
    case "SET_DOCS":
      return { ...state, docs: action.payload };
    case "REMOVE_DOC":
      return {
        ...state,
        docs: state.docs.filter((d) => d.documentId !== action.payload),
        confirming: null,
        openHistory:
          state.openHistory === action.payload ? null : state.openHistory,
      };
    case "SET_UPLOADING":
      return { ...state, uploading: action.payload };
    case "SET_UPDATING_ID":
      return { ...state, updatingId: action.payload };
    case "SET_OPEN_HISTORY":
      return {
        ...state,
        openHistory: action.payload.id,
        versions:
          action.payload.versions ??
          (action.payload.id === null ? [] : state.versions),
      };
    case "SET_VERSIONS":
      return { ...state, versions: action.payload };
    case "SET_VIEWING":
      return { ...state, viewing: action.payload };
    case "SET_CONFIRMING":
      return { ...state, confirming: action.payload };
    default:
      return state;
  }
}

/**
 * DocumentsView component for managing RAG knowledge base documents,
 * multi-file upload dropzone, version history, and content inspection.
 */
export const DocumentsView: React.FC = () => {
  const toast = useToast();
  useDocumentTitle("Documents");
  const [state, dispatch] = useReducer(reducer, initialState);
  const {
    docs,
    uploading,
    updatingId,
    openHistory,
    versions,
    viewing,
    confirming,
  } = state;
  const fileRef = useRef<HTMLInputElement>(null);

  const refresh = () =>
    listDocuments().then((docs) =>
      dispatch({ type: "SET_DOCS", payload: docs }),
    );
  useEffect(() => {
    refresh();
  }, []);

  const onUpload = async (files: File[]) => {
    if (files.length === 0) return;
    if (files.length > 7) {
      toast.error("Max 7 files at a time.");
      if (fileRef.current) fileRef.current.value = "";
      return;
    }
    dispatch({ type: "SET_UPLOADING", payload: true });
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
        toast.success(
          `Uploaded ${ok.length}, ${failed.length} failed (${failNames}).`,
        );
      }
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Upload failed.");
    } finally {
      dispatch({ type: "SET_UPLOADING", payload: false });
      if (fileRef.current) fileRef.current.value = "";
    }
  };

  const onUpdate = async (documentId: string, file: File) => {
    dispatch({ type: "SET_UPDATING_ID", payload: documentId });
    try {
      const res = await updateDocument(documentId, file);
      await refresh();
      if (openHistory === documentId) await loadHistory(documentId);
      toast.success(`Updated ${res.source} -> v${res.version}.`);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Update failed.");
    } finally {
      dispatch({ type: "SET_UPDATING_ID", payload: null });
    }
  };

  const onRemove = async () => {
    if (!confirming) return;
    const { documentId, source } = confirming;
    dispatch({ type: "REMOVE_DOC", payload: documentId });
    toast.success(`Deleted ${source}.`);
    try {
      await deleteDocument(documentId);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Delete failed.");
      refresh();
    }
  };

  const loadHistory = async (documentId: string) => {
    const list = await getVersions(documentId);
    dispatch({ type: "SET_VERSIONS", payload: list });
  };

  const toggleHistory = async (documentId: string) => {
    if (openHistory === documentId) {
      dispatch({ type: "SET_OPEN_HISTORY", payload: { id: null } });
      return;
    }
    dispatch({ type: "SET_OPEN_HISTORY", payload: { id: documentId } });
    await loadHistory(documentId);
  };

  const onViewVersion = async (documentId: string, version: number) => {
    const content = await getVersionContent(documentId, version);
    dispatch({ type: "SET_VIEWING", payload: content });
  };

  const totalChunks = docs.reduce((acc, d) => acc + (d.chunks || 0), 0);

  return (
    <main className="flex min-w-0 flex-1 flex-col" style={{ minHeight: 0 }}>
      <div
        className="scroll-fine overflow-y-auto"
        style={{ flex: 1, minHeight: 0 }}
      >
        <div className="mx-auto max-w-3xl px-4 py-8 sm:px-6">
          {/* Header section */}
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 mb-6">
            <div>
              <h2 className="text-2xl font-bold tracking-tight bg-gradient-to-r from-[var(--text)] to-[var(--accent)] bg-clip-text text-transparent">
                Knowledge Base (RAG)
              </h2>
              <p
                className="mt-1 text-xs leading-relaxed"
                style={{ color: "var(--text-secondary)" }}
              >
                Upload documents to empower J.A.R.V.I.S. vector search
                capabilities.
              </p>
            </div>

            {/* Metrics cards */}
            <div className="flex items-center gap-3">
              <div
                className="rounded-2xl border px-4 py-2 text-center backdrop-blur-md"
                style={{
                  backgroundColor: "var(--bg-raised)",
                  borderColor: "var(--border)",
                }}
              >
                <p className="text-[10px] font-mono uppercase text-[var(--text-tertiary)]">
                  Documents
                </p>
                <p className="text-lg font-bold font-mono text-[var(--accent)]">
                  {docs.length}
                </p>
              </div>
              <div
                className="rounded-2xl border px-4 py-2 text-center backdrop-blur-md"
                style={{
                  backgroundColor: "var(--bg-raised)",
                  borderColor: "var(--border)",
                }}
              >
                <p className="text-[10px] font-mono uppercase text-[var(--text-tertiary)]">
                  Total Chunks
                </p>
                <p className="text-lg font-bold font-mono text-[var(--accent-violet)]">
                  {totalChunks}
                </p>
              </div>
            </div>
          </div>

          {/* Upload zone */}
          <label
            className={`group relative flex cursor-pointer flex-col items-center justify-center gap-3 rounded-2xl border-2 border-dashed px-6 py-10 text-center transition-all duration-300 hover:border-[var(--accent)] hover:shadow-[0_0_30px_-5px_var(--glow-cyan)] ${
              uploading ? "pointer-events-none opacity-60" : ""
            }`}
            style={{
              borderColor: "var(--border)",
              backgroundColor: "var(--surface)",
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
              className="flex h-14 w-14 items-center justify-center rounded-2xl shadow-lg transition-transform duration-300 group-hover:scale-110"
              style={{
                background:
                  "linear-gradient(135deg, rgba(0,240,255,0.2) 0%, rgba(139,92,246,0.2) 100%)",
                border: "1px solid rgba(0, 240, 255, 0.3)",
                color: "var(--accent)",
              }}
            >
              <UploadIcon width={24} height={24} />
            </div>
            <div>
              <span
                className="text-sm font-semibold block"
                style={{ color: "var(--text)" }}
              >
                {uploading
                  ? "Embedding & vectorizing documents..."
                  : "Click or drag & drop files to upload"}
              </span>
              <p className="mt-1 text-[11px] text-[var(--text-tertiary)] font-mono">
                Supported extensions:{" "}
                <code className="text-[var(--accent)] bg-[var(--accent-bg)] px-1.5 py-0.5 rounded">
                  .txt
                </code>{" "}
                <code className="text-[var(--accent)] bg-[var(--accent-bg)] px-1.5 py-0.5 rounded">
                  .md
                </code>{" "}
                <code className="text-[var(--accent)] bg-[var(--accent-bg)] px-1.5 py-0.5 rounded">
                  .pdf
                </code>{" "}
                (max 7 files per batch)
              </p>
            </div>
          </label>

          {/* Document list */}
          <div className="mt-8">
            <div className="flex items-center justify-between mb-3">
              <h3
                className="text-[11px] font-mono font-bold uppercase tracking-widest"
                style={{ color: "var(--text-tertiary)" }}
              >
                Indexed Files ({docs.length})
              </h3>
            </div>

            {docs.length === 0 ? (
              <div
                className="rounded-2xl px-4 py-12 text-center text-xs backdrop-blur-md"
                style={{
                  backgroundColor: "var(--bg-raised)",
                  color: "var(--text-tertiary)",
                  border: "1px solid var(--border)",
                }}
              >
                No documents uploaded yet. Upload your first text or PDF
                document above.
              </div>
            ) : (
              <ul className="space-y-2.5">
                {docs.map((d) => (
                  <li
                    key={d.documentId}
                    className="rounded-2xl px-4 py-3.5 transition-all duration-200 backdrop-blur-md hover:border-[rgba(0,240,255,0.3)] shadow-sm"
                    style={{
                      backgroundColor: "var(--surface)",
                      border: "1px solid var(--border)",
                    }}
                  >
                    <div className="group flex items-center gap-3.5">
                      <div
                        onClick={() => onViewVersion(d.documentId, d.version)}
                        className="flex h-9 w-9 shrink-0 cursor-pointer items-center justify-center rounded-xl transition hover:scale-105 hover:shadow-sm"
                        style={{
                          backgroundColor: "var(--accent-bg)",
                          color: "var(--accent)",
                          border: "1px solid rgba(0,240,255,0.2)",
                        }}
                        title="View document content"
                      >
                        <DocIcon width={18} height={18} />
                      </div>
                      <div
                        className="min-w-0 flex-1 cursor-pointer group"
                        onClick={() => onViewVersion(d.documentId, d.version)}
                      >
                        <p
                          className="flex items-center gap-2 truncate text-xs font-semibold group-hover:text-[var(--accent)] transition"
                          style={{ color: "var(--text)" }}
                        >
                          {d.source}
                          <span
                            className="rounded-full px-2 py-0.5 text-[10px] font-mono font-medium"
                            style={{
                              backgroundColor: "var(--accent-bg)",
                              color: "var(--accent)",
                              border: "1px solid rgba(0, 240, 255, 0.2)",
                            }}
                          >
                            v{d.version}
                          </span>
                        </p>
                        <p
                          className="text-[10px] font-mono"
                          style={{ color: "var(--text-tertiary)" }}
                        >
                          {d.chunks} vector chunks indexed
                        </p>
                      </div>

                      <div className="flex items-center gap-1.5">
                        <button
                          type="button"
                          onClick={() => onViewVersion(d.documentId, d.version)}
                          className="rounded-xl border px-3 py-1.5 text-[11px] font-medium transition hover:bg-[var(--accent-bg)] hover:border-[var(--accent)]"
                          style={{
                            color: "var(--accent)",
                            borderColor: "var(--border)",
                          }}
                        >
                          View
                        </button>

                        <label
                          className={`cursor-pointer rounded-xl border px-3 py-1.5 text-[11px] font-medium transition hover:bg-[var(--bg-raised)] hover:border-[var(--accent)] ${
                            updatingId === d.documentId
                              ? "pointer-events-none opacity-60"
                              : ""
                          }`}
                          style={{
                            color: "var(--text-secondary)",
                            borderColor: "var(--border)",
                          }}
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
                          {updatingId === d.documentId
                            ? "Updating..."
                            : "Update"}
                        </label>

                        <button
                          type="button"
                          onClick={() => toggleHistory(d.documentId)}
                          className="rounded-xl border px-3 py-1.5 text-[11px] font-medium transition hover:bg-[var(--bg-raised)] hover:border-[var(--accent)]"
                          style={{
                            color: "var(--text-secondary)",
                            borderColor: "var(--border)",
                          }}
                        >
                          History
                        </button>

                        <button
                          type="button"
                          onClick={() =>
                            dispatch({ type: "SET_CONFIRMING", payload: d })
                          }
                          aria-label={`Delete ${d.source}`}
                          className="rounded-xl p-2 transition hover:bg-[var(--danger-bg)] hover:text-[var(--danger)] text-[var(--text-tertiary)]"
                        >
                          <TrashIcon width={14} height={14} />
                        </button>
                      </div>
                    </div>

                    {openHistory === d.documentId && (
                      <div
                        className="mt-3.5 border-t pt-3 space-y-2 animate-fade-in"
                        style={{ borderColor: "var(--border)" }}
                      >
                        <p className="text-[10px] font-mono font-bold uppercase tracking-wider text-[var(--text-tertiary)]">
                          Version History
                        </p>
                        <ul className="space-y-1">
                          {versions.map((v) => (
                            <li
                              key={v.version}
                              className="flex items-center gap-2 rounded-xl px-3 py-1.5 text-xs bg-[var(--bg-raised)]"
                            >
                              <span
                                className="rounded-lg px-2 py-0.5 text-[10px] font-mono font-medium"
                                style={{
                                  backgroundColor: "var(--surface)",
                                  color: "var(--accent)",
                                  border: "1px solid var(--border)",
                                }}
                              >
                                v{v.version}
                              </span>
                              <span
                                className="min-w-0 flex-1 truncate font-mono text-[11px]"
                                style={{ color: "var(--text-secondary)" }}
                              >
                                {v.source}
                              </span>
                              {v.isLatest && (
                                <span
                                  className="rounded-full px-2 py-0.5 text-[9px] font-mono font-bold uppercase"
                                  style={{
                                    backgroundColor: "var(--success-bg)",
                                    color: "var(--success)",
                                  }}
                                >
                                  Active
                                </span>
                              )}
                              <button
                                type="button"
                                onClick={() =>
                                  onViewVersion(d.documentId, v.version)
                                }
                                className="rounded-lg px-2.5 py-1 text-[10px] font-medium transition hover:bg-[var(--accent-bg)]"
                                style={{ color: "var(--accent)" }}
                              >
                                Preview
                              </button>
                            </li>
                          ))}
                        </ul>
                      </div>
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
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-md p-4 animate-fade-in"
          onClick={() => dispatch({ type: "SET_VIEWING", payload: null })}
        >
          <div
            className="flex max-h-[80vh] w-full max-w-3xl flex-col rounded-3xl p-6 shadow-2xl backdrop-blur-xl"
            style={{
              backgroundColor: "var(--surface)",
              border: "1px solid var(--border)",
            }}
            onClick={(e) => e.stopPropagation()}
          >
            <div
              className="mb-4 flex items-center gap-3 border-b pb-4"
              style={{ borderColor: "var(--border)" }}
            >
              <div className="flex h-8 w-8 items-center justify-center rounded-xl bg-[var(--accent-bg)] text-[var(--accent)]">
                <DocIcon width={16} height={16} />
              </div>
              <h3
                className="min-w-0 flex-1 truncate text-sm font-bold"
                style={{ color: "var(--text)" }}
              >
                {viewing.source}{" "}
                <span className="font-mono text-xs text-[var(--accent)]">
                  (v{viewing.version})
                </span>
              </h3>
              <button
                type="button"
                onClick={() => dispatch({ type: "SET_VIEWING", payload: null })}
                aria-label="Close"
                className="rounded-xl p-2 transition hover:bg-[var(--bg-raised)] text-[var(--text-tertiary)]"
              >
                <CloseIcon width={16} height={16} />
              </button>
            </div>
            <pre
              className="scroll-fine flex-1 overflow-y-auto whitespace-pre-wrap break-words rounded-2xl px-4 py-3.5 text-xs font-mono leading-relaxed"
              style={{
                backgroundColor: "var(--bg)",
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
        confirmLabel="Delete"
        danger
        onConfirm={onRemove}
        onCancel={() => dispatch({ type: "SET_CONFIRMING", payload: null })}
      />
    </main>
  );
};

export default DocumentsView;
