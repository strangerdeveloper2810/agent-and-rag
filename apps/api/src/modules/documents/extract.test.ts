import { describe, it, expect } from "vitest";
import { fileKind, extractDocumentText, UnsupportedFileError } from "./extract";

describe("fileKind", () => {
  it("nhận .txt/.md/.markdown là text", () => {
    expect(fileKind("a.txt")).toBe("text");
    expect(fileKind("README.MD")).toBe("text");
    expect(fileKind("note.markdown")).toBe("text");
  });

  it("nhận .pdf là pdf (không phân biệt hoa thường)", () => {
    expect(fileKind("doc.pdf")).toBe("pdf");
    expect(fileKind("DOC.PDF")).toBe("pdf");
  });

  it("còn lại là unsupported", () => {
    expect(fileKind("image.png")).toBe("unsupported");
    expect(fileKind("data.docx")).toBe("unsupported");
    expect(fileKind("noext")).toBe("unsupported");
  });
});

describe("extractDocumentText", () => {
  it("đọc UTF-8 cho file text", async () => {
    const text = await extractDocumentText("a.md", Buffer.from("# Tiêu đề"));
    expect(text).toBe("# Tiêu đề");
  });

  it("ném UnsupportedFileError với định dạng lạ", async () => {
    await expect(
      extractDocumentText("x.png", Buffer.from("..")),
    ).rejects.toBeInstanceOf(UnsupportedFileError);
  });
});
