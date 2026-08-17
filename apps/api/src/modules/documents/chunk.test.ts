import { describe, it, expect } from "vitest";
import { chunkText } from "./chunk";

describe("chunkText", () => {
  it("splits long text into multiple chunks", async () => {
    const text = "câu một. ".repeat(500);
    const chunks = await chunkText(text);
    expect(chunks.length).toBeGreaterThan(1);
    expect(chunks[0].length).toBeGreaterThan(0);
  });

  it("returns single chunk for short text, không thêm breadcrumb khi không có source/heading", async () => {
    const chunks = await chunkText("ngắn thôi");
    expect(chunks).toEqual(["ngắn thôi"]);
  });

  it("thêm breadcrumb [source] khi có source nhưng không có heading (vd PDF/resume)", async () => {
    const chunks = await chunkText("nội dung không có heading nào cả", "resume.pdf");
    expect(chunks).toEqual(["[resume.pdf]\nnội dung không có heading nào cả"]);
  });

  it("tách theo heading Markdown và prepend breadcrumb [source › H1 › H2]", async () => {
    const text = [
      "# Indexing",
      "Giới thiệu chung về indexing.",
      "",
      "## B-tree Index",
      "Nội dung về B-tree.",
      "",
      "## Hash Index",
      "Nội dung về Hash index.",
    ].join("\n");

    const chunks = await chunkText(text, "postgresql.md");

    expect(chunks).toHaveLength(3);
    expect(chunks[0]).toBe("[postgresql.md › Indexing]\nGiới thiệu chung về indexing.");
    expect(chunks[1]).toBe(
      "[postgresql.md › Indexing › B-tree Index]\nNội dung về B-tree.",
    );
    expect(chunks[2]).toBe(
      "[postgresql.md › Indexing › Hash Index]\nNội dung về Hash index.",
    );
  });

  it("heading cùng cấp hoặc cấp cao hơn thay thế đúng phần breadcrumb tương ứng (không cộng dồn sai)", async () => {
    const text = [
      "## A",
      "text A",
      "## B",
      "text B",
      "# Root2",
      "text Root2",
    ].join("\n");

    const chunks = await chunkText(text, "doc.md");

    expect(chunks[0]).toBe("[doc.md › A]\ntext A");
    // "## B" cùng cấp "## A" -> phải THAY THẾ A trong breadcrumb, không phải [doc.md › A › B]
    expect(chunks[1]).toBe("[doc.md › B]\ntext B");
    // "# Root2" cấp cao hơn -> breadcrumb reset về [doc.md › Root2], không giữ lại B
    expect(chunks[2]).toBe("[doc.md › Root2]\ntext Root2");
  });

  it("văn bản trước heading đầu tiên có breadcrumb chỉ gồm source (chưa vào section nào)", async () => {
    const text = ["Đoạn mở đầu không thuộc heading nào.", "", "# Phần 1", "Nội dung phần 1."].join(
      "\n",
    );

    const chunks = await chunkText(text, "doc.md");

    expect(chunks[0]).toBe("[doc.md]\nĐoạn mở đầu không thuộc heading nào.");
    expect(chunks[1]).toBe("[doc.md › Phần 1]\nNội dung phần 1.");
  });

  it("KHÔNG coi dòng '#' bên trong code fence là heading (bash comment, không bị cắt vụn/xoá mất)", async () => {
    const text = [
      "# Cài đặt",
      "Chạy lệnh sau:",
      "",
      "```bash",
      "# comment giải thích lệnh",
      "npm install",
      "# comment khác",
      "npm run build",
      "```",
      "",
      "## Bước tiếp theo",
      "Xong phần cài đặt.",
    ].join("\n");

    const chunks = await chunkText(text, "guide.md");

    // Vẫn chỉ 2 section thật (Cài đặt, Bước tiếp theo) — code fence không tạo section riêng.
    expect(chunks).toHaveLength(2);
    // Toàn bộ nội dung bash (kể cả 2 dòng "#") phải còn nguyên trong section "Cài đặt".
    expect(chunks[0]).toBe(
      [
        "[guide.md › Cài đặt]",
        "Chạy lệnh sau:",
        "",
        "```bash",
        "# comment giải thích lệnh",
        "npm install",
        "# comment khác",
        "npm run build",
        "```",
      ].join("\n"),
    );
    expect(chunks[1]).toBe("[guide.md › Cài đặt › Bước tiếp theo]\nXong phần cài đặt.");
  });

  it("code fence dùng ~~~ cũng được nhận diện như ```", async () => {
    const text = ["# Tiêu đề", "~~~python", "# python comment", "~~~"].join("\n");
    const chunks = await chunkText(text, "doc.md");
    expect(chunks).toHaveLength(1);
    expect(chunks[0]).toBe("[doc.md › Tiêu đề]\n~~~python\n# python comment\n~~~");
  });

  it("chunk dài trong 1 section vẫn được cắt tiếp bởi splitter, mỗi mảnh đều có breadcrumb", async () => {
    const longSection = "câu dài. ".repeat(200);
    const text = `# Section dài\n${longSection}`;

    const chunks = await chunkText(text, "doc.md");

    expect(chunks.length).toBeGreaterThan(1);
    for (const c of chunks) {
      expect(c.startsWith("[doc.md › Section dài]\n")).toBe(true);
    }
  });
});
