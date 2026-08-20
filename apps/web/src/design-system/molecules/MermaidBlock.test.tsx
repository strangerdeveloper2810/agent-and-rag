import { render, waitFor } from "@testing-library/react";
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

const renderBlock = (code: string) =>
  render(
    <I18nextProvider i18n={i18n}>
      <MermaidBlock code={code} />
    </I18nextProvider>,
  );

describe("MermaidBlock", () => {
  beforeEach(() => {
    renderCalls.length = 0;
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
});
