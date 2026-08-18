import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nextProvider } from "react-i18next";
import { beforeEach, describe, expect, it, vi, type Mock } from "vitest";
import i18n from "@/i18n";
import { ToastProvider } from "@/design-system/molecules/Toast";
import type { DocumentInfo, DocumentVersion, VersionContent } from "@app/api-client";
import { DocumentsView } from "./DocumentsView";

vi.mock("@/modules/documents/documents.api", () => ({
  listDocuments: vi.fn(),
  uploadDocuments: vi.fn(),
  updateDocument: vi.fn(),
  deleteDocument: vi.fn(),
  getVersions: vi.fn(),
  getVersionContent: vi.fn(),
}));

import {
  listDocuments,
  uploadDocuments,
  deleteDocument,
  getVersions,
  getVersionContent,
} from "@/modules/documents/documents.api";

const mockListDocuments = listDocuments as unknown as Mock;
const mockUploadDocuments = uploadDocuments as unknown as Mock;
const mockDeleteDocument = deleteDocument as unknown as Mock;
const mockGetVersions = getVersions as unknown as Mock;
const mockGetVersionContent = getVersionContent as unknown as Mock;

const renderView = () =>
  render(
    <I18nextProvider i18n={i18n}>
      <ToastProvider>
        <DocumentsView />
      </ToastProvider>
    </I18nextProvider>,
  );

const oneDoc: DocumentInfo = {
  documentId: "doc-1",
  source: "report.pdf",
  version: 2,
  chunks: 5,
};

const oneVersion: DocumentVersion = {
  version: 2,
  source: "report.pdf",
  isLatest: true,
};

const versionContent: VersionContent = {
  found: true,
  documentId: "doc-1",
  version: 2,
  source: "report.pdf",
  content: "hello world",
  isLatest: true,
};

