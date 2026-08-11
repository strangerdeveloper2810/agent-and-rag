import type { DocumentInfo, DocumentVersion, VersionContent } from "@app/api-client";

/** Props for DocumentsView page component. */
export interface DocumentsViewProps {
  onRefresh?: () => void;
}

/** State for document upload progress modal. */
export interface UploadModalState {
  open: boolean;
  files: File[];
  loading: boolean;
}

/** State for document version drawer modal. */
export interface VersionModalState {
  open: boolean;
  doc: DocumentInfo | null;
  versions: DocumentVersion[];
  selectedVersion: VersionContent | null;
  loading: boolean;
}
