import { render, waitFor, screen } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { describe, it, expect, vi, beforeEach } from "vitest";
import i18n from "@/i18n";
import { MermaidBlock } from "./MermaidBlock";

const renderCalls: string[] = [];

vi.mock("mermaid", () => ({
  default: {
    initialize: vi.fn(),
    render: vi.fn((id: string) => {
      renderCalls.push(id);
      return Promise.resolve({ svg: "<svg></svg>" });
    }),
  },
}));

// mermaid mặc định trong mock trả OK ngay — test riêng dưới override tạm bằng
// mockRejectedValueOnce/mockResolvedValueOnce cho từng lần gọi cụ thể.
import mermaid from "mermaid";

const renderBlock = (code: string) =>
  render(
    <I18nextProvider i18n={i18n}>
      <MermaidBlock code={code} />
    </I18nextProvider>,
  );

describe("MermaidBlock", () => {
  beforeEach(() => {
    renderCalls.length = 0;
    vi.mocked(mermaid.render).mockClear();
  });

  it("dùng ID khác nhau cho mỗi lần render (không tái sử dụng 1 id cố định qua các lần code thay đổi)", async () => {
    const { rerender } = renderBlock("graph TD\n  A");
    await waitFor(() => expect(renderCalls.length).toBeGreaterThan(0));

    rerender(
      <I18nextProvider i18n={i18n}>
        <MermaidBlock code={"graph TD\n  A --> B"} />
      </I18nextProvider>,
    );
    await waitFor(() => expect(renderCalls.length).toBeGreaterThanOrEqual(2));

    rerender(
      <I18nextProvider i18n={i18n}>
        <MermaidBlock code={"graph TD\n  A --> B --> C"} />
      </I18nextProvider>,
    );
    await waitFor(() => expect(renderCalls.length).toBeGreaterThanOrEqual(3));

    const uniqueIds = new Set(renderCalls);
    expect(uniqueIds.size).toBe(renderCalls.length);
  });

  // Bug thật (report + tái hiện bằng mermaid thật trong jsdom): mermaid tự đo
  // text bằng cách gắn 1 <svg> tạm vào <body> rồi gọi getBBox() — timing nội
  // bộ này thi thoảng ném "getBBox is not a function" dù cú pháp sơ đồ hoàn
  // toàn đúng, và render lại NGAY SAU (không đổi gì) lại thành công. Component
  // phải tự thử lại, không được rơi thẳng về "hiện mã nguồn" ngay lần lỗi đầu.
  it("lỗi getBBox thoáng qua (transient) thì tự thử lại và vẫn hiện được sơ đồ", async () => {
    vi.mocked(mermaid.render).mockRejectedValueOnce(
      new Error("text2.getBBox is not a function"),
    );

    renderBlock("graph TD\n  A --> B");

    // mockRejectedValueOnce THAY HẲN implementation cho lần gọi đó (không
    // chạy qua thân hàm mock gốc), nên đếm bằng renderCalls (chỉ tăng khi
    // implementation gốc chạy) không phản ánh đúng — dùng thẳng
    // vi.fn().mock.calls, luôn đếm MỌI lần gọi bất kể override hay không.
    await waitFor(() =>
      expect(vi.mocked(mermaid.render).mock.calls.length).toBeGreaterThanOrEqual(2),
    );

    // Không rơi về view "hiện mã nguồn" (banner lỗi) — phải render được sơ đồ.
    expect(
      screen.queryByText(/Hiển thị mã nguồn|Displaying source code/i),
    ).not.toBeInTheDocument();
  });

  // Lỗi KHÔNG thuộc nhóm transient (cú pháp thật sai) — không nên tốn 3 lần
  // gọi vô ích, báo lỗi ngay từ lần đầu.
  it("lỗi cú pháp thật (không phải getBBox) thì báo lỗi ngay, không thử lại", async () => {
    vi.mocked(mermaid.render).mockRejectedValueOnce(
      new Error("Parse error on line 1: unexpected token"),
    );

    renderBlock("graph TD\n  A -->");

    await waitFor(() =>
      expect(
        screen.getByText(/Hiển thị mã nguồn|Displaying source code/i),
      ).toBeInTheDocument(),
    );

    expect(vi.mocked(mermaid.render).mock.calls.length).toBe(1);
  });
});