describe("DocumentsView", () => {
  beforeEach(async () => {
    vi.clearAllMocks();
    mockListDocuments.mockResolvedValue([]);
    mockUploadDocuments.mockResolvedValue({ results: [] });
    mockGetVersions.mockResolvedValue([]);
    await i18n.changeLanguage("vi");
  });

  it("renders translated header, subtitle, metrics and empty state in Vietnamese", async () => {
    renderView();

    expect(
      await screen.findByText("Cơ Sở Tri Thức (RAG)"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Tải tài liệu lên để nâng cao khả năng tìm kiếm vector của J.A.R.V.I.S.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("Tài Liệu")).toBeInTheDocument();
    expect(screen.getByText("Tổng Số Đoạn")).toBeInTheDocument();
    expect(
      screen.getByText("Nhấn hoặc kéo thả tệp để tải lên"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Định dạng hỗ trợ:", { exact: false, selector: "p" }),
    ).toBeInTheDocument();
    expect(document.body.textContent).toContain("(tối đa 7 tệp mỗi lượt)");
    await waitFor(() => {
      expect(
        screen.getByText(
          "Chưa có tài liệu nào được tải lên. Hãy tải lên tài liệu văn bản hoặc PDF đầu tiên của bạn ở trên.",
        ),
      ).toBeInTheDocument();
    });
  });

  it("renders translated header, subtitle, metrics and empty state in English", async () => {
    await i18n.changeLanguage("en");
    renderView();

    expect(
      await screen.findByText("Knowledge Base (RAG)"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Upload documents to boost J.A.R.V.I.S. vector search capabilities.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("Documents")).toBeInTheDocument();
    expect(screen.getByText("Total Chunks")).toBeInTheDocument();
    expect(
      screen.getByText("Click or drag & drop files to upload"),
    ).toBeInTheDocument();
    await waitFor(() => {
      expect(
        screen.getByText(
          "No documents uploaded yet. Upload your first text or PDF document above.",
        ),
      ).toBeInTheDocument();
    });
  });

  it("shows the translated max-files toast in Vietnamese", async () => {
    const user = userEvent.setup();
    renderView();
    await waitFor(() => expect(mockListDocuments).toHaveBeenCalled());

    const input = document.querySelector(
      'input[type="file"]',
    ) as HTMLInputElement;
    const files = Array.from(
      { length: 8 },
      (_, i) => new File(["x"], `f${i}.txt`, { type: "text/plain" }),
    );
    await user.upload(input, files);

    expect(
      await screen.findByText("Tối đa 7 tệp mỗi lượt."),
    ).toBeInTheDocument();
    expect(mockUploadDocuments).not.toHaveBeenCalled();
  });

  it("shows the translated max-files toast in English", async () => {
    await i18n.changeLanguage("en");
    const user = userEvent.setup();
    renderView();
    await waitFor(() => expect(mockListDocuments).toHaveBeenCalled());

    const input = document.querySelector(
      'input[type="file"]',
    ) as HTMLInputElement;
    const files = Array.from(
      { length: 8 },
      (_, i) => new File(["x"], `f${i}.txt`, { type: "text/plain" }),
    );
    await user.upload(input, files);

    expect(
      await screen.findByText("You can only upload up to 7 files at once."),
    ).toBeInTheDocument();
  });

  it("shows the translated generic upload failure toast when the error is not an Error instance", async () => {
    mockUploadDocuments.mockRejectedValue("boom");
    const user = userEvent.setup();
    renderView();
    await waitFor(() => expect(mockListDocuments).toHaveBeenCalled());

    const input = document.querySelector(
      'input[type="file"]',
    ) as HTMLInputElement;
    await user.upload(input, new File(["x"], "a.txt", { type: "text/plain" }));

    expect(
      await screen.findByText("Tải tài liệu lên thất bại."),
    ).toBeInTheDocument();
  });

  it("renders document list item actions and the translated delete confirmation dialog in Vietnamese", async () => {
    mockListDocuments.mockResolvedValue([oneDoc]);
    const user = userEvent.setup();
    renderView();

    expect(await screen.findByText("report.pdf")).toBeInTheDocument();
    expect(screen.getByText("Xem")).toBeInTheDocument();
    expect(screen.getByText("Cập nhật")).toBeInTheDocument();
    expect(screen.getByText("Lịch sử")).toBeInTheDocument();
    expect(
      screen.getByText("5 đoạn vector đã lập chỉ mục"),
    ).toBeInTheDocument();

    const deleteBtn = screen.getByLabelText("Xóa report.pdf");
    await user.click(deleteBtn);

    expect(await screen.findByText("Xóa tài liệu?")).toBeInTheDocument();
    expect(
      screen.getByText(
        (_, node) =>
          node?.textContent ===
          'Tất cả các phiên bản của "report.pdf" sẽ bị xóa vĩnh viễn. Hành động này không thể hoàn tác.',
      ),
    ).toBeInTheDocument();

    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByText("Xóa")).toBeInTheDocument();
    expect(within(dialog).getByText("Hủy")).toBeInTheDocument();
  });

  it("renders document list item actions in English", async () => {
    await i18n.changeLanguage("en");
    mockListDocuments.mockResolvedValue([oneDoc]);
    renderView();

    expect(await screen.findByText("report.pdf")).toBeInTheDocument();
    expect(screen.getByText("View")).toBeInTheDocument();
    expect(screen.getByText("Update")).toBeInTheDocument();
    expect(screen.getByText("History")).toBeInTheDocument();
    expect(screen.getByText("5 vector chunks indexed")).toBeInTheDocument();
  });

  it("shows the translated generic delete failure toast when the error is not an Error instance", async () => {
    mockListDocuments.mockResolvedValue([oneDoc]);
    mockDeleteDocument.mockRejectedValue("boom");
    const user = userEvent.setup();
    renderView();

    const deleteBtn = await screen.findByLabelText("Xóa report.pdf");
    await user.click(deleteBtn);
    const dialog = await screen.findByRole("dialog");
    await user.click(within(dialog).getByText("Xóa"));

    expect(
      await screen.findByText("Xóa tài liệu thất bại."),
    ).toBeInTheDocument();
  });

  it("shows the version history section with translated labels in Vietnamese", async () => {
    mockListDocuments.mockResolvedValue([oneDoc]);
    mockGetVersions.mockResolvedValue([oneVersion]);
    const user = userEvent.setup();
    renderView();

    const historyBtn = await screen.findByText("Lịch sử");
    await user.click(historyBtn);

    expect(await screen.findByText("Lịch sử phiên bản")).toBeInTheDocument();
    expect(screen.getByText("Đang dùng")).toBeInTheDocument();
    expect(screen.getByText("Xem trước")).toBeInTheDocument();
  });

  it("shows the version content modal with a translated close button", async () => {
    mockListDocuments.mockResolvedValue([oneDoc]);
    mockGetVersionContent.mockResolvedValue(versionContent);
    const user = userEvent.setup();
    renderView();

    await user.click(await screen.findByText("Xem"));

    expect(await screen.findByText("hello world")).toBeInTheDocument();
    expect(screen.getByLabelText("Đóng")).toBeInTheDocument();
  });
});
